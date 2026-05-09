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

func (s *OpenAIGatewayService) ForwardWindsurfResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*OpenAIForwardResult, error) {
	if account == nil {
		return nil, errors.New("nil account")
	}
	startTime := time.Now()

	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	originalModel := responsesReq.Model
	chatReq, err := windsurfResponsesToChatCompletions(&responsesReq)
	if err != nil {
		return nil, err
	}
	toolInstruction := windsurfBuildToolInstruction(chatReq.Tools, chatReq.ToolChoice, originalModel)
	toolsEnabled := toolInstruction != ""
	if toolsEnabled {
		chatReq.Instructions = windsurfJoinSections(chatReq.Instructions, toolInstruction)
		chatReq.Messages = windsurfInjectToolUserHint(chatReq.Messages, windsurfBuildToolUserHint(chatReq.Tools, chatReq.ToolChoice, originalModel))
	}
	messages := buildWindsurfRawMessages(chatReq)
	if len(messages) == 0 {
		return nil, errors.New("windsurf responses request requires at least one message")
	}

	billingModel := resolveOpenAIForwardModel(account, originalModel, "")
	modelInfo, upstreamModel, err := resolveWindsurfModel(billingModel, originalModel)
	if err != nil {
		return nil, err
	}
	apiKey, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get windsurf api key: %w", err)
	}

	responseID := "resp_" + uuid.NewString()
	itemID := "msg_" + uuid.NewString()
	reasoningItemID := "rs_" + uuid.NewString()
	createdAt := time.Now().Unix()
	var firstTokenMs *int
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	markFirstToken := func() {
		if firstTokenMs == nil {
			v := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &v
		}
	}

	streaming := responsesReq.Stream && c != nil
	sequence := 0
	nextOutputIndex := 0
	messageIndex := -1
	reasoningIndex := -1
	if streaming {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.created",
			SequenceNumber: sequence,
			Response: &apicompat.ResponsesResponse{
				ID:        responseID,
				Object:    "response",
				CreatedAt: createdAt,
				Model:     originalModel,
				Status:    "in_progress",
				Output:    []apicompat.ResponsesOutput{},
			},
		}); err != nil {
			return nil, err
		}
		sequence++
	}

	ensureReasoningItem := func() error {
		if !streaming || reasoningIndex >= 0 {
			return nil
		}
		reasoningIndex = nextOutputIndex
		nextOutputIndex++
		err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.output_item.added",
			SequenceNumber: sequence,
			OutputIndex:    reasoningIndex,
			Item: &apicompat.ResponsesOutput{
				Type:    "reasoning",
				ID:      reasoningItemID,
				Status:  "in_progress",
				Summary: []apicompat.ResponsesSummary{},
			},
		})
		sequence++
		return err
	}
	ensureMessageItem := func() error {
		if !streaming || messageIndex >= 0 {
			return nil
		}
		messageIndex = nextOutputIndex
		nextOutputIndex++
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.output_item.added",
			SequenceNumber: sequence,
			OutputIndex:    messageIndex,
			Item: &apicompat.ResponsesOutput{
				Type:    "message",
				ID:      itemID,
				Role:    "assistant",
				Status:  "in_progress",
				Content: []apicompat.ResponsesContentPart{},
			},
		}); err != nil {
			return err
		}
		sequence++
		if err := writeWindsurfResponsesEvent(c, windsurfBuildResponsesContentPartEventAt(
			"response.content_part.added",
			sequence,
			messageIndex,
			itemID,
			"",
		)); err != nil {
			return err
		}
		sequence++
		return nil
	}

	onChunk := func(text string) error {
		if text == "" {
			return nil
		}
		markFirstToken()
		contentBuilder.WriteString(text)
		if !streaming {
			return nil
		}
		if err := ensureMessageItem(); err != nil {
			return err
		}
		err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.output_text.delta",
			SequenceNumber: sequence,
			OutputIndex:    messageIndex,
			ContentIndex:   0,
			Delta:          text,
			ItemID:         itemID,
		})
		sequence++
		return err
	}
	onThinking := func(text string) error {
		if text == "" {
			return nil
		}
		markFirstToken()
		reasoningBuilder.WriteString(text)
		if !streaming {
			return nil
		}
		if err := ensureReasoningItem(); err != nil {
			return err
		}
		err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.reasoning_summary_text.delta",
			SequenceNumber: sequence,
			OutputIndex:    reasoningIndex,
			SummaryIndex:   0,
			Delta:          text,
			ItemID:         reasoningItemID,
		})
		sequence++
		return err
	}

	callbacks := windsurfResponsesCallbacks{
		OnText:     onChunk,
		OnThinking: onThinking,
	}
	if !streaming || toolsEnabled {
		callbacks.OnText = nil
	}
	usage, text, err := s.runWindsurfResponsesRequestWithCallbacks(ctx, account, apiKey, messages, modelInfo, callbacks, windsurfResponsesRunOptions{
		CascadeToolInstruction: toolInstruction,
	})
	if err != nil {
		if c != nil && !responsesReq.Stream {
			writeResponsesError(c, http.StatusBadGateway, "upstream_error", err.Error())
		}
		return nil, err
	}

	var toolCalls []windsurfParsedToolCall
	cleanedToolText := ""
	if toolsEnabled {
		var parseStatus string
		toolCalls, cleanedToolText, parseStatus = windsurfParseToolCallsDetailedFromText(text, chatReq.Tools)
		logger.L().Debug("windsurf.tool_bridge_parse",
			zap.Int64("account_id", account.ID),
			zap.String("model", originalModel),
			zap.String("status", parseStatus),
			zap.Int("tool_call_count", len(toolCalls)),
			zap.Int("raw_text_len", len(text)),
			zap.Int("cleaned_text_len", len(cleanedToolText)),
		)
	}
	if len(toolCalls) > 0 && strings.TrimSpace(cleanedToolText) != "" {
		contentBuilder.WriteString(cleanedToolText)
	}
	if callbacks.OnText == nil && len(toolCalls) == 0 {
		contentBuilder.WriteString(text)
	}
	fullText := contentBuilder.String()
	if strings.TrimSpace(fullText) == "" && len(toolCalls) == 0 && toolsEnabled {
		fullText = windsurfToolBridgeEmptyFallbackText(chatReq.ToolChoice)
	}
	reasoningText := reasoningBuilder.String()

	if !responsesReq.Stream {
		if c != nil {
			output := windsurfBuildResponsesOutputs(reasoningItemID, reasoningText, itemID, fullText, toolCalls)
			c.JSON(http.StatusOK, windsurfBuildResponsesResponseWithOutputs(responseID, originalModel, output, usage, createdAt))
		}
	} else if c != nil {
		if toolsEnabled && len(toolCalls) > 0 {
			markFirstToken()
			if strings.TrimSpace(fullText) != "" {
				if err := ensureMessageItem(); err != nil {
					return nil, err
				}
				if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
					Type:           "response.output_text.delta",
					SequenceNumber: sequence,
					OutputIndex:    messageIndex,
					ContentIndex:   0,
					Delta:          fullText,
					ItemID:         itemID,
				}); err != nil {
					return nil, err
				}
				sequence++
			}
			if err := windsurfWriteResponsesToolCallEvents(c, toolCalls, &sequence, &nextOutputIndex); err != nil {
				return nil, err
			}
		} else if fullText != "" {
			markFirstToken()
			if err := ensureMessageItem(); err != nil {
				return nil, err
			}
			if toolsEnabled {
				if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
					Type:           "response.output_text.delta",
					SequenceNumber: sequence,
					OutputIndex:    messageIndex,
					ContentIndex:   0,
					Delta:          fullText,
					ItemID:         itemID,
				}); err != nil {
					return nil, err
				}
				sequence++
			}
		}
		if reasoningIndex >= 0 {
			if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
				Type:           "response.reasoning_summary_text.done",
				SequenceNumber: sequence,
				OutputIndex:    reasoningIndex,
				SummaryIndex:   0,
				Text:           reasoningText,
				ItemID:         reasoningItemID,
			}); err != nil {
				return nil, err
			}
			sequence++
			if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.done",
				SequenceNumber: sequence,
				OutputIndex:    reasoningIndex,
				Item: &apicompat.ResponsesOutput{
					Type:   "reasoning",
					ID:     reasoningItemID,
					Status: "completed",
					Summary: []apicompat.ResponsesSummary{{
						Type: "summary_text",
						Text: reasoningText,
					}},
				},
			}); err != nil {
				return nil, err
			}
			sequence++
		}
		if messageIndex >= 0 {
			if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
				Type:           "response.output_text.done",
				SequenceNumber: sequence,
				OutputIndex:    messageIndex,
				ContentIndex:   0,
				Text:           fullText,
				ItemID:         itemID,
			}); err != nil {
				return nil, err
			}
			sequence++
			if err := writeWindsurfResponsesEvent(c, windsurfBuildResponsesContentPartEventAt(
				"response.content_part.done",
				sequence,
				messageIndex,
				itemID,
				fullText,
			)); err != nil {
				return nil, err
			}
			sequence++
			if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.done",
				SequenceNumber: sequence,
				OutputIndex:    messageIndex,
				Item: &apicompat.ResponsesOutput{
					Type:   "message",
					ID:     itemID,
					Role:   "assistant",
					Status: "completed",
					Content: []apicompat.ResponsesContentPart{{
						Type: "output_text",
						Text: fullText,
					}},
				},
			}); err != nil {
				return nil, err
			}
			sequence++
		}
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.completed",
			SequenceNumber: sequence,
			Response:       windsurfBuildResponsesResponseWithOutputs(responseID, originalModel, windsurfBuildResponsesOutputs(reasoningItemID, reasoningText, itemID, fullText, toolCalls), usage, createdAt),
		}); err != nil {
			return nil, err
		}
		_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	}

	return &OpenAIForwardResult{
		RequestID:       "",
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ServiceTier:     extractOpenAIServiceTierFromBody(body),
		ReasoningEffort: ExtractResponsesReasoningEffortFromBody(body),
		Stream:          responsesReq.Stream,
		OpenAIWSMode:    false,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

func (s *OpenAIGatewayService) ForwardWindsurfAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*OpenAIForwardResult, error) {
	if account == nil {
		return nil, errors.New("nil account")
	}
	startTime := time.Now()

	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := strings.TrimSpace(anthropicReq.Model)
	if originalModel == "" {
		return nil, errors.New("windsurf anthropic request requires model")
	}

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic request: %w", err)
	}
	responsesReq.Model = originalModel
	responsesReq.Stream = anthropicReq.Stream

	chatReq, err := windsurfResponsesToChatCompletions(responsesReq)
	if err != nil {
		return nil, err
	}
	toolInstruction := windsurfBuildToolInstruction(chatReq.Tools, chatReq.ToolChoice, originalModel)
	toolsEnabled := toolInstruction != ""
	if toolsEnabled {
		chatReq.Instructions = windsurfJoinSections(chatReq.Instructions, toolInstruction)
		chatReq.Messages = windsurfInjectToolUserHint(chatReq.Messages, windsurfBuildToolUserHint(chatReq.Tools, chatReq.ToolChoice, originalModel))
	}
	messages := buildWindsurfRawMessages(chatReq)
	if len(messages) == 0 {
		return nil, errors.New("windsurf anthropic request requires at least one message")
	}

	billingModel := resolveOpenAIForwardModel(account, originalModel, "")
	modelInfo, upstreamModel, err := resolveWindsurfModel(billingModel, originalModel)
	if err != nil {
		return nil, err
	}
	apiKey, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get windsurf api key: %w", err)
	}

	messageID := "msg_" + uuid.NewString()
	blockIndex := 0
	blockOpen := false
	blockType := ""
	var firstTokenMs *int
	var textBuilder strings.Builder
	var thinkingBuilder strings.Builder
	markFirstToken := func() {
		if firstTokenMs == nil {
			v := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &v
		}
	}

	streaming := anthropicReq.Stream && c != nil
	if streaming {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type: "message_start",
			Message: &apicompat.AnthropicResponse{
				ID:      messageID,
				Type:    "message",
				Role:    "assistant",
				Content: []apicompat.AnthropicContentBlock{},
				Model:   originalModel,
				Usage:   apicompat.AnthropicUsage{},
			},
		}); err != nil {
			return nil, err
		}
	}
	closeBlock := func() error {
		if !streaming || !blockOpen {
			return nil
		}
		idx := blockIndex
		if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type:  "content_block_stop",
			Index: &idx,
		}); err != nil {
			return err
		}
		blockOpen = false
		blockType = ""
		blockIndex++
		return nil
	}
	ensureBlock := func(nextType string) error {
		if !streaming {
			return nil
		}
		if blockOpen && blockType == nextType {
			return nil
		}
		if err := closeBlock(); err != nil {
			return err
		}
		idx := blockIndex
		blockOpen = true
		blockType = nextType
		block := &apicompat.AnthropicContentBlock{Type: nextType}
		switch nextType {
		case "thinking":
			block.Thinking = ""
		default:
			block.Text = ""
		}
		return writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type:         "content_block_start",
			Index:        &idx,
			ContentBlock: block,
		})
	}

	onChunk := func(text string) error {
		if text == "" {
			return nil
		}
		markFirstToken()
		textBuilder.WriteString(text)
		if !streaming {
			return nil
		}
		if err := ensureBlock("text"); err != nil {
			return err
		}
		idx := blockIndex
		return writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &apicompat.AnthropicDelta{
				Type: "text_delta",
				Text: text,
			},
		})
	}
	onThinking := func(text string) error {
		if text == "" {
			return nil
		}
		markFirstToken()
		thinkingBuilder.WriteString(text)
		if !streaming {
			return nil
		}
		if err := ensureBlock("thinking"); err != nil {
			return err
		}
		idx := blockIndex
		return writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &apicompat.AnthropicDelta{
				Type:     "thinking_delta",
				Thinking: text,
			},
		})
	}

	callbacks := windsurfResponsesCallbacks{
		OnText:     onChunk,
		OnThinking: onThinking,
	}
	if !streaming || toolsEnabled {
		callbacks.OnText = nil
	}
	usage, text, err := s.runWindsurfResponsesRequestWithCallbacks(ctx, account, apiKey, messages, modelInfo, callbacks, windsurfResponsesRunOptions{
		CascadeToolInstruction: toolInstruction,
	})
	if err != nil {
		if c != nil && !anthropicReq.Stream {
			writeAnthropicError(c, http.StatusBadGateway, "api_error", err.Error())
		}
		return nil, err
	}

	var toolCalls []windsurfParsedToolCall
	cleanedToolText := ""
	if toolsEnabled {
		var parseStatus string
		toolCalls, cleanedToolText, parseStatus = windsurfParseToolCallsDetailedFromText(text, chatReq.Tools)
		logger.L().Debug("windsurf.tool_bridge_parse",
			zap.Int64("account_id", account.ID),
			zap.String("model", originalModel),
			zap.String("status", parseStatus),
			zap.Int("tool_call_count", len(toolCalls)),
			zap.Int("raw_text_len", len(text)),
			zap.Int("cleaned_text_len", len(cleanedToolText)),
		)
	}
	if len(toolCalls) > 0 && strings.TrimSpace(cleanedToolText) != "" {
		textBuilder.WriteString(cleanedToolText)
	}
	if callbacks.OnText == nil && len(toolCalls) == 0 {
		textBuilder.WriteString(text)
	}
	fullText := textBuilder.String()
	if strings.TrimSpace(fullText) == "" && len(toolCalls) == 0 && toolsEnabled {
		fullText = windsurfToolBridgeEmptyFallbackText(chatReq.ToolChoice)
	}
	thinkingText := thinkingBuilder.String()
	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	}

	if !anthropicReq.Stream {
		if c != nil {
			c.JSON(http.StatusOK, buildWindsurfAnthropicResponseWithBlocks(messageID, originalModel, windsurfBuildAnthropicBlocks(thinkingText, fullText, toolCalls), stopReason, usage))
		}
	} else if c != nil {
		if len(toolCalls) > 0 {
			markFirstToken()
			if strings.TrimSpace(fullText) != "" {
				if err := ensureBlock("text"); err != nil {
					return nil, err
				}
				idx := blockIndex
				if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
					Type:  "content_block_delta",
					Index: &idx,
					Delta: &apicompat.AnthropicDelta{
						Type: "text_delta",
						Text: fullText,
					},
				}); err != nil {
					return nil, err
				}
			}
			if err := closeBlock(); err != nil {
				return nil, err
			}
			for _, call := range toolCalls {
				idx := blockIndex
				input := strings.TrimSpace(call.Arguments)
				if input == "" {
					input = "{}"
				}
				if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
					Type:  "content_block_start",
					Index: &idx,
					ContentBlock: &apicompat.AnthropicContentBlock{
						Type:  "tool_use",
						ID:    call.ID,
						Name:  call.Name,
						Input: json.RawMessage("{}"),
					},
				}); err != nil {
					return nil, err
				}
				if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
					Type:  "content_block_delta",
					Index: &idx,
					Delta: &apicompat.AnthropicDelta{
						Type:        "input_json_delta",
						PartialJSON: input,
					},
				}); err != nil {
					return nil, err
				}
				if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
					Type:  "content_block_stop",
					Index: &idx,
				}); err != nil {
					return nil, err
				}
				blockIndex++
			}
		} else if fullText != "" {
			markFirstToken()
			if err := ensureBlock("text"); err != nil {
				return nil, err
			}
			if toolsEnabled {
				idx := blockIndex
				if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
					Type:  "content_block_delta",
					Index: &idx,
					Delta: &apicompat.AnthropicDelta{
						Type: "text_delta",
						Text: fullText,
					},
				}); err != nil {
					return nil, err
				}
			}
		}
		if err := closeBlock(); err != nil {
			return nil, err
		}
		if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{
			Type: "message_delta",
			Delta: &apicompat.AnthropicDelta{
				StopReason: stopReason,
			},
			Usage: openAIUsageToAnthropicUsage(usage),
		}); err != nil {
			return nil, err
		}
		if err := writeWindsurfAnthropicEvent(c, apicompat.AnthropicStreamEvent{Type: "message_stop"}); err != nil {
			return nil, err
		}
	}

	return &OpenAIForwardResult{
		RequestID:       "",
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ServiceTier:     extractOpenAIServiceTierFromBody(body),
		ReasoningEffort: extractCCReasoningEffortFromBody(body),
		Stream:          anthropicReq.Stream,
		OpenAIWSMode:    false,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

type windsurfResponsesCallbacks struct {
	OnText     func(string) error
	OnThinking func(string) error
}

type windsurfResponsesRunOptions struct {
	CascadeToolInstruction string
}

func (s *OpenAIGatewayService) runWindsurfResponsesRequest(
	ctx context.Context,
	account *Account,
	apiKey string,
	messages []windsurfRawMessage,
	modelInfo windsurfModelInfo,
	onChunk func(string) error,
) (OpenAIUsage, string, error) {
	return s.runWindsurfResponsesRequestWithCallbacks(ctx, account, apiKey, messages, modelInfo, windsurfResponsesCallbacks{
		OnText: onChunk,
	}, windsurfResponsesRunOptions{})
}

func (s *OpenAIGatewayService) runWindsurfResponsesRequestWithCallbacks(
	ctx context.Context,
	account *Account,
	apiKey string,
	messages []windsurfRawMessage,
	modelInfo windsurfModelInfo,
	callbacks windsurfResponsesCallbacks,
	options windsurfResponsesRunOptions,
) (OpenAIUsage, string, error) {
	if strings.TrimSpace(modelInfo.ModelUID) != "" {
		return runWindsurfCascadeChatWithCallbacks(ctx, account, apiKey, messages, modelInfo, windsurfCascadeCallbacks{
			OnText:     callbacks.OnText,
			OnThinking: callbacks.OnThinking,
		}, windsurfCascadeOptions{
			ToolInstruction: options.CascadeToolInstruction,
		})
	}
	sessionID := uuid.NewString()
	protoBody := windsurfBuildRawGetChatMessageRequest(apiKey, messages, modelInfo.EnumValue, modelInfo.Name, sessionID)
	req, _, err := s.buildWindsurfRawRequest(ctx, account, protoBody)
	if err != nil {
		return OpenAIUsage{}, "", err
	}
	resp, err := newWindsurfGRPCClient().Do(req)
	if err != nil {
		return OpenAIUsage{}, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return OpenAIUsage{}, "", fmt.Errorf("windsurf language server returned %d: %s", resp.StatusCode, msg)
	}
	return windsurfCollectRawResponse(resp.Body, messages, callbacks.OnText)
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
				args := strings.TrimSpace(tc.Function.Arguments)
				if args == "" {
					args = "{}"
				}
				var parsedArgs any
				if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
					parsedArgs = map[string]any{"input": args}
				}
				payload, _ := json.Marshal(map[string]any{
					"function_call": map[string]any{
						"name":      tc.Function.Name,
						"arguments": parsedArgs,
					},
				})
				parts = append(parts, string(payload))
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
		if role == "tool" {
			id := strings.TrimSpace(msg.ToolCallID)
			if id == "" {
				id = "unknown"
			}
			content = fmt.Sprintf("<tool_result tool_call_id=%q>\n%s\n</tool_result>", id, content)
			role = "user"
		}
		messages = append(messages, windsurfRawMessage{Role: role, Content: content})
	}
	return messages
}

