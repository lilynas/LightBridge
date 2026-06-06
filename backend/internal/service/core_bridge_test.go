package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
	infraerrors "github.com/Wei-Shaw/LightBridge/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

const coreBridgeTestModuleID = "lightbridge.provider.test"

func TestProductionCoreBridgeGetModuleConfig(t *testing.T) {
	store := newCoreBridgeTestStore()
	bridge := NewProductionCoreBridge(store, store, nil, nil)

	summary, err := bridge.GetModuleConfig(context.Background(), modules.CoreBridgeModuleConfigRequest{
		ModuleID: coreBridgeTestModuleID,
		Key:      "summary",
	})
	require.NoError(t, err)
	require.Equal(t, coreBridgeTestModuleID, summary.Config["id"])
	require.Equal(t, modules.ModuleStatusEnabled, summary.Config["status"])
	require.Equal(t, []modules.Capability{modules.CapabilityProviderAdapter}, summary.Config["capabilities"])

	manifest, err := bridge.GetModuleConfig(context.Background(), modules.CoreBridgeModuleConfigRequest{
		ModuleID: coreBridgeTestModuleID,
		Key:      "manifest",
	})
	require.NoError(t, err)
	require.IsType(t, modules.Manifest{}, manifest.Value)
	require.Equal(t, coreBridgeTestModuleID, manifest.Value.(modules.Manifest).ID)

	permissions, err := bridge.GetModuleConfig(context.Background(), modules.CoreBridgeModuleConfigRequest{
		ModuleID: coreBridgeTestModuleID,
		Key:      "permissions",
	})
	require.NoError(t, err)
	require.Len(t, permissions.Value, 1)
}

func TestProductionCoreBridgeUpdateProviderRuntimeStatusUsesCallerModule(t *testing.T) {
	store := newCoreBridgeTestStore()
	bridge := NewProductionCoreBridge(store, store, nil, nil)

	now := time.Now().UTC()
	err := bridge.UpdateProviderRuntimeStatus(context.Background(), modules.CoreBridgeRuntimeStatusRequest{
		ModuleID:        coreBridgeTestModuleID,
		ProviderID:      "lightbridge.provider.other",
		Status:          modules.RuntimeStatusRunning,
		Message:         "ignored when last error exists",
		LastError:       "last error wins",
		LastHeartbeatAt: &now,
	})
	require.NoError(t, err)
	require.Equal(t, coreBridgeTestModuleID, store.lastRuntimeUpdate.ModuleID)
	require.Equal(t, modules.RuntimeStatusRunning, store.lastRuntimeUpdate.Status)
	require.Equal(t, "last error wins", store.lastRuntimeUpdate.LastError)
	require.Equal(t, &now, store.lastRuntimeUpdate.LastHeartbeatAt)
}

func TestProductionCoreBridgeGetUserSummary(t *testing.T) {
	store := newCoreBridgeTestStore()
	users := &coreBridgeTestUserReader{
		byID: map[int64]*User{
			7: {
				ID:            7,
				Username:      "module-user",
				Email:         "user@example.test",
				Role:          RoleUser,
				Status:        StatusActive,
				AllowedGroups: []int64{10, 11},
			},
		},
	}
	users.byEmail = map[string]*User{"user@example.test": users.byID[7]}
	bridge := NewProductionCoreBridge(store, store, users, nil)

	byID, err := bridge.GetUserSummary(context.Background(), modules.CoreBridgeUserRequest{
		ModuleID: coreBridgeTestModuleID,
		UserID:   "7",
	})
	require.NoError(t, err)
	require.Equal(t, "7", byID.UserID)
	require.Equal(t, []string{"10", "11"}, byID.Groups)
	require.True(t, byID.Enabled)

	byEmail, err := bridge.GetUserSummary(context.Background(), modules.CoreBridgeUserRequest{
		ModuleID: coreBridgeTestModuleID,
		Email:    "user@example.test",
	})
	require.NoError(t, err)
	require.Equal(t, byID.UserID, byEmail.UserID)
}

