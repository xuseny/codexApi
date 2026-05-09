package service

import (
	"encoding/json"
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