func windsurfResponsesToChatCompletions(req *apicompat.ResponsesRequest) (apicompat.ChatCompletionsRequest, error) {
	if req == nil {
		return apicompat.ChatCompletionsRequest{}, errors.New("nil responses request")
	}
	out := apicompat.ChatCompletionsRequest{
		Model:           req.Model,
		Instructions:    strings.TrimSpace(req.Instructions),
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		Stream:          req.Stream,
		ServiceTier:     req.ServiceTier,
		ReasoningEffort: "",
	}
	out.Tools = windsurfResponsesToolsToChatTools(req.Tools)
	if len(req.ToolChoice) > 0 {
		out.ToolChoice = append(json.RawMessage(nil), req.ToolChoice...)
	}
	if req.MaxOutputTokens != nil {
		v := *req.MaxOutputTokens
		out.MaxCompletionTokens = &v
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = strings.TrimSpace(req.Reasoning.Effort)
	}
	messages, err := windsurfResponsesInputToChatMessages(req.Input)
	if err != nil {
		return apicompat.ChatCompletionsRequest{}, err
	}
	out.Messages = messages
	return out, nil
}

func windsurfResponsesInputToChatMessages(input json.RawMessage) ([]apicompat.ChatMessage, error) {
	if len(input) == 0 || string(input) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(input, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		raw, _ := json.Marshal(text)
		return []apicompat.ChatMessage{{Role: "user", Content: raw}}, nil
	}
	var items []apicompat.ResponsesInputItem
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("parse responses input: %w", err)
	}
	messages := make([]apicompat.ChatMessage, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "function_call":
			args := item.Arguments
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			messages = append(messages, apicompat.ChatMessage{
				Role: "assistant",
				ToolCalls: []apicompat.ChatToolCall{{
					ID:   item.CallID,
					Type: "function",
					Function: apicompat.ChatFunctionCall{
						Name:      item.Name,
						Arguments: args,
					},
				}},
			})
		case "function_call_output":
			raw, _ := json.Marshal(item.Output)
			messages = append(messages, apicompat.ChatMessage{
				Role:       "tool",
				Content:    raw,
				ToolCallID: item.CallID,
			})
		default:
			role := strings.TrimSpace(item.Role)
			if role == "" {
				role = "user"
			}
			content := strings.TrimSpace(windsurfResponsesContentText(item.Content))
			if content == "" {
				continue
			}
			raw, _ := json.Marshal(content)
			messages = append(messages, apicompat.ChatMessage{Role: role, Content: raw})
		}
	}
	return messages, nil
}

func windsurfResponsesContentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []apicompat.ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			switch part.Type {
			case "input_text", "output_text", "text":
				if part.Text != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(part.Text)
				}
			}
		}
		return b.String()
	}
	return ""
}

func windsurfResponsesToolsToChatTools(tools []apicompat.ResponsesTool) []apicompat.ChatTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]apicompat.ChatTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		out = append(out, apicompat.ChatTool{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        strings.TrimSpace(tool.Name),
				Description: strings.TrimSpace(tool.Description),
				Parameters:  append(json.RawMessage(nil), tool.Parameters...),
				Strict:      tool.Strict,
			},
		})
	}
	return out
}

func windsurfJoinSections(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			trimmed = append(trimmed, s)
		}
	}
	return strings.Join(trimmed, "\n\n")
}

func windsurfBuildToolInstruction(tools []apicompat.ChatTool, toolChoice json.RawMessage, model string) string {
	mode, forceName := windsurfResolveToolChoice(toolChoice)
	if mode == "none" {
		return ""
	}
	names := windsurfFunctionToolNames(tools)
	if len(names) == 0 {
		return ""
	}

	dialect := windsurfToolDialect(model)
	var b strings.Builder
	if dialect == "gpt_native" {
		b.WriteString("You have access to the following client-side functions. They are real callable tools; the caller will execute them and return results in the next turn.\n\n")
		b.WriteString("To call a function, output one valid JSON object on a single line, with no markdown and no prose before or after:\n")
		b.WriteString(`{"function_call":{"name":"<function_name>","arguments":{}}}` + "\n\n")
	} else {
		b.WriteString("You have access to the following client-side functions. They are real callable tools; the caller will execute them and return results in the next turn.\n\n")
		b.WriteString("To call a function, output one block on a single line, with no prose before or after:\n")
		b.WriteString(`<tool_call>{"name":"<function_name>","arguments":{}}</tool_call>` + "\n\n")
	}
	b.WriteString("Rules:\n")
	b.WriteString("1. If a function is relevant, call it instead of claiming you cannot access files, commands, tools, or live data.\n")
	b.WriteString("2. Never invent file contents, command outputs, timestamps, or tool results. Tool results come from the caller after your function_call.\n")
	b.WriteString("3. After emitting the function call, stop generating.\n")
	if mode == "required" {
		b.WriteString("4. tool_choice requires at least one function call; do not answer directly.\n")
	}
	if forceName != "" {
		b.WriteString(fmt.Sprintf("5. tool_choice requires the function %q; do not call a different function.\n", forceName))
	}
	b.WriteString("\nAvailable functions:\n")
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil || strings.TrimSpace(tool.Function.Name) == "" {
			continue
		}
		b.WriteString("\n### ")
		b.WriteString(strings.TrimSpace(tool.Function.Name))
		b.WriteByte('\n')
		if desc := strings.TrimSpace(tool.Function.Description); desc != "" {
			b.WriteString(desc)
			b.WriteByte('\n')
		}
		if params := windsurfCompactJSON(tool.Function.Parameters); params != "" {
			b.WriteString("Parameters: ")
			b.WriteString(params)
			b.WriteByte('\n')
			if required := windsurfToolRequiredParameters(tool.Function.Parameters); len(required) > 0 {
				b.WriteString("Required parameters: ")
				b.WriteString(strings.Join(required, ", "))
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func windsurfBuildToolUserHint(tools []apicompat.ChatTool, toolChoice json.RawMessage, model string) string {
	mode, forceName := windsurfResolveToolChoice(toolChoice)
	if mode == "none" {
		return ""
	}
	names := windsurfFunctionToolNames(tools)
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Tools available this turn: ")
	b.WriteString(strings.Join(names, ", "))
	b.WriteString(". ")
	if windsurfToolDialect(model) == "gpt_native" {
		b.WriteString(`If a tool is needed, output only {"function_call":{"name":"TOOL_NAME","arguments":{...}}}. `)
	} else {
		b.WriteString(`If a tool is needed, output only <tool_call>{"name":"TOOL_NAME","arguments":{...}}</tool_call>. `)
	}
	if mode == "required" {
		b.WriteString("At least one tool call is required. ")
	}
	if forceName != "" {
		b.WriteString("Required tool: ")
		b.WriteString(forceName)
		b.WriteString(". ")
	}
	b.WriteString("Do not say you cannot read files or run commands when a listed tool can do it.")
	return b.String()
}

func windsurfInjectToolUserHint(messages []apicompat.ChatMessage, hint string) []apicompat.ChatMessage {
	hint = strings.TrimSpace(hint)
	if hint == "" || len(messages) == 0 {
		return messages
	}
	out := append([]apicompat.ChatMessage(nil), messages...)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role != "user" {
			continue
		}
		text := strings.TrimSpace(kiroChatContentText(out[i].Content))
		if strings.HasPrefix(text, "<tool_result") {
			return out
		}
		raw, _ := json.Marshal(windsurfJoinSections(hint, text))
		out[i].Content = raw
		return out
	}
	return out
}

func windsurfFunctionToolNames(tools []apicompat.ChatTool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil {
			continue
		}
		if name := strings.TrimSpace(tool.Function.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func windsurfResolveToolChoice(raw json.RawMessage) (mode, forceName string) {
	mode = "auto"
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || string(raw) == "null" {
		return mode, ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "none":
			return "none", ""
		case "required", "any":
			return "required", ""
		default:
			return "auto", ""
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return mode, ""
	}
	if name, _ := obj["name"].(string); strings.TrimSpace(name) != "" {
		return "required", strings.TrimSpace(name)
	}
	if fn, _ := obj["function"].(map[string]any); fn != nil {
		if name, _ := fn["name"].(string); strings.TrimSpace(name) != "" {
			return "required", strings.TrimSpace(name)
		}
	}
	if typ, _ := obj["type"].(string); strings.EqualFold(typ, "none") {
		return "none", ""
	}
	return mode, ""
}

func windsurfToolDialect(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(normalized, "gpt-") || strings.HasPrefix(normalized, "o3") || strings.HasPrefix(normalized, "o4") {
		return "gpt_native"
	}
	return "openai_json_xml"
}

func windsurfCompactJSON(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return buf.String()
}

type windsurfParsedToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type windsurfToolCallCandidate struct {
	ID        string
	Name      string
	Arguments any
}

func windsurfParseToolCallsFromText(text string, tools []apicompat.ChatTool) ([]windsurfParsedToolCall, string) {
	available := map[string]string{}
	for _, name := range windsurfFunctionToolNames(tools) {
		available[strings.ToLower(name)] = name
	}
	validate := func(name string) (string, bool) {
		name = strings.TrimSpace(name)
		if name == "" {
			return "", false
		}
		if len(available) == 0 {
			return name, true
		}
		actual, ok := available[strings.ToLower(name)]
		return actual, ok
	}
	addCandidate := func(c windsurfToolCallCandidate, calls *[]windsurfParsedToolCall) bool {
		name, ok := validate(c.Name)
		if !ok {
			return false
		}
		id := strings.TrimSpace(c.ID)
		if id == "" {
			id = "call_" + uuid.NewString()
		}
		*calls = append(*calls, windsurfParsedToolCall{
			ID:        id,
			Name:      name,
			Arguments: windsurfNormalizeToolCallArguments(name, windsurfToolArgumentsJSON(c.Arguments), tools),
		})
		return true
	}

	var calls []windsurfParsedToolCall
	cleaned := windsurfParseXMLToolCalls(text, addCandidate, &calls)
	cleaned = windsurfParseJSONToolCalls(cleaned, addCandidate, &calls)
	if len(calls) == 0 {
		return nil, text
	}
	return calls, strings.TrimSpace(cleaned)
}

func windsurfParseToolCallsDetailedFromText(text string, tools []apicompat.ChatTool) ([]windsurfParsedToolCall, string, string) {
	calls, cleaned := windsurfParseToolCallsFromText(text, tools)
	switch {
	case len(calls) > 0:
		return calls, cleaned, "parsed"
	case windsurfLooksLikeToolCallText(text):
		return nil, text, "unparsed_tool_marker"
	case strings.TrimSpace(text) == "":
		return nil, text, "empty_response"
	default:
		return nil, text, "no_tool_call"
	}
}

func windsurfLooksLikeToolCallText(text string) bool {
	normalized := strings.ToLower(text)
	return strings.Contains(normalized, "<tool_call") ||
		strings.Contains(normalized, "function_call") ||
		strings.Contains(normalized, "tool_calls")
}

func windsurfToolBridgeEmptyFallbackText(toolChoice json.RawMessage) string {
	mode, forceName := windsurfResolveToolChoice(toolChoice)
	if mode == "required" {
		if forceName != "" {
			return fmt.Sprintf("The upstream model returned no valid %s tool call. Please retry this request.", forceName)
		}
		return "The upstream model returned no valid tool call. Please retry this request."
	}
	return "The upstream model returned an empty response. Please retry this request."
}

func windsurfParseXMLToolCalls(text string, add func(windsurfToolCallCandidate, *[]windsurfParsedToolCall) bool, calls *[]windsurfParsedToolCall) string {
	const openTag = "<tool_call>"
	const closeTag = "</tool_call>"
	var out strings.Builder
	cursor := 0
	for {
		startRel := strings.Index(text[cursor:], openTag)
		if startRel < 0 {
			out.WriteString(text[cursor:])
			break
		}
		start := cursor + startRel
		bodyStart := start + len(openTag)
		endRel := strings.Index(text[bodyStart:], closeTag)
		if endRel < 0 {
			out.WriteString(text[cursor:])
			break
		}
		end := bodyStart + endRel
		out.WriteString(text[cursor:start])
		body := strings.TrimSpace(text[bodyStart:end])
		var parsed any
		added := false
		if err := json.Unmarshal([]byte(body), &parsed); err == nil {
			for _, candidate := range windsurfExtractToolCallCandidates(parsed) {
				if add(candidate, calls) {
					added = true
				}
			}
		}
		if !added {
			out.WriteString(text[start : end+len(closeTag)])
		}
		cursor = end + len(closeTag)
	}
	return out.String()
}

func windsurfParseJSONToolCalls(text string, add func(windsurfToolCallCandidate, *[]windsurfParsedToolCall) bool, calls *[]windsurfParsedToolCall) string {
	working := text
	for {
		changed := false
		for i := 0; i < len(working); i++ {
			if working[i] != '{' {
				continue
			}
			end := windsurfFindBalancedJSONEnd(working, i)
			if end < 0 {
				continue
			}
			slice := working[i : end+1]
			var parsed any
			if err := json.Unmarshal([]byte(slice), &parsed); err != nil {
				continue
			}
			candidates := windsurfExtractToolCallCandidates(parsed)
			if len(candidates) == 0 {
				continue
			}
			before := len(*calls)
			for _, candidate := range candidates {
				add(candidate, calls)
			}
			if len(*calls) == before {
				continue
			}
			working = working[:i] + working[end+1:]
			changed = true
			break
		}
		if !changed {
			return working
		}
	}
}

func windsurfExtractToolCallCandidates(parsed any) []windsurfToolCallCandidate {
	obj, ok := parsed.(map[string]any)
	if !ok {
		return nil
	}
	if name, _ := obj["name"].(string); strings.TrimSpace(name) != "" {
		if args, ok := obj["arguments"]; ok {
			return []windsurfToolCallCandidate{{Name: name, Arguments: args}}
		}
	}
	for _, key := range []string{"function_call", "function"} {
		if inner, _ := obj[key].(map[string]any); inner != nil {
			if name, _ := inner["name"].(string); strings.TrimSpace(name) != "" {
				return []windsurfToolCallCandidate{{
					Name:      name,
					Arguments: inner["arguments"],
				}}
			}
		}
	}
	if rawCalls, _ := obj["tool_calls"].([]any); len(rawCalls) > 0 {
		out := make([]windsurfToolCallCandidate, 0, len(rawCalls))
		for _, raw := range rawCalls {
			item, _ := raw.(map[string]any)
			if item == nil {
				continue
			}
			inner, _ := item["function"].(map[string]any)
			if inner == nil {
				inner = item
			}
			name, _ := inner["name"].(string)
			if strings.TrimSpace(name) == "" {
				continue
			}
			id, _ := item["id"].(string)
			out = append(out, windsurfToolCallCandidate{
				ID:        id,
				Name:      name,
				Arguments: inner["arguments"],
			})
		}
		return out
	}
	return nil
}

func windsurfToolArgumentsJSON(value any) string {
	switch v := value.(type) {
	case nil:
		return "{}"
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "{}"
		}
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return trimmed
		}
		buf, _ := json.Marshal(map[string]any{"input": trimmed})
		return string(buf)
	default:
		buf, err := json.Marshal(v)
		if err != nil || string(buf) == "null" {
			return "{}"
		}
		return string(buf)
	}
}

func windsurfNormalizeToolCallArguments(name string, raw string, tools []apicompat.ChatTool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !windsurfToolRequiresParameter(tools, name, "description") {
		return raw
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return raw
	}
	if value, ok := obj["description"]; ok {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return raw
		}
	}
	obj["description"] = windsurfToolDescriptionFallback(name, obj)
	buf, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return string(buf)
}

func windsurfToolRequiresParameter(tools []apicompat.ChatTool, name string, parameter string) bool {
	parameter = strings.TrimSpace(parameter)
	if parameter == "" {
		return false
	}
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(tool.Function.Name), strings.TrimSpace(name)) {
			continue
		}
		for _, required := range windsurfToolRequiredParameters(tool.Function.Parameters) {
			if required == parameter {
				return true
			}
		}
	}
	return false
}

