package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const (
	serverAgentModeHeader     = "X-Sub2API-Agent-Mode"
	serverAgentMaxTurnsHeader = "X-Sub2API-Agent-Max-Turns"
	serverAgentWorkdirHeader  = "X-Sub2API-Agent-Workdir"

	serverAgentDefaultMaxTurns = 8
	serverAgentMaxTurnsLimit   = 20
	serverAgentToolTimeout     = 20 * time.Second
	serverAgentMaxReadBytes    = 200 * 1024
	serverAgentMaxWriteBytes   = 2 * 1024 * 1024
	serverAgentMaxMatchCount   = 200
	serverAgentMaxListCount    = 500
)

type serverAgentMode string

const (
	serverAgentModeAuto   serverAgentMode = "auto"
	serverAgentModeClient serverAgentMode = "client"
	serverAgentModeServer serverAgentMode = "server"
)

type serverAgentExecution struct {
	Enabled    bool
	Mode       serverAgentMode
	Profile    ClientProfile
	MaxTurns   int
	WorkingDir string
}

type serverAgentTurnExecutor interface {
	ExecuteTurn(context.Context, *apicompat.ResponsesRequest) (*apicompat.ResponsesResponse, error)
}

type serverAgentLoopResult struct {
	Response *apicompat.ResponsesResponse
	Turns    int
}

type serverToolRuntime struct {
	account   *Account
	rootDir   string
	todoState string
}

type serverAgentToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

type openAIResponsesTurnExecutor struct {
	service *OpenAIGatewayService
	ctx     *gin.Context
	account *Account
}

type anthropicResponsesTurnExecutor struct {
	service *GatewayService
	ctx     *gin.Context
	account *Account
}

func resolveServerAgentExecution(c *gin.Context, fallbackWireProtocol string, toolsPresent bool) serverAgentExecution {
	profile := DetectClientProfile(nil, fallbackWireProtocol)
	if c != nil {
		profile = DetectClientProfile(c.Request, fallbackWireProtocol)
		profile = normalizeServerAgentClientProfile(c, profile, fallbackWireProtocol)
	}
	mode := parseServerAgentMode(c)
	if mode == "" {
		mode = serverAgentModeAuto
	}
	enabled := false
	if toolsPresent {
		switch mode {
		case serverAgentModeServer:
			enabled = true
		case serverAgentModeClient:
			enabled = false
		default:
			enabled = !profile.RequiresClientTools
		}
	}
	return serverAgentExecution{
		Enabled:    enabled,
		Mode:       mode,
		Profile:    profile,
		MaxTurns:   parseServerAgentMaxTurns(c),
		WorkingDir: resolveServerAgentWorkingDir(c),
	}
}

func normalizeServerAgentClientProfile(c *gin.Context, profile ClientProfile, fallbackWireProtocol string) ClientProfile {
	if c == nil || c.Request == nil {
		return profile
	}
	ua := strings.ToLower(strings.TrimSpace(c.Request.UserAgent()))
	switch profile.ID {
	case ClientProfileCodex:
		if !openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) &&
			!strings.Contains(ua, "opencode") &&
			!strings.Contains(ua, "codex") {
			if generic, ok := ClientProfileByID(ClientProfileOpenAIResponses); ok {
				return generic
			}
			return DetectClientProfile(nil, fallbackWireProtocol)
		}
	case ClientProfileClaudeCode:
		if !strings.Contains(ua, "claude") && !strings.Contains(ua, "opencode") {
			if generic, ok := ClientProfileByID(ClientProfileAnthropicMessages); ok {
				return generic
			}
			return DetectClientProfile(nil, fallbackWireProtocol)
		}
	}
	return profile
}

func parseServerAgentMode(c *gin.Context) serverAgentMode {
	if c == nil {
		return serverAgentModeAuto
	}
	switch strings.ToLower(strings.TrimSpace(c.GetHeader(serverAgentModeHeader))) {
	case "", "auto":
		return serverAgentModeAuto
	case "client":
		return serverAgentModeClient
	case "server":
		return serverAgentModeServer
	default:
		return serverAgentModeAuto
	}
}

func parseServerAgentMaxTurns(c *gin.Context) int {
	value := serverAgentDefaultMaxTurns
	if c == nil {
		return value
	}
	raw := strings.TrimSpace(c.GetHeader(serverAgentMaxTurnsHeader))
	if raw == "" {
		return value
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return value
	}
	if n < 1 {
		return 1
	}
	if n > serverAgentMaxTurnsLimit {
		return serverAgentMaxTurnsLimit
	}
	return n
}

