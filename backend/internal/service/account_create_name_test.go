package service

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveCreateAccountName(t *testing.T) {
	t.Run("explicit name wins", func(t *testing.T) {
		name, err := resolveCreateAccountName("  operator name  ", PlatformGrok, AccountTypeOAuth, map[string]any{"email": "oauth@example.com"}, nil)
		require.NoError(t, err)
		require.Equal(t, "operator name", name)
	})

	t.Run("oauth email becomes name", func(t *testing.T) {
		name, err := resolveCreateAccountName("", PlatformGrok, AccountTypeOAuth, map[string]any{"email": "grok@example.com"}, nil)
		require.NoError(t, err)
		require.Equal(t, "grok@example.com", name)
	})

	t.Run("id token email is used when credentials email is absent", func(t *testing.T) {
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"subject-1","email":"jwt@example.com"}`))
		idToken := "header." + payload + ".signature"
		name, err := resolveCreateAccountName("", PlatformOpenAI, AccountTypeOAuth, map[string]any{"id_token": idToken}, nil)
		require.NoError(t, err)
		require.Equal(t, "jwt@example.com", name)
	})

	t.Run("subject is a stable fallback", func(t *testing.T) {
		name, err := resolveCreateAccountName("", PlatformGrok, AccountTypeOAuth, map[string]any{"sub": "xai-subject"}, nil)
		require.NoError(t, err)
		require.Equal(t, "xai-subject", name)
	})

	t.Run("platform fallback remains deterministic", func(t *testing.T) {
		name, err := resolveCreateAccountName("", PlatformGrok, AccountTypeOAuth, map[string]any{"access_token": "token"}, nil)
		require.NoError(t, err)
		require.Equal(t, "Grok OAuth Account", name)
	})

	t.Run("non oauth account still requires a name", func(t *testing.T) {
		_, err := resolveCreateAccountName("", PlatformCustom, AccountTypeAPIKey, map[string]any{"api_key": "secret"}, nil)
		require.EqualError(t, err, "account name is required")
	})
}

func TestResolveCreateAccountName_SetupTokenAndAccessTokenJWT(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"preferred_username":"setup@example.com"}`))
	name, err := resolveCreateAccountName("", PlatformAnthropic, AccountTypeSetupToken, map[string]any{
		"access_token": "header." + payload + ".signature",
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "setup@example.com", name)
}
