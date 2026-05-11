package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestWindsurfResponsesToChatCompletions_StringInput(t *testing.T) {
	req := &apicompat.ResponsesRequest{
		Model:        "gpt-5.5",
		Instructions: "Be brief.",
		Input:        json.RawMessage(`"hi"`),
		Stream:       true,
		Reasoning:    &apicompat.ResponsesReasoning{Effort: "high"},
	}

	chatReq, err := windsurfResponsesToChatCompletions(req)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", chatReq.Model)
	require.Equal(t, "Be brief.", chatReq.Instructions)
	require.True(t, chatReq.Stream)
	require.Equal(t, "high", chatReq.ReasoningEffort)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "user", chatReq.Messages[0].Role)
	require.JSONEq(t, `"hi"`, string(chatReq.Messages[0].Content))
}

func TestWindsurfResponsesToChatCompletions_MessageArrayInput(t *testing.T) {
	req := &apicompat.ResponsesRequest{
		Model: "gpt-5.5",
		Input: json.RawMessage(`[
			{"role":"system","content":"You are concise."},
			{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"data:image/png;base64,abc"}]},
			{"type":"function_call","call_id":"call_1","name":"search","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"done"}
		]`),
	}

	chatReq, err := windsurfResponsesToChatCompletions(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 4)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.JSONEq(t, `"You are concise."`, string(chatReq.Messages[0].Content))
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.JSONEq(t, `"hello"`, string(chatReq.Messages[1].Content))
	require.Equal(t, "assistant", chatReq.Messages[2].Role)
	require.Len(t, chatReq.Messages[2].ToolCalls, 1)
	require.Equal(t, "search", chatReq.Messages[2].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", chatReq.Messages[3].Role)
	require.Equal(t, "call_1", chatReq.Messages[3].ToolCallID)
	require.JSONEq(t, `"done"`, string(chatReq.Messages[3].Content))
}

func TestWindsurfResponsesToChatCompletions_ToolsAndChoice(t *testing.T) {
	strict := true
	req := &apicompat.ResponsesRequest{
		Model: "gpt-5.5",
		Input: json.RawMessage(`"read package"`),
		Tools: []apicompat.ResponsesTool{{
			Type:        "function",
			Name:        "Read",
			Description: "Read a file",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"file_path":{"type":"string"}}}`),
			Strict:      &strict,
		}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"Read"}`),
	}

	chatReq, err := windsurfResponsesToChatCompletions(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "Read", chatReq.Tools[0].Function.Name)
	require.True(t, *chatReq.Tools[0].Function.Strict)
	require.JSONEq(t, `{"type":"function","name":"Read"}`, string(chatReq.ToolChoice))

	instruction := windsurfBuildToolInstruction(chatReq.Tools, chatReq.ToolChoice, chatReq.Model)
	require.Contains(t, instruction, `{"function_call":{"name":"<function_name>","arguments":{}}}`)
	require.Contains(t, instruction, `tool_choice requires the function "Read"`)
}

func TestWindsurfApplyBridgeInstructionsKeepsToolProtocolOutOfUserMessage(t *testing.T) {
	req := apicompat.ChatCompletionsRequest{
		Model: "claude-opus-4-7",
		Messages: []apicompat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"分析下项目 你能调用什么工具"`),
		}},
		Tools: []apicompat.ChatTool{{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        "bash",
				Description: "Run shell commands",
				Parameters:  json.RawMessage(`{"type":"object","required":["command","description"],"properties":{"command":{"type":"string"},"description":{"type":"string"}}}`),
			},
		}},
	}

	toolInstruction := windsurfBuildToolInstruction(req.Tools, req.ToolChoice, req.Model)
	windsurfApplyBridgeInstructions(&req, toolInstruction, false)
	messages := buildWindsurfRawMessages(req)

	require.Len(t, messages, 2)
	require.Equal(t, "system", messages[0].Role)
	require.Contains(t, messages[0].Content, "external API client")
	require.NotContains(t, messages[0].Content, "Available functions")
	require.NotContains(t, messages[1].Content, "<tool_call>")
	require.NotContains(t, messages[1].Content, "Tools available this turn")
}

func TestWindsurfApplyBridgeInstructionsIncludesToolProtocolForRawMode(t *testing.T) {
	req := apicompat.ChatCompletionsRequest{
		Model: "gpt-5.5",
		Messages: []apicompat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"分析下项目"`),
		}},
		Tools: []apicompat.ChatTool{{
			Type: "function",
			Function: &apicompat.ChatFunction{
				Name:        "shell_command",
				Description: "Run a shell command",
				Parameters:  json.RawMessage(`{"type":"object","required":["command","description"],"properties":{"command":{"type":"string"},"description":{"type":"string"}}}`),
			},
		}},
	}

	toolInstruction := windsurfBuildToolInstruction(req.Tools, req.ToolChoice, req.Model)
	windsurfApplyBridgeInstructions(&req, toolInstruction, true)
	messages := buildWindsurfRawMessages(req)

	require.Len(t, messages, 2)
	require.Contains(t, messages[0].Content, "Available functions")
	require.Contains(t, messages[0].Content, "shell_command")
	require.NotContains(t, messages[1].Content, "Tools available this turn")
}

