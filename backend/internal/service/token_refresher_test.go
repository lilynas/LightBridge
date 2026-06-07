//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAITokenRefresher_CanRefresh(t *testing.T) {
	refresher := &OpenAITokenRefresher{}

	tests := []struct {
		name     string
		platform string
		accType  string
		want     bool
	}{
		{
			name:     "openai oauth - can refresh",
			platform: PlatformOpenAI,
			accType:  AccountTypeOAuth,
			want:     true,
		},
		{
			name:     "openai apikey - cannot refresh",
			platform: PlatformOpenAI,
			accType:  AccountTypeAPIKey,
			want:     false,
		},
		{
			name:     "anthropic oauth - cannot refresh in core",
			platform: PlatformAnthropic,
			accType:  AccountTypeOAuth,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform: tt.platform,
				Type:     tt.accType,
			}
			require.Equal(t, tt.want, refresher.CanRefresh(account))
		})
	}
}

func TestOpenAITokenRefresher_NeedsRefresh(t *testing.T) {
	refresher := &OpenAITokenRefresher{}
	refreshWindow := 30 * time.Minute
	expiresAt := time.Now().Add(15 * time.Minute).Unix()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"expires_at":    expiresAt,
		},
	}

	require.True(t, refresher.NeedsRefresh(account, refreshWindow))
}
