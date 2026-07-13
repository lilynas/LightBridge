//go:build unit

package repository

import (
	"testing"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestNewGrokOAuthClientRejectsUnsafeTokenURL(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "")
	t.Setenv(xai.EnvTokenURL, "http://127.0.0.1:9876/token")
	client, err := NewGrokOAuthClient()
	require.Error(t, err)
	require.Nil(t, client)
	require.Contains(t, err.Error(), "invalid xAI OAuth token URL")
}

func TestNewGrokOAuthClientAcceptsDefaultTokenURL(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "")
	t.Setenv(xai.EnvTokenURL, "")
	client, err := NewGrokOAuthClient()
	require.NoError(t, err)
	require.NotNil(t, client)
}
