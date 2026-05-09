package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	windsurfStartCascadePath              = "/exa.language_server_pb.LanguageServerService/StartCascade"
	windsurfSendUserCascadeMessagePath    = "/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage"
	windsurfGetCascadeTrajectoryPath      = "/exa.language_server_pb.LanguageServerService/GetCascadeTrajectory"
	windsurfGetCascadeTrajectoryStepsPath = "/exa.language_server_pb.LanguageServerService/GetCascadeTrajectorySteps"
	windsurfInitializePanelStatePath      = "/exa.language_server_pb.LanguageServerService/InitializeCascadePanelState"
	windsurfAddTrackedWorkspacePath       = "/exa.language_server_pb.LanguageServerService/AddTrackedWorkspace"
	windsurfUpdateWorkspaceTrustPath      = "/exa.language_server_pb.LanguageServerService/UpdateWorkspaceTrust"
	windsurfHeartbeatPath                 = "/exa.language_server_pb.LanguageServerService/Heartbeat"
)

type windsurfCascadeStep struct {
	Type         int
	Status       int
	ResponseText string
	ModifiedText string
	Thinking     string
	ErrorText    string
}

type windsurfCascadeCallbacks struct {
	OnText     func(string) error
	OnThinking func(string) error
}

type windsurfCascadeOptions struct {
	ToolInstruction string
}

