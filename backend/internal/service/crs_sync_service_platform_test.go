package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestCRSGeminiOAuthTargetPlatformPreservesExistingOpenAI(t *testing.T) {
	existing := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	credentials := map[string]any{
		"refresh_token": "incoming-rt",
	}

	if got := crsGeminiOAuthTargetPlatform(existing, credentials, nil); got != PlatformOpenAI {
		t.Fatalf("crsGeminiOAuthTargetPlatform() = %q, want %q", got, PlatformOpenAI)
	}
}

func TestCRSGeminiOAuthTargetPlatformDetectsOpenAIMetadata(t *testing.T) {
	credentials := map[string]any{
		"refresh_token":      "incoming-rt",
		"chatgpt_account_id": "chatgpt-acc",
		"plan_type":          "plus",
	}

	if got := crsGeminiOAuthTargetPlatform(nil, credentials, nil); got != PlatformOpenAI {
		t.Fatalf("crsGeminiOAuthTargetPlatform() = %q, want %q", got, PlatformOpenAI)
	}
}

func TestCRSGeminiOAuthTargetPlatformDetectsOpenAIPlanType(t *testing.T) {
	credentials := map[string]any{
		"access_token":  "incoming-at",
		"refresh_token": "incoming-rt",
		"plan_type":     "team",
	}

	if got := crsGeminiOAuthTargetPlatform(nil, credentials, nil); got != PlatformOpenAI {
		t.Fatalf("crsGeminiOAuthTargetPlatform() = %q, want %q", got, PlatformOpenAI)
	}
}

func TestCRSGeminiOAuthTargetPlatformDetectsOpenAIIDToken(t *testing.T) {
	credentials := map[string]any{
		"refresh_token": "incoming-rt",
		"id_token":      fakeCRSOpenAIIDToken(t, "chatgpt-acc", "team"),
	}

	if got := crsGeminiOAuthTargetPlatform(nil, credentials, nil); got != PlatformOpenAI {
		t.Fatalf("crsGeminiOAuthTargetPlatform() = %q, want %q", got, PlatformOpenAI)
	}
}

func TestCRSGeminiOAuthTargetPlatformLeavesGeminiOAuth(t *testing.T) {
	credentials := map[string]any{
		"refresh_token": "gemini-rt",
		"oauth_type":    "code_assist",
		"project_id":    "project-123",
		"plan_type":     "Pro",
	}

	if got := crsGeminiOAuthTargetPlatform(nil, credentials, nil); got != PlatformGemini {
		t.Fatalf("crsGeminiOAuthTargetPlatform() = %q, want %q", got, PlatformGemini)
	}
}

func fakeCRSOpenAIIDToken(t *testing.T, accountID, planType string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"iss": "https://auth.openai.com",
		"aud": []string{"https://api.openai.com"},
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_plan_type":  planType,
		},
	})
	if err != nil {
		t.Fatalf("marshal fake token payload: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