func TestWindsurfBuildCascadeConfigUsesCompactToolHint(t *testing.T) {
	fullInstruction := strings.Repeat("tool schema ", 200) + "\nAvailable functions:\n### Read"
	config := string(windsurfBuildCascadeConfig(1, "model_uid", fullInstruction))

	require.Contains(t, config, "Client-side tools are available")
	require.NotContains(t, config, "Available functions")
	require.NotContains(t, config, "tool schema tool schema")
}

func TestWindsurfBuildCascadeConfigIncludesCompactToolNames(t *testing.T) {
	config := string(windsurfBuildCascadeConfig(1, "model_uid", "Available functions:\n### shell_command\nRun shell command"))

	require.Contains(t, config, "shell_command")
	require.Contains(t, config, "real callable tools")
	require.NotContains(t, config, "Windsurf language server")
}

func TestBuildWindsurfRawMessagesToolHistory(t *testing.T) {
	rawArgs := json.RawMessage(`{"file_path":"README.md"}`)
	req := apicompat.ChatCompletionsRequest{
		Messages: []apicompat.ChatMessage{
			{
				Role: "assistant",
				ToolCalls: []apicompat.ChatToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: apicompat.ChatFunctionCall{
						Name:      "Read",
						Arguments: string(rawArgs),
					},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    json.RawMessage(`"file contents"`),
			},
		},
	}

	messages := buildWindsurfRawMessages(req)
	require.Len(t, messages, 2)
	require.Equal(t, "assistant", messages[0].Role)
	require.Contains(t, messages[0].Content, `"function_call"`)
	require.Contains(t, messages[0].Content, `"Read"`)
	require.Equal(t, "user", messages[1].Role)
	require.Contains(t, messages[1].Content, `<tool_result tool_call_id="call_1">`)
	require.Contains(t, messages[1].Content, "file contents")
}

func TestWindsurfParseToolCallsFromText_GPTNative(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name: "Read",
		},
	}}

	calls, cleaned := windsurfParseToolCallsFromText(`{"function_call":{"name":"Read","arguments":{"file_path":"README.md"}}}`, tools)
	require.Empty(t, cleaned)
	require.Len(t, calls, 1)
	require.NotEmpty(t, calls[0].ID)
	require.Equal(t, "Read", calls[0].Name)
	require.JSONEq(t, `{"file_path":"README.md"}`, calls[0].Arguments)
}

func TestWindsurfParseToolCallsAddsRequiredDescription(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name:       "bash",
			Parameters: json.RawMessage(`{"type":"object","required":["command","description"],"properties":{"command":{"type":"string"},"description":{"type":"string"}}}`),
		},
	}}

	calls, cleaned := windsurfParseToolCallsFromText(`{"function_call":{"name":"bash","arguments":{"command":"pwd && ls"}}}`, tools)

	require.Empty(t, cleaned)
	require.Len(t, calls, 1)
	require.Equal(t, "bash", calls[0].Name)
	require.JSONEq(t, `{"command":"pwd && ls","description":"pwd && ls"}`, calls[0].Arguments)
}

func TestWindsurfParseToolCallsDetailedPreservesPrefixText(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name: "Read",
		},
	}}

	calls, cleaned, status := windsurfParseToolCallsDetailedFromText(
		"我先读取文件。\n<tool_call>{\"name\":\"Read\",\"arguments\":{\"file_path\":\"README.md\"}}</tool_call>",
		tools,
	)

	require.Equal(t, "parsed", status)
	require.Equal(t, "我先读取文件。", cleaned)
	require.Len(t, calls, 1)
	require.Equal(t, "Read", calls[0].Name)
}

