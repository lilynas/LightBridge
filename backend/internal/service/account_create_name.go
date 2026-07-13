package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// resolveCreateAccountName centralizes account naming for every creation path
// (admin UI, OAuth callbacks, batch imports and compatibility importers).
//
// OAuth accounts may omit a name. We prefer the authenticated email and then
// stable provider identifiers. Non-OAuth accounts keep the explicit-name
// requirement so API-key/upstream channels cannot silently receive ambiguous
// generated names.
func resolveCreateAccountName(
	requestedName string,
	platform string,
	accountType string,
	credentials map[string]any,
	extra map[string]any,
) (string, error) {
	if name := strings.TrimSpace(requestedName); name != "" {
		return name, nil
	}
	accountType = strings.TrimSpace(accountType)
	if accountType != AccountTypeOAuth && accountType != AccountTypeSetupToken {
		return "", errors.New("account name is required")
	}

	if email := firstAccountNameValue(credentials,
		"email", "email_address", "emailAddress", "account_email", "user_email", "preferred_username"); email != "" {
		return email, nil
	}
	if email := firstAccountNameValue(extra,
		"email", "email_address", "emailAddress", "account_email", "user_email", "preferred_username"); email != "" {
		return email, nil
	}
	if email := emailFromAccountCredentialJWT(credentials); email != "" {
		return email, nil
	}
	if displayName := firstAccountNameValue(extra, "name", "display_name", "displayName"); displayName != "" {
		return displayName, nil
	}
	if stableID := firstAccountNameValue(
		credentials,
		"sub",
		"subject",
		"chatgpt_account_id",
		"account_id",
		"google_account_id",
		"chatgpt_user_id",
		"user_id",
		"project_id",
	); stableID != "" {
		return stableID, nil
	}

	return defaultOAuthAccountName(platform), nil
}

func firstAccountNameValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case json.Number:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func emailFromAccountCredentialJWT(credentials map[string]any) string {
	for _, key := range []string{"id_token", "idToken", "access_token", "accessToken"} {
		token := firstAccountNameValue(credentials, key)
		parts := strings.Split(token, ".")
		if len(parts) < 2 {
			continue
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			continue
		}
		var claims map[string]any
		if err := json.Unmarshal(payload, &claims); err != nil {
			continue
		}
		if email := firstAccountNameValue(claims, "email", "preferred_username", "upn"); email != "" {
			return email
		}
	}
	return ""
}

func defaultOAuthAccountName(platform string) string {
	switch normalized, subPlatform := NormalizePlatform(platform); {
	case subPlatform == SubPlatformAntigravity:
		return "Antigravity OAuth Account"
	case normalized == PlatformAnthropic:
		return "Anthropic OAuth Account"
	case normalized == PlatformOpenAI:
		return "OpenAI OAuth Account"
	case normalized == PlatformGemini:
		return "Gemini OAuth Account"
	case normalized == PlatformGrok:
		return "Grok OAuth Account"
	default:
		return "OAuth Account"
	}
}