func resolveServerAgentWorkingDir(c *gin.Context) string {
	if c != nil {
		if raw := strings.TrimSpace(c.GetHeader(serverAgentWorkdirHeader)); raw != "" {
			return raw
		}
	}
	if raw := strings.TrimSpace(os.Getenv("SUB2API_AGENT_WORKDIR")); raw != "" {
		return raw
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func hasResponsesTools(req *apicompat.ResponsesRequest) bool {
	return req != nil && len(req.Tools) > 0
}

func hasAnthropicTools(req *apicompat.AnthropicRequest) bool {
	return req != nil && len(req.Tools) > 0
}

func hasChatCompletionsTools(req *apicompat.ChatCompletionsRequest) bool {
	if req == nil {
		return false
	}
	return len(req.Tools) > 0 || len(req.Functions) > 0
}

func newServerToolRuntime(account *Account, workingDir string) *serverToolRuntime {
	root := strings.TrimSpace(workingDir)
	if root == "" {
		root = "."
	}
	return &serverToolRuntime{
		account: account,
		rootDir: root,
	}
}

func runServerAgentLoop(
	ctx context.Context,
	initial *apicompat.ResponsesRequest,
	executor serverAgentTurnExecutor,
	runtime *serverToolRuntime,
	maxTurns int,
) (*serverAgentLoopResult, error) {
	if initial == nil {
		return nil, errors.New("server agent requires request")
	}
	if executor == nil {
		return nil, errors.New("server agent executor is nil")
	}
	if runtime == nil {
		return nil, errors.New("server agent runtime is nil")
	}
	current := cloneResponsesRequest(initial)
	var finalResp *apicompat.ResponsesResponse
	totalUsage := &apicompat.ResponsesUsage{}
	for turn := 1; turn <= maxTurns; turn++ {
		resp, err := executor.ExecuteTurn(ctx, current)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("server agent executor returned nil response")
		}
		mergeServerAgentResponsesUsage(totalUsage, resp.Usage)
		finalResp = resp
		calls := collectFunctionCalls(resp.Output)
		if len(calls) == 0 {
			finalClone := cloneResponsesResponse(resp)
			if totalUsage.TotalTokens > 0 || totalUsage.InputTokens > 0 || totalUsage.OutputTokens > 0 {
				finalClone.Usage = totalUsage
			}
			return &serverAgentLoopResult{Response: finalClone, Turns: turn}, nil
		}

		items, err := responsesRequestInputToItems(current.Input)
		if err != nil {
			return nil, fmt.Errorf("server agent decode input: %w", err)
		}
		items = append(items, responsesOutputToInputItems(resp.Output)...)
		for _, call := range calls {
			output := runtime.executeTool(ctx, call.Name, call.Arguments)
			items = append(items, apicompat.ResponsesInputItem{
				Type:   "function_call_output",
				CallID: call.CallID,
				Output: output,
			})
		}
		nextInput, err := json.Marshal(items)
		if err != nil {
			return nil, fmt.Errorf("server agent encode input: %w", err)
		}
		current.Input = nextInput
		current.Stream = false
	}
	if finalResp == nil {
		return nil, errors.New("server agent produced no final response")
	}
	return nil, fmt.Errorf("server agent exceeded max turns (%d)", maxTurns)
}

func mergeServerAgentResponsesUsage(total *apicompat.ResponsesUsage, next *apicompat.ResponsesUsage) {
	if total == nil || next == nil {
		return
	}
	total.InputTokens += next.InputTokens
	total.OutputTokens += next.OutputTokens
	total.TotalTokens += next.TotalTokens
	if next.InputTokensDetails != nil && next.InputTokensDetails.CachedTokens > 0 {
		if total.InputTokensDetails == nil {
			total.InputTokensDetails = &apicompat.ResponsesInputTokensDetails{}
		}
		total.InputTokensDetails.CachedTokens += next.InputTokensDetails.CachedTokens
	}
	if next.OutputTokensDetails != nil && next.OutputTokensDetails.ReasoningTokens > 0 {
		if total.OutputTokensDetails == nil {
			total.OutputTokensDetails = &apicompat.ResponsesOutputTokensDetails{}
		}
		total.OutputTokensDetails.ReasoningTokens += next.OutputTokensDetails.ReasoningTokens
	}
}

func cloneResponsesRequest(req *apicompat.ResponsesRequest) *apicompat.ResponsesRequest {
	if req == nil {
		return nil
	}
	clone := *req
	clone.Include = append([]string(nil), req.Include...)
	clone.Tools = append([]apicompat.ResponsesTool(nil), req.Tools...)
	if req.ToolChoice != nil {
		clone.ToolChoice = append(json.RawMessage(nil), req.ToolChoice...)
	}
	if req.Input != nil {
		clone.Input = append(json.RawMessage(nil), req.Input...)
	}
	if req.Reasoning != nil {
		reasoning := *req.Reasoning
		clone.Reasoning = &reasoning
	}
	if req.MaxOutputTokens != nil {
		v := *req.MaxOutputTokens
		clone.MaxOutputTokens = &v
	}
	if req.Temperature != nil {
		v := *req.Temperature
		clone.Temperature = &v
	}
	if req.TopP != nil {
		v := *req.TopP
		clone.TopP = &v
	}
	if req.Store != nil {
		v := *req.Store
		clone.Store = &v
	}
	return &clone
}

func responsesRequestInputToItems(raw json.RawMessage) ([]apicompat.ResponsesInputItem, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		content, _ := json.Marshal(text)
		return []apicompat.ResponsesInputItem{{
			Role:    "user",
			Content: content,
		}}, nil
	}
	var items []apicompat.ResponsesInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func responsesOutputToInputItems(outputs []apicompat.ResponsesOutput) []apicompat.ResponsesInputItem {
	items := make([]apicompat.ResponsesInputItem, 0, len(outputs))
	for _, output := range outputs {
		switch output.Type {
		case "message":
			content, err := json.Marshal(output.Content)
			if err != nil {
				continue
			}
			items = append(items, apicompat.ResponsesInputItem{
				Role:    "assistant",
				Content: content,
			})
		case "function_call":
			items = append(items, apicompat.ResponsesInputItem{
				Type:      "function_call",
				CallID:    output.CallID,
				Name:      output.Name,
				Arguments: output.Arguments,
				ID:        output.ID,
			})
		}
	}
	return items
}

func collectFunctionCalls(outputs []apicompat.ResponsesOutput) []serverAgentToolCall {
	calls := make([]serverAgentToolCall, 0, len(outputs))
	for _, output := range outputs {
		if output.Type != "function_call" {
			continue
		}
		callID := strings.TrimSpace(output.CallID)
		if callID == "" {
			callID = strings.TrimSpace(output.ID)
		}
		calls = append(calls, serverAgentToolCall{
			CallID:    callID,
			Name:      strings.TrimSpace(output.Name),
			Arguments: strings.TrimSpace(output.Arguments),
		})
	}
	return calls
}

func responsesUsageToOpenAIUsage(usage *apicompat.ResponsesUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	out := OpenAIUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
	if usage.InputTokensDetails != nil {
		out.CacheReadInputTokens = usage.InputTokensDetails.CachedTokens
	}
	return out
}

func responsesUsageToClaudeUsage(usage *apicompat.ResponsesUsage) ClaudeUsage {
	if usage == nil {
		return ClaudeUsage{}
	}
	out := ClaudeUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
	if usage.InputTokensDetails != nil {
		out.CacheReadInputTokens = usage.InputTokensDetails.CachedTokens
	}
	return out
}

func cloneResponsesResponse(resp *apicompat.ResponsesResponse) *apicompat.ResponsesResponse {
	if resp == nil {
		return nil
	}
	clone := *resp
	clone.Output = append([]apicompat.ResponsesOutput(nil), resp.Output...)
	if resp.Usage != nil {
		usage := *resp.Usage
		if resp.Usage.InputTokensDetails != nil {
			details := *resp.Usage.InputTokensDetails
			usage.InputTokensDetails = &details
		}
		if resp.Usage.OutputTokensDetails != nil {
			details := *resp.Usage.OutputTokensDetails
			usage.OutputTokensDetails = &details
		}
		clone.Usage = &usage
	}
	if resp.IncompleteDetails != nil {
		details := *resp.IncompleteDetails
		clone.IncompleteDetails = &details
	}
	if resp.Error != nil {
		errObj := *resp.Error
		clone.Error = &errObj
	}
	return &clone
}

func synthesizeResponsesEvents(resp *apicompat.ResponsesResponse) []apicompat.ResponsesStreamEvent {
	if resp == nil {
		return nil
	}
	events := make([]apicompat.ResponsesStreamEvent, 0, len(resp.Output)*4+2)
	sequence := 0

	created := cloneResponsesResponse(resp)
	created.Status = "in_progress"
	created.Output = nil
	created.Usage = nil
	created.IncompleteDetails = nil
	created.Error = nil
	events = append(events, apicompat.ResponsesStreamEvent{
		Type:           "response.created",
		SequenceNumber: sequence,
		Response:       created,
	})
	sequence++

	for outputIndex, output := range resp.Output {
		switch output.Type {
		case "reasoning":
			item := output
			item.Summary = nil
			item.Status = "in_progress"
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.added",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &item,
			})
			sequence++
			for summaryIndex, summary := range output.Summary {
				if summary.Type != "summary_text" || summary.Text == "" {
					continue
				}
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.reasoning_summary_text.delta",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					SummaryIndex:   summaryIndex,
					Delta:          summary.Text,
					ItemID:         output.ID,
				})
				sequence++
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.reasoning_summary_text.done",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					SummaryIndex:   summaryIndex,
					Text:           summary.Text,
					ItemID:         output.ID,
				})
				sequence++
			}
			outputDone := output
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.done",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &outputDone,
			})
			sequence++
		case "function_call":
			item := output
			item.Arguments = ""
			item.Status = "in_progress"
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.added",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &item,
			})
			sequence++
			if strings.TrimSpace(output.Arguments) != "" {
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.function_call_arguments.delta",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					CallID:         output.CallID,
					Name:           output.Name,
					Delta:          output.Arguments,
				})
				sequence++
			}
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.function_call_arguments.done",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				CallID:         output.CallID,
				Name:           output.Name,
				Arguments:      output.Arguments,
			})
			sequence++
			outputDone := output
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.done",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &outputDone,
			})
			sequence++
		case "message":
			item := output
			item.Status = "in_progress"
			item.Content = nil
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.added",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &item,
			})
			sequence++
			for contentIndex, part := range output.Content {
				if part.Type != "output_text" {
					continue
				}
				startPart := part
				startPart.Text = ""
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.content_part.added",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					ContentIndex:   contentIndex,
					ItemID:         output.ID,
					Part:           &startPart,
				})
				sequence++
				if part.Text != "" {
					events = append(events, apicompat.ResponsesStreamEvent{
						Type:           "response.output_text.delta",
						SequenceNumber: sequence,
						OutputIndex:    outputIndex,
						ContentIndex:   contentIndex,
						ItemID:         output.ID,
						Delta:          part.Text,
					})
					sequence++
				}
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.output_text.done",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					ContentIndex:   contentIndex,
					ItemID:         output.ID,
					Text:           part.Text,
				})
				sequence++
				donePart := part
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:           "response.content_part.done",
					SequenceNumber: sequence,
					OutputIndex:    outputIndex,
					ContentIndex:   contentIndex,
					ItemID:         output.ID,
					Part:           &donePart,
				})
				sequence++
			}
			outputDone := output
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:           "response.output_item.done",
				SequenceNumber: sequence,
				OutputIndex:    outputIndex,
				Item:           &outputDone,
			})
			sequence++
		}
	}

	terminalType := "response.completed"
	switch resp.Status {
	case "failed":
		terminalType = "response.failed"
	case "incomplete":
		terminalType = "response.incomplete"
	}
	events = append(events, apicompat.ResponsesStreamEvent{
		Type:           terminalType,
		SequenceNumber: sequence,
		Response:       resp,
	})
	return events
}

