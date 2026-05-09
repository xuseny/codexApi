package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type windsurfModelInfo struct {
	Name       string
	EnumValue  int
	ModelUID   string
	Provider   string
	Deprecated bool
}

type windsurfRawMessage struct {
	Role    string
	Content string
}

var windsurfRawModels = buildWindsurfModelLookup()

func (s *OpenAIGatewayService) ForwardWindsurfChatCompletions(
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
	if account.Type != AccountTypeOAuth || !account.GetCredentialBool("windsurf_builtin") {
		return s.ForwardOpenAICompatibleChatCompletions(ctx, c, account, body, promptCacheKey, defaultMappedModel)
	}

	startTime := time.Now()
	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return nil, fmt.Errorf("parse chat completions request: %w", err)
	}
	originalModel := chatReq.Model
	billingModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	modelInfo, upstreamModel, err := resolveWindsurfModel(billingModel, originalModel)
	if err != nil {
		return nil, err
	}
	messages := buildWindsurfRawMessages(chatReq)
	if len(messages) == 0 {
		return nil, errors.New("windsurf request requires at least one message")
	}

	apiKey, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get windsurf api key: %w", err)
	}
	if strings.TrimSpace(modelInfo.ModelUID) != "" {
		return s.forwardWindsurfCascadeChatCompletions(ctx, c, account, body, chatReq, messages, modelInfo, originalModel, billingModel, upstreamModel, apiKey, startTime)
	}
	sessionID := ""
	if trimmed := strings.TrimSpace(promptCacheKey); trimmed != "" {
		sessionID = generateSessionUUID(trimmed)
	}
	protoBody := windsurfBuildRawGetChatMessageRequest(apiKey, messages, modelInfo.EnumValue, modelInfo.Name, sessionID)
	req, entry, err := s.buildWindsurfRawRequest(ctx, account, protoBody)
	if err != nil {
		return nil, err
	}
	setOpsUpstreamRequestBody(c, body)
	if c != nil {
		c.Set("openai_passthrough", true)
	}

	logger.L().Debug("windsurf chat_completions: builtin language server request",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Int("ls_port", entry.port),
		zap.Bool("stream", chatReq.Stream),
	)

	upstreamStart := time.Now()
	resp, err := newWindsurfGRPCClient().Do(req)
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
			writeChatCompletionsError(c, http.StatusBadGateway, "upstream_error", "Windsurf language server request failed")
		}
		return nil, fmt.Errorf("windsurf language server request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		msg := sanitizeUpstreamErrorMessage(strings.TrimSpace(string(respBody)))
		if msg == "" {
			msg = resp.Status
		}
		if shouldFailoverOpenAIPassthroughResponse(resp.StatusCode) {
			return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: respBody}
		}
		if c != nil {
			writeChatCompletionsError(c, resp.StatusCode, "upstream_error", msg)
		}
		return nil, fmt.Errorf("windsurf language server returned %d: %s", resp.StatusCode, msg)
	}

	var usage OpenAIUsage
	var firstTokenMs *int
	if chatReq.Stream {
		usage, firstTokenMs, err = s.writeWindsurfStreamingChatResponse(ctx, c, resp.Body, originalModel, upstreamModel, startTime)
	} else {
		usage, err = s.writeWindsurfBufferedChatResponse(c, resp.Body, originalModel, upstreamModel)
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
		ReasoningEffort: extractCCReasoningEffortFromBody(body),
		Stream:          chatReq.Stream,
		OpenAIWSMode:    false,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

func (s *OpenAIGatewayService) buildWindsurfRawRequest(ctx context.Context, account *Account, protoBody []byte) (*http.Request, *windsurfLSEntry, error) {
	return buildWindsurfGRPCRequest(ctx, account, windsurfRawGetChatMessagePath, protoBody)
}

func buildWindsurfGRPCRequest(ctx context.Context, account *Account, path string, protoBody []byte) (*http.Request, *windsurfLSEntry, error) {
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	entry, err := ensureWindsurfLanguageServer(ctx, proxyURL)
	if err != nil {
		return nil, nil, err
	}
	target := fmt.Sprintf("http://127.0.0.1:%d%s", entry.port, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(windsurfWrapGRPCFrame(protoBody)))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("content-type", "application/grpc")
	req.Header.Set("te", "trailers")
	req.Header.Set("user-agent", "grpc-node/1.108.2")
	req.Header.Set("x-codeium-csrf-token", entry.csrfToken)
	return req, entry, nil
}

func buildWindsurfRawMessages(req apicompat.ChatCompletionsRequest) []windsurfRawMessage {
	messages := make([]windsurfRawMessage, 0, len(req.Messages)+1)
	if strings.TrimSpace(req.Instructions) != "" {
		messages = append(messages, windsurfRawMessage{Role: "system", Content: strings.TrimSpace(req.Instructions)})
	}
	for _, msg := range req.Messages {
		content := strings.TrimSpace(kiroChatContentText(msg.Content))
		if len(msg.ToolCalls) > 0 {
			parts := make([]string, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				parts = append(parts, fmt.Sprintf("[called tool %s with %s]", tc.Function.Name, tc.Function.Arguments))
			}
			if content != "" {
				content += "\n"
			}
			content += strings.Join(parts, "\n")
		}
		if content == "" {
			continue
		}
		role := msg.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, windsurfRawMessage{Role: role, Content: content})
	}
	return messages
}

