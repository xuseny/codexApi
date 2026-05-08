package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var kiroModelDigitDashRe = regexp.MustCompile(`(\d)-(\d)`)

func (s *OpenAIGatewayService) ForwardKiroChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	promptCacheKey string,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	if account == nil {
		return nil, errors.New("nil account")
	}
	if account.Type != AccountTypeOAuth {
		return s.ForwardOpenAICompatibleChatCompletions(ctx, c, account, body, promptCacheKey, defaultMappedModel)
	}
	startTime := time.Now()

	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return nil, fmt.Errorf("parse chat completions request: %w", err)
	}
	originalModel := chatReq.Model
	reqStream := chatReq.Stream
	reasoningEffort := extractCCReasoningEffortFromBody(body)
	billingModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	upstreamModel := normalizeKiroModelForUpstream(billingModel)
	if upstreamModel == "" {
		upstreamModel = normalizeKiroModelForUpstream(originalModel)
	}

	kiroBody, err := buildKiroChatPayload(chatReq, upstreamModel, account)
	if err != nil {
		return nil, err
	}
	kiroBodyBytes, err := json.Marshal(kiroBody)
	if err != nil {
		return nil, fmt.Errorf("marshal Kiro request: %w", err)
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	upstreamReq, err := s.buildKiroUpstreamRequest(ctx, account, kiroBodyBytes, token)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(promptCacheKey); trimmed != "" {
		upstreamReq.Header.Set("session_id", generateSessionUUID(trimmed))
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	setOpsUpstreamRequestBody(c, kiroBodyBytes)
	if c != nil {
		c.Set("openai_passthrough", true)
	}

	logger.L().Debug("kiro chat_completions: direct OAuth request",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", reqStream),
	)

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Passthrough:        true,
			Kind:               "request_error",
			Message:            safeErr,
		})
		if c != nil {
			writeChatCompletionsError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		}
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		if shouldFailoverOpenAIPassthroughResponse(resp.StatusCode) {
			return nil, s.handleFailoverErrorResponsePassthrough(ctx, resp, c, account, body)
		}
		return nil, s.handleErrorResponsePassthrough(ctx, resp, c, account, body)
	}

	var usage OpenAIUsage
	var firstTokenMs *int
	if reqStream {
		usage, firstTokenMs, err = s.writeKiroStreamingChatResponse(ctx, c, resp.Body, originalModel, upstreamModel, startTime)
	} else {
		usage, err = s.writeKiroBufferedChatResponse(c, resp.Body, originalModel, upstreamModel)
	}
	if err != nil {
		return nil, err
	}

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ServiceTier:     extractOpenAIServiceTierFromBody(body),
		ReasoningEffort: reasoningEffort,
		Stream:          reqStream,
		OpenAIWSMode:    false,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

func (s *OpenAIGatewayService) buildKiroUpstreamRequest(ctx context.Context, account *Account, body []byte, token string) (*http.Request, error) {
	region := account.GetCredential("api_region")
	if region == "" {
		region = account.GetCredential("region")
	}
	targetURL := strings.TrimRight(KiroAPIBaseURL(region), "/") + "/generateAssistantResponse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	ua := fmt.Sprintf("aws-sdk-rust/1.0.0 ua/2.1 os/other lang/rust api/codewhispererstreaming#1.28.3 m/E app/AmazonQ-For-CLI md/appVersion-1.28.3-%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	req.Header.Set("content-type", "application/x-amz-json-1.0")
	req.Header.Set("accept", "application/json")
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("x-amz-target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", ua)
	req.Header.Set("user-agent", ua)
	return req, nil
}