func TestProductionCoreBridgeGetAccountCredentialsRejectsLegacyProviderAccount(t *testing.T) {
	store := newCoreBridgeTestStore()
	accounts := &coreBridgeTestAccountReader{accounts: map[int64]*Account{
		1: {
			ID:       1,
			Name:     "legacy openai",
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		},
	}}
	bridge := NewProductionCoreBridge(store, store, nil, accounts)

	_, err := bridge.GetAccountCredentials(context.Background(), modules.CoreBridgeAccountRequest{
		ModuleID:  coreBridgeTestModuleID,
		AccountID: "1",
	})
	require.Error(t, err)
	require.Equal(t, 403, infraerrors.Code(err))
	require.Equal(t, "CORE_BRIDGE_ACCOUNT_NOT_MODULE_PROVIDER", infraerrors.Reason(err))
}

func TestProductionCoreBridgeGetAccountCredentialsRejectsOtherModuleAccount(t *testing.T) {
	store := newCoreBridgeTestStore()
	accounts := &coreBridgeTestAccountReader{accounts: map[int64]*Account{
		2: coreBridgeModuleAccount(2, "lightbridge.provider.other"),
	}}
	bridge := NewProductionCoreBridge(store, store, nil, accounts)

	_, err := bridge.GetAccountCredentials(context.Background(), modules.CoreBridgeAccountRequest{
		ModuleID:  coreBridgeTestModuleID,
		AccountID: "2",
	})
	require.Error(t, err)
	require.Equal(t, 403, infraerrors.Code(err))
	require.Equal(t, "CORE_BRIDGE_ACCOUNT_MODULE_MISMATCH", infraerrors.Reason(err))
}

func TestProductionCoreBridgeGetAccountCredentialsHidesSecretsWithoutApprovedPermission(t *testing.T) {
	store := newCoreBridgeTestStore()
	store.permissions[coreBridgeTestModuleID] = nil
	accounts := &coreBridgeTestAccountReader{accounts: map[int64]*Account{
		3: coreBridgeModuleAccount(3, coreBridgeTestModuleID),
	}}
	bridge := NewProductionCoreBridge(store, store, nil, accounts)

	credentials, err := bridge.GetAccountCredentials(context.Background(), modules.CoreBridgeAccountRequest{
		ModuleID:      coreBridgeTestModuleID,
		AccountID:     "3",
		CredentialRef: "runtime-account",
	})
	require.NoError(t, err)
	require.Equal(t, coreBridgeTestModuleID, credentials.ModuleID)
	require.Equal(t, "3", credentials.AccountID)
	require.Equal(t, coreBridgeTestModuleID, credentials.ProviderID)
	require.Equal(t, "runtime-account", credentials.CredentialRef)
	require.Equal(t, "config-value", credentials.Config["config_key"])
	require.Nil(t, credentials.Secrets)
}

func TestProductionCoreBridgeGetAccountCredentialsReturnsSecretsWithApprovedPermission(t *testing.T) {
	store := newCoreBridgeTestStore()
	store.permissions[coreBridgeTestModuleID] = []modules.PermissionRecord{
		{
			ModuleID:        coreBridgeTestModuleID,
			PermissionType:  "secrets",
			PermissionValue: "api_key",
			Approved:        true,
		},
	}
	accounts := &coreBridgeTestAccountReader{accounts: map[int64]*Account{
		4: coreBridgeModuleAccount(4, coreBridgeTestModuleID),
	}}
	bridge := NewProductionCoreBridge(store, store, nil, accounts)

	credentials, err := bridge.GetAccountCredentials(context.Background(), modules.CoreBridgeAccountRequest{
		ModuleID:  coreBridgeTestModuleID,
		AccountID: "4",
	})
	require.NoError(t, err)
	require.Equal(t, "secret-value", credentials.Secrets["api_key"])
	require.NotContains(t, credentials.Secrets, "refresh_token")
	require.Equal(t, coreBridgeTestModuleID, credentials.Config["module_id"])
}

type coreBridgeTestStore struct {
	installed         map[string]modules.InstalledModule
	permissions       map[string][]modules.PermissionRecord
	lastRuntimeUpdate modules.RuntimeInstanceUpdate
}

