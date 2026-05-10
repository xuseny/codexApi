package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeServerAgentExecutor struct {
	t     *testing.T
	calls int
}

func (e *fakeServerAgentExecutor) ExecuteTurn(_ context.Context, req *apicompat.ResponsesRequest) (*apicompat.ResponsesResponse, error) {
	e.calls++
	switch e.calls {
	case 1:
		return &apicompat.ResponsesResponse{
			ID:     "resp_1",
			Object: "response",
			Model:  req.Model,
			Status: "completed",
			Output: []apicompat.ResponsesOutput{{
				Type:      "function_call",
				ID:        "fc_1",
				CallID:    "call_read_1",
				Name:      "read",
				Arguments: `{"filePath":"note.txt"}`,
				Status:    "completed",
			}},
			Usage: &apicompat.ResponsesUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5},
		}, nil
	case 2:
		items, err := responsesRequestInputToItems(req.Input)
		require.NoError(e.t, err)
		require.Len(e.t, items, 3)
		require.Equal(e.t, "function_call", items[1].Type)
		require.Equal(e.t, "function_call_output", items[2].Type)
		require.Equal(e.t, "call_read_1", items[2].CallID)
		require.Contains(e.t, items[2].Output, `"hello world"`)
		return &apicompat.ResponsesResponse{
			ID:     "resp_2",
			Object: "response",
			Model:  req.Model,
			Status: "completed",
			Output: []apicompat.ResponsesOutput{{
				Type:   "message",
				ID:     "msg_1",
				Role:   "assistant",
				Status: "completed",
				Content: []apicompat.ResponsesContentPart{{
					Type: "output_text",
					Text: "done",
				}},
			}},
			Usage: &apicompat.ResponsesUsage{InputTokens: 7, OutputTokens: 4, TotalTokens: 11},
		}, nil
	default:
		e.t.Fatalf("unexpected extra turn %d", e.calls)
		return nil, nil
	}
}

func TestResolveServerAgentExecutionAutoDisablesForCodex(t *testing.T) {
	t.Setenv("SUB2API_AGENT_WORKDIR", "")
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.Header.Set("User-Agent", "codex-cli/1.0")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	exec := resolveServerAgentExecution(c, WireProtocolOpenAIResponses, true)
	require.False(t, exec.Enabled)
	require.Equal(t, ClientProfileCodex, exec.Profile.ID)
}

func TestResolveServerAgentExecutionAutoEnablesForPlainResponsesClient(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	exec := resolveServerAgentExecution(c, WireProtocolOpenAIResponses, true)
	require.True(t, exec.Enabled)
	require.Equal(t, ClientProfileOpenAIResponses, exec.Profile.ID)
}

func TestRunServerAgentLoopExecutesToolAndAggregatesUsage(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "note.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello world"), 0o644))

	inputJSON, err := json.Marshal([]apicompat.ResponsesInputItem{{
		Role:    "user",
		Content: json.RawMessage(`"read the file"`),
	}})
	require.NoError(t, err)

	req := &apicompat.ResponsesRequest{
		Model: "gpt-5.5",
		Input: inputJSON,
		Tools: []apicompat.ResponsesTool{{
			Type: "function",
			Name: "read",
		}},
	}

	result, err := runServerAgentLoop(
		context.Background(),
		req,
		&fakeServerAgentExecutor{t: t},
		newServerToolRuntime(nil, tempDir),
		4,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, result.Turns)
	require.NotNil(t, result.Response)
	require.Equal(t, "resp_2", result.Response.ID)
	require.Len(t, result.Response.Output, 1)
	require.Equal(t, "message", result.Response.Output[0].Type)
	require.Equal(t, "done", result.Response.Output[0].Content[0].Text)
	require.NotNil(t, result.Response.Usage)
	require.Equal(t, 10, result.Response.Usage.InputTokens)
	require.Equal(t, 6, result.Response.Usage.OutputTokens)
	require.Equal(t, 16, result.Response.Usage.TotalTokens)
}
