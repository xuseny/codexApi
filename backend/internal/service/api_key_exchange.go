package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	APIKeyExchangeStatusUnused    = "unused"
	APIKeyExchangeStatusActivated = "activated"
	APIKeyExchangeStatusDisabled  = "disabled"

	APIKeyExchangeActionActivated = "activated"
	APIKeyExchangeActionQueried   = "queried"
)

var (
	ErrAPIKeyExchangeCodeNotFound      = infraerrors.NotFound("API_KEY_EXCHANGE_CODE_NOT_FOUND", "api key exchange code not found")
	ErrAPIKeyExchangeCodeDisabled      = infraerrors.Forbidden("API_KEY_EXCHANGE_CODE_DISABLED", "api key exchange code is disabled")
	ErrAPIKeyExchangeInvalidConfig     = infraerrors.BadRequest("API_KEY_EXCHANGE_INVALID_CONFIG", "invalid api key exchange config")
	ErrAPIKeyExchangeOrphanedAPIKey    = infraerrors.Conflict("API_KEY_EXCHANGE_ORPHANED_API_KEY", "exchange code is activated but linked api key is missing")
	ErrAPIKeyExchangeTooManyRequested  = infraerrors.BadRequest("API_KEY_EXCHANGE_TOO_MANY_REQUESTED", "too many codes requested")
	ErrAPIKeyExchangeNegativeQuota     = infraerrors.BadRequest("API_KEY_EXCHANGE_NEGATIVE_QUOTA", "quota must be greater than or equal to 0")
	ErrAPIKeyExchangeNegativeExpiry    = infraerrors.BadRequest("API_KEY_EXCHANGE_NEGATIVE_EXPIRY", "expires_in_days must be greater than or equal to 0")
	ErrAPIKeyExchangeCodeNotActivated  = infraerrors.Conflict("API_KEY_EXCHANGE_CODE_NOT_ACTIVATED", "exchange code has not been activated yet")
	ErrAPIKeyExchangeQuotaDisabled     = infraerrors.Forbidden("API_KEY_EXCHANGE_QUOTA_DISABLED", "api key status does not allow quota recharge")
	ErrAPIKeyExchangeQuotaUnlimited    = infraerrors.BadRequest("API_KEY_EXCHANGE_QUOTA_UNLIMITED", "api key has unlimited quota and does not need recharge")
	ErrAPIKeyExchangeRedeemCodeInvalid = infraerrors.BadRequest("API_KEY_EXCHANGE_REDEEM_CODE_INVALID", "redeem code is not for api key quota recharge")
)

type APIKeyExchangeCode struct {
	ID            int64
	Code          string
	OwnerUserID   int64
	CreatedBy     *int64
	GroupID       *int64
	Quota         float64
	ExpiresInDays int
	Status        string
	APIKeyID      *int64
	ActivatedAt   *time.Time
	ActivatedIP   *string
	BatchNo       string
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Group  *Group
	APIKey *APIKey
}

type APIKeyExchangeCodeListFilters struct {
	Search string
	Status string
}

type GenerateAPIKeyExchangeCodesInput struct {
	OwnerUserID   int64
	CreatedBy     int64
	Count         int
	GroupID       *int64
	Quota         float64
	ExpiresInDays int
	BatchNo       string
	Notes         string
}

type APIKeyExchangeUsageSummary struct {
	TotalRequests   int64
	TodayActualCost float64
	TotalActualCost float64
}

type APIKeyExchangeResolveResult struct {
	Code            string
	Status          string
	Action          string
	ActivatedAt     *time.Time
	APIKeyID        int64
	APIKey          string
	APIKeyName      string
	APIKeyStatus    string
	Quota           float64
	QuotaUsed       float64
	ExpiresAt       *time.Time
	TodayActualCost float64
	TotalActualCost float64
	TotalRequests   int64
	Group           *Group
}

type APIKeyExchangeQuotaRedeemResult struct {
	Amount     float64
	RedeemCode string
	Exchange   *APIKeyExchangeResolveResult
}

type APIKeyExchangeKickOfflineResult struct {
	Code       string `json:"code"`
	APIKeyID   int64  `json:"api_key_id"`
	APIKeyName string `json:"api_key_name"`
	Released   bool   `json:"released"`
}

type APIKeyExchangeRepository interface {
	CreateBatch(ctx context.Context, codes []APIKeyExchangeCode) error
	List(ctx context.Context, params pagination.PaginationParams, filters APIKeyExchangeCodeListFilters) ([]APIKeyExchangeCode, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*APIKeyExchangeCode, error)
	GetByCode(ctx context.Context, code string) (*APIKeyExchangeCode, error)
	Delete(ctx context.Context, id int64) error
	Resolve(ctx context.Context, code string, apiKeyName string, apiKeyValue string, activatedIP string) (*APIKeyExchangeCode, string, error)
	RedeemQuota(ctx context.Context, exchangeCode string, redeemCode string) (*APIKeyExchangeCode, float64, error)
	GetUsageSummary(ctx context.Context, apiKeyID int64, todayStart, end time.Time) (*APIKeyExchangeUsageSummary, error)
}

