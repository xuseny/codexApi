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
	if !googleStyle {
		return true
	}
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
		msg = "API key 只能同时在线一台设备，请先在原设备退出，或前往 Key 兑换页面执行踢下线。"
	}

	appErr := infraerrors.FromError(err)
	if appErr == nil || appErr.Metadata == nil {
		return msg
	}
	if onlineDevice := strings.TrimSpace(appErr.Metadata["online_device"]); onlineDevice != "" {
		return fmt.Sprintf("%s 当前占用设备：%s", msg, onlineDevice)
	}
	return msg
}
