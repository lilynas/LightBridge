//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeTokenProvider_GetAccessToken_NilAccount(t *testing.T) {
	provider := NewClaudeTokenProvider(nil, nil)

	token, err := provider.GetAccessToken(context.Background(), nil)

	require.Error(t, err)
	require.Empty(t, token)
	require.Contains(t, err.Error(), "account is nil")
}

func TestClaudeTokenProvider_GetAccessToken_RejectsAnthropicOAuth(t *testing.T) {
	provider := NewClaudeTokenProvider(nil, nil)
	account := &Account{
		ID:       100,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "module-token",
		},
	}

	token, err := provider.GetAccessToken(context.Background(), account)

	require.Error(t, err)
	require.Empty(t, token)
	require.Contains(t, err.Error(), "not an anthropic service account")
}
