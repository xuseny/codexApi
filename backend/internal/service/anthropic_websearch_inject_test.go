package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestMaybeInjectAnthropicWebSearchToolInjectsForCouponURLQuery(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"https://cloud.speedidc.cn 帮我找下这个网站的优惠码"}],
		"stream":false
	}`)

	out, changed, err := MaybeInjectAnthropicWebSearchTool(body)
	require.NoError(t, err)
	require.True(t, changed)

	var req apicompat.AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &req))
	require.Len(t, req.Tools, 1)
	require.Equal(t, "web_search_20250305", req.Tools[0].Type)
	require.Equal(t, "web_search", req.Tools[0].Name)
	var systemText string
	require.NoError(t, json.Unmarshal(req.System, &systemText))
	require.Contains(t, systemText, anthropicWebSearchMarker)
}

func TestMaybeInjectAnthropicWebSearchToolSkipsWhenExistingToolPresent(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"https://cloud.speedidc.cn 帮我找下这个网站的优惠码"}],
		"tools":[{"type":"web_search_20250305","name":"web_search"}],
		"stream":false
	}`)

	out, changed, err := MaybeInjectAnthropicWebSearchTool(body)
	require.NoError(t, err)
	require.True(t, changed, "system hint should still be added once")

	var req apicompat.AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &req))
	require.Len(t, req.Tools, 1)
	var systemText string
	require.NoError(t, json.Unmarshal(req.System, &systemText))
	require.Contains(t, systemText, anthropicWebSearchMarker)
}

func TestMaybeInjectAnthropicWebSearchToolSkipsOnExplicitToolChoice(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"https://cloud.speedidc.cn 帮我找下这个网站的优惠码"}],
		"tool_choice":{"type":"none"},
		"stream":false
	}`)

	out, changed, err := MaybeInjectAnthropicWebSearchTool(body)
	require.NoError(t, err)
	require.False(t, changed)
	require.JSONEq(t, string(body), string(out))
}
