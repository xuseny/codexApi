package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	windsurfFirebaseAPIKey      = "AIzaSyDsOl-1XpT5err0Tcnx8FFod1H8gVGIycY"
	windsurfFirebaseAuthURL     = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + windsurfFirebaseAPIKey
	windsurfFirebaseRefreshURL  = "https://securetoken.googleapis.com/v1/token?key=" + windsurfFirebaseAPIKey
	windsurfAuth1ConnectionsURL = "https://windsurf.com/_devin-auth/connections"
	windsurfAuth1PasswordURL    = "https://windsurf.com/_devin-auth/password/login"
	windsurfCheckLoginMethodURL = "https://windsurf.com/_backend/exa.seat_management_pb.SeatManagementService/CheckUserLoginMethod"
	windsurfPostAuthURL         = "https://server.self-serve.windsurf.com/exa.seat_management_pb.SeatManagementService/WindsurfPostAuth"
	windsurfPostAuthURLNew      = "https://windsurf.com/_backend/exa.seat_management_pb.SeatManagementService/WindsurfPostAuth"
	windsurfRegisterURL         = "https://register.windsurf.com/exa.seat_management_pb.SeatManagementService/RegisterUser"
	windsurfRegisterFallbackURL = "https://api.codeium.com/register_user/"
	windsurfDefaultAPIServerURL = "https://server.self-serve.windsurf.com"
)

var windsurfSessionTokenRe = regexp.MustCompile(`devin-session-token\$[a-zA-Z0-9._-]+`)

type WindsurfAuthService struct {
	accountRepo AccountRepository
	proxyRepo   ProxyRepository
}

func NewWindsurfAuthService(accountRepo AccountRepository, proxyRepo ProxyRepository) *WindsurfAuthService {
	return &WindsurfAuthService{
		accountRepo: accountRepo,
		proxyRepo:   proxyRepo,
	}
}

type WindsurfLoginInput struct {
	Email    string
	Password string
	Token    string
	APIKey   string
	ProxyID  *int64
}

type WindsurfTokenInfo struct {
	APIKey       string `json:"api_key"`
	APIServerURL string `json:"api_server_url,omitempty"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
	Auth1Token   string `json:"auth1_token,omitempty"`
}

type windsurfRegisterResponse struct {
	APIKey       string `json:"api_key"`
	APIKeyCamel  string `json:"apiKey"`
	Name         string `json:"name"`
	APIServerURL string `json:"api_server_url"`
	ServerURL    string `json:"apiServerUrl"`
}

type windsurfFirebaseAuthResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
	Email        string `json:"email"`
	LocalID      string `json:"localId"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type windsurfFirebaseRefreshResponse struct {
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *WindsurfAuthService) Login(ctx context.Context, input WindsurfLoginInput) (*WindsurfTokenInfo, error) {
	if s == nil {
		return nil, errors.New("windsurf auth service is not configured")
	}
	proxyURL, err := s.proxyURL(ctx, input.ProxyID)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.TrimSpace(input.APIKey) != "":
		return &WindsurfTokenInfo{
			APIKey:       strings.TrimSpace(input.APIKey),
			APIServerURL: windsurfDefaultAPIServerURL,
			AuthMethod:   "api_key",
		}, nil
	case strings.TrimSpace(input.Token) != "":
		return s.LoginByToken(ctx, strings.TrimSpace(input.Token), proxyURL)
	case strings.TrimSpace(input.Email) != "" && strings.TrimSpace(input.Password) != "":
		return s.LoginByEmailPassword(ctx, strings.TrimSpace(input.Email), input.Password, proxyURL)
	default:
		return nil, errors.New("windsurf login requires token, api_key, or email/password")
	}
}

func (s *WindsurfAuthService) LoginByToken(ctx context.Context, token, proxyURL string) (*WindsurfTokenInfo, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("token is required")
	}
	reg, err := windsurfRegisterWithFirebaseToken(ctx, newWindsurfHTTPClient(proxyURL), token)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(firstNonEmpty(reg.APIKey, reg.APIKeyCamel))
	if apiKey == "" {
		return nil, errors.New("windsurf register returned empty api key")
	}
	return &WindsurfTokenInfo{
		APIKey:       apiKey,
		APIServerURL: firstNonEmpty(reg.APIServerURL, reg.ServerURL, windsurfDefaultAPIServerURL),
		Name:         reg.Name,
		AuthMethod:   "token",
		IDToken:      token,
	}, nil
}