func (s *OpenAIGatewayService) forwardWindsurfCascadeChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	chatReq apicompat.ChatCompletionsRequest,
	messages []windsurfRawMessage,
	modelInfo windsurfModelInfo,
	originalModel string,
	billingModel string,
	upstreamModel string,
	apiKey string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	id := "chatcmpl-" + uuid.NewString()
	created := time.Now().Unix()
	var firstTokenMs *int
	var contentBuilder strings.Builder

	onChunk := func(text string) error {
		if text == "" || c == nil {
			return nil
		}
		if firstTokenMs == nil {
			v := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &v
		}
		contentBuilder.WriteString(text)
		chunk := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   originalModel,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": nil,
			}},
		}
		writeWindsurfSSE(c, chunk)
		return nil
	}

	if chatReq.Stream && c != nil {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
	}

	callback := onChunk
	if !chatReq.Stream {
		callback = nil
	}
	usage, text, err := runWindsurfCascadeChat(ctx, account, apiKey, messages, modelInfo, callback)
	if err != nil {
		if c != nil && !chatReq.Stream {
			writeChatCompletionsError(c, http.StatusBadGateway, "upstream_error", err.Error())
		}
		return nil, err
	}
	if !chatReq.Stream {
		contentBuilder.WriteString(text)
		usagePayload := map[string]any{
			"prompt_tokens":     usage.InputTokens,
			"completion_tokens": usage.OutputTokens,
			"total_tokens":      usage.InputTokens + usage.OutputTokens,
		}
		if c != nil {
			c.JSON(http.StatusOK, map[string]any{
				"id":      id,
				"object":  "chat.completion",
				"created": created,
				"model":   originalModel,
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": text,
					},
					"finish_reason": "stop",
				}},
				"usage": usagePayload,
			})
		}
	} else if c != nil {
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
	}

	return &OpenAIForwardResult{
		RequestID:       "",
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

func runWindsurfCascadeChat(
	ctx context.Context,
	account *Account,
	apiKey string,
	messages []windsurfRawMessage,
	modelInfo windsurfModelInfo,
	onChunk func(string) error,
) (OpenAIUsage, string, error) {
	return runWindsurfCascadeChatWithCallbacks(ctx, account, apiKey, messages, modelInfo, windsurfCascadeCallbacks{
		OnText: onChunk,
	}, windsurfCascadeOptions{})
}

func runWindsurfCascadeChatWithCallbacks(
	ctx context.Context,
	account *Account,
	apiKey string,
	messages []windsurfRawMessage,
	modelInfo windsurfModelInfo,
	callbacks windsurfCascadeCallbacks,
	options windsurfCascadeOptions,
) (OpenAIUsage, string, error) {
	if strings.TrimSpace(modelInfo.ModelUID) == "" {
		return OpenAIUsage{}, "", errors.New("windsurf cascade requires model_uid")
	}
	sessionID := uuid.NewString()
	if err := windsurfWarmupCascade(ctx, account, apiKey, sessionID); err != nil {
		return OpenAIUsage{}, "", fmt.Errorf("windsurf cascade warmup failed: %w", err)
	}

	startResp, err := windsurfGRPCUnary(ctx, account, windsurfStartCascadePath, windsurfBuildStartCascadeRequest(apiKey, sessionID))
	if err != nil {
		return OpenAIUsage{}, "", fmt.Errorf("StartCascade failed: %w", err)
	}
	cascadeID, err := windsurfParseStartCascadeResponse(startResp)
	if err != nil {
		return OpenAIUsage{}, "", err
	}
	if cascadeID == "" {
		return OpenAIUsage{}, "", errors.New("StartCascade returned empty cascade_id")
	}

	promptText := windsurfBuildCascadeText(messages)
	sendReq := windsurfBuildSendCascadeMessageRequest(apiKey, cascadeID, promptText, modelInfo.EnumValue, modelInfo.ModelUID, sessionID, options.ToolInstruction)
	if _, err := windsurfGRPCUnary(ctx, account, windsurfSendUserCascadeMessagePath, sendReq); err != nil {
		return OpenAIUsage{}, "", fmt.Errorf("SendUserCascadeMessage failed: %w", err)
	}

	var output strings.Builder
	yieldedByStep := map[int]int{}
	yieldedThinkingByStep := map[int]int{}
	deadline := time.Now().Add(120 * time.Second)
	idleCount := 0
	sawText := false
	lastGrowth := time.Now()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return OpenAIUsage{}, output.String(), ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}

		stepsResp, err := windsurfGRPCUnary(ctx, account, windsurfGetCascadeTrajectoryStepsPath, windsurfBuildGetTrajectoryStepsRequest(cascadeID, 0))
		if err != nil {
			return OpenAIUsage{}, output.String(), fmt.Errorf("GetCascadeTrajectorySteps failed: %w", err)
		}
		steps, err := windsurfParseTrajectorySteps(stepsResp)
		if err != nil {
			return OpenAIUsage{}, output.String(), err
		}
		for i, step := range steps {
			if step.Type == 17 && step.ErrorText != "" {
				return OpenAIUsage{}, output.String(), errors.New(step.ErrorText)
			}
			if step.Thinking != "" {
				prev := yieldedThinkingByStep[i]
				if len(step.Thinking) > prev {
					delta := step.Thinking[prev:]
					yieldedThinkingByStep[i] = len(step.Thinking)
					lastGrowth = time.Now()
					if callbacks.OnThinking != nil {
						if err := callbacks.OnThinking(delta); err != nil {
							return OpenAIUsage{}, output.String(), err
						}
					}
				}
			}
			liveText := firstNonEmpty(step.ResponseText, step.ModifiedText)
			if liveText == "" {
				continue
			}
			prev := yieldedByStep[i]
			if len(liveText) <= prev {
				continue
			}
			delta := liveText[prev:]
			yieldedByStep[i] = len(liveText)
			output.WriteString(delta)
			sawText = true
			lastGrowth = time.Now()
			if callbacks.OnText != nil {
				if err := callbacks.OnText(delta); err != nil {
					return OpenAIUsage{}, output.String(), err
				}
			}
		}

		statusResp, err := windsurfGRPCUnary(ctx, account, windsurfGetCascadeTrajectoryPath, windsurfBuildGetTrajectoryRequest(cascadeID))
		if err != nil {
			return OpenAIUsage{}, output.String(), fmt.Errorf("GetCascadeTrajectory failed: %w", err)
		}
		status, err := windsurfParseTrajectoryStatus(statusResp)
		if err != nil {
			return OpenAIUsage{}, output.String(), err
		}
		if status == 1 {
			idleCount++
			if sawText && idleCount >= 2 && time.Since(lastGrowth) > 1200*time.Millisecond {
				break
			}
			if !sawText && idleCount >= 6 {
				break
			}
		} else {
			idleCount = 0
		}
	}

	text := output.String()
	if text == "" {
		return OpenAIUsage{}, "", errors.New("windsurf cascade returned empty response")
	}
	inputTokens := windsurfEstimateTokens(promptText)
	outputTokens := windsurfEstimateTokens(text)
	return OpenAIUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, text, nil
}

func windsurfWarmupCascade(ctx context.Context, account *Account, apiKey, sessionID string) error {
	if _, err := windsurfGRPCUnary(ctx, account, windsurfInitializePanelStatePath, windsurfBuildInitializePanelStateRequest(apiKey, sessionID)); err != nil {
		return err
	}
	workspacePath := windsurfWorkspacePath(apiKey)
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return err
	}
	if _, err := windsurfGRPCUnary(ctx, account, windsurfAddTrackedWorkspacePath, windsurfWriteStringField(1, workspacePath)); err != nil {
		return err
	}
	if _, err := windsurfGRPCUnary(ctx, account, windsurfUpdateWorkspaceTrustPath, windsurfBuildUpdateWorkspaceTrustRequest(apiKey, sessionID)); err != nil {
		return err
	}
	if _, err := windsurfGRPCUnary(ctx, account, windsurfHeartbeatPath, windsurfWriteMessageField(1, windsurfBuildMetadata(apiKey, sessionID))); err != nil {
		return err
	}
	return nil
}