func windsurfToolRequiredParameters(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	required := make([]string, 0, len(schema.Required))
	for _, name := range schema.Required {
		name = strings.TrimSpace(name)
		if name != "" {
			required = append(required, name)
		}
	}
	return required
}

func windsurfToolDescriptionFallback(name string, args map[string]any) string {
	for _, key := range []string{"description", "command", "cmd", "query", "path", "file_path"} {
		value, _ := args[key].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = strings.Join(strings.Fields(value), " ")
		if len(value) > 120 {
			return value[:120]
		}
		return value
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "Tool call"
	}
	return "Call " + name
}

func windsurfFindBalancedJSONEnd(s string, start int) int {
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
		if inString {
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i
			}
			if depth < 0 {
				return -1
			}
		}
	}
	return -1
}

func windsurfCollectRawResponse(reader io.Reader, messages []windsurfRawMessage, onChunk func(string) error) (OpenAIUsage, string, error) {
	var contentBuilder strings.Builder
	var buffer []byte
	tmp := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(tmp)
		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			frames, rest := windsurfDrainGRPCFrames(buffer)
			buffer = rest
			for _, frame := range frames {
				text, _, isErr, err := windsurfParseRawResponse(frame)
				if err != nil {
					return OpenAIUsage{}, contentBuilder.String(), err
				}
				if isErr {
					return OpenAIUsage{}, contentBuilder.String(), errors.New("windsurf upstream returned error frame")
				}
				if text == "" {
					continue
				}
				contentBuilder.WriteString(text)
				if onChunk != nil {
					if err := onChunk(text); err != nil {
						return OpenAIUsage{}, contentBuilder.String(), err
					}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return OpenAIUsage{}, contentBuilder.String(), readErr
		}
	}
	text := contentBuilder.String()
	if strings.TrimSpace(text) == "" {
		return OpenAIUsage{}, "", errors.New("windsurf returned empty response")
	}
	var prompt strings.Builder
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		prompt.WriteString(msg.Content)
		prompt.WriteByte('\n')
	}
	return OpenAIUsage{
		InputTokens:  windsurfEstimateTokens(prompt.String()),
		OutputTokens: windsurfEstimateTokens(text),
	}, text, nil
}

