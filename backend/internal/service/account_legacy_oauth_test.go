package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestAccountGetCredentialSupportsLegacyOAuthAliases(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"token": "legacy-access-token",
			"rt":    "legacy-refresh-token",
			"st":    "legacy-session-token",
		},
	}

	if got := account.GetCredential("access_token"); got != "legacy-access-token" {
		t.Fatalf("access_token = %q, want legacy-access-token", got)
	}
	if got := account.GetCredential("refresh_token"); got != "legacy-refresh-token" {
		t.Fatalf("refresh_token = %q, want legacy-refresh-token", got)
	}
	if got := account.GetCredential("session_token"); got != "legacy-session-token" {
		t.Fatalf("session_token = %q, want legacy-session-token", got)
	}
}

func TestAccountOpenAIJWTFallbacks(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": buildTestJWT(t, map[string]any{
				"https://api.openai.com/auth": map[string]any{
					"chatgpt_account_id": "chatgpt-acc",
					"chatgpt_user_id":    "chatgpt-user",
					"organizations": []map[string]any{
						{
							"id":         "org-default",
							"is_default": true,
						},
					},
				},
			}),
		},
	}

	if got := account.GetChatGPTAccountID(); got != "chatgpt-acc" {
		t.Fatalf("GetChatGPTAccountID() = %q, want chatgpt-acc", got)
	}
	if got := account.GetChatGPTUserID(); got != "chatgpt-user" {
		t.Fatalf("GetChatGPTUserID() = %q, want chatgpt-user", got)
	}
	if got := account.GetOpenAIOrganizationID(); got != "org-default" {
		t.Fatalf("GetOpenAIOrganizationID() = %q, want org-default", got)
	}
}

func buildTestJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(body) + ".sig"
}
