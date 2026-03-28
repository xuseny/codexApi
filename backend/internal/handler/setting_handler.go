package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler handles public settings and public utility endpoints.
type SettingHandler struct {
	settingService        *service.SettingService
	apiKeyExchangeService *service.APIKeyExchangeService
	version               string
}

func NewSettingHandler(settingService *service.SettingService, apiKeyExchangeService *service.APIKeyExchangeService, version string) *SettingHandler {
	return &SettingHandler{
		settingService:        settingService,
		apiKeyExchangeService: apiKeyExchangeService,
		version:               version,
	}
}

type ResolveAPIKeyExchangeRequest struct {
	Code     string `json:"code" binding:"required"`
	Timezone string `json:"timezone"`
}

// GetPublicSettings handles GET /api/v1/settings/public.
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.PublicSettings{
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings.PromoCodeEnabled,
		PasswordResetEnabled:             settings.PasswordResetEnabled,
		InvitationCodeEnabled:            settings.InvitationCodeEnabled,
		TotpEnabled:                      settings.TotpEnabled,
		TurnstileEnabled:                 settings.TurnstileEnabled,
		TurnstileSiteKey:                 settings.TurnstileSiteKey,
		SiteName:                         settings.SiteName,
		SiteLogo:                         settings.SiteLogo,
		SiteSubtitle:                     settings.SiteSubtitle,
		APIBaseURL:                       settings.APIBaseURL,
		ContactInfo:                      settings.ContactInfo,
		DocURL:                           settings.DocURL,
		HomeContent:                      settings.HomeContent,
		HideCcsImportButton:              settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:      settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:          settings.PurchaseSubscriptionURL,
		CustomMenuItems:                  dto.ParseUserVisibleMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                  dto.ParseCustomEndpoints(settings.CustomEndpoints),
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		SoraClientEnabled:                settings.SoraClientEnabled,
		BackendModeEnabled:               settings.BackendModeEnabled,
		Version:                          h.version,
	})
}

// ResolveAPIKeyExchange handles POST /api/v1/key-exchange/resolve.
func (h *SettingHandler) ResolveAPIKeyExchange(c *gin.Context) {
	if h.apiKeyExchangeService == nil {
		response.InternalError(c, "api key exchange service not configured")
		return
	}

	var req ResolveAPIKeyExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.apiKeyExchangeService.Resolve(c.Request.Context(), req.Code, c.ClientIP(), req.Timezone)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.APIKeyExchangeResolveResponseFromService(result))
}
