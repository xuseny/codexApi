package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func enforceAPIKeySingleDevice(c *gin.Context, apiKeyService *service.APIKeyService, apiKey *service.APIKey, googleStyle bool) bool {
	if c == nil || apiKeyService == nil || apiKey == nil {
		return true
	}

	fingerprint, deviceLabel, clientIP, userAgent := resolveAPIKeyDeviceFingerprint(c)
	if fingerprint == "" {
		return true
	}

	err := apiKeyService.EnforceSingleDeviceLock(c.Request.Context(), apiKey.ID, fingerprint, deviceLabel, clientIP, userAgent)
	if err == nil {
		return true
	}

	reason := infraerrors.Reason(err)
	if strings.TrimSpace(reason) == "" {
		reason = "API_KEY_SINGLE_DEVICE_LIMIT"
	}
	message := buildSingleDeviceLimitMessage(err)
	if googleStyle {
		abortWithGoogleError(c, 409, message)
	} else {
		AbortWithError(c, 409, reason, message)
	}
	return false
}

func resolveAPIKeyDeviceFingerprint(c *gin.Context) (fingerprint string, deviceLabel string, clientIP string, userAgent string) {
	if c == nil {
		return "", "", "", ""
	}

	for _, headerKey := range []string{"x-device-id", "x-client-id", "x-sub2api-device-id"} {
		if headerValue := strings.TrimSpace(c.GetHeader(headerKey)); headerValue != "" {
			return hashDeviceFingerprint(headerKey + ":" + headerValue),
				truncateDeviceLabel(fmt.Sprintf("%s:%s", headerKey, headerValue)),
				ip.GetTrustedClientIP(c),
				strings.TrimSpace(c.GetHeader("User-Agent"))
		}
	}

	clientIP = strings.TrimSpace(ip.GetTrustedClientIP(c))
	userAgent = strings.TrimSpace(c.GetHeader("User-Agent"))
	normalizedUA := service.NormalizeSessionUserAgent(userAgent)
	if clientIP == "" && normalizedUA == "" {
		return "", "", clientIP, userAgent
	}

	labelParts := make([]string, 0, 2)
	if clientIP != "" {
		labelParts = append(labelParts, "IP "+clientIP)
	}
	if normalizedUA != "" {
		labelParts = append(labelParts, normalizedUA)
	}

	return hashDeviceFingerprint("ip:" + clientIP + "|ua:" + normalizedUA),
		truncateDeviceLabel(strings.Join(labelParts, " / ")),
		clientIP,
		userAgent
}

func hashDeviceFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func truncateDeviceLabel(label string) string {
	label = strings.TrimSpace(label)
	if len(label) <= 120 {
		return label
	}
	return label[:120]
}

func buildSingleDeviceLimitMessage(err error) string {
	msg := infraerrors.Message(err)
	if strings.TrimSpace(msg) == "" {
		msg = "API key \u53ea\u80fd\u540c\u65f6\u5728\u7ebf\u4e00\u53f0\u8bbe\u5907\uff0c\u8bf7\u5148\u5728\u539f\u8bbe\u5907\u9000\u51fa\uff0c\u6216\u524d\u5f80 Key \u5151\u6362\u9875\u9762\u6267\u884c\u8e22\u4e0b\u7ebf\u3002"
	}

	appErr := infraerrors.FromError(err)
	if appErr == nil || appErr.Metadata == nil {
		return msg
	}
	if onlineDevice := strings.TrimSpace(appErr.Metadata["online_device"]); onlineDevice != "" {
		return fmt.Sprintf("%s \u5f53\u524d\u5360\u7528\u8bbe\u5907\uff1a%s", msg, onlineDevice)
	}
	return msg
}