func newCoreBridgeTestStore() *coreBridgeTestStore {
	return &coreBridgeTestStore{
		installed: map[string]modules.InstalledModule{
			coreBridgeTestModuleID: {
				ID:          coreBridgeTestModuleID,
				Name:        "Test Provider",
				Type:        modules.ModuleTypeProvider,
				Version:     "0.1.0",
				Status:      modules.ModuleStatusEnabled,
				InstallPath: "/tmp/lightbridge-provider-test",
				Manifest: modules.Manifest{
					APIVersion:   modules.ManifestAPIVersionV1Alpha1,
					ID:           coreBridgeTestModuleID,
					Name:         "Test Provider",
					Type:         modules.ModuleTypeProvider,
					Version:      "0.1.0",
					Capabilities: []modules.Capability{modules.CapabilityProviderAdapter},
					Backend: &modules.BackendSpec{
						Kind:     modules.BackendKindSidecar,
						Command:  "./backend/darwin-arm64/provider-test",
						Protocol: modules.BackendProtocolGRPC,
					},
					Frontend: &modules.FrontendSpec{
						Kind:  modules.FrontendKindViteRemoteESM,
						Entry: "./frontend/remoteEntry.js",
					},
				},
			},
		},
		permissions: map[string][]modules.PermissionRecord{
			coreBridgeTestModuleID: {
				{
					ModuleID:        coreBridgeTestModuleID,
					PermissionType:  "network",
					PermissionValue: "https://example.test/*",
					Approved:        true,
				},
			},
		},
	}
}

func (s *coreBridgeTestStore) ListInstalled(context.Context) ([]modules.InstalledModule, error) {
	out := make([]modules.InstalledModule, 0, len(s.installed))
	for _, module := range s.installed {
		out = append(out, module)
	}
	return out, nil
}

func (s *coreBridgeTestStore) GetInstalled(_ context.Context, id string) (*modules.InstalledModule, error) {
	module, ok := s.installed[id]
	if !ok {
		return nil, nil
	}
	return &module, nil
}

func (s *coreBridgeTestStore) SaveInstalled(_ context.Context, module modules.InstalledModule) error {
	s.installed[module.ID] = module
	return nil
}

func (s *coreBridgeTestStore) SavePermissions(_ context.Context, moduleID string, permissions []modules.PermissionRecord) error {
	s.permissions[moduleID] = permissions
	return nil
}

func (s *coreBridgeTestStore) ListPermissions(_ context.Context, moduleID string) ([]modules.PermissionRecord, error) {
	return s.permissions[moduleID], nil
}

func (s *coreBridgeTestStore) ApprovePermissions(_ context.Context, moduleID string) error {
	permissions := s.permissions[moduleID]
	for i := range permissions {
		permissions[i].Approved = true
	}
	s.permissions[moduleID] = permissions
	return nil
}

func (s *coreBridgeTestStore) ApplyMigration(context.Context, string, string, string, string) error {
	return nil
}

func (s *coreBridgeTestStore) SetStatus(_ context.Context, id string, status modules.ModuleStatus, lastError string) error {
	module := s.installed[id]
	module.Status = status
	module.LastError = lastError
	s.installed[id] = module
	return nil
}

func (s *coreBridgeTestStore) UpdateRuntimeInstance(_ context.Context, update modules.RuntimeInstanceUpdate) error {
	s.lastRuntimeUpdate = update
	return nil
}

type coreBridgeTestUserReader struct {
	byID    map[int64]*User
	byEmail map[string]*User
}

func (r *coreBridgeTestUserReader) GetByID(_ context.Context, id int64) (*User, error) {
	return r.byID[id], nil
}

func (r *coreBridgeTestUserReader) GetByEmail(_ context.Context, email string) (*User, error) {
	return r.byEmail[email], nil
}

type coreBridgeTestAccountReader struct {
	accounts map[int64]*Account
}

func (r *coreBridgeTestAccountReader) GetByID(_ context.Context, id int64) (*Account, error) {
	return r.accounts[id], nil
}

func coreBridgeModuleAccount(id int64, moduleID string) *Account {
	return &Account{
		ID:         id,
		Name:       "module account",
		Platform:   PlatformModule,
		ProviderID: moduleID,
		Type:       AccountTypeModule,
		Credentials: map[string]any{
			"api_key":       "secret-value",
			"refresh_token": "refresh-secret",
		},
		Extra: map[string]any{
			"module_id":   moduleID,
			"provider_id": moduleID,
			"config_key":  "config-value",
		},
		Status: StatusActive,
	}
}