func (s *WindsurfAuthService) LoginByEmailPassword(ctx context.Context, email, password, proxyURL string) (*WindsurfTokenInfo, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}
	client := newWindsurfHTTPClient(proxyURL)
	fp := newWindsurfFingerprint()

	method, hasPassword := s.resolveLoginMethod(ctx, client, fp, email)
	if method == "auth1" && hasPassword {
		if info, err := windsurfLoginViaAuth1(ctx, client, fp, email, password); err == nil {
			return info, nil
		}
	}
	return windsurfLoginViaFirebase(ctx, client, fp, email, password)
}

func (s *WindsurfAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*WindsurfTokenInfo, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	if account.Platform != PlatformWindsurf || account.Type != AccountTypeOAuth {
		return nil, errors.New("not a windsurf oauth account")
	}
	refreshToken := strings.TrimSpace(account.GetCredential("refresh_token"))
	if refreshToken == "" {
		if apiKey := strings.TrimSpace(account.GetCredential("api_key")); apiKey != "" {
			return &WindsurfTokenInfo{
				APIKey:       apiKey,
				APIServerURL: firstNonEmpty(account.GetCredential("api_server_url"), windsurfDefaultAPIServerURL),
				Email:        account.GetCredential("email"),
				Name:         account.GetCredential("name"),
				AuthMethod:   firstNonEmpty(account.GetCredential("auth_method"), "api_key"),
			}, nil
		}
		return nil, errors.New("windsurf account has no refresh_token or api_key")
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tokenInfo, err := s.RefreshToken(ctx, refreshToken, proxyURL)
	if err != nil {
		return nil, err
	}
	tokenInfo.Email = firstNonEmpty(tokenInfo.Email, account.GetCredential("email"))
	tokenInfo.Name = firstNonEmpty(tokenInfo.Name, account.GetCredential("name"))

	newCredentials := MergeCredentials(account.Credentials, s.BuildAccountCredentials(tokenInfo))
	if err := persistAccountCredentials(ctx, s.accountRepo, account, newCredentials); err != nil {
		return nil, err
	}
	return tokenInfo, nil
}

func (s *WindsurfAuthService) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*WindsurfTokenInfo, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("refresh_token is required")
	}
	client := newWindsurfHTTPClient(proxyURL)
	var refreshResp windsurfFirebaseRefreshResponse
	if err := postWindsurfForm(ctx, client, windsurfFirebaseRefreshURL, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}, &refreshResp); err != nil {
		return nil, err
	}
	if refreshResp.Error != nil && refreshResp.Error.Message != "" {
		return nil, fmt.Errorf("windsurf firebase refresh failed: %s", refreshResp.Error.Message)
	}
	if strings.TrimSpace(refreshResp.IDToken) == "" {
		return nil, errors.New("windsurf refresh returned empty id_token")
	}
	reg, err := windsurfRegisterWithFirebaseToken(ctx, client, refreshResp.IDToken)
	if err != nil {
		return nil, err
	}
	expiresIn := parseWindsurfExpiresIn(refreshResp.ExpiresIn)
	return &WindsurfTokenInfo{
		APIKey:       firstNonEmpty(reg.APIKey, reg.APIKeyCamel),
		APIServerURL: firstNonEmpty(reg.APIServerURL, reg.ServerURL, windsurfDefaultAPIServerURL),
		Name:         reg.Name,
		AuthMethod:   "firebase",
		IDToken:      refreshResp.IDToken,
		RefreshToken: firstNonEmpty(refreshResp.RefreshToken, refreshToken),
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Unix() + expiresIn,
	}, nil
}

func (s *WindsurfAuthService) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformWindsurf || account.Type != AccountTypeOAuth {
		return "", errors.New("not a windsurf oauth account")
	}
	if expiresAt := account.GetCredentialAsTime("expires_at"); expiresAt == nil || time.Until(*expiresAt) > time.Minute {
		if apiKey := strings.TrimSpace(account.GetCredential("api_key")); apiKey != "" {
			return apiKey, nil
		}
	}
	tokenInfo, err := s.RefreshAccountToken(ctx, account)
	if err != nil {
		return "", err
	}
	if tokenInfo.APIKey == "" {
		return "", errors.New("windsurf refresh returned empty api_key")
	}
	return tokenInfo.APIKey, nil
}

func (s *WindsurfAuthService) BuildAccountCredentials(tokenInfo *WindsurfTokenInfo) map[string]any {
	creds := map[string]any{"windsurf_builtin": true, "windsurf_transport": "language_server"}
	if tokenInfo == nil {
		return creds
	}
	creds["api_key"] = tokenInfo.APIKey
	creds["api_server_url"] = firstNonEmpty(tokenInfo.APIServerURL, windsurfDefaultAPIServerURL)
	if tokenInfo.Email != "" {
		creds["email"] = tokenInfo.Email
	}
	if tokenInfo.Name != "" {
		creds["name"] = tokenInfo.Name
	}
	if tokenInfo.AuthMethod != "" {
		creds["auth_method"] = tokenInfo.AuthMethod
	}
	if tokenInfo.IDToken != "" {
		creds["id_token"] = tokenInfo.IDToken
	}
	if tokenInfo.RefreshToken != "" {
		creds["refresh_token"] = tokenInfo.RefreshToken
	}
	if tokenInfo.ExpiresAt > 0 {
		creds["expires_at"] = tokenInfo.ExpiresAt
	}
	if tokenInfo.SessionToken != "" {
		creds["session_token"] = tokenInfo.SessionToken
	}
	if tokenInfo.Auth1Token != "" {
		creds["auth1_token"] = tokenInfo.Auth1Token
	}
	return creds
}

