package modules

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoreBridgeGRPCRoundTripBindsModuleIdentity(t *testing.T) {
	socketPath := CoreBridgeSocketPath(t.TempDir(), "lightbridge.provider.real")
	bridge := &fakeCoreBridge{}
	closer, err := StartCoreBridgeServer("lightbridge.provider.real", socketPath, bridge)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = closer.Close()
	})

	client, err := NewGRPCCoreBridgeClient(socketPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	user, err := client.GetUserSummary(ctx, CoreBridgeUserRequest{ModuleID: "spoofed", UserID: "user_1"})
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.real", user.ModuleID)
	require.Equal(t, "user_1", user.UserID)

	account, err := client.GetAccountCredentials(ctx, CoreBridgeAccountRequest{ModuleID: "spoofed", AccountID: "acct_1", ProviderID: "openai"})
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.real", account.ModuleID)
	require.Equal(t, "acct_1", account.AccountID)

	require.NoError(t, client.WriteAuditLog(ctx, CoreBridgeAuditLog{
		ModuleID:     "spoofed",
		Action:       "provider.account.test",
		ResourceType: "account",
		ResourceID:   "acct_1",
	}))

	config, err := client.GetModuleConfig(ctx, CoreBridgeModuleConfigRequest{ModuleID: "spoofed", Key: "provider"})
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.real", config.ModuleID)
	require.Equal(t, "provider", config.Key)

	heartbeat := time.Now().UTC()
	require.NoError(t, client.UpdateProviderRuntimeStatus(ctx, CoreBridgeRuntimeStatusRequest{
		ModuleID:        "spoofed",
		ProviderID:      "openai",
		Status:          RuntimeStatusRunning,
		LastHeartbeatAt: &heartbeat,
	}))

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	require.Equal(t, "lightbridge.provider.real", bridge.lastUserRequest.ModuleID)
	require.Equal(t, "lightbridge.provider.real", bridge.lastAccountRequest.ModuleID)
	require.Equal(t, "lightbridge.provider.real", bridge.lastAuditLog.ModuleID)
	require.Equal(t, "lightbridge.provider.real", bridge.lastConfigRequest.ModuleID)
	require.Equal(t, "lightbridge.provider.real", bridge.lastRuntimeStatus.ModuleID)
}

func TestCoreBridgeServerCloseRemovesSocket(t *testing.T) {
	socketPath := CoreBridgeSocketPath(t.TempDir(), "lightbridge.provider.real")
	closer, err := StartCoreBridgeServer("lightbridge.provider.real", socketPath, &fakeCoreBridge{})
	require.NoError(t, err)
	require.FileExists(t, socketPath)

	require.NoError(t, closer.Close())
	require.NoFileExists(t, socketPath)
}

type fakeCoreBridge struct {
	mu                 sync.Mutex
	lastUserRequest    CoreBridgeUserRequest
	lastAccountRequest CoreBridgeAccountRequest
	lastAuditLog       CoreBridgeAuditLog
	lastConfigRequest  CoreBridgeModuleConfigRequest
	lastRuntimeStatus  CoreBridgeRuntimeStatusRequest
}

func (b *fakeCoreBridge) GetUserSummary(_ context.Context, req CoreBridgeUserRequest) (*CoreBridgeUserSummary, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastUserRequest = req
	return &CoreBridgeUserSummary{
		UserID:   req.UserID,
		Username: "test-user",
		Email:    "test@example.com",
		Enabled:  true,
		Groups:   []string{"default"},
	}, nil
}

func (b *fakeCoreBridge) GetAccountCredentials(_ context.Context, req CoreBridgeAccountRequest) (*CoreBridgeAccountCredentials, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastAccountRequest = req
	return &CoreBridgeAccountCredentials{
		AccountID:   req.AccountID,
		ProviderID:  req.ProviderID,
		DisplayName: "Test Account",
		Config:      map[string]any{"model": "test-model"},
		Secrets:     map[string]any{"api_key": "secret"},
	}, nil
}

func (b *fakeCoreBridge) WriteAuditLog(_ context.Context, req CoreBridgeAuditLog) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastAuditLog = req
	return nil
}

func (b *fakeCoreBridge) GetModuleConfig(_ context.Context, req CoreBridgeModuleConfigRequest) (*CoreBridgeModuleConfig, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastConfigRequest = req
	return &CoreBridgeModuleConfig{
		Key:    req.Key,
		Config: map[string]any{"enabled": true},
	}, nil
}

func (b *fakeCoreBridge) UpdateProviderRuntimeStatus(_ context.Context, req CoreBridgeRuntimeStatusRequest) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastRuntimeStatus = req
	return nil
}

func requireSocketRemoved(t *testing.T, path string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return os.IsNotExist(err)
	}, 2*time.Second, 25*time.Millisecond)
}
