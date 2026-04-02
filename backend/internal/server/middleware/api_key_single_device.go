package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func enforceAPIKeySingleDevice(_ *gin.Context, _ *service.APIKeyService, _ *service.APIKey, _ bool) bool {
	return true
}