func buildWindsurfAnthropicResponse(messageID, model, text string, usage OpenAIUsage) *apicompat.AnthropicResponse {
	return buildWindsurfAnthropicResponseWithBlocks(messageID, model, windsurfBuildAnthropicBlocks("", text, nil), "end_turn", usage)
}

func buildWindsurfAnthropicResponseWithBlocks(messageID, model string, blocks []apicompat.AnthropicContentBlock, stopReason string, usage OpenAIUsage) *apicompat.AnthropicResponse {
	if len(blocks) == 0 {
		blocks = []apicompat.AnthropicContentBlock{{Type: "text", Text: ""}}
	}
	if strings.TrimSpace(stopReason) == "" {
		stopReason = "end_turn"
	}
	return &apicompat.AnthropicResponse{
		ID:         messageID,
		Type:       "message",
		Role:       "assistant",
		Content:    blocks,
		Model:      model,
		StopReason: stopReason,
		Usage:      *openAIUsageToAnthropicUsage(usage),
	}
}

func windsurfBuildAnthropicBlocks(thinkingText, text string, toolCalls []windsurfParsedToolCall) []apicompat.AnthropicContentBlock {
	blocks := make([]apicompat.AnthropicContentBlock, 0, 2+len(toolCalls))
	if strings.TrimSpace(thinkingText) != "" {
		blocks = append(blocks, apicompat.AnthropicContentBlock{
			Type:     "thinking",
			Thinking: thinkingText,
		})
	}
	if text != "" {
		blocks = append(blocks, apicompat.AnthropicContentBlock{
			Type: "text",
			Text: text,
		})
	}
	for _, call := range toolCalls {
		args := strings.TrimSpace(call.Arguments)
		if args == "" {
			args = "{}"
		}
		blocks = append(blocks, apicompat.AnthropicContentBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Name,
			Input: json.RawMessage(args),
		})
	}
	return blocks
}

