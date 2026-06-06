package service

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
	infraerrors "github.com/Wei-Shaw/LightBridge/internal/pkg/errors"
)

type coreBridgeUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}

type coreBridgeAccountReader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type ProductionCoreBridge struct {
	store        modules.Store
	runtimeStore modules.RuntimeStore
	userRepo     coreBridgeUserReader
	accountRepo  coreBridgeAccountReader
}

func ProvideCoreBridge(store modules.Store, runtimeStore modules.RuntimeStore, userRepo UserRepository, accountRepo AccountRepository) modules.CoreBridge {
	return NewProductionCoreBridge(store, runtimeStore, userRepo, accountRepo)
}

func NewProductionCoreBridge(store modules.Store, runtimeStore modules.RuntimeStore, userRepo coreBridgeUserReader, accountRepo coreBridgeAccountReader) *ProductionCoreBridge {
	return &ProductionCoreBridge{
		store:        store,
		runtimeStore: runtimeStore,
		userRepo:     userRepo,
		accountRepo:  accountRepo,
	}
}

func (b *ProductionCoreBridge) GetUserSummary(ctx context.Context, req modules.CoreBridgeUserRequest) (*modules.CoreBridgeUserSummary, error) {
	moduleID, err := b.requireModule(ctx, req.ModuleID)
	if err != nil {
		return nil, err
	}
	if b.userRepo == nil {
		return nil, infraerrors.ServiceUnavailable("CORE_BRIDGE_USER_REPO_UNAVAILABLE", "user service is not available to modules")
	}

	var user *User
	if id := strings.TrimSpace(req.UserID); id != "" {
		userID, err := strconv.ParseInt(id, 10, 64)
		if err != nil || userID <= 0 {
			return nil, infraerrors.BadRequest("CORE_BRIDGE_INVALID_USER_ID", "user id must be a positive integer")
		}
		user, err = b.userRepo.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}
	} else if email := strings.TrimSpace(req.Email); email != "" {
		var err error
		user, err = b.userRepo.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, infraerrors.BadRequest("CORE_BRIDGE_USER_SELECTOR_REQUIRED", "user id or email is required")
	}
	if user == nil {
		return nil, infraerrors.NotFound("CORE_BRIDGE_USER_NOT_FOUND", "user not found")
	}

	groups := make([]string, 0, len(user.AllowedGroups))
	for _, groupID := range user.AllowedGroups {
		groups = append(groups, strconv.FormatInt(groupID, 10))
	}
	return &modules.CoreBridgeUserSummary{
		ModuleID: moduleID,
		UserID:   strconv.FormatInt(user.ID, 10),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		Groups:   groups,
		Enabled:  user.IsActive(),
		Metadata: map[string]any{
			"status": user.Status,
		},
	}, nil
}

