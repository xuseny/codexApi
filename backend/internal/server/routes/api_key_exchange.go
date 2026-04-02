package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// RegisterAPIKeyExchangeRoutes registers public api key exchange routes.
func RegisterAPIKeyExchangeRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	keyExchange := v1.Group("/key-exchange")
	{
		keyExchange.POST("/resolve", h.Setting.ResolveAPIKeyExchange)
	}
}
