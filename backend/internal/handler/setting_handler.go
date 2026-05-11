package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService        *service.SettingService
	apiKeyExchangeService *service.APIKeyExchangeService
	usageService          *service.UsageService
	version               string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, args ...any) *SettingHandler {
	h := &SettingHandler{settingService: settingService}
	switch len(args) {
	case 1:
		if version, ok := args[0].(string); ok {
			h.version = version
		}
	case 3:
		if svc, ok := args[0].(*service.APIKeyExchangeService); ok {
			h.apiKeyExchangeService = svc
		}
		if svc, ok := args[1].(*service.UsageService); ok {
			h.usageService = svc
		}
		if version, ok := args[2].(string); ok {
			h.version = version
		}
	}
	return h
}

type ResolveAPIKeyExchangeRequest struct {
	Code     string `json:"code" binding:"required"`
	Timezone string `json:"timezone"`
}

type RedeemAPIKeyExchangeQuotaRequest struct {
	ExchangeCode string `json:"exchange_code" binding:"required"`
	RedeemCode   string `json:"redeem_code" binding:"required"`
	Timezone     string `json:"timezone"`
}

type ListAPIKeyExchangeUsageLogsRequest struct {
	Code     string `json:"code" binding:"required"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.PublicSettings{
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		ForceEmailOnThirdPartySignup:     settings.ForceEmailOnThirdPartySignup,
		RegistrationEmailSuffixWhitelist: settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings.PromoCodeEnabled,
		PasswordResetEnabled:             settings.PasswordResetEnabled,
		InvitationCodeEnabled:            settings.InvitationCodeEnabled,
		TotpEnabled:                      settings.TotpEnabled,
		LoginAgreementEnabled:            settings.LoginAgreementEnabled,
		LoginAgreementMode:               settings.LoginAgreementMode,
		LoginAgreementUpdatedAt:          settings.LoginAgreementUpdatedAt,
		LoginAgreementRevision:           settings.LoginAgreementRevision,
		LoginAgreementDocuments:          publicLoginAgreementDocumentsToDTO(settings.LoginAgreementDocuments),
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
		TableDefaultPageSize:             settings.TableDefaultPageSize,
		TablePageSizeOptions:             settings.TablePageSizeOptions,
		CustomMenuItems:                  dto.ParseUserVisibleMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                  dto.ParseCustomEndpoints(settings.CustomEndpoints),
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		WeChatOAuthEnabled:               settings.WeChatOAuthEnabled,
		WeChatOAuthOpenEnabled:           settings.WeChatOAuthOpenEnabled,
		WeChatOAuthMPEnabled:             settings.WeChatOAuthMPEnabled,
		WeChatOAuthMobileEnabled:         settings.WeChatOAuthMobileEnabled,
		OIDCOAuthEnabled:                 settings.OIDCOAuthEnabled,
		OIDCOAuthProviderName:            settings.OIDCOAuthProviderName,
		GitHubOAuthEnabled:               settings.GitHubOAuthEnabled,
		GoogleOAuthEnabled:               settings.GoogleOAuthEnabled,
		BackendModeEnabled:               settings.BackendModeEnabled,
		PaymentEnabled:                   settings.PaymentEnabled,
		Version:                          h.version,
		BalanceLowNotifyEnabled:          settings.BalanceLowNotifyEnabled,
		AccountQuotaNotifyEnabled:        settings.AccountQuotaNotifyEnabled,
		BalanceLowNotifyThreshold:        settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:      settings.BalanceLowNotifyRechargeURL,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled: settings.AvailableChannelsEnabled,

		AffiliateEnabled: settings.AffiliateEnabled,

		RiskControlEnabled: settings.RiskControlEnabled,
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

// RedeemAPIKeyExchangeQuota handles POST /api/v1/key-exchange/redeem-quota.
func (h *SettingHandler) RedeemAPIKeyExchangeQuota(c *gin.Context) {
	if h.apiKeyExchangeService == nil {
		response.InternalError(c, "api key exchange service not configured")
		return
	}

	var req RedeemAPIKeyExchangeQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.apiKeyExchangeService.RedeemQuota(c.Request.Context(), req.ExchangeCode, req.RedeemCode, req.Timezone)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.APIKeyExchangeQuotaRedeemResponseFromService(result))
}

// ListAPIKeyExchangeUsageLogs handles POST /api/v1/key-exchange/usage-logs.
func (h *SettingHandler) ListAPIKeyExchangeUsageLogs(c *gin.Context) {
	if h.apiKeyExchangeService == nil || h.usageService == nil {
		response.InternalError(c, "api key exchange usage service not configured")
		return
	}

	var req ListAPIKeyExchangeUsageLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	apiKeyID, err := h.apiKeyExchangeService.GetUsageLogAPIKeyIDByCode(c.Request.Context(), req.Code)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items, paginationResult, err := h.usageService.ListByAPIKey(c.Request.Context(), apiKeyID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.APIKeyExchangeUsageLog, 0, len(items))
	for i := range items {
		out = append(out, *dto.APIKeyExchangeUsageLogFromService(&items[i]))
	}

	if paginationResult == nil {
		response.Paginated(c, out, int64(len(out)), page, pageSize)
		return
	}
	response.Paginated(c, out, paginationResult.Total, paginationResult.Page, paginationResult.PageSize)
}

func publicLoginAgreementDocumentsToDTO(items []service.LoginAgreementDocument) []dto.LoginAgreementDocument {
	result := make([]dto.LoginAgreementDocument, 0, len(items))
	for _, item := range items {
		result = append(result, dto.LoginAgreementDocument{
			ID:        item.ID,
			Title:     item.Title,
			ContentMD: item.ContentMD,
		})
	}
	return result
}