type APIKeyExchangeService struct {
	repo          APIKeyExchangeRepository
	groupRepo     GroupRepository
	apiKeyService *APIKeyService
}

func NewAPIKeyExchangeService(repo APIKeyExchangeRepository, groupRepo GroupRepository, apiKeyService *APIKeyService) *APIKeyExchangeService {
	return &APIKeyExchangeService{
		repo:          repo,
		groupRepo:     groupRepo,
		apiKeyService: apiKeyService,
	}
}

func (s *APIKeyExchangeService) GenerateRandomCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	hexCode := strings.ToUpper(hex.EncodeToString(bytes))
	parts := []string{
		hexCode[0:4],
		hexCode[4:8],
		hexCode[8:12],
		hexCode[12:16],
	}
	return strings.Join(parts, "-"), nil
}

func (s *APIKeyExchangeService) GenerateCodes(ctx context.Context, input GenerateAPIKeyExchangeCodesInput) ([]APIKeyExchangeCode, error) {
	if input.OwnerUserID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrAPIKeyExchangeInvalidConfig
	}
	if input.Count <= 0 {
		return nil, infraerrors.BadRequest("API_KEY_EXCHANGE_COUNT_INVALID", "count must be greater than 0")
	}
	if input.Count > 500 {
		return nil, ErrAPIKeyExchangeTooManyRequested
	}
	if input.Quota < 0 {
		return nil, ErrAPIKeyExchangeNegativeQuota
	}
	if input.ExpiresInDays < 0 {
		return nil, ErrAPIKeyExchangeNegativeExpiry
	}
	if input.GroupID != nil {
		if _, err := s.groupRepo.GetByID(ctx, *input.GroupID); err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}
	}

	batchNo := strings.TrimSpace(input.BatchNo)
	if batchNo == "" {
		batchNo = time.Now().Format("20060102-150405")
	}

	codes := make([]APIKeyExchangeCode, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		code, err := s.GenerateRandomCode()
		if err != nil {
			return nil, err
		}
		createdBy := input.CreatedBy
		codes = append(codes, APIKeyExchangeCode{
			Code:          code,
			OwnerUserID:   input.OwnerUserID,
			CreatedBy:     &createdBy,
			GroupID:       input.GroupID,
			Quota:         input.Quota,
			ExpiresInDays: input.ExpiresInDays,
			Status:        APIKeyExchangeStatusUnused,
			BatchNo:       batchNo,
			Notes:         strings.TrimSpace(input.Notes),
		})
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = s.repo.CreateBatch(ctx, codes)
		if lastErr == nil {
			return codes, nil
		}
		if !errors.Is(lastErr, ErrAPIKeyExists) {
			return nil, lastErr
		}
		for i := range codes {
			newCode, err := s.GenerateRandomCode()
			if err != nil {
				return nil, err
			}
			codes[i].Code = newCode
		}
	}

	return nil, lastErr
}

func (s *APIKeyExchangeService) ListCodes(ctx context.Context, params pagination.PaginationParams, filters APIKeyExchangeCodeListFilters) ([]APIKeyExchangeCode, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, params, filters)
}