func writeSyntheticResponsesStream(c *gin.Context, resp *apicompat.ResponsesResponse) error {
	if c == nil || resp == nil {
		return nil
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	for _, evt := range synthesizeResponsesEvents(resp) {
		sse, err := apicompat.ResponsesEventToSSE(evt)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(c.Writer, sse); err != nil {
			return err
		}
		c.Writer.Flush()
	}
	return nil
}

func writeSyntheticAnthropicStream(c *gin.Context, resp *apicompat.ResponsesResponse, model string) error {
	if c == nil || resp == nil {
		return nil
	}
	state := apicompat.NewResponsesEventToAnthropicState()
	state.Model = model
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	for _, evt := range synthesizeResponsesEvents(resp) {
		for _, anthEvt := range apicompat.ResponsesEventToAnthropicEvents(&evt, state) {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(anthEvt)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(c.Writer, sse); err != nil {
				return err
			}
			c.Writer.Flush()
		}
	}
	if finalEvents := apicompat.FinalizeResponsesAnthropicStream(state); len(finalEvents) > 0 {
		for _, anthEvt := range finalEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(anthEvt)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(c.Writer, sse); err != nil {
				return err
			}
			c.Writer.Flush()
		}
	}
	return nil
}

func writeSyntheticChatCompletionsStream(c *gin.Context, resp *apicompat.ResponsesResponse, model string, includeUsage bool) error {
	if c == nil || resp == nil {
		return nil
	}
	state := apicompat.NewResponsesEventToChatState()
	state.Model = model
	state.IncludeUsage = includeUsage
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	for _, evt := range synthesizeResponsesEvents(resp) {
		for _, chunk := range apicompat.ResponsesEventToChatChunks(&evt, state) {
			sse, err := apicompat.ChatChunkToSSE(chunk)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(c.Writer, sse); err != nil {
				return err
			}
			c.Writer.Flush()
		}
	}
	if finalChunks := apicompat.FinalizeResponsesChatStream(state); len(finalChunks) > 0 {
		for _, chunk := range finalChunks {
			sse, err := apicompat.ChatChunkToSSE(chunk)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(c.Writer, sse); err != nil {
				return err
			}
			c.Writer.Flush()
		}
	}
	_, err := io.WriteString(c.Writer, "data: [DONE]\n\n")
	if err == nil {
		c.Writer.Flush()
	}
	return err
}