func openAIUsageToAnthropicUsage(usage OpenAIUsage) *apicompat.AnthropicUsage {
	return &apicompat.AnthropicUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
}

func windsurfBuildResponsesResponse(responseID, itemID, model, text string, usage OpenAIUsage, createdAt int64) *apicompat.ResponsesResponse {
	return windsurfBuildResponsesResponseWithOutputs(responseID, model, windsurfBuildResponsesOutputs("", "", itemID, text, nil), usage, createdAt)
}

func windsurfBuildResponsesOutputs(reasoningItemID, reasoningText, messageItemID, text string, toolCalls []windsurfParsedToolCall) []apicompat.ResponsesOutput {
	output := make([]apicompat.ResponsesOutput, 0, 2+len(toolCalls))
	if strings.TrimSpace(reasoningText) != "" {
		if reasoningItemID == "" {
			reasoningItemID = "rs_" + uuid.NewString()
		}
		output = append(output, apicompat.ResponsesOutput{
			Type:   "reasoning",
			ID:     reasoningItemID,
			Status: "completed",
			Summary: []apicompat.ResponsesSummary{{
				Type: "summary_text",
				Text: reasoningText,
			}},
		})
	}
	if text != "" {
		if messageItemID == "" {
			messageItemID = "msg_" + uuid.NewString()
		}
		output = append(output, apicompat.ResponsesOutput{
			Type:   "message",
			ID:     messageItemID,
			Role:   "assistant",
			Status: "completed",
			Content: []apicompat.ResponsesContentPart{{
				Type: "output_text",
				Text: text,
			}},
		})
	}
	for _, call := range toolCalls {
		output = append(output, apicompat.ResponsesOutput{
			Type:      "function_call",
			ID:        "fc_" + uuid.NewString(),
			CallID:    call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
			Status:    "completed",
		})
	}
	return output
}