func (b *ProductionCoreBridge) GetAccountCredentials(ctx context.Context, req modules.CoreBridgeAccountRequest) (*modules.CoreBridgeAccountCredentials, error) {
	moduleID, err := b.requireModule(ctx, req.ModuleID)
	if err != nil {
		return nil, err
	}
	if b.accountRepo == nil {
		return nil, infraerrors.ServiceUnavailable("CORE_BRIDGE_ACCOUNT_REPO_UNAVAILABLE", "account service is not available to modules")
	}

	accountID, err := strconv.ParseInt(strings.TrimSpace(req.AccountID), 10, 64)
	if err != nil || accountID <= 0 {
		return nil, infraerrors.BadRequest("CORE_BRIDGE_INVALID_ACCOUNT_ID", "account id must be a positive integer")
	}
	account, err := b.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, infraerrors.NotFound("CORE_BRIDGE_ACCOUNT_NOT_FOUND", "account not found")
	}
	if !accountUsesModuleProvider(account) {
		return nil, infraerrors.Forbidden("CORE_BRIDGE_ACCOUNT_NOT_MODULE_PROVIDER", "module can only read module provider accounts")
	}

	effectiveProviderID := effectiveServiceProviderID(account)
	requestProviderID := strings.TrimSpace(req.ProviderID)
	if requestProviderID == "" {
		requestProviderID = effectiveProviderID
	}
	if effectiveProviderID == "" {
		return nil, infraerrors.Forbidden("CORE_BRIDGE_ACCOUNT_PROVIDER_REQUIRED", "module provider account has no provider id")
	}
	if requestProviderID != effectiveProviderID {
		return nil, infraerrors.Forbidden("CORE_BRIDGE_ACCOUNT_PROVIDER_MISMATCH", "module provider account does not match requested provider")
	}
	if ownerModuleID := moduleIDFromAccount(account); ownerModuleID != "" && ownerModuleID != moduleID {
		return nil, infraerrors.Forbidden("CORE_BRIDGE_ACCOUNT_MODULE_MISMATCH", "module provider account belongs to another module")
	}
	if ownerModuleID := moduleIDFromAccount(account); ownerModuleID == "" && effectiveProviderID != moduleID {
		return nil, infraerrors.Forbidden("CORE_BRIDGE_ACCOUNT_MODULE_MISMATCH", "module provider account is not owned by this module")
	}

	providerAccount := providerAccountFromService(account, effectiveProviderID)
	approvedSecretKeys := b.moduleApprovedSecretKeys(ctx, moduleID)
	secrets := filterApprovedSecrets(providerAccount.Secrets, approvedSecretKeys)
	if len(secrets) > 0 {
		b.writeSecretReadAudit(ctx, moduleID, providerAccount.ID, effectiveProviderID, secretKeyList(secrets), req.Purpose, req.CredentialRef)
	}

	return &modules.CoreBridgeAccountCredentials{
		ModuleID:      moduleID,
		AccountID:     providerAccount.ID,
		ProviderID:    providerAccount.ProviderID,
		DisplayName:   providerAccount.DisplayName,
		CredentialRef: strings.TrimSpace(req.CredentialRef),
		Config:        providerAccount.Config,
		Secrets:       secrets,
		Metadata:      providerAccount.Metadata,
		ExpiresAt:     account.ExpiresAt,
	}, nil
}

func (b *ProductionCoreBridge) WriteAuditLog(ctx context.Context, req modules.CoreBridgeAuditLog) error {
	moduleID, err := b.requireModule(ctx, req.ModuleID)
	if err != nil {
		return err
	}
	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.UTC()
	}
	level := slog.LevelInfo
	if strings.EqualFold(req.Severity, "error") || strings.EqualFold(req.Severity, "critical") {
		level = slog.LevelError
	} else if strings.EqualFold(req.Severity, "warn") || strings.EqualFold(req.Severity, "warning") {
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, "module audit event",
		"module_id", moduleID,
		"actor_user_id", req.ActorUserID,
		"action", req.Action,
		"resource_type", req.ResourceType,
		"resource_id", req.ResourceID,
		"severity", req.Severity,
		"message", req.Message,
		"metadata", req.Metadata,
		"occurred_at", occurredAt,
	)
	return nil
}

func (b *ProductionCoreBridge) GetModuleConfig(ctx context.Context, req modules.CoreBridgeModuleConfigRequest) (*modules.CoreBridgeModuleConfig, error) {
	moduleID, err := b.requireModule(ctx, req.ModuleID)
	if err != nil {
		return nil, err
	}
	installed, err := b.store.GetInstalled(ctx, moduleID)
	if err != nil {
		return nil, err
	}
	if installed == nil {
		return nil, infraerrors.NotFound("CORE_BRIDGE_MODULE_NOT_FOUND", "module not found")
	}

	key := strings.TrimSpace(req.Key)
	config := map[string]any{
		"id":           installed.ID,
		"name":         installed.Name,
		"type":         installed.Type,
		"version":      installed.Version,
		"status":       installed.Status,
		"capabilities": installed.Manifest.Capabilities,
	}
	switch key {
	case "", "summary":
		return &modules.CoreBridgeModuleConfig{ModuleID: moduleID, Key: key, Config: config}, nil
	case "manifest":
		return &modules.CoreBridgeModuleConfig{ModuleID: moduleID, Key: key, Value: installed.Manifest}, nil
	case "permissions":
		permissions, err := b.store.ListPermissions(ctx, moduleID)
		if err != nil {
			return nil, err
		}
		return &modules.CoreBridgeModuleConfig{ModuleID: moduleID, Key: key, Value: permissions}, nil
	case "frontend":
		return &modules.CoreBridgeModuleConfig{ModuleID: moduleID, Key: key, Value: installed.Manifest.Frontend}, nil
	case "backend":
		return &modules.CoreBridgeModuleConfig{ModuleID: moduleID, Key: key, Value: installed.Manifest.Backend}, nil
	default:
		return nil, infraerrors.NotFound("CORE_BRIDGE_MODULE_CONFIG_NOT_FOUND", "module config key not found")
	}
}

