package middleware

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func acquireAPIKeyConcurrency(c *gin.Context, apiKeyService *service.APIKeyService, apiKey *service.APIKey, googleStyle bool) (func(), bool) {
	if c == nil || apiKeyService == nil || apiKey == nil {
		return func() {}, true
	}

	release, err := apiKeyService.AcquireConcurrencySlot(c.Request.Context(), apiKey)
	if err == nil {
		return release, true
	}

	if errors.Is(err, service.ErrAPIKeyConcurrencyExceeded) {
		if googleStyle {
			abortWithGoogleError(c, 429, "API key concurrency limit exceeded")
		} else {
			AbortWithError(c, 429, "API_KEY_CONCURRENCY_EXCEEDED", "API key concurrency limit exceeded")
		}
		return nil, false
	}

	if googleStyle {
		abortWithGoogleError(c, 500, "Failed to acquire API key concurrency slot")
	} else {
		AbortWithError(c, 500, "INTERNAL_ERROR", "Failed to acquire API key concurrency slot")
	}
	return nil, false
}