func buildKiroChatPayload(req apicompat.ChatCompletionsRequest, modelID string, account *Account) (map[string]any, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("messages is required")
	}

	systemPrompt := strings.TrimSpace(req.Instructions)
	nonSystem := make([]apicompat.ChatMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if text := strings.TrimSpace(kiroChatContentText(msg.Content)); text != "" {
				if systemPrompt != "" {
					systemPrompt += "\n"
				}
				systemPrompt += text
			}
			continue
		}
		nonSystem = append(nonSystem, msg)
	}
	if len(nonSystem) == 0 {
		return nil, errors.New("Kiro request requires at least one non-system message")
	}

	current := nonSystem[len(nonSystem)-1]
	historyMessages := nonSystem[:len(nonSystem)-1]
	history := make([]map[string]any, 0, len(historyMessages))
	for _, msg := range historyMessages {
		content := kiroMessageContentForHistory(msg)
		if content == "" {
			continue
		}
		switch msg.Role {
		case "assistant":
			history = append(history, map[string]any{
				"assistantResponseMessage": map[string]any{"content": content},
			})
		case "user", "tool", "function":
			history = append(history, map[string]any{
				"userInputMessage": map[string]any{
					"content": content,
					"modelId": modelID,
					"origin":  "AI_EDITOR",
				},
			})
		}
	}

	currentContent := kiroMessageContentForHistory(current)
	if current.Role == "assistant" {
		if currentContent != "" {
			history = append(history, map[string]any{
				"assistantResponseMessage": map[string]any{"content": currentContent},
			})
		}
		currentContent = "Continue"
	}
	if systemPrompt != "" {
		currentContent = systemPrompt + "\n\n" + currentContent
	}
	if strings.TrimSpace(currentContent) == "" {
		currentContent = "Continue"
	}

	payload := map[string]any{
		"conversationState": map[string]any{
			"chatTriggerType": "MANUAL",
			"conversationId":  uuid.NewString(),
			"currentMessage": map[string]any{
				"userInputMessage": map[string]any{
					"content": currentContent,
					"modelId": modelID,
					"origin":  "AI_EDITOR",
				},
			},
		},
		"agentMode": "vibe",
	}
	if len(history) > 0 {
		payload["conversationState"].(map[string]any)["history"] = history
	}
	if profileARN := strings.TrimSpace(account.GetCredential("profile_arn")); profileARN != "" && account.GetCredential("auth_method") == "" {
		payload["profileArn"] = profileARN
	}
	return payload, nil
}

func kiroMessageContentForHistory(msg apicompat.ChatMessage) string {
	text := kiroChatContentText(msg.Content)
	if msg.Role == "tool" || msg.Role == "function" {
		name := strings.TrimSpace(msg.Name)
		if name == "" {
			name = strings.TrimSpace(msg.ToolCallID)
		}
		if name != "" {
			return fmt.Sprintf("Tool result (%s): %s", name, text)
		}
		return "Tool result: " + text
	}
	return text
}

func kiroChatContentText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			if part.Type == "text" && part.Text != "" {
				b.WriteString(part.Text)
			}
		}
		return b.String()
	}
	var value any
	if err := json.Unmarshal(raw, &value); err == nil {
		return extractTextFromContent(value)
	}
	return ""
}

func normalizeKiroModelForUpstream(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	return kiroModelDigitDashRe.ReplaceAllString(model, "$1.$2")
}

func (s *OpenAIGatewayService) writeKiroStreamingChatResponse(ctx context.Context, c *gin.Context, reader io.Reader, originalModel, upstreamModel string, start time.Time) (OpenAIUsage, *int, error) {
	if c == nil {
		return OpenAIUsage{}, nil, errors.New("gin context is nil")
	}
	c.Header("content-type", "text/event-stream; charset=utf-8")
	c.Header("cache-control", "no-cache")
	c.Header("connection", "keep-alive")
	c.Status(http.StatusOK)

	parser := newKiroEventParser()
	completionID := "chatcmpl-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	created := time.Now().Unix()
	firstChunk := true
	var firstTokenMs *int
	var contentBuilder strings.Builder
	scanner := bufio.NewReader(reader)
	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return OpenAIUsage{}, firstTokenMs, ctx.Err()
		default:
		}
		n, readErr := scanner.Read(buf)
		if n > 0 {
			events := parser.Feed(buf[:n])
			for _, event := range events {
				if event.Content == "" {
					continue
				}
				if firstTokenMs == nil {
					ms := int(time.Since(start).Milliseconds())
					firstTokenMs = &ms
				}
				contentBuilder.WriteString(event.Content)
				chunk := map[string]any{
					"id":      completionID,
					"object":  "chat.completion.chunk",
					"created": created,
					"model":   originalModel,
					"choices": []map[string]any{{
						"index":         0,
						"delta":         kiroOpenAIContentDelta(event.Content, firstChunk),
						"finish_reason": nil,
					}},
				}
				firstChunk = false
				writeKiroSSE(c, chunk)
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return OpenAIUsage{}, firstTokenMs, readErr
			}
			break
		}
	}

	finalChunk := map[string]any{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   originalModel,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	writeKiroSSE(c, finalChunk)
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()

	return OpenAIUsage{OutputTokens: estimateKiroOutputTokens(contentBuilder.String(), upstreamModel)}, firstTokenMs, nil
}