func windsurfBuildResponsesResponseWithOutputs(responseID, model string, output []apicompat.ResponsesOutput, usage OpenAIUsage, createdAt int64) *apicompat.ResponsesResponse {
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return &apicompat.ResponsesResponse{
		ID:        responseID,
		Object:    "response",
		CreatedAt: createdAt,
		Model:     model,
		Status:    "completed",
		Output:    output,
		Usage: &apicompat.ResponsesUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.InputTokens + usage.OutputTokens,
		},
	}
}

func windsurfBuildResponsesContentPartEvent(eventType string, sequence int, itemID, text string) apicompat.ResponsesStreamEvent {
	return windsurfBuildResponsesContentPartEventAt(eventType, sequence, 0, itemID, text)
}

func windsurfBuildResponsesContentPartEventAt(eventType string, sequence, outputIndex int, itemID, text string) apicompat.ResponsesStreamEvent {
	annotations := []any{}
	return apicompat.ResponsesStreamEvent{
		Type:           eventType,
		SequenceNumber: sequence,
		OutputIndex:    outputIndex,
		ContentIndex:   0,
		ItemID:         itemID,
		Part: &apicompat.ResponsesContentPart{
			Type:        "output_text",
			Text:        text,
			Annotations: &annotations,
		},
	}
}