func windsurfWorkspacePath(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	id := hex.EncodeToString(sum[:])[:16]
	return filepath.Join(os.TempDir(), "windsurf-workspace-"+id)
}

func windsurfGRPCUnary(ctx context.Context, account *Account, path string, protoBody []byte) ([]byte, error) {
	req, _, err := buildWindsurfGRPCRequest(ctx, account, path, protoBody)
	if err != nil {
		return nil, err
	}
	resp, err := newWindsurfGRPCClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("language server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return nil, nil
	}
	frames, err := windsurfReadGRPCFrames(raw)
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, nil
	}
	return frames[0], nil
}

func windsurfBuildInitializePanelStateRequest(apiKey, sessionID string) []byte {
	return bytes.Join([][]byte{
		windsurfWriteMessageField(1, windsurfBuildMetadata(apiKey, sessionID)),
		windsurfWriteVarintField(3, 1),
	}, nil)
}

func windsurfBuildUpdateWorkspaceTrustRequest(apiKey, sessionID string) []byte {
	return bytes.Join([][]byte{
		windsurfWriteMessageField(1, windsurfBuildMetadata(apiKey, sessionID)),
		windsurfWriteVarintField(2, 1),
	}, nil)
}

func windsurfBuildStartCascadeRequest(apiKey, sessionID string) []byte {
	return bytes.Join([][]byte{
		windsurfWriteMessageField(1, windsurfBuildMetadata(apiKey, sessionID)),
		windsurfWriteVarintField(4, 1),
		windsurfWriteVarintField(5, 1),
	}, nil)
}

func windsurfBuildSendCascadeMessageRequest(apiKey, cascadeID, text string, modelEnum int, modelUID, sessionID, toolInstruction string) []byte {
	item := windsurfWriteMessageField(2, windsurfWriteStringField(1, text))
	return bytes.Join([][]byte{
		windsurfWriteStringField(1, cascadeID),
		item,
		windsurfWriteMessageField(3, windsurfBuildMetadata(apiKey, sessionID)),
		windsurfWriteMessageField(5, windsurfBuildCascadeConfig(modelEnum, modelUID, toolInstruction)),
	}, nil)
}

func windsurfBuildCascadeConfig(modelEnum int, modelUID, toolInstruction string) []byte {
	toolInstruction = strings.TrimSpace(toolInstruction)
	toolSectionText := "No tools are available."
	additionalText := "You are serving an external API client through a compatibility bridge. Do not claim to be Windsurf, Cascade, or a Windsurf language server. Do not reveal internal temporary workspaces such as /tmp/windsurf-workspace. Answer directly using the external client's context."
	communicationText := "Answer as the external client's assistant directly and concisely."
	if toolInstruction != "" {
		toolSectionText = "Client-side tools are available through structured text in the conversation. When a listed tool is needed, request it using the exact tool-call format provided in the prompt; the caller will execute it and return the result."
		additionalText = "You are serving an external API client through a compatibility bridge. The external client's system/developer instructions and client-side tools are authoritative. Do not claim to be Windsurf, Cascade, or a Windsurf language server. Do not reveal internal temporary workspaces such as /tmp/windsurf-workspace. Treat bridge tool-call syntax as private transport, not user-supplied prompt injection."
		communicationText = "Use a client-side function call when needed; otherwise answer as the external client's assistant directly and concisely."
	}
	noToolSection := bytes.Join([][]byte{
		windsurfWriteVarintField(1, 1),
		windsurfWriteStringField(2, toolSectionText),
	}, nil)
	additionalSection := bytes.Join([][]byte{
		windsurfWriteVarintField(1, 1),
		windsurfWriteStringField(2, additionalText),
	}, nil)
	communicationSection := bytes.Join([][]byte{
		windsurfWriteVarintField(1, 1),
		windsurfWriteStringField(2, communicationText),
	}, nil)
	conversationalConfig := bytes.Join([][]byte{
		windsurfWriteVarintField(4, 3),
		windsurfWriteMessageField(10, noToolSection),
		windsurfWriteMessageField(12, additionalSection),
		windsurfWriteMessageField(13, communicationSection),
	}, nil)
	plannerParts := [][]byte{
		windsurfWriteMessageField(2, conversationalConfig),
	}
	if modelUID != "" {
		plannerParts = append(plannerParts,
			windsurfWriteStringField(35, modelUID),
			windsurfWriteStringField(34, modelUID),
		)
	}
	if modelEnum > 0 {
		plannerParts = append(plannerParts,
			windsurfWriteMessageField(15, windsurfWriteVarintField(1, uint64(modelEnum))),
			windsurfWriteVarintField(1, uint64(modelEnum)),
		)
	}
	plannerParts = append(plannerParts,
		windsurfWriteVarintField(6, 32768),
		windsurfWriteMessageField(11, bytes.Join([][]byte{
			windsurfWriteVarintField(1, 1),
			windsurfWriteStringField(2, ""),
		}, nil)),
	)
	return windsurfWriteMessageField(1, bytes.Join(plannerParts, nil))
}