func (b *ProductionCoreBridge) UpdateProviderRuntimeStatus(ctx context.Context, req modules.CoreBridgeRuntimeStatusRequest) error {
	moduleID, err := b.requireModule(ctx, req.ModuleID)
	if err != nil {
		return err
	}
	if b.runtimeStore == nil {
		return infraerrors.ServiceUnavailable("CORE_BRIDGE_RUNTIME_STORE_UNAVAILABLE", "runtime store is not available to modules")
	}
	return b.runtimeStore.UpdateRuntimeInstance(ctx, modules.RuntimeInstanceUpdate{
		ModuleID:        moduleID,
		Status:          req.Status,
		LastHeartbeatAt: req.LastHeartbeatAt,
		LastError:       coreBridgeFirstNonEmpty(req.LastError, req.Message),
	})
}

func (b *ProductionCoreBridge) requireModule(ctx context.Context, moduleID string) (string, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return "", infraerrors.BadRequest("CORE_BRIDGE_MODULE_ID_REQUIRED", "module id is required")
	}
	if b == nil || b.store == nil {
		return "", infraerrors.ServiceUnavailable("CORE_BRIDGE_STORE_UNAVAILABLE", "module store is not available")
	}
	installed, err := b.store.GetInstalled(ctx, moduleID)
	if err != nil {
		return "", err
	}
	if installed == nil {
		return "", infraerrors.NotFound("CORE_BRIDGE_MODULE_NOT_FOUND", "module not found")
	}
	return moduleID, nil
}

func (b *ProductionCoreBridge) moduleApprovedSecretKeys(ctx context.Context, moduleID string) map[string]struct{} {
	if b == nil || b.store == nil {
		return nil
	}
	permissions, err := b.store.ListPermissions(ctx, moduleID)
	if err != nil {
		slog.Warn("failed to check module secret permissions", "module_id", moduleID, "error", err)
		return nil
	}
	allowed := make(map[string]struct{})
	for _, permission := range permissions {
		if !permission.Approved || !strings.EqualFold(permission.PermissionType, "secrets") {
			continue
		}
		key := strings.TrimSpace(permission.PermissionValue)
		if key != "" {
			allowed[key] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func filterApprovedSecrets(secrets map[string]any, allowed map[string]struct{}) map[string]any {
	if len(secrets) == 0 || len(allowed) == 0 {
		return nil
	}
	filtered := make(map[string]any)
	for key, value := range secrets {
		if _, ok := allowed[key]; ok {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func secretKeyList(secrets map[string]any) []string {
	if len(secrets) == 0 {
		return nil
	}
	keys := make([]string, 0, len(secrets))
	for key := range secrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (b *ProductionCoreBridge) writeSecretReadAudit(ctx context.Context, moduleID string, accountID string, providerID string, secretKeys []string, purpose string, credentialRef string) {
	slog.Log(ctx, slog.LevelInfo, "module secret read",
		"module_id", moduleID,
		"account_id", accountID,
		"provider_id", providerID,
		"secret_keys", secretKeys,
		"purpose", strings.TrimSpace(purpose),
		"credential_ref", strings.TrimSpace(credentialRef),
		"action", "module.secret.read",
	)
}

func moduleIDFromAccount(account *Account) string {
	if account == nil || account.Extra == nil {
		return ""
	}
	if raw, ok := account.Extra["module_id"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func coreBridgeFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var _ modules.CoreBridge = (*ProductionCoreBridge)(nil)