func TestWindsurfParseToolCallsDetailedReportsUnparsedMarker(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name: "Read",
		},
	}}

	calls, cleaned, status := windsurfParseToolCallsDetailedFromText(
		`{"function_call":{"name":"Unknown","arguments":{}}}`,
		tools,
	)

	require.Equal(t, "unparsed_tool_marker", status)
	require.Empty(t, calls)
	require.Contains(t, cleaned, "function_call")
}

func TestWindsurfSynthesizeToolCallFromWebRefusal(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name:        "websearch",
			Description: "Search the web",
			Parameters:  json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"}}}`),
		},
	}}
	call, ok := windsurfSynthesizeToolCallFromRefusal(
		"我无法访问外部网站，也无法实时获取优惠码信息。",
		[]windsurfRawMessage{{Role: "user", Content: "https://cloud.speedidc.cn 帮我找下这个网站的优惠码"}},
		tools,
	)

	require.True(t, ok)
	require.Equal(t, "websearch", call.Name)
	require.JSONEq(t, `{"query":"https://cloud.speedidc.cn 帮我找下这个网站的优惠码"}`, call.Arguments)
}

func TestWindsurfSynthesizeToolCallFromShellRefusalAddsDescription(t *testing.T) {
	tools := []apicompat.ChatTool{{
		Type: "function",
		Function: &apicompat.ChatFunction{
			Name:       "bash",
			Parameters: json.RawMessage(`{"type":"object","required":["command","description"],"properties":{"command":{"type":"string"},"description":{"type":"string"}}}`),
		},
	}}
	call, ok := windsurfSynthesizeToolCallFromRefusal(
		"当前这个会话里，系统实际没有给我开放 shell_command 工具。",
		[]windsurfRawMessage{{Role: "user", Content: "分析下项目 你能调用什么工具"}},
		tools,
	)

	require.True(t, ok)
	require.Equal(t, "bash", call.Name)
	require.JSONEq(t, `{"command":"pwd && find . -maxdepth 2 -type f | sort | head -200","description":"Inspect the current workspace for the user's request."}`, call.Arguments)
}

func TestWindsurfResponsesToolsToChatToolsKeepsNamedNonFunctionTools(t *testing.T) {
	tools := windsurfResponsesToolsToChatTools([]apicompat.ResponsesTool{{
		Type:        "web_search",
		Name:        "websearch",
		Description: "Search the web",
		Parameters:  json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"}}}`),
	}})

	require.Len(t, tools, 1)
	require.Equal(t, "function", tools[0].Type)
	require.Equal(t, "websearch", tools[0].Function.Name)
}

func TestWindsurfToolBridgeEmptyFallbackText(t *testing.T) {
	require.Equal(
		t,
		"The upstream model returned no valid Read tool call. Please retry this request.",
		windsurfToolBridgeEmptyFallbackText(json.RawMessage(`{"type":"function","name":"Read"}`)),
	)
	require.Equal(
		t,
		"The upstream model returned an empty response. Please retry this request.",
		windsurfToolBridgeEmptyFallbackText(nil),
	)
}

func TestWindsurfBuildResponsesOutputsReasoningAndToolCall(t *testing.T) {
	output := windsurfBuildResponsesOutputs("rs_1", "I considered the tools.", "msg_1", "", []windsurfParsedToolCall{{
		ID:        "call_1",
		Name:      "Read",
		Arguments: `{"file_path":"README.md"}`,
	}})

	require.Len(t, output, 2)
	require.Equal(t, "reasoning", output[0].Type)
	require.Equal(t, "I considered the tools.", output[0].Summary[0].Text)
	require.Equal(t, "function_call", output[1].Type)
	require.Equal(t, "call_1", output[1].CallID)
	require.Equal(t, "Read", output[1].Name)
	require.JSONEq(t, `{"file_path":"README.md"}`, output[1].Arguments)
}

