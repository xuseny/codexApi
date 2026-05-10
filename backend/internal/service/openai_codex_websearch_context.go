package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
)

const (
	codexWebSearchResultsMarker = "<sub2api-codex-web-search-results>"
	codexWebSearchContextTTL    = 15 * time.Second
)

var codexURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func maybeInjectCodexWebSearchContext(ctx context.Context, reqBody map[string]any, account *Account) bool {
	if len(reqBody) == 0 || !hasOpenAIWebSearchTool(reqBody) {
		return false
	}
	if existing, _ := reqBody["instructions"].(string); strings.Contains(existing, codexWebSearchResultsMarker) {
		return false
	}

	userText := latestCodexUserText(reqBody)
	if !codexShouldPreloadWebSearch(userText) {
		return false
	}

	mgr := getWebSearchManager()
	if mgr == nil {
		slog.Warn("codex web_search preload skipped: websearch manager is not configured")
		return false
	}

	query := codexBuildWebSearchQuery(userText)
	if query == "" {
		return false
	}

	searchCtx, cancel := context.WithTimeout(ctx, codexWebSearchContextTTL)
	defer cancel()

	resp, providerName, err := doWebSearch(searchCtx, account, query)
	if err != nil {
		slog.Warn("codex web_search preload failed", "query", query, "error", err)
		return false
	}

	injectCodexWebSearchResults(reqBody, query, providerName, resp.Results)
	return true
}

func latestCodexUserText(reqBody map[string]any) string {
	if text, ok := reqBody["input"].(string); ok {
		return strings.TrimSpace(text)
	}
	if text := latestUserTextFromItems(reqBody["input"]); text != "" {
		return text
	}
	if text := latestUserTextFromItems(reqBody["messages"]); text != "" {
		return text
	}
	return ""
}

func latestUserTextFromItems(raw any) string {
	items, ok := raw.([]any)
	if !ok {
		return ""
	}
	for i := len(items) - 1; i >= 0; i-- {
		item, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		if role := strings.TrimSpace(firstNonEmptyString(item["role"])); role != "user" {
			continue
		}
		if text := strings.TrimSpace(extractTextFromContent(item["content"])); text != "" {
			return text
		}
	}
	return ""
}

func codexShouldPreloadWebSearch(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	triggers := []string{
		"http://", "https://", "www.", "site:",
		"优惠码", "优惠券", "折扣", "促销", "活动", "价格", "最新", "实时", "今天", "现在", "官网", "网页", "网站", "搜索", "查询", "查找", "找下",
		"coupon", "promo", "promotion", "discount", "latest", "current", "real-time", "website", "web page", "search",
	}
	for _, trigger := range triggers {
		if strings.Contains(normalized, trigger) {
			return true
		}
	}
	return false
}

func codexBuildWebSearchQuery(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	host := firstPublicHostFromText(text)
	if host != "" && codexLooksLikeCouponQuery(text) {
		return host + " 优惠码 折扣 促销 活动 coupon promo discount"
	}
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func firstPublicHostFromText(text string) string {
	match := codexURLPattern.FindString(text)
	if match == "" {
		return ""
	}
	parsed, err := url.Parse(match)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
}

func codexLooksLikeCouponQuery(text string) bool {
	normalized := strings.ToLower(text)
	for _, token := range []string{"优惠码", "优惠券", "折扣", "促销", "coupon", "promo", "discount", "promotion"} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func injectCodexWebSearchResults(reqBody map[string]any, query, provider string, results []websearch.SearchResult) {
	var b strings.Builder
	b.WriteString(codexWebSearchResultsMarker)
	b.WriteString("\nThe gateway already performed web search for this request. Use these results before answering. Do not say you cannot browse, search, access URLs, or get real-time information.\n")
	fmt.Fprintf(&b, "Query: %s\n", query)
	if provider != "" {
		fmt.Fprintf(&b, "Provider: %s\n", provider)
	}
	if len(results) == 0 {
		b.WriteString("Results: none.\n")
	} else {
		b.WriteString("Results:\n")
		for i, r := range results {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "%d. %s\nURL: %s\nSnippet: %s\n", i+1, strings.TrimSpace(r.Title), strings.TrimSpace(r.URL), truncateString(strings.TrimSpace(r.Snippet), 700))
			if r.PageAge != "" {
				fmt.Fprintf(&b, "Page age: %s\n", strings.TrimSpace(r.PageAge))
			}
		}
	}
	b.WriteString("If these results do not contain a verified coupon code, say that no verified coupon code was found in the search results and suggest official channels.\n")
	b.WriteString("</sub2api-codex-web-search-results>")

	existing, _ := reqBody["instructions"].(string)
	existing = strings.TrimRight(existing, " \t\r\n")
	if strings.TrimSpace(existing) == "" {
		reqBody["instructions"] = b.String()
		return
	}
	reqBody["instructions"] = existing + "\n\n" + b.String()
}