func windsurfWriteResponsesToolCallEvents(c *gin.Context, calls []windsurfParsedToolCall, sequence *int, nextOutputIndex *int) error {
	for _, call := range calls {
		outputIndex := *nextOutputIndex
		(*nextOutputIndex)++
		itemID := "fc_" + uuid.NewString()
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.output_item.added",
			SequenceNumber: *sequence,
			OutputIndex:    outputIndex,
			Item: &apicompat.ResponsesOutput{
				Type:      "function_call",
				ID:        itemID,
				CallID:    call.ID,
				Name:      call.Name,
				Arguments: "",
				Status:    "in_progress",
			},
		}); err != nil {
			return err
		}
		(*sequence)++
		if call.Arguments != "" {
			if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
				Type:           "response.function_call_arguments.delta",
				SequenceNumber: *sequence,
				OutputIndex:    outputIndex,
				Delta:          call.Arguments,
				ItemID:         itemID,
				CallID:         call.ID,
				Name:           call.Name,
			}); err != nil {
				return err
			}
			(*sequence)++
		}
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.function_call_arguments.done",
			SequenceNumber: *sequence,
			OutputIndex:    outputIndex,
			Arguments:      call.Arguments,
			ItemID:         itemID,
			CallID:         call.ID,
			Name:           call.Name,
		}); err != nil {
			return err
		}
		(*sequence)++
		if err := writeWindsurfResponsesEvent(c, apicompat.ResponsesStreamEvent{
			Type:           "response.output_item.done",
			SequenceNumber: *sequence,
			OutputIndex:    outputIndex,
			Item: &apicompat.ResponsesOutput{
				Type:      "function_call",
				ID:        itemID,
				CallID:    call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
				Status:    "completed",
			},
		}); err != nil {
			return err
		}
		(*sequence)++
	}
	return nil
}

func writeWindsurfResponsesEvent(c *gin.Context, evt apicompat.ResponsesStreamEvent) error {
	if c == nil {
		return nil
	}
	line, err := apicompat.ResponsesEventToSSE(evt)
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte(line)); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

func writeWindsurfAnthropicEvent(c *gin.Context, evt apicompat.AnthropicStreamEvent) error {
	if c == nil {
		return nil
	}
	line, err := apicompat.ResponsesAnthropicEventToSSE(evt)
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte(line)); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

func resolveWindsurfModel(candidates ...string) (windsurfModelInfo, string, error) {
	for _, candidate := range candidates {
		normalized := normalizeWindsurfModelAlias(candidate)
		if normalized == "" {
			continue
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
