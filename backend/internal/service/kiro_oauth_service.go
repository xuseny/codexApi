package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	kiroBuilderIDStartURL = "https://view.awsapps.com/start"
	kiroBuilderIDRegion   = "us-east-1"
	kiroTokenRefreshSkew  = 3 * time.Minute
)

var kiroSSOScopes = []string{
	"codewhisperer:completions",
	"codewhisperer:analysis",
	"codewhisperer:conversations",
	"codewhisperer:transformations",
	"codewhisperer:taskassist",
}

type KiroOAuthService struct {
	accountRepo AccountRepository
	proxyRepo   ProxyRepository
	sessions    *kiroOAuthSessionStore
}

func NewKiroOAuthService(accountRepo AccountRepository, proxyRepo ProxyRepository) *KiroOAuthService {
	return &KiroOAuthService{
		accountRepo: accountRepo,
		proxyRepo:   proxyRepo,
		sessions:    newKiroOAuthSessionStore(),
	}
}

type KiroDeviceAuthInput struct {
	StartURL string
	Region   string
	ProxyID  *int64
}

type KiroDeviceAuthResult struct {
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	UserCode                string `json:"user_code"`
	SessionID               string `json:"session_id"`
	Region                  string `json:"region"`
	AuthMethod              string `json:"auth_method"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type KiroTokenInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Region       string `json:"region,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
	StartURL     string `json:"start_url,omitempty"`
	ProfileARN   string `json:"profile_arn,omitempty"`
}

type kiroOAuthSession struct {
	ClientID     string
	ClientSecret string
	DeviceCode   string
	StartURL     string
	Region       string
	AuthMethod   string
	ProxyURL     string
	ExpiresAt    time.Time
	Interval     time.Duration
}

type kiroOAuthSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*kiroOAuthSession
}

func newKiroOAuthSessionStore() *kiroOAuthSessionStore {
	return &kiroOAuthSessionStore{sessions: make(map[string]*kiroOAuthSession)}
}

func (s *kiroOAuthSessionStore) Set(id string, session *kiroOAuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

func (s *kiroOAuthSessionStore) Get(id string) (*kiroOAuthSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok || session == nil {
		return nil, false
	}
	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		delete(s.sessions, id)
		return nil, false
	}
	return session, true
}