func (s *APIKeyExchangeService) GetCodeByID(ctx context.Context, id int64) (*APIKeyExchangeCode, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *APIKeyExchangeService) DeleteCode(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *APIKeyExchangeService) BatchDeleteCodes(ctx context.Context, ids []int64) (int64, error) {
	var deleted int64
	for _, id := range ids {
		if err := s.repo.Delete(ctx, id); err != nil {
			if errors.Is(err, ErrAPIKeyExchangeCodeNotFound) {
				continue
			}
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *APIKeyExchangeService) KickOffline(ctx context.Context, code string) (*APIKeyExchangeKickOfflineResult, error) {
	if s == nil || s.apiKeyService == nil {
		return nil, infraerrors.InternalServer("API_KEY_DEVICE_LOCK_NOT_CONFIGURED", "api key device lock service not configured")
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, infraerrors.BadRequest("API_KEY_EXCHANGE_CODE_REQUIRED", "code is required")
	}

	record, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if record.Status == APIKeyExchangeStatusDisabled {
		return nil, ErrAPIKeyExchangeCodeDisabled
	}
	if record.APIKey == nil || record.APIKeyID == nil {
		if record.Status == APIKeyExchangeStatusUnused {
			return nil, ErrAPIKeyExchangeCodeNotActivated
		}
		return nil, ErrAPIKeyExchangeOrphanedAPIKey
	}

	if err := s.apiKeyService.ClearDeviceLock(ctx, *record.APIKeyID); err != nil {
		return nil, fmt.Errorf("clear api key device lock: %w", err)
	}

	return &APIKeyExchangeKickOfflineResult{
		Code:       record.Code,
		APIKeyID:   *record.APIKeyID,
		APIKeyName: record.APIKey.Name,
		Released:   true,
	}, nil
}

func (s *APIKeyExchangeService) Resolve(ctx context.Context, code string, activatedIP, userTimezone string) (*APIKeyExchangeResolveResult, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, infraerrors.BadRequest("API_KEY_EXCHANGE_CODE_REQUIRED", "code is required")
	}

	var (
		record *APIKeyExchangeCode
		action string
		err    error
	)

	for attempt := 0; attempt < 3; attempt++ {
		apiKeyValue, keyErr := s.apiKeyService.GenerateKey()
		if keyErr != nil {
			return nil, fmt.Errorf("generate api key: %w", keyErr)
		}
		record, action, err = s.repo.Resolve(ctx, code, buildAPIKeyExchangeName(code), apiKeyValue, activatedIP)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrAPIKeyExists) {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	if record == nil || record.APIKey == nil || record.APIKeyID == nil {
		return nil, ErrAPIKeyExchangeOrphanedAPIKey
	}

	return s.buildResolveResult(ctx, record, action, userTimezone)
}

func (s *APIKeyExchangeService) RedeemQuota(ctx context.Context, exchangeCode string, redeemCode string, userTimezone string) (*APIKeyExchangeQuotaRedeemResult, error) {
	exchangeCode = strings.ToUpper(strings.TrimSpace(exchangeCode))
	if exchangeCode == "" {
		return nil, infraerrors.BadRequest("API_KEY_EXCHANGE_CODE_REQUIRED", "exchange code is required")
	}

	redeemCode = strings.ToUpper(strings.TrimSpace(redeemCode))
	if redeemCode == "" {
		return nil, infraerrors.BadRequest("REDEEM_CODE_REQUIRED", "redeem code is required")
	}

	record, amount, err := s.repo.RedeemQuota(ctx, exchangeCode, redeemCode)
	if err != nil {
		return nil, err
	}
	if record == nil || record.APIKey == nil || record.APIKeyID == nil {
		return nil, ErrAPIKeyExchangeOrphanedAPIKey
	}

	if s.apiKeyService != nil && strings.TrimSpace(record.APIKey.Key) != "" {
		s.apiKeyService.InvalidateAuthCacheByKey(ctx, record.APIKey.Key)
	}

	exchange, err := s.buildResolveResult(ctx, record, APIKeyExchangeActionQueried, userTimezone)
	if err != nil {
		return nil, err
	}

	return &APIKeyExchangeQuotaRedeemResult{
		Amount:     amount,
		RedeemCode: redeemCode,
		Exchange:   exchange,
	}, nil
}

func (s *APIKeyExchangeService) buildResolveResult(ctx context.Context, record *APIKeyExchangeCode, action string, userTimezone string) (*APIKeyExchangeResolveResult, error) {
	now := timezone.NowInUserLocation(userTimezone)
	todayStart := timezone.StartOfDayInUserLocation(now, userTimezone)
	summary, err := s.repo.GetUsageSummary(ctx, *record.APIKeyID, todayStart, now)
	if err != nil {
		return nil, fmt.Errorf("get usage summary: %w", err)
	}

	status := normalizeAPIKeyExchangeAPIKeyStatus(record.APIKey)
	return &APIKeyExchangeResolveResult{
		Code:            record.Code,
		Status:          record.Status,
		Action:          action,
		ActivatedAt:     record.ActivatedAt,
		APIKeyID:        record.APIKey.ID,
		APIKey:          record.APIKey.Key,
		APIKeyName:      record.APIKey.Name,
		APIKeyStatus:    status,
		Quota:           record.APIKey.Quota,
		QuotaUsed:       record.APIKey.QuotaUsed,
		ExpiresAt:       record.APIKey.ExpiresAt,
		TodayActualCost: summary.TodayActualCost,
		TotalActualCost: summary.TotalActualCost,
		TotalRequests:   summary.TotalRequests,
		Group:           record.Group,
	}, nil
}

func buildAPIKeyExchangeName(code string) string {
	code = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(code)), "-", "")
	if len(code) > 8 {
		code = code[:8]
	}
	return "Exchange-" + code
}

func normalizeAPIKeyExchangeAPIKeyStatus(apiKey *APIKey) string {
	if apiKey == nil {
		return ""
	}
	if apiKey.IsExpired() {
		return StatusAPIKeyExpired
	}
	if apiKey.IsQuotaExhausted() {
		return StatusAPIKeyQuotaExhausted
	}
	if apiKey.Status == StatusAPIKeyDisabled || apiKey.Status == "inactive" {
		return StatusAPIKeyDisabled
	}
	return StatusAPIKeyActive
}