func (s *WindsurfAuthService) resolveLoginMethod(ctx context.Context, client *http.Client, fp map[string]string, email string) (string, bool) {
	body := map[string]any{"email": email}
	headers := windsurfJSONHeaders(fp, map[string]string{"Connect-Protocol-Version": "1"})
	var checkResp map[string]any
	if err := postWindsurfJSON(ctx, client, windsurfCheckLoginMethodURL, body, headers, &checkResp); err == nil {
		if _, hasUser := checkResp["userExists"]; hasUser {
			if checkResp["userExists"] == false {
				return "", false
			}
			return "auth1", checkResp["hasPassword"] != false
		}
		if _, hasPassword := checkResp["hasPassword"]; hasPassword {
			return "auth1", checkResp["hasPassword"] != false
		}
	}

	var connResp map[string]any
	headers = windsurfJSONHeaders(fp, nil)
	if err := postWindsurfJSON(ctx, client, windsurfAuth1ConnectionsURL, map[string]any{
		"product": "windsurf",
		"email":   email,
	}, headers, &connResp); err != nil {
		return "", false
	}
	if raw, ok := connResp["auth_method"].(map[string]any); ok {
		method, _ := raw["method"].(string)
		return method, raw["has_password"] != false
	}
	if conns, ok := connResp["connections"].([]any); ok {
		for _, item := range conns {
			m, ok := item.(map[string]any)
			if !ok || m["type"] != "email" {
				continue
			}
			return "auth1", m["enabled"] == true
		}
	}
	return "", false
}

func (s *WindsurfAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
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

func windsurfLoginViaAuth1(ctx context.Context, client *http.Client, fp map[string]string, email, password string) (*WindsurfTokenInfo, error) {
	var loginResp map[string]any
	if err := postWindsurfJSON(ctx, client, windsurfAuth1PasswordURL, map[string]any{
		"email":    email,
		"password": password,
	}, windsurfJSONHeaders(fp, nil), &loginResp); err != nil {
		return nil, err
	}
	if detail := windsurfDetailMessage(loginResp["detail"]); detail != "" {
		return nil, fmt.Errorf("windsurf auth1 login failed: %s", detail)
	}
	auth1Token, _ := loginResp["token"].(string)
	if strings.TrimSpace(auth1Token) == "" {
		return nil, errors.New("windsurf auth1 login returned empty token")
	}

	sessionToken, err := windsurfPostAuth(ctx, client, fp, auth1Token)
	if err != nil {
		return nil, err
	}
	return &WindsurfTokenInfo{
		APIKey:       sessionToken,
		APIServerURL: windsurfDefaultAPIServerURL,
		Email:        email,
		Name:         email,
		AuthMethod:   "auth1",
		SessionToken: sessionToken,
		Auth1Token:   auth1Token,
	}, nil
}

func windsurfLoginViaFirebase(ctx context.Context, client *http.Client, fp map[string]string, email, password string) (*WindsurfTokenInfo, error) {
	var fbResp windsurfFirebaseAuthResponse
	if err := postWindsurfJSON(ctx, client, windsurfFirebaseAuthURL, map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	}, windsurfJSONHeaders(fp, nil), &fbResp); err != nil {
		return nil, err
	}
	if fbResp.Error != nil && fbResp.Error.Message != "" {
		return nil, fmt.Errorf("windsurf firebase login failed: %s", fbResp.Error.Message)
	}
	if strings.TrimSpace(fbResp.IDToken) == "" {
		return nil, errors.New("windsurf firebase login returned empty id_token")
	}
	reg, err := windsurfRegisterWithFirebaseToken(ctx, client, fbResp.IDToken)
	if err != nil {
		return nil, err
	}
	expiresIn := parseWindsurfExpiresIn(fbResp.ExpiresIn)
	return &WindsurfTokenInfo{
		APIKey:       firstNonEmpty(reg.APIKey, reg.APIKeyCamel),
		APIServerURL: firstNonEmpty(reg.APIServerURL, reg.ServerURL, windsurfDefaultAPIServerURL),
		Email:        email,
		Name:         firstNonEmpty(reg.Name, email),
		AuthMethod:   "firebase",
		IDToken:      fbResp.IDToken,
		RefreshToken: fbResp.RefreshToken,
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Unix() + expiresIn,
	}, nil
}

