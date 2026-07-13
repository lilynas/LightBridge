//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grokLockRaceCache struct {
	mu       sync.Mutex
	getCalls int
	token    string
}

func (c *grokLockRaceCache) GetAccessToken(context.Context, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCalls++
	if c.getCalls >= 2 {
		return c.token, nil
	}
	return "", nil
}
func (c *grokLockRaceCache) SetAccessToken(context.Context, string, string, time.Duration) error {
	return nil
}
func (c *grokLockRaceCache) DeleteAccessToken(context.Context, string) error { return nil }
func (c *grokLockRaceCache) AcquireRefreshLock(context.Context, string, time.Duration) (bool, error) {
	return false, nil
}
func (c *grokLockRaceCache) ReleaseRefreshLock(context.Context, string) error { return nil }

type grokLockRaceExecutor struct{}

func (grokLockRaceExecutor) CacheKey(account *Account) string { return GrokTokenCacheKey(account) }
func (grokLockRaceExecutor) CanRefresh(*Account) bool         { return true }
func (grokLockRaceExecutor) NeedsRefresh(*Account, time.Duration) bool {
	return true
}
func (grokLockRaceExecutor) Refresh(context.Context, *Account) (map[string]any, error) {
	return nil, nil
}

func TestGrokTokenProviderWaitsForFreshTokenWhenRefreshLockHeld(t *testing.T) {
	cache := &grokLockRaceCache{token: "fresh-token"}
	provider := NewGrokTokenProvider(nil, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(nil, cache), grokLockRaceExecutor{})
	expired := time.Now().Add(-time.Minute)
	account := &Account{
		ID:       777,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "expired-token",
			"refresh_token": "refresh-token",
			"expires_at":    expired,
		},
	}

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "fresh-token", token)
	require.GreaterOrEqual(t, cache.getCalls, 2)
}