func (s *kiroOAuthSessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

type kiroClientRegisterResponse struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type kiroDeviceAuthorizationResponse struct {
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	UserCode                string `json:"userCode"`
	DeviceCode              string `json:"deviceCode"`
	Interval                int64  `json:"interval"`
	ExpiresIn               int64  `json:"expiresIn"`
}

type kiroCreateTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (s *KiroOAuthService) StartDeviceAuth(ctx context.Context, input KiroDeviceAuthInput) (*KiroDeviceAuthResult, error) {
	if s == nil {
		return nil, errors.New("kiro oauth service is not configured")
	}
	startURL := strings.TrimSpace(input.StartURL)
	authMethod := "idc"
	if startURL == "" {
		startURL = kiroBuilderIDStartURL
		authMethod = "builder-id"
	}
	region := strings.TrimSpace(input.Region)
	if region == "" {
		if authMethod == "builder-id" {
			region = kiroBuilderIDRegion
		} else {
			return nil, errors.New("region is required for Kiro IAM Identity Center OAuth")
		}
	}
	proxyURL, err := s.proxyURL(ctx, input.ProxyID)
	if err != nil {
		return nil, err
	}

	endpoint := kiroOIDCEndpoint(region)
	client := newKiroOAuthHTTPClient(proxyURL)

	registerReq := map[string]any{
		"clientName": "sub2api",
		"clientType": "public",
		"scopes":     kiroSSOScopes,
		"grantTypes": []string{
			"urn:ietf:params:oauth:grant-type:device_code",
			"refresh_token",
		},
	}
	var registerResp kiroClientRegisterResponse
	if err := postKiroOAuthJSON(ctx, client, endpoint+"/client/register", registerReq, &registerResp); err != nil {
		return nil, fmt.Errorf("register Kiro OAuth client: %w", err)
	}
	if strings.TrimSpace(registerResp.ClientID) == "" || strings.TrimSpace(registerResp.ClientSecret) == "" {
		return nil, errors.New("Kiro OAuth client registration returned empty clientId/clientSecret")
	}

	deviceReq := map[string]any{
		"clientId":     registerResp.ClientID,
		"clientSecret": registerResp.ClientSecret,
		"startUrl":     startURL,
	}
	var deviceResp kiroDeviceAuthorizationResponse
	if err := postKiroOAuthJSON(ctx, client, endpoint+"/device_authorization", deviceReq, &deviceResp); err != nil {
		return nil, fmt.Errorf("start Kiro device authorization: %w", err)
	}
	if strings.TrimSpace(deviceResp.DeviceCode) == "" || strings.TrimSpace(deviceResp.VerificationURIComplete) == "" {
		return nil, errors.New("Kiro device authorization returned incomplete response")
	}

	sessionID, err := newKiroSessionID()
	if err != nil {
		return nil, err
	}
	expiresIn := deviceResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 600
	}
	interval := deviceResp.Interval
	if interval <= 0 {
		interval = 5
	}
	s.sessions.Set(sessionID, &kiroOAuthSession{
		ClientID:     registerResp.ClientID,
		ClientSecret: registerResp.ClientSecret,
		DeviceCode:   deviceResp.DeviceCode,
		StartURL:     startURL,
		Region:       region,
		AuthMethod:   authMethod,
		ProxyURL:     proxyURL,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		Interval:     time.Duration(interval) * time.Second,
	})

	return &KiroDeviceAuthResult{
		VerificationURI:         deviceResp.VerificationURI,
		VerificationURIComplete: deviceResp.VerificationURIComplete,
		UserCode:                deviceResp.UserCode,
		SessionID:               sessionID,
		Region:                  region,
		AuthMethod:              authMethod,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

func (s *KiroOAuthService) ExchangeDeviceCode(ctx context.Context, sessionID string) (*KiroTokenInfo, error) {
	if s == nil {
		return nil, errors.New("kiro oauth service is not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session_id is required")
	}
	session, ok := s.sessions.Get(sessionID)
	if !ok {
		return nil, errors.New("Kiro OAuth session not found or expired")
	}

	tokenResp, err := s.createToken(ctx, session.ProxyURL, session.Region, map[string]any{
		"clientId":     session.ClientID,
		"clientSecret": session.ClientSecret,
		"deviceCode":   session.DeviceCode,
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
	})
	if err != nil {
		return nil, err
	}
	if tokenResp.Error != "" {
		if tokenResp.Error == "authorization_pending" || tokenResp.Error == "slow_down" {
			return nil, fmt.Errorf("Kiro authorization is not completed yet: %s", tokenResp.Error)
		}
		if tokenResp.ErrorDesc != "" {
			return nil, fmt.Errorf("Kiro authorization failed: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
		return nil, fmt.Errorf("Kiro authorization failed: %s", tokenResp.Error)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return nil, errors.New("Kiro token response missing accessToken/refreshToken")
	}

	s.sessions.Delete(sessionID)
	return &KiroTokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    normalizeKiroExpiresIn(tokenResp.ExpiresIn),
		ExpiresAt:    time.Now().Unix() + normalizeKiroExpiresIn(tokenResp.ExpiresIn),
		ClientID:     session.ClientID,
		ClientSecret: session.ClientSecret,
		Region:       session.Region,
		AuthMethod:   session.AuthMethod,
		StartURL:     session.StartURL,
	}, nil
}

func (s *KiroOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*KiroTokenInfo, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return nil, errors.New("not a kiro oauth account")
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tokenInfo, err := s.RefreshToken(ctx, KiroTokenInfo{
		RefreshToken: account.GetCredential("refresh_token"),
		ClientID:     account.GetCredential("client_id"),
		ClientSecret: account.GetCredential("client_secret"),
		Region:       account.GetCredential("region"),
		AuthMethod:   account.GetCredential("auth_method"),
		StartURL:     account.GetCredential("start_url"),
		ProfileARN:   account.GetCredential("profile_arn"),
	}, proxyURL)
	if err != nil {
		return nil, err
	}

	newCredentials := MergeCredentials(account.Credentials, s.BuildAccountCredentials(tokenInfo))
	if err := persistAccountCredentials(ctx, s.accountRepo, account, newCredentials); err != nil {
		return nil, err
	}
	return tokenInfo, nil
}

func (s *KiroOAuthService) RefreshToken(ctx context.Context, input KiroTokenInfo, proxyURL string) (*KiroTokenInfo, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	clientID := strings.TrimSpace(input.ClientID)
	clientSecret := strings.TrimSpace(input.ClientSecret)
	region := strings.TrimSpace(input.Region)
	if refreshToken == "" {
		return nil, errors.New("refresh_token is required")
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("client_id and client_secret are required for Kiro OAuth refresh")
	}
	if region == "" {
		region = kiroBuilderIDRegion
	}

	tokenResp, err := s.createToken(ctx, proxyURL, region, map[string]any{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	})
	if err != nil {
		return nil, err
	}
	if tokenResp.Error != "" {
		if tokenResp.ErrorDesc != "" {
			return nil, fmt.Errorf("Kiro token refresh failed: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
		return nil, fmt.Errorf("Kiro token refresh failed: %s", tokenResp.Error)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return nil, errors.New("Kiro token refresh response missing accessToken")
	}
	if strings.TrimSpace(tokenResp.RefreshToken) == "" {
		tokenResp.RefreshToken = refreshToken
	}
	expiresIn := normalizeKiroExpiresIn(tokenResp.ExpiresIn)
	return &KiroTokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Unix() + expiresIn,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Region:       region,
		AuthMethod:   strings.TrimSpace(input.AuthMethod),
		StartURL:     strings.TrimSpace(input.StartURL),
		ProfileARN:   strings.TrimSpace(input.ProfileARN),
	}, nil
}

func (s *KiroOAuthService) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return "", errors.New("not a kiro oauth account")
	}
	if expiresAt := account.GetCredentialAsTime("expires_at"); expiresAt != nil && time.Until(*expiresAt) > kiroTokenRefreshSkew {
		if accessToken := strings.TrimSpace(account.GetCredential("access_token")); accessToken != "" {
			return accessToken, nil
		}
	}
	tokenInfo, err := s.RefreshAccountToken(ctx, account)
	if err != nil {
		return "", err
	}
	return tokenInfo.AccessToken, nil
}

func (s *KiroOAuthService) BuildAccountCredentials(tokenInfo *KiroTokenInfo) map[string]any {
	creds := map[string]any{}
	if tokenInfo == nil {
		return creds
	}
	creds["access_token"] = tokenInfo.AccessToken
	creds["refresh_token"] = tokenInfo.RefreshToken
	creds["expires_at"] = tokenInfo.ExpiresAt
	creds["client_id"] = tokenInfo.ClientID
	creds["client_secret"] = tokenInfo.ClientSecret
	creds["region"] = tokenInfo.Region
	if tokenInfo.AuthMethod != "" {
		creds["auth_method"] = tokenInfo.AuthMethod
	}
	if tokenInfo.StartURL != "" {
		creds["start_url"] = tokenInfo.StartURL
	}
	if tokenInfo.ProfileARN != "" {
		creds["profile_arn"] = tokenInfo.ProfileARN
	}
	return creds
}

func (s *KiroOAuthService) createToken(ctx context.Context, proxyURL, region string, payload map[string]any) (*kiroCreateTokenResponse, error) {
	client := newKiroOAuthHTTPClient(proxyURL)
	var tokenResp kiroCreateTokenResponse
	if err := postKiroOAuthJSONAllowErrorResponse(ctx, client, kiroOIDCEndpoint(region)+"/token", payload, &tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

func (s *KiroOAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil || s.proxyRepo == nil {
		return "", nil
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", fmt.Errorf("proxy not found: %w", err)
	}
	if proxy == nil {
		return "", nil
	}
	return proxy.URL(), nil
}

func newKiroOAuthHTTPClient(proxyURL string) *http.Client {
	transport := &http.Transport{}
	if trimmed := strings.TrimSpace(proxyURL); trimmed != "" {
		if parsed, err := url.Parse(trimmed); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func postKiroOAuthJSON(ctx context.Context, client *http.Client, endpoint string, payload any, out any) error {
	return postKiroOAuthJSONResponse(ctx, client, endpoint, payload, out, false)
}

func postKiroOAuthJSONAllowErrorResponse(ctx context.Context, client *http.Client, endpoint string, payload any, out any) error {
	return postKiroOAuthJSONResponse(ctx, client, endpoint, payload, out, true)
}

func postKiroOAuthJSONResponse(ctx context.Context, client *http.Client, endpoint string, payload any, out any, allowErrorResponse bool) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("user-agent", "sub2api")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	trimmedBody := bytes.TrimSpace(respBody)
	decoded := false
	if out != nil && len(trimmedBody) > 0 {
		if err := json.Unmarshal(trimmedBody, out); err != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return err
			}
		} else {
			decoded = true
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if allowErrorResponse && decoded {
			return nil
		}
		return fmt.Errorf("Kiro OAuth upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil {
		return nil
	}
	if len(trimmedBody) == 0 {
		return errors.New("Kiro OAuth upstream returned empty response")
	}
	return nil
}

func kiroOIDCEndpoint(region string) string {
	return fmt.Sprintf("https://oidc.%s.amazonaws.com", strings.TrimSpace(region))
}

func KiroAPIBaseURL(region string) string {
	apiRegion := ResolveKiroAPIRegion(region)
	return fmt.Sprintf("https://q.%s.amazonaws.com", apiRegion)
}

func ResolveKiroAPIRegion(region string) string {
	switch strings.TrimSpace(region) {
	case "":
		return "us-east-1"
	case "us-west-1", "us-west-2", "us-east-2":
		return "us-east-1"
	case "eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "eu-south-1", "eu-south-2", "eu-central-2":
		return "eu-central-1"
	default:
		return strings.TrimSpace(region)
	}
}

func normalizeKiroExpiresIn(v int64) int64 {
	if v <= 0 {
		return 3600
	}
	return v
}

func newKiroSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
