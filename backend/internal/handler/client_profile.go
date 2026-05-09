package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	clientProfileContextKey = "sub2api_client_profile"
	wireProtocolContextKey  = "sub2api_wire_protocol"
)

func bindClientProfile(c *gin.Context, fallbackWireProtocol string) service.ClientProfile {
	profile := service.DetectClientProfile(nil, fallbackWireProtocol)
	if c != nil {
		profile = service.DetectClientProfile(c.Request, fallbackWireProtocol)
		c.Set(clientProfileContextKey, profile.ID)
		c.Set(wireProtocolContextKey, profile.WireProtocol)
		c.Header("X-Sub2API-Client-Profile", profile.ID)
		c.Header("X-Sub2API-Wire-Protocol", profile.WireProtocol)
	}
	return profile
}