func windsurfPostAuth(ctx context.Context, client *http.Client, fp map[string]string, auth1Token string) (string, error) {
	headers := map[string]string{}
	for k, v := range fp {
		headers[k] = v
	}
	headers["Content-Type"] = "application/proto"
	headers["Connect-Protocol-Version"] = "1"
	headers["X-Devin-Auth1-Token"] = auth1Token
	headers["Referer"] = "https://windsurf.com/account/login"
	var lastErr error
	for _, endpoint := range []string{windsurfPostAuthURLNew, windsurfPostAuthURL} {
		raw, status, err := postWindsurfRaw(ctx, client, endpoint, nil, headers)
		if err != nil {
			lastErr = err
			continue
		}
		session := parseWindsurfSessionToken(raw)
		if status >= 200 && status < 300 && session != "" {
			return session, nil
		}
		lastErr = fmt.Errorf("windsurf post auth returned %d: %s", status, strings.TrimSpace(string(raw)))
	}
	if lastErr == nil {
		lastErr = errors.New("windsurf post auth failed")
	}
	return "", lastErr
}

func windsurfRegisterWithFirebaseToken(ctx context.Context, client *http.Client, token string) (*windsurfRegisterResponse, error) {
	payload := map[string]any{"firebase_id_token": token}
	var lastErr error
	for _, endpoint := range []string{windsurfRegisterURL, windsurfRegisterFallbackURL} {
		var reg windsurfRegisterResponse
		err := postWindsurfJSON(ctx, client, endpoint, payload, windsurfJSONHeaders(newWindsurfFingerprint(), map[string]string{
			"Connect-Protocol-Version": "1",
			"Accept":                   "application/json",
		}), &reg)
		if err == nil && strings.TrimSpace(firstNonEmpty(reg.APIKey, reg.APIKeyCamel)) != "" {
			return &reg, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errors.New("register response missing api key")
		}
	}
	return nil, fmt.Errorf("windsurf register failed: %w", lastErr)
}

func newWindsurfHTTPClient(proxyURL string) *http.Client {
	transport := &http.Transport{}
	if trimmed := strings.TrimSpace(proxyURL); trimmed != "" {
		if parsed, err := url.Parse(trimmed); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func postWindsurfJSON(ctx context.Context, client *http.Client, endpoint string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw, status, err := postWindsurfRaw(ctx, client, endpoint, body, headers)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) > 0 && out != nil {
		if err := json.Unmarshal(raw, out); err != nil && status >= 200 && status < 300 {
			return err
		}
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("windsurf upstream returned %d: %s", status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func postWindsurfForm(ctx context.Context, client *http.Client, endpoint string, values url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", windsurfDefaultUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) > 0 && out != nil {
		if err := json.Unmarshal(raw, out); err != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("windsurf upstream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func postWindsurfRaw(ctx context.Context, client *http.Client, endpoint string, body []byte, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", windsurfDefaultUserAgent())
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func windsurfJSONHeaders(fp map[string]string, extra map[string]string) map[string]string {
	headers := map[string]string{}
	for k, v := range fp {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"
	for k, v := range extra {
		headers[k] = v
	}
	return headers
}

func newWindsurfFingerprint() map[string]string {
	return map[string]string{
		"User-Agent":         windsurfDefaultUserAgent(),
		"Accept-Language":    "en-US,en;q=0.9",
		"Accept":             "application/json, text/plain, */*",
		"Accept-Encoding":    "identity",
		"sec-ch-ua":          `"Chromium";v="134", "Google Chrome";v="134", "Not-A.Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "cross-site",
		"Origin":             "https://windsurf.com",
		"Referer":            "https://windsurf.com/",
	}
}

func windsurfDefaultUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
}

func parseWindsurfSessionToken(raw []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if token, _ := obj["sessionToken"].(string); strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
	}
	return windsurfSessionTokenRe.FindString(string(raw))
}

func windsurfDetailMessage(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if msg, _ := m["msg"].(string); msg != "" {
					parts = append(parts, msg)
					continue
				}
				if typ, _ := m["type"].(string); typ != "" {
					parts = append(parts, typ)
				}
			}
		}
		return strings.Join(parts, "; ")
	default:
		return ""
	}
}

func parseWindsurfExpiresIn(raw string) int64 {
	if raw == "" {
		return 3600
	}
	var n int64
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 3600
		}
		n = n*10 + int64(ch-'0')
	}
	if n <= 0 {
		return 3600
	}
	return n
}