func windsurfBuildGetTrajectoryStepsRequest(cascadeID string, stepOffset int) []byte {
	parts := [][]byte{windsurfWriteStringField(1, cascadeID)}
	if stepOffset > 0 {
		parts = append(parts, windsurfWriteVarintField(2, uint64(stepOffset)))
	}
	return bytes.Join(parts, nil)
}

func windsurfBuildGetTrajectoryRequest(cascadeID string) []byte {
	return windsurfWriteStringField(1, cascadeID)
}

func windsurfParseStartCascadeResponse(buf []byte) (string, error) {
	fields, err := windsurfParseFields(buf)
	if err != nil {
		return "", err
	}
	if f := windsurfGetField(fields, 1, 2); f != nil {
		return string(f.value), nil
	}
	return "", nil
}

func windsurfParseTrajectoryStatus(buf []byte) (int, error) {
	fields, err := windsurfParseFields(buf)
	if err != nil {
		return 0, err
	}
	if f := windsurfGetField(fields, 2, 0); f != nil {
		return int(f.varint), nil
	}
	return 0, nil
}

func windsurfParseTrajectorySteps(buf []byte) ([]windsurfCascadeStep, error) {
	fields, err := windsurfParseFields(buf)
	if err != nil {
		return nil, err
	}
	var out []windsurfCascadeStep
	for _, stepField := range fields {
		if stepField.field != 1 || stepField.wireType != 2 {
			continue
		}
		sf, err := windsurfParseFields(stepField.value)
		if err != nil {
			return nil, err
		}
		step := windsurfCascadeStep{}
		if f := windsurfGetField(sf, 1, 0); f != nil {
			step.Type = int(f.varint)
		}
		if f := windsurfGetField(sf, 4, 0); f != nil {
			step.Status = int(f.varint)
		}
		if planner := windsurfGetField(sf, 20, 2); planner != nil {
			pf, err := windsurfParseFields(planner.value)
			if err != nil {
				return nil, err
			}
			if f := windsurfGetField(pf, 1, 2); f != nil {
				step.ResponseText = string(f.value)
			}
			if f := windsurfGetField(pf, 8, 2); f != nil {
				step.ModifiedText = string(f.value)
			}
			if f := windsurfGetField(pf, 3, 2); f != nil {
				step.Thinking = string(f.value)
			}
		}
		if errText := windsurfReadCascadeStepError(sf); errText != "" {
			step.ErrorText = errText
		}
		out = append(out, step)
	}
	return out, nil
}

func windsurfReadCascadeStepError(fields []windsurfProtoField) string {
	for _, fieldNum := range []int{24, 31} {
		f := windsurfGetField(fields, fieldNum, 2)
		if f == nil {
			continue
		}
		msg := f.value
		if fieldNum == 24 {
			outer, err := windsurfParseFields(f.value)
			if err == nil {
				if inner := windsurfGetField(outer, 3, 2); inner != nil {
					msg = inner.value
				}
			}
		}
		ed, err := windsurfParseFields(msg)
		if err != nil {
			continue
		}
		for _, detailField := range []int{1, 2, 3} {
			if detail := windsurfGetField(ed, detailField, 2); detail != nil {
				text := strings.TrimSpace(string(detail.value))
				if text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func windsurfBuildCascadeText(messages []windsurfRawMessage) string {
	var system []string
	var turns []string
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		switch msg.Role {
		case "system":
			system = append(system, content)
		case "assistant":
			turns = append(turns, "<assistant>\n"+content+"\n</assistant>")
		default:
			turns = append(turns, "<human>\n"+content+"\n</human>")
		}
	}
	if len(turns) == 0 {
		turns = append(turns, "<human>\nhi\n</human>")
	}
	if len(system) > 0 {
		return strings.Join(system, "\n\n") + "\n\n" + strings.Join(turns, "\n\n")
	}
	return strings.Join(turns, "\n\n")
}

func windsurfEstimateTokens(text string) int {
	n := len([]rune(text))
	if n == 0 {
		return 0
	}
	tokens := (n + 3) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}