func bufferResponsesStreamBody(reader io.Reader, cfg *config.Config) (*apicompat.ResponsesResponse, error) {
	scanner := bufio.NewScanner(reader)
	maxLineSize := defaultMaxLineSize
	if cfg != nil && cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var finalResponse *apicompat.ResponsesResponse
	acc := apicompat.NewBufferedResponseAccumulator()
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		payload := line[6:]
		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		acc.ProcessEvent(&event)
		switch event.Type {
		case "response.completed", "response.done", "response.incomplete", "response.failed":
			if event.Response != nil {
				finalResponse = event.Response
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if finalResponse == nil {
		return nil, errors.New("upstream stream ended without terminal response event")
	}
	acc.SupplementResponseOutput(finalResponse)
	return finalResponse, nil
}

func bufferAnthropicStreamBody(reader io.Reader, cfg *config.Config) (*apicompat.AnthropicResponse, error) {
	scanner := bufio.NewScanner(reader)
	maxLineSize := defaultMaxLineSize
	if cfg != nil && cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var finalResp *apicompat.AnthropicResponse
	var usage apicompat.AnthropicUsage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "event: ") {
			continue
		}
		if !scanner.Scan() {
			break
		}
		dataLine := scanner.Text()
		if !strings.HasPrefix(dataLine, "data: ") {
			continue
		}
		payload := dataLine[6:]
		var event apicompat.AnthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if event.Type == "message_start" && event.Message != nil {
			finalResp = event.Message
			usage = event.Message.Usage
			continue
		}
		if event.Type == "message_delta" {
			if event.Usage != nil {
				usage = *event.Usage
			}
			if event.Delta != nil && event.Delta.StopReason != "" && finalResp != nil {
				finalResp.StopReason = event.Delta.StopReason
			}
			continue
		}
		if finalResp == nil {
			continue
		}
		if event.Type == "content_block_start" && event.ContentBlock != nil {
			finalResp.Content = append(finalResp.Content, *event.ContentBlock)
			continue
		}
		if event.Type == "content_block_delta" && event.Delta != nil && event.Index != nil {
			idx := *event.Index
			if idx < 0 || idx >= len(finalResp.Content) {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				finalResp.Content[idx].Text += event.Delta.Text
			case "thinking_delta":
				finalResp.Content[idx].Thinking += event.Delta.Thinking
			case "input_json_delta":
				finalResp.Content[idx].Input = appendRawJSON(finalResp.Content[idx].Input, event.Delta.PartialJSON)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if finalResp == nil {
		return nil, errors.New("upstream stream ended without a response")
	}
	finalResp.Usage = usage
	return finalResp, nil
}

func (e *openAIResponsesTurnExecutor) ExecuteTurn(ctx context.Context, req *apicompat.ResponsesRequest) (*apicompat.ResponsesResponse, error) {
	if e == nil || e.service == nil || e.account == nil {
		return nil, errors.New("openai server agent executor is not configured")
	}
	request := cloneResponsesRequest(req)
	request.Stream = true
	normalizeResponsesRequestServiceTier(request)
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	if e.account.Type == AccountTypeOAuth {
		var reqBody map[string]any
		if err := json.Unmarshal(body, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal responses request: %w", err)
		}
		codexResult := applyCodexOAuthTransform(reqBody, false, false)
		if codexResult.NormalizedModel != "" {
			request.Model = codexResult.NormalizedModel
		}
		body, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("remarshal responses request: %w", err)
		}
	}
	token, _, err := e.service.GetAccessToken(ctx, e.account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	upstreamReq, err := e.service.buildUpstreamRequest(ctx, e.ctx, e.account, body, token, true, "", false)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	proxyURL := ""
	if e.account.Proxy != nil {
		proxyURL = e.account.Proxy.URL()
	}
	resp, err := e.service.httpUpstream.Do(upstreamReq, proxyURL, e.account.ID, e.account.Concurrency)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return nil, fmt.Errorf("openai upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	}
	return bufferResponsesStreamBody(resp.Body, e.service.cfg)
}

func (e *anthropicResponsesTurnExecutor) ExecuteTurn(ctx context.Context, req *apicompat.ResponsesRequest) (*apicompat.ResponsesResponse, error) {
	if e == nil || e.service == nil || e.account == nil {
		return nil, errors.New("anthropic server agent executor is not configured")
	}
	request := cloneResponsesRequest(req)
	request.Stream = false
	anthropicReq, err := apicompat.ResponsesToAnthropicRequest(request)
	if err != nil {
		return nil, fmt.Errorf("convert responses to anthropic: %w", err)
	}
	anthropicReq.Stream = true
	mappedModel := request.Model
	if e.account.Type == AccountTypeAPIKey {
		mappedModel = e.account.GetMappedModel(mappedModel)
	}
	if mappedModel == request.Model && e.account.Platform == PlatformAnthropic && e.account.Type != AccountTypeAPIKey {
		if normalized := claude.NormalizeModelID(request.Model); normalized != "" {
			mappedModel = normalized
		}
	}
	anthropicReq.Model = mappedModel
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}
	shouldMimicClaudeCode := e.account.IsOAuth()
	if shouldMimicClaudeCode {
		body = e.service.applyClaudeCodeOAuthMimicryToBody(ctx, e.ctx, e.account, body, anthropicReq.System, mappedModel)
	}
	body = enforceCacheControlLimit(body)
	token, tokenType, err := e.service.GetAccessToken(ctx, e.account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	upstreamReq, err := e.service.buildUpstreamRequest(ctx, e.ctx, e.account, body, token, tokenType, mappedModel, true, shouldMimicClaudeCode)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	proxyURL := ""
	if e.account.ProxyID != nil && e.account.Proxy != nil {
		proxyURL = e.account.Proxy.URL()
	}
	resp, err := e.service.httpUpstream.DoWithTLS(upstreamReq, proxyURL, e.account.ID, e.account.Concurrency, e.service.tlsFPProfileService.ResolveTLSProfile(e.account))
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return nil, fmt.Errorf("anthropic upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	}
	finalResp, err := bufferAnthropicStreamBody(resp.Body, e.service.cfg)
	if err != nil {
		return nil, err
	}
	return apicompat.AnthropicToResponsesResponse(finalResp), nil
}

func (r *serverToolRuntime) executeTool(ctx context.Context, name, rawArguments string) string {
	args := decodeServerToolArgs(rawArguments)
	normalized := normalizeServerToolName(name)
	result := map[string]any{
		"ok":   false,
		"tool": name,
	}
	var data any
	var err error

	switch normalized {
	case "read", "readfile", "openfile":
		data, err = r.execRead(args)
	case "write", "writefile":
		data, err = r.execWrite(args)
	case "edit", "applypatch":
		data, err = r.execEdit(args)
	case "glob", "listfiles":
		data, err = r.execGlob(args)
	case "list", "ls":
		data, err = r.execList(args)
	case "grep", "searchfiles", "ripgrep":
		data, err = r.execGrep(args)
	case "bash", "shellcommand", "executebash", "execbash", "runcommand", "terminal":
		data, err = r.execBash(ctx, args)
	case "webfetch", "fetch":
		data, err = r.execWebFetch(ctx, args)
	case "websearch":
		data, err = r.execWebSearch(ctx, args)
	case "todowrite", "updateplan":
		data, err = r.execTodoWrite(args)
	case "todoread", "readplan":
		data, err = r.execTodoRead()
	default:
		if normalized == "search" && strings.TrimSpace(firstStringArg(args, "query", "q", "search")) != "" {
			data, err = r.execWebSearch(ctx, args)
		} else {
			err = fmt.Errorf("unsupported server tool %q", name)
		}
	}

	if err != nil {
		result["error"] = err.Error()
	} else {
		result["ok"] = true
		result["result"] = data
	}
	encoded, encodeErr := json.Marshal(result)
	if encodeErr != nil {
		return fmt.Sprintf(`{"ok":false,"tool":%q,"error":"failed to encode tool output"}`, name)
	}
	return string(encoded)
}

func (r *serverToolRuntime) execRead(args map[string]any) (map[string]any, error) {
	target, err := r.resolvePath(firstStringArg(args, "filePath", "file_path", "path", "file"))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	text, truncated := truncateBytes(data, serverAgentMaxReadBytes)
	return map[string]any{
		"path":      target,
		"content":   text,
		"truncated": truncated,
	}, nil
}

func (r *serverToolRuntime) execWrite(args map[string]any) (map[string]any, error) {
	target, err := r.resolvePath(firstStringArg(args, "filePath", "file_path", "path", "file"))
	if err != nil {
		return nil, err
	}
	content := firstStringArg(args, "content", "text", "data")
	if len(content) > serverAgentMaxWriteBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", serverAgentMaxWriteBytes)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"path":    target,
		"bytes":   len(content),
		"written": true,
	}, nil
}

func (r *serverToolRuntime) execEdit(args map[string]any) (map[string]any, error) {
	target, err := r.resolvePath(firstStringArg(args, "filePath", "file_path", "path", "file"))
	if err != nil {
		return nil, err
	}
	oldString := firstStringArg(args, "oldString", "old_string")
	newString := firstStringArg(args, "newString", "new_string")
	replaceAll := firstBoolArg(args, "replaceAll", "replace_all")
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if oldString == "" {
		return nil, errors.New("oldString is required for edit")
	}
	count := strings.Count(content, oldString)
	if count == 0 {
		return nil, errors.New("oldString not found")
	}
	if !replaceAll && count > 1 {
		return nil, fmt.Errorf("oldString matched %d times; set replaceAll to true", count)
	}
	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldString, newString)
	} else {
		updated = strings.Replace(content, oldString, newString, 1)
	}
	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"path":          target,
		"replacements":  ternaryInt(replaceAll, count, 1),
		"bytes_written": len(updated),
	}, nil
}

func (r *serverToolRuntime) execGlob(args map[string]any) (map[string]any, error) {
	pattern := strings.TrimSpace(firstStringArg(args, "pattern", "include", "glob"))
	if pattern == "" {
		pattern = "**/*"
	}
	baseDir := r.rootDir
	if rawPath := strings.TrimSpace(firstStringArg(args, "path")); rawPath != "" {
		var err error
		baseDir, err = r.resolvePath(rawPath)
		if err != nil {
			return nil, err
		}
	}
	matcher, err := compileServerAgentGlob(pattern)
	if err != nil {
		return nil, err
	}
	matches := make([]string, 0, 32)
	err = filepath.WalkDir(baseDir, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if current == baseDir {
			return nil
		}
		rel, err := filepath.Rel(baseDir, current)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if matcher.MatchString(rel) {
			matches = append(matches, current)
			if len(matches) >= serverAgentMaxListCount {
				return io.EOF
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	sort.Strings(matches)
	return map[string]any{
		"base_path": baseDir,
		"pattern":   pattern,
		"matches":   matches,
	}, nil
}

func (r *serverToolRuntime) execList(args map[string]any) (map[string]any, error) {
	target := r.rootDir
	if rawPath := strings.TrimSpace(firstStringArg(args, "path")); rawPath != "" {
		var err error
		target, err = r.resolvePath(rawPath)
		if err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, minInt(len(entries), serverAgentMaxListCount))
	for i, entry := range entries {
		if i >= serverAgentMaxListCount {
			break
		}
		items = append(items, map[string]any{
			"name":      entry.Name(),
			"is_dir":    entry.IsDir(),
			"full_path": filepath.Join(target, entry.Name()),
		})
	}
	return map[string]any{
		"path":    target,
		"entries": items,
	}, nil
}

func (r *serverToolRuntime) execGrep(args map[string]any) (map[string]any, error) {
	pattern := strings.TrimSpace(firstStringArg(args, "pattern", "query", "search"))
	if pattern == "" {
		return nil, errors.New("pattern is required for grep")
	}
	baseDir := r.rootDir
	if rawPath := strings.TrimSpace(firstStringArg(args, "path")); rawPath != "" {
		var err error
		baseDir, err = r.resolvePath(rawPath)
		if err != nil {
			return nil, err
		}
	}
	includePattern := strings.TrimSpace(firstStringArg(args, "include", "glob"))
	includeMatcher, err := compileServerAgentGlob(firstNonEmpty(includePattern, "**/*"))
	if err != nil {
		return nil, err
	}
	var regex *regexp.Regexp
	regex, err = regexp.Compile(pattern)
	useRegex := err == nil
	matches := make([]map[string]any, 0, 32)
	_ = filepath.WalkDir(baseDir, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, current)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !includeMatcher.MatchString(rel) {
			return nil
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return nil
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			matched := false
			if useRegex {
				matched = regex.MatchString(line)
			} else {
				matched = strings.Contains(line, pattern)
			}
			if matched {
				matches = append(matches, map[string]any{
					"path": current,
					"line": lineNo,
					"text": line,
				})
				if len(matches) >= serverAgentMaxMatchCount {
					return io.EOF
				}
			}
		}
		return nil
	})
	return map[string]any{
		"base_path": baseDir,
		"pattern":   pattern,
		"matches":   matches,
		"regex":     useRegex,
	}, nil
}

func (r *serverToolRuntime) execBash(ctx context.Context, args map[string]any) (map[string]any, error) {
	command := strings.TrimSpace(firstStringArg(args, "command", "cmd", "script"))
	if command == "" {
		return nil, errors.New("command is required for bash")
	}
	workdir := r.rootDir
	if rawPath := strings.TrimSpace(firstStringArg(args, "workdir", "cwd", "path")); rawPath != "" {
		var err error
		workdir, err = r.resolvePath(rawPath)
		if err != nil {
			return nil, err
		}
	}
	toolCtx, cancel := context.WithTimeout(ctx, serverAgentToolTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(toolCtx, "powershell", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.CommandContext(toolCtx, "/bin/sh", "-lc", command)
	}
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	text, truncated := truncateBytes(output, serverAgentMaxReadBytes)
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return map[string]any{
			"command":    command,
			"workdir":    workdir,
			"stdout":     text,
			"truncated":  truncated,
			"exit_code":  exitCode,
			"timed_out":  errors.Is(toolCtx.Err(), context.DeadlineExceeded),
			"succeeded":  false,
		}, err
	}
	return map[string]any{
		"command":   command,
		"workdir":   workdir,
		"stdout":    text,
		"truncated": truncated,
		"exit_code": exitCode,
		"succeeded": true,
	}, nil
}

func (r *serverToolRuntime) execWebFetch(ctx context.Context, args map[string]any) (map[string]any, error) {
	rawURL := strings.TrimSpace(firstStringArg(args, "url", "uri"))
	if rawURL == "" {
		return nil, errors.New("url is required for webfetch")
	}
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") && !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	toolCtx, cancel := context.WithTimeout(ctx, serverAgentToolTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(toolCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, serverAgentMaxReadBytes+1))
	if err != nil {
		return nil, err
	}
	text, truncated := truncateBytes(body, serverAgentMaxReadBytes)
	return map[string]any{
		"url":         parsed.String(),
		"status_code": resp.StatusCode,
		"content":     text,
		"truncated":   truncated,
	}, nil
}

func (r *serverToolRuntime) execWebSearch(ctx context.Context, args map[string]any) (map[string]any, error) {
	query := strings.TrimSpace(firstStringArg(args, "query", "q", "search"))
	if query == "" {
		return nil, errors.New("query is required for web_search")
	}
	resp, providerName, err := doWebSearch(ctx, r.account, query)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"query":     query,
		"provider":  providerName,
		"summary":   buildTextSummary(query, resp.Results),
		"results":   resp.Results,
	}, nil
}

func (r *serverToolRuntime) execTodoWrite(args map[string]any) (map[string]any, error) {
	if raw, ok := args["content"].(string); ok && strings.TrimSpace(raw) != "" {
		r.todoState = raw
	} else if raw, ok := args["text"].(string); ok && strings.TrimSpace(raw) != "" {
		r.todoState = raw
	} else {
		encoded, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		r.todoState = string(encoded)
	}
	return map[string]any{
		"stored":  true,
		"content": r.todoState,
	}, nil
}

func (r *serverToolRuntime) execTodoRead() (map[string]any, error) {
	return map[string]any{
		"content": r.todoState,
		"empty":   strings.TrimSpace(r.todoState) == "",
	}, nil
}

func (r *serverToolRuntime) resolvePath(rawPath string) (string, error) {
	rootAbs, err := filepath.Abs(r.rootDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rawPath) == "" {
		return rootAbs, nil
	}
	target := rawPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(rootAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes agent workspace root", rawPath)
	}
	return targetAbs, nil
}

func decodeServerToolArgs(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return map[string]any{
			"input": raw,
		}
	}
	return args
}

func normalizeServerToolName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstStringArg(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := args[key]; ok {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return v
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return v.String()
				}
			}
		}
	}
	return ""
}

func firstBoolArg(args map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := args[key]; ok {
			switch v := value.(type) {
			case bool:
				return v
			case string:
				lower := strings.ToLower(strings.TrimSpace(v))
				return lower == "1" || lower == "true" || lower == "yes"
			}
		}
	}
	return false
}

func compileServerAgentGlob(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" || pattern == "**" {
		pattern = "**/*"
	}
	pattern = filepath.ToSlash(pattern)
	var expr strings.Builder
	expr.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				expr.WriteString(".*")
				i++
			} else {
				expr.WriteString(`[^/]*`)
			}
		case '?':
			expr.WriteString(`[^/]`)
		default:
			expr.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	expr.WriteString("$")
	return regexp.Compile(expr.String())
}

func truncateBytes(data []byte, limit int) (string, bool) {
	if len(data) <= limit {
		return string(data), false
	}
	return string(data[:limit]), true
}

func ternaryInt(condition bool, whenTrue, whenFalse int) int {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
