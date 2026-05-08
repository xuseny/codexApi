package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type WindsurfAuthHandler struct {
	windsurfAuthService *service.WindsurfAuthService
	adminService        service.AdminService
}

func NewWindsurfAuthHandler(windsurfAuthService *service.WindsurfAuthService, adminService service.AdminService) *WindsurfAuthHandler {
	return &WindsurfAuthHandler{
		windsurfAuthService: windsurfAuthService,
		adminService:        adminService,
	}
}

type WindsurfLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Token    string `json:"token"`
	APIKey   string `json:"api_key"`
	ProxyID  *int64 `json:"proxy_id"`
}

func (h *WindsurfAuthHandler) Login(c *gin.Context) {
	var req WindsurfLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.windsurfAuthService.Login(c.Request.Context(), service.WindsurfLoginInput{
		Email:    strings.TrimSpace(req.Email),
		Password: req.Password,
		Token:    strings.TrimSpace(req.Token),
		APIKey:   strings.TrimSpace(req.APIKey),
		ProxyID:  req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

type WindsurfRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	ProxyID      *int64 `json:"proxy_id"`
}

func (h *WindsurfAuthHandler) RefreshToken(c *gin.Context) {
	var req WindsurfRefreshTokenRequest
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

	tokenInfo, err := h.windsurfAuthService.RefreshToken(c.Request.Context(), strings.TrimSpace(req.RefreshToken), proxyURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *WindsurfAuthHandler) RefreshAccountToken(c *gin.Context) {
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
	if account.Platform != service.PlatformWindsurf || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Account is not a Windsurf OAuth account")
		return
	}
	if _, err := h.windsurfAuthService.RefreshAccountToken(c.Request.Context(), account); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}
