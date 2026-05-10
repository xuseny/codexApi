package service

import (
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

const (
	anthropicWebSearchMarker = "<sub2api-anthropic-web-search>"
)

const anthropicWebSearchInstruction = anthropicWebSearchMarker + "\nUse the web_search tool for current, latest, real-time, website, page, source-cited, promo, coupon, discount, or pricing questions when it helps answer accurately. Do not say that browsing or web search is unavailable when the web_search tool is present.\n</sub2api-anthropic-web-search>"

func MaybeInjectAnthropicWebSearchTool(body []byte) ([]byte, bool, error) {
	var req apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return body, false, err
	}
	if !shouldAutoInjectAnthropicWebSearch(&req) {
		return body, false, nil
	}

	changed := false
	if !anthropicHasWebSearchTool(req.Tools) {
		req.Tools = append(req.Tools, apicompat.AnthropicTool{
			Type: "web_search_20250305",
			Name: "web_search",
		})
		changed = true
	}
	if system, systemChanged, err := appendAnthropicSystemText(req.System, anthropicWebSearchInstruction); err == nil && systemChanged {
		req.System = system
		changed = true
	}
	if !changed {
		return body, false, nil
	}
	out, err := json.Marshal(&req)
	if err != nil {
		return body, false, err
	}
	return out, true, nil
}

func shouldAutoInjectAnthropicWebSearch(req *apicompat.AnthropicRequest) bool {
	if req == nil {
		return false
	}
	if anthropicToolChoiceExplicit(req.ToolChoice) {
		return false
	}
	text := latestAnthropicUserText(req.Messages)
	if text == "" {
		return false
	}
	return anthropicLooksLikeWebSearchQuery(text)
}

func anthropicToolChoiceExplicit(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	typ, _ := m["type"].(string)
	typ = strings.TrimSpace(strings.ToLower(typ))
	switch typ {
	case "", "auto":
		return false
	default:
		return true
	}
}

func anthropicHasWebSearchTool(tools []apicompat.AnthropicTool) bool {
	for _, tool := range tools {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(tool.Type)), "web_search") {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(tool.Name), "web_search") {
			return true
		}
	}
	return false
}

func latestAnthropicUserText(messages []apicompat.AnthropicMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(strings.ToLower(messages[i].Role)) != "user" {
			continue
		}
		if text := strings.TrimSpace(extractAnthropicMessageText(messages[i].Content)); text != "" {
			return text
		}
	}
	return ""
}

func extractAnthropicMessageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func anthropicLooksLikeWebSearchQuery(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	if codexURLPattern.FindString(normalized) != "" {
		return true
	}
	triggers := []string{
		"优惠码", "优惠券", "折扣", "促销", "活动", "价格", "官网", "网站", "网页", "最新", "实时", "今天", "现在", "搜索", "查询", "查找", "找下",
		"coupon", "promo", "promotion", "discount", "price", "pricing", "latest", "current", "real-time", "website", "web page", "official site", "search",
	}
	for _, trigger := range triggers {
		if strings.Contains(normalized, trigger) {
			return true
		}
	}
	return false
}

func appendAnthropicSystemText(raw json.RawMessage, text string) (json.RawMessage, bool, error) {
	if strings.TrimSpace(text) == "" {
		return raw, false, nil
	}
	if strings.Contains(string(raw), anthropicWebSearchMarker) {
		return raw, false, nil
	}

	var asString string
	if len(raw) == 0 || string(raw) == "null" {
		next, err := json.Marshal(text)
		return next, true, err
	}
	if err := json.Unmarshal(raw, &asString); err == nil {
		if strings.TrimSpace(asString) == "" {
			next, err := json.Marshal(text)
			return next, true, err
		}
		next, err := json.Marshal(strings.TrimRight(asString, " \t\r\n") + "\n\n" + text)
		return next, true, err
	}

	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return raw, false, err
	}
	blocks = append(blocks, apicompat.AnthropicContentBlock{
		Type: "text",
		Text: text,
	})
	next, err := json.Marshal(blocks)
	if err != nil {
		return raw, false, err
	}
	return next, true, nil
}
