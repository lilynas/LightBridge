//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestGrokOAuthSessionStoreConsumeIsAtomicAndOneTime(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewGrokOAuthSessionStore(client)
	ctx := context.Background()
	session := &xai.OAuthSession{State: "state", CodeVerifier: "verifier", RedirectURI: "http://localhost/callback", Mode: xai.OAuthModeBuildProxy, CreatedAt: time.Now()}

	require.NoError(t, store.Set(ctx, "session-id", session, xai.SessionTTL))
	got, ok, err := store.Consume(ctx, "session-id")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, session.State, got.State)
	require.Equal(t, xai.OAuthModeBuildProxy, got.Mode)

	got, ok, err = store.Consume(ctx, "session-id")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, got)
}