func resolveWindsurfModel(candidates ...string) (windsurfModelInfo, string, error) {
	for _, candidate := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		if alias := windsurfModelAliases[normalized]; alias != "" {
			normalized = alias
		}
		if info, ok := windsurfRawModels[normalized]; ok {
			return info, normalized, nil
		}
	}
	return windsurfModelInfo{}, "", fmt.Errorf("windsurf builtin does not support model %q; choose a model from the Windsurf model list or configure model_mapping", firstNonEmpty(candidates...))
}

func (s *OpenAIGatewayService) writeWindsurfStreamingChatResponse(ctx context.Context, c *gin.Context, reader io.Reader, originalModel, upstreamModel string, start time.Time) (OpenAIUsage, *int, error) {
	if c == nil {
		return OpenAIUsage{}, nil, errors.New("gin context is nil")
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	id := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()
	var contentBuilder strings.Builder
	var buffer []byte
	firstChunk := true
	var firstTokenMs *int
	tmp := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(tmp)
		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			var frames [][]byte
			frames, buffer = windsurfDrainGRPCFrames(buffer)
			for _, frame := range frames {
				text, _, isErr, err := windsurfParseRawResponse(frame)
				if err != nil {
					return OpenAIUsage{}, firstTokenMs, err
				}
				if isErr {
					return OpenAIUsage{}, firstTokenMs, errors.New("windsurf upstream returned error frame")
				}
				if text == "" {
					continue
				}
				if firstTokenMs == nil {
					v := int(time.Since(start).Milliseconds())
					firstTokenMs = &v
				}
				contentBuilder.WriteString(text)
				chunk := map[string]any{
					"id":      id,
					"object":  "chat.completion.chunk",
					"created": created,
					"model":   originalModel,
					"choices": []map[string]any{{
						"index":         0,
						"delta":         windsurfOpenAIContentDelta(text, firstChunk),
						"finish_reason": nil,
					}},
				}
				writeWindsurfSSE(c, chunk)
				firstChunk = false
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			select {
			case <-ctx.Done():
				return OpenAIUsage{}, firstTokenMs, ctx.Err()
			default:
				return OpenAIUsage{}, firstTokenMs, readErr
			}
		}
	}
	finalChunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   originalModel,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	writeWindsurfSSE(c, finalChunk)
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()
	return OpenAIUsage{OutputTokens: estimateKiroOutputTokens(contentBuilder.String(), upstreamModel)}, firstTokenMs, nil
}

func (s *OpenAIGatewayService) writeWindsurfBufferedChatResponse(c *gin.Context, reader io.Reader, originalModel, upstreamModel string) (OpenAIUsage, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return OpenAIUsage{}, err
	}
	frames, err := windsurfReadGRPCFrames(raw)
	if err != nil {
		return OpenAIUsage{}, err
	}
	var content strings.Builder
	for _, frame := range frames {
		text, _, isErr, err := windsurfParseRawResponse(frame)
		if err != nil {
			return OpenAIUsage{}, err
		}
		if isErr {
			return OpenAIUsage{}, errors.New("windsurf upstream returned error frame")
		}
		content.WriteString(text)
	}
	text := content.String()
	usage := map[string]any{
		"prompt_tokens":     0,
		"completion_tokens": estimateKiroOutputTokens(text, upstreamModel),
		"total_tokens":      estimateKiroOutputTokens(text, upstreamModel),
	}
	if c != nil {
		c.JSON(http.StatusOK, map[string]any{
			"id":      "chatcmpl-" + uuid.NewString(),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   originalModel,
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": "stop",
			}},
			"usage": usage,
		})
	}
	return OpenAIUsage{OutputTokens: estimateKiroOutputTokens(text, upstreamModel)}, nil
}

func windsurfOpenAIContentDelta(content string, first bool) map[string]any {
	delta := map[string]any{"content": content}
	if first {
		delta["role"] = "assistant"
	}
	return delta
}

func writeWindsurfSSE(c *gin.Context, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = c.Writer.Write([]byte("data: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()
}

func windsurfDrainGRPCFrames(buffer []byte) ([][]byte, []byte) {
	var frames [][]byte
	for len(buffer) >= 5 {
		size := int(binary.BigEndian.Uint32(buffer[1:5]))
		if len(buffer) < 5+size {
			break
		}
		frames = append(frames, buffer[5:5+size])
		buffer = buffer[5+size:]
	}
	return frames, buffer
}
