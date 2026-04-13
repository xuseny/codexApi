package service

import (
	"net/http"
	"strings"
)

// ShouldPreserveUpstreamStatusByDefault returns whether the gateway should
// preserve the upstream status instead of remapping it to a generic gateway
// status. This helps distinguish provider-side rate limiting/overload from
// gateway-generated limits.
func ShouldPreserveUpstreamStatusByDefault(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, 529:
		return true
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// DefaultUpstreamPassthroughMessage extracts a sanitized upstream message for
// the statuses that are preserved by default.
func DefaultUpstreamPassthroughMessage(statusCode int, responseBody []byte) (string, bool) {
	if !ShouldPreserveUpstreamStatusByDefault(statusCode) {
		return "", false
	}

	msg := strings.TrimSpace(ExtractUpstreamErrorMessage(responseBody))
	if msg == "" {
		return "", false
	}
	return sanitizeUpstreamErrorMessage(msg), true
}
