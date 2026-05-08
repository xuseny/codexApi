package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type KiroOAuthHandler struct {
	kiroOAuthService *service.KiroOAuthService
	adminService     service.AdminService
}

func NewKiroOAuthHandler(kiroOAuthService *service.KiroOAuthService, adminService service.AdminService) *KiroOAuthHandler {
	return &KiroOAuthHandler{
		kiroOAuthService: kiroOAuthService,
		adminService:     adminService,
	}
}

type KiroStartDeviceAuthRequest struct {
	StartURL string `json:"start_url"`
	Region   string `json:"region"`
	ProxyID  *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) StartDeviceAuth(c *gin.Context) {
	var req KiroStartDeviceAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = KiroStartDeviceAuthRequest{}
	}

	result, err := h.kiroOAuthService.StartDeviceAuth(c.Request.Context(), service.KiroDeviceAuthInput{
		StartURL: req.StartURL,
		Region:   req.Region,
		ProxyID:  req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type KiroExchangeDeviceCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

func (h *KiroOAuthHandler) ExchangeDeviceCode(c *gin.Context) {
	var req KiroExchangeDeviceCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.kiroOAuthService.ExchangeDeviceCode(c.Request.Context(), req.SessionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

type KiroRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Region       string `json:"region"`
	AuthMethod   string `json:"auth_method"`
	StartURL     string `json:"start_url"`
	ProfileARN   string `json:"profile_arn"`
	ProxyID      *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) RefreshToken(c *gin.Context) {
	var req KiroRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	proxyURL := ""
	if req.ProxyID != nil && h.adminService != nil {
		proxy, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	tokenInfo, err := h.kiroOAuthService.RefreshToken(c.Request.Context(), service.KiroTokenInfo{
		RefreshToken: strings.TrimSpace(req.RefreshToken),
		ClientID:     strings.TrimSpace(req.ClientID),
		ClientSecret: strings.TrimSpace(req.ClientSecret),
		Region:       strings.TrimSpace(req.Region),
		AuthMethod:   strings.TrimSpace(req.AuthMethod),
		StartURL:     strings.TrimSpace(req.StartURL),
		ProfileARN:   strings.TrimSpace(req.ProfileARN),
	}, proxyURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *KiroOAuthHandler) RefreshAccountToken(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.Platform != service.PlatformKiro || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Account is not a Kiro OAuth account")
		return
	}
	if _, err := h.kiroOAuthService.RefreshAccountToken(c.Request.Context(), account); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}