func (s *OpenAIGatewayService) writeKiroBufferedChatResponse(c *gin.Context, reader io.Reader, originalModel, upstreamModel string) (OpenAIUsage, error) {
	parser := newKiroEventParser()
	buf := make([]byte, 8192)
	var contentBuilder strings.Builder
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for _, event := range parser.Feed(buf[:n]) {
				contentBuilder.WriteString(event.Content)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return OpenAIUsage{}, err
		}
	}
	content := contentBuilder.String()
	resp := map[string]any{
		"id":      "chatcmpl-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   originalModel,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     0,
			"completion_tokens": estimateKiroOutputTokens(content, upstreamModel),
			"total_tokens":      estimateKiroOutputTokens(content, upstreamModel),
		},
	}
	c.JSON(http.StatusOK, resp)
	return OpenAIUsage{OutputTokens: estimateKiroOutputTokens(content, upstreamModel)}, nil
}

func kiroOpenAIContentDelta(content string, first bool) map[string]any {
	delta := map[string]any{"content": content}
	if first {
		delta["role"] = "assistant"
	}
	return delta
}

func writeKiroSSE(c *gin.Context, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = c.Writer.Write([]byte("data: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()
}

func estimateKiroOutputTokens(content, _ string) int {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	return len([]rune(content))/4 + 1
}

type kiroParsedEvent struct {
	Content                string
	ContextUsagePercentage float64
	Usage                  any
}

type kiroEventParser struct {
	buffer      string
	lastContent string
}

func newKiroEventParser() *kiroEventParser {
	return &kiroEventParser{}
}

func (p *kiroEventParser) Feed(chunk []byte) []kiroParsedEvent {
	p.buffer += string(chunk)
	var events []kiroParsedEvent
	for {
		pos, typ := p.findNextEvent()
		if pos < 0 {
			if len(p.buffer) > 1<<20 {
				p.buffer = p.buffer[len(p.buffer)-(1<<20):]
			}
			return events
		}
		end := findKiroJSONEnd(p.buffer, pos)
		if end < 0 {
			return events
		}
		raw := p.buffer[pos : end+1]
		p.buffer = p.buffer[end+1:]
		switch typ {
		case "content":
			content := gjson.Get(raw, "content").String()
			if content != "" && content != p.lastContent && !gjson.Get(raw, "followupPrompt").Bool() {
				p.lastContent = content
				events = append(events, kiroParsedEvent{Content: content})
			}
		case "context":
			events = append(events, kiroParsedEvent{ContextUsagePercentage: gjson.Get(raw, "contextUsagePercentage").Float()})
		case "usage":
			events = append(events, kiroParsedEvent{Usage: gjson.Get(raw, "usage").Value()})
		}
	}
}

func (p *kiroEventParser) findNextEvent() (int, string) {
	patterns := []struct {
		pattern string
		typ     string
	}{
		{`{"content":`, "content"},
		{`{"contextUsagePercentage":`, "context"},
		{`{"usage":`, "usage"},
	}
	best := -1
	bestType := ""
	for _, pattern := range patterns {
		pos := strings.Index(p.buffer, pattern.pattern)
		if pos >= 0 && (best < 0 || pos < best) {
			best = pos
			bestType = pattern.typ
		}
	}
	return best, bestType
}

func findKiroJSONEnd(s string, start int) int {
	if start < 0 || start >= len(s) || s[start] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString && ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