func TestWindsurfBuildAnthropicBlocksReasoningAndToolUse(t *testing.T) {
	blocks := windsurfBuildAnthropicBlocks("I considered the tools.", "", []windsurfParsedToolCall{{
		ID:        "call_1",
		Name:      "Read",
		Arguments: `{"file_path":"README.md"}`,
	}})

	require.Len(t, blocks, 2)
	require.Equal(t, "thinking", blocks[0].Type)
	require.Equal(t, "I considered the tools.", blocks[0].Thinking)
	require.Equal(t, "tool_use", blocks[1].Type)
	require.Equal(t, "call_1", blocks[1].ID)
	require.Equal(t, "Read", blocks[1].Name)
	require.JSONEq(t, `{"file_path":"README.md"}`, string(blocks[1].Input))

	resp := buildWindsurfAnthropicResponseWithBlocks("msg_test", "claude-opus-4-7", blocks, "tool_use", OpenAIUsage{
		InputTokens:  3,
		OutputTokens: 2,
	})
	require.Equal(t, "tool_use", resp.StopReason)
	require.Len(t, resp.Content, 2)
	require.Equal(t, "tool_use", resp.Content[1].Type)
}

func TestWindsurfResponsesContentPartEventsSerialize(t *testing.T) {
	itemSSE, err := apicompat.ResponsesEventToSSE(apicompat.ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &apicompat.ResponsesOutput{
			Type: "message",
			ID:   "msg_test",
		},
	})
	require.NoError(t, err)
	require.Contains(t, itemSSE, "event: response.output_item.added")
	require.Contains(t, itemSSE, `"output_index":0`)

	added := windsurfBuildResponsesContentPartEvent("response.content_part.added", 2, "msg_test", "")
	require.Equal(t, "response.content_part.added", added.Type)
	require.Equal(t, 2, added.SequenceNumber)
	require.Equal(t, "msg_test", added.ItemID)
	require.NotNil(t, added.Part)
	require.Equal(t, "output_text", added.Part.Type)

	addedSSE, err := apicompat.ResponsesEventToSSE(added)
	require.NoError(t, err)
	require.Contains(t, addedSSE, "event: response.content_part.added")
	require.Contains(t, addedSSE, `"item_id":"msg_test"`)
	require.Contains(t, addedSSE, `"output_index":0`)
	require.Contains(t, addedSSE, `"content_index":0`)
	require.Contains(t, addedSSE, `"type":"output_text"`)

	deltaSSE, err := apicompat.ResponsesEventToSSE(apicompat.ResponsesStreamEvent{
		Type:         "response.output_text.delta",
		OutputIndex:  0,
		ContentIndex: 0,
		ItemID:       "msg_test",
		Delta:        "he",
	})
	require.NoError(t, err)
	require.Contains(t, deltaSSE, `"output_index":0`)
	require.Contains(t, deltaSSE, `"content_index":0`)

	done := windsurfBuildResponsesContentPartEvent("response.content_part.done", 5, "msg_test", "hello")
	require.Equal(t, "hello", done.Part.Text)
	doneSSE, err := apicompat.ResponsesEventToSSE(done)
	require.NoError(t, err)
	require.Contains(t, doneSSE, "event: response.content_part.done")
	require.Contains(t, doneSSE, `"text":"hello"`)
}

func TestResolveWindsurfModelSupportsClaudeOpus47Alias(t *testing.T) {
	info, upstream, err := resolveWindsurfModel("claude-opus-4-7")
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-7-medium", upstream)
	require.Equal(t, "claude-opus-4-7-medium", info.Name)
	require.True(t, IsWindsurfBuiltinModel("claude-opus-4-7"))
	require.True(t, IsWindsurfBuiltinModel("anthropic/claude-sonnet-4-6"))
}

func TestBuildWindsurfAnthropicResponse(t *testing.T) {
	resp := buildWindsurfAnthropicResponse("msg_test", "claude-opus-4-7", "hello", OpenAIUsage{
		InputTokens:              3,
		OutputTokens:             2,
		CacheCreationInputTokens: 1,
		CacheReadInputTokens:     4,
	})

	require.Equal(t, "msg_test", resp.ID)
	require.Equal(t, "message", resp.Type)
	require.Equal(t, "assistant", resp.Role)
	require.Equal(t, "claude-opus-4-7", resp.Model)
	require.Equal(t, "end_turn", resp.StopReason)
	require.Len(t, resp.Content, 1)
	require.Equal(t, "hello", resp.Content[0].Text)
	require.Equal(t, 3, resp.Usage.InputTokens)
	require.Equal(t, 2, resp.Usage.OutputTokens)
	require.Equal(t, 1, resp.Usage.CacheCreationInputTokens)
	require.Equal(t, 4, resp.Usage.CacheReadInputTokens)
}
