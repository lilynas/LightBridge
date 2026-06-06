package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
	infraerrors "github.com/Wei-Shaw/LightBridge/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestModuleServiceEnable(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = approvedPermissions(manifest.ID)
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}

	svc := NewModuleService(store)
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusEnabled, got.Status)
	require.Equal(t, modules.ModuleStatusEnabled, store.items[manifest.ID].Status)
	require.NotNil(t, store.items[manifest.ID].EnabledAt)
}

func TestModuleServiceEnableStartsProviderRuntime(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = approvedPermissions(manifest.ID)
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{}
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusEnabled, got.Status)
	require.Equal(t, []string{manifest.ID}, runtime.started)
}

func TestModuleServiceEnableMarksFailedWhenProviderRuntimeFails(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = approvedPermissions(manifest.ID)
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{startErr: errors.New("sidecar failed")}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{}
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.Equal(t, "MODULE_RUNTIME_START_FAILED", infraerrors.Reason(err))
	require.Equal(t, modules.ModuleStatusFailed, store.items[manifest.ID].Status)
	require.Equal(t, "sidecar failed", store.items[manifest.ID].LastError)
}

func TestModuleServiceDisableStopsProviderRuntime(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	got, err := svc.Disable(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusDisabled, got.Status)
	require.Equal(t, []string{manifest.ID}, runtime.stopped)
}

func TestModuleServiceEnableVerifiesInstalledPackageBeforeRuntime(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = approvedPermissions(manifest.ID)
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        "Stale DB Name",
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	verified := store.items[manifest.ID]
	verified.Name = manifest.Name
	verified.Manifest = manifest
	runtime := &fakeProviderRuntime{}
	verifier := &fakeModuleInstaller{verifiedModule: &verified}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = verifier
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusEnabled, got.Status)
	require.Equal(t, []string{manifest.ID}, runtime.started)
	require.Len(t, runtime.startedModules, 1)
	require.Equal(t, manifest.Name, runtime.startedModules[0].Name)
	require.Len(t, verifier.verified, 1)
	require.Equal(t, manifest.ID, verifier.verified[0].ID)
}

func TestModuleServiceEnableMarksFailedWhenPackageVerificationFails(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = approvedPermissions(manifest.ID)
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{verifyErr: errors.New("checksum mismatch")}
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.Equal(t, "MODULE_PACKAGE_VERIFY_FAILED", infraerrors.Reason(err))
	require.Empty(t, runtime.started)
	require.Equal(t, modules.ModuleStatusFailed, store.items[manifest.ID].Status)
	require.Equal(t, "checksum mismatch", store.items[manifest.ID].LastError)
}

func TestModuleServiceDisableKeepsModuleFiles(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	baseDir := t.TempDir()
	installPath := modules.InstallDir(baseDir, manifest.ID, manifest.Version)
	require.NoError(t, os.MkdirAll(installPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "module.yaml"), []byte("manifest"), 0o644))
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: installPath,
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}

	svc := NewModuleService(store)
	svc.moduleDataDir = baseDir
	got, err := svc.Disable(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusDisabled, got.Status)
	require.FileExists(t, filepath.Join(installPath, "module.yaml"))
}

func TestModuleServiceUninstallDeletesModuleFilesOnly(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	baseDir := t.TempDir()
	installPath := modules.InstallDir(baseDir, manifest.ID, manifest.Version)
	require.NoError(t, os.MkdirAll(installPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "module.yaml"), []byte("manifest"), 0o644))
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: installPath,
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleDataDir = baseDir
	got, err := svc.Uninstall(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusUninstalled, got.Status)
	require.NoDirExists(t, installPath)
	require.Equal(t, []string{manifest.ID}, runtime.stopped)
	require.False(t, store.purgedData[manifest.ID])
}

func TestModuleServicePurgeDeletesModuleFilesAndPrivateData(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	manifest.Permissions.Database = []string{"provider_openai_*"}
	baseDir := t.TempDir()
	installPath := modules.InstallDir(baseDir, manifest.ID, manifest.Version)
	require.NoError(t, os.MkdirAll(installPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "module.yaml"), []byte("manifest"), 0o644))
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusDisabled,
		InstallPath: installPath,
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}

	svc := NewModuleService(store)
	svc.moduleDataDir = baseDir
	got, err := svc.Purge(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.Equal(t, modules.ModuleStatusPurged, got.Status)
	require.NoDirExists(t, installPath)
	require.True(t, store.purgedData[manifest.ID])
}

func TestRemoveModuleInstallPathRejectsUnexpectedPath(t *testing.T) {
	baseDir := t.TempDir()
	manifest := serviceTestModuleManifest()
	expectedPath := modules.InstallDir(baseDir, manifest.ID, manifest.Version)
	require.NoError(t, os.MkdirAll(expectedPath, 0o755))
	outside := filepath.Join(t.TempDir(), "module")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	err := removeModuleInstallPath(baseDir, manifest.ID, manifest.Version, outside)
	require.ErrorContains(t, err, "does not match expected module directory")
	require.DirExists(t, outside)
	require.DirExists(t, expectedPath)
}

func TestModuleServiceEnableRejectsUninstalled(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusUninstalled,
		Manifest: manifest,
	}

	svc := NewModuleService(store)
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 409, infraerrors.Code(err))
	require.Equal(t, "MODULE_UNINSTALLED", infraerrors.Reason(err))
}

func TestModuleServiceEnableRejectsUnapprovedPermissions(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.permissions[manifest.ID] = []modules.PermissionRecord{{
		ModuleID:        manifest.ID,
		PermissionType:  "network",
		PermissionValue: "https://api.openai.com/*",
		Approved:        false,
	}}
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusInstalled,
		Manifest: manifest,
	}

	svc := NewModuleService(store)
	got, err := svc.Enable(context.Background(), manifest.ID)
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 409, infraerrors.Code(err))
	require.Equal(t, "MODULE_PERMISSIONS_NOT_APPROVED", infraerrors.Reason(err))
}

func TestModuleServiceApprovePermissions(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusInstalled,
		Manifest: manifest,
	}
	store.permissions[manifest.ID] = []modules.PermissionRecord{{
		ModuleID:        manifest.ID,
		PermissionType:  "network",
		PermissionValue: "https://api.openai.com/*",
		Approved:        false,
	}}

	svc := NewModuleService(store)
	got, err := svc.ApprovePermissions(context.Background(), manifest.ID)
	require.NoError(t, err)
	require.True(t, got.Approved)
	require.True(t, got.Permissions[0].Approved)
}

func TestModuleServiceUIManifestOnlyEnabledFrontendModules(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusEnabled,
		Manifest: manifest,
	}
	disabled := manifest
	disabled.ID = "lightbridge.provider.disabled"
	disabled.Name = "Disabled Provider"
	store.items[disabled.ID] = modules.InstalledModule{
		ID:       disabled.ID,
		Name:     disabled.Name,
		Type:     disabled.Type,
		Version:  disabled.Version,
		Status:   modules.ModuleStatusDisabled,
		Manifest: disabled,
	}

	svc := NewModuleService(store)
	got, err := svc.UIManifest(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, manifest.ID, got[0].ModuleID)
	require.Equal(t, "/modules/lightbridge.provider.openai-api/0.1.0/frontend/remoteEntry.js", got[0].RemoteEntry)
	require.Len(t, got[0].Routes, 1)
	require.Equal(t, got[0].RemoteEntry, got[0].Routes[0].RemoteEntry)
	require.Equal(t, "./OpenAIProviderSettings", got[0].Routes[0].ExposedModule)
	require.Len(t, got[0].AccountForms, 1)
	require.Equal(t, "./OpenAIAccountForm", got[0].AccountForms[0].ExposedModule)
}

func TestModuleServiceGetInstalledMapsMissingModule(t *testing.T) {
	svc := NewModuleService(newFakeModuleStore())
	got, err := svc.GetInstalled(context.Background(), "missing")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 404, infraerrors.Code(err))
	require.Equal(t, "MODULE_NOT_FOUND", infraerrors.Reason(err))
}

func TestModuleServiceProviderAdaptersReturnsRegisteredAdaptersSorted(t *testing.T) {
	registry := modules.NewProviderRegistry()
	registry.Register(fakeProviderAdapter{id: "lightbridge.provider.zed"})
	registry.Register(fakeProviderAdapter{id: "lightbridge.provider.alpha"})
	svc := ProvideModuleService(nil, newFakeModuleStore(), nil, nil, registry)

	got, err := svc.ProviderAdapters(context.Background())
	require.NoError(t, err)
	require.Equal(t, []ModuleProviderAdapterStatus{
		{ID: "lightbridge.provider.alpha", Status: "registered"},
		{ID: "lightbridge.provider.zed", Status: "registered"},
	}, got)
}

func TestModuleServiceMarketplaceReturnsEmptyWhenUnconfigured(t *testing.T) {
	svc := NewModuleService(newFakeModuleStore())

	got, err := svc.Marketplace(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got.Modules)
}

func TestModuleServiceMarketplaceLoadsLocalRegistryAndInstalledStatus(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusEnabled,
		Manifest: manifest,
	}
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           manifest.ID,
		Name:         manifest.Name,
		Type:         manifest.Type,
		Version:      manifest.Version,
		Core:         manifest.Core.Compatible,
		DownloadURL:  "/tmp/lightbridge-module-openai.tar.zst",
		SHA256:       strings.Repeat("a", 64),
		Capabilities: manifest.Capabilities,
		Permissions:  manifest.Permissions,
	}}})
	svc := NewModuleService(store)
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.Marketplace(context.Background())
	require.NoError(t, err)
	require.Len(t, got.Modules, 1)
	require.Equal(t, manifest.ID, got.Modules[0].ID)
	require.Equal(t, modules.ModuleStatusEnabled, got.Modules[0].InstalledStatus)
	require.Equal(t, manifest.Version, got.Modules[0].InstalledVersion)
}

func TestModuleServiceMarketplaceRejectsUnsupportedCapability(t *testing.T) {
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           "lightbridge.provider.bad",
		Name:         "Bad Provider",
		Type:         modules.ModuleTypeProvider,
		Version:      "0.1.0",
		Core:         ">=0.1.0 <0.2.0",
		DownloadURL:  "/tmp/lightbridge-module-bad.tar.zst",
		Capabilities: []modules.Capability{"auth.factor"},
	}}})
	svc := NewModuleService(newFakeModuleStore())
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.Marketplace(context.Background())
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, 400, infraerrors.Code(err))
	require.Equal(t, "MODULE_MARKETPLACE_INVALID_ENTRY", infraerrors.Reason(err))
}

func TestModuleServiceInstallFromMarketplaceDownloadsPackageAndDelegatesInstaller(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	packagePath := filepath.Join(t.TempDir(), "lightbridge-module-openai.tar.zst")
	packageBytes := []byte("fake package")
	require.NoError(t, os.WriteFile(packagePath, packageBytes, 0o600))
	sum := sha256.Sum256(packageBytes)
	installer := &fakeModuleInstaller{installed: &modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusInstalled,
		Manifest: manifest,
	}}
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           manifest.ID,
		Name:         manifest.Name,
		Type:         manifest.Type,
		Version:      manifest.Version,
		Core:         manifest.Core.Compatible,
		DownloadURL:  packagePath,
		SHA256:       hex.EncodeToString(sum[:]),
		Capabilities: manifest.Capabilities,
		Permissions:  manifest.Permissions,
	}}})
	svc := NewModuleService(store)
	svc.installer = installer
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.InstallFromMarketplace(context.Background(), manifest.ID, manifest.Version)
	require.NoError(t, err)
	require.Equal(t, manifest.ID, got.ID)
	require.Len(t, installer.archivePaths, 1)
	require.Equal(t, packageBytes, installer.archiveBytes[0])
}

func TestModuleServiceUpgradeFromMarketplaceStopsRuntimeAndInstallsNewerVersion(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  "0.1.0",
		Status:   modules.ModuleStatusEnabled,
		Manifest: manifest,
	}
	nextManifest := manifest
	nextManifest.Version = "0.2.0"
	packagePath := filepath.Join(t.TempDir(), "lightbridge-module-openai.tar.zst")
	packageBytes := []byte("fake package 0.2.0")
	require.NoError(t, os.WriteFile(packagePath, packageBytes, 0o600))
	sum := sha256.Sum256(packageBytes)
	installer := &fakeModuleInstaller{installed: &modules.InstalledModule{
		ID:       nextManifest.ID,
		Name:     nextManifest.Name,
		Type:     nextManifest.Type,
		Version:  nextManifest.Version,
		Status:   modules.ModuleStatusInstalled,
		Manifest: nextManifest,
	}}
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           nextManifest.ID,
		Name:         nextManifest.Name,
		Type:         nextManifest.Type,
		Version:      nextManifest.Version,
		Core:         nextManifest.Core.Compatible,
		DownloadURL:  packagePath,
		SHA256:       hex.EncodeToString(sum[:]),
		Capabilities: nextManifest.Capabilities,
		Permissions:  nextManifest.Permissions,
	}}})
	runtime := &fakeProviderRuntime{}
	svc := NewModuleService(store)
	svc.installer = installer
	svc.providerRuntime = runtime
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.UpgradeFromMarketplace(context.Background(), manifest.ID, nextManifest.Version)
	require.NoError(t, err)
	require.Equal(t, "0.2.0", got.Version)
	require.Equal(t, []string{manifest.ID}, runtime.stopped)
	require.Len(t, installer.archivePaths, 1)
	require.Equal(t, packageBytes, installer.archiveBytes[0])
	require.Equal(t, modules.ModuleStatusDisabled, store.items[manifest.ID].Status)
}

func TestModuleServiceRollbackFromMarketplaceStopsRuntimeAndInstallsOlderVersion(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	currentManifest := manifest
	currentManifest.Version = "0.2.0"
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       currentManifest.ID,
		Name:     currentManifest.Name,
		Type:     currentManifest.Type,
		Version:  currentManifest.Version,
		Status:   modules.ModuleStatusEnabled,
		Manifest: currentManifest,
	}
	previousManifest := manifest
	previousManifest.Version = "0.1.0"
	packagePath := filepath.Join(t.TempDir(), "lightbridge-module-openai.tar.zst")
	packageBytes := []byte("fake package 0.1.0")
	require.NoError(t, os.WriteFile(packagePath, packageBytes, 0o600))
	sum := sha256.Sum256(packageBytes)
	installer := &fakeModuleInstaller{installed: &modules.InstalledModule{
		ID:       previousManifest.ID,
		Name:     previousManifest.Name,
		Type:     previousManifest.Type,
		Version:  previousManifest.Version,
		Status:   modules.ModuleStatusInstalled,
		Manifest: previousManifest,
	}}
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           previousManifest.ID,
		Name:         previousManifest.Name,
		Type:         previousManifest.Type,
		Version:      previousManifest.Version,
		Core:         previousManifest.Core.Compatible,
		DownloadURL:  packagePath,
		SHA256:       hex.EncodeToString(sum[:]),
		Capabilities: previousManifest.Capabilities,
		Permissions:  previousManifest.Permissions,
	}}})
	runtime := &fakeProviderRuntime{}
	svc := NewModuleService(store)
	svc.installer = installer
	svc.providerRuntime = runtime
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.RollbackFromMarketplace(context.Background(), manifest.ID, previousManifest.Version)
	require.NoError(t, err)
	require.Equal(t, "0.1.0", got.Version)
	require.Equal(t, []string{manifest.ID}, runtime.stopped)
	require.Len(t, installer.archivePaths, 1)
	require.Equal(t, packageBytes, installer.archiveBytes[0])
	require.Equal(t, modules.ModuleStatusDisabled, store.items[manifest.ID].Status)
}

func TestModuleServiceChangeMarketplaceVersionRejectsInvalidDirection(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  "0.2.0",
		Status:   modules.ModuleStatusDisabled,
		Manifest: manifest,
	}
	svc := NewModuleService(store)

	got, err := svc.UpgradeFromMarketplace(context.Background(), manifest.ID, "0.1.0")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "MODULE_UPGRADE_TARGET_NOT_NEWER", infraerrors.Reason(err))

	got, err = svc.RollbackFromMarketplace(context.Background(), manifest.ID, "0.3.0")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "MODULE_ROLLBACK_TARGET_NOT_OLDER", infraerrors.Reason(err))

	got, err = svc.UpgradeFromMarketplace(context.Background(), manifest.ID, "0.2.0")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "MODULE_VERSION_UNCHANGED", infraerrors.Reason(err))

	got, err = svc.UpgradeFromMarketplace(context.Background(), manifest.ID, "0.3.x")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, "MODULE_VERSION_INVALID", infraerrors.Reason(err))
}

func TestModuleServiceUpgradeFromMarketplaceMarksFailedWhenInstallFails(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  "0.1.0",
		Status:   modules.ModuleStatusEnabled,
		Manifest: manifest,
	}
	packagePath := filepath.Join(t.TempDir(), "lightbridge-module-openai.tar.zst")
	packageBytes := []byte("fake package 0.2.0")
	require.NoError(t, os.WriteFile(packagePath, packageBytes, 0o600))
	sum := sha256.Sum256(packageBytes)
	registryPath := writeMarketplaceRegistry(t, ModuleMarketplaceResult{Modules: []ModuleMarketplaceEntry{{
		ID:           manifest.ID,
		Name:         manifest.Name,
		Type:         manifest.Type,
		Version:      "0.2.0",
		Core:         manifest.Core.Compatible,
		DownloadURL:  packagePath,
		SHA256:       hex.EncodeToString(sum[:]),
		Capabilities: manifest.Capabilities,
		Permissions:  manifest.Permissions,
	}}})
	runtime := &fakeProviderRuntime{}
	svc := NewModuleService(store)
	svc.installer = &fakeModuleInstaller{err: errors.New("install failed")}
	svc.providerRuntime = runtime
	svc.marketplaceRegistryPath = registryPath

	got, err := svc.UpgradeFromMarketplace(context.Background(), manifest.ID, "0.2.0")
	require.Nil(t, got)
	require.Error(t, err)
	require.Equal(t, []string{manifest.ID}, runtime.stopped)
	require.Equal(t, modules.ModuleStatusFailed, store.items[manifest.ID].Status)
	require.Contains(t, store.items[manifest.ID].LastError, "install failed")
}

func TestModuleServiceStartEnabledModulesStartsOnlyEnabled(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	disabledManifest := manifest
	disabledManifest.ID = "lightbridge.provider.disabled"
	store.items[disabledManifest.ID] = modules.InstalledModule{
		ID:          disabledManifest.ID,
		Name:        disabledManifest.Name,
		Type:        disabledManifest.Type,
		Version:     disabledManifest.Version,
		Status:      modules.ModuleStatusDisabled,
		InstallPath: "/data/modules/lightbridge.provider.disabled/0.1.0",
		Manifest:    disabledManifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{}
	err := svc.StartEnabledModules(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{manifest.ID}, runtime.started)
}

func TestModuleServiceStartEnabledModulesMarksFailedRuntimeErrors(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{startErr: errors.New("sidecar failed")}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{}
	err := svc.StartEnabledModules(context.Background())
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.Equal(t, "MODULE_RUNTIME_RESTORE_FAILED", infraerrors.Reason(err))
	require.Equal(t, modules.ModuleStatusFailed, store.items[manifest.ID].Status)
	require.Equal(t, "sidecar failed", store.items[manifest.ID].LastError)
}

func TestModuleServiceStartEnabledModulesVerifiesInstalledPackageBeforeRuntime(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        "Stale DB Name",
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	verified := store.items[manifest.ID]
	verified.Name = manifest.Name
	verified.Manifest = manifest
	runtime := &fakeProviderRuntime{}
	verifier := &fakeModuleInstaller{verifiedModule: &verified}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = verifier
	err := svc.StartEnabledModules(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{manifest.ID}, runtime.started)
	require.Len(t, runtime.startedModules, 1)
	require.Equal(t, manifest.Name, runtime.startedModules[0].Name)
	require.Len(t, verifier.verified, 1)
	require.Equal(t, manifest.ID, verifier.verified[0].ID)
}

func TestModuleServiceStartEnabledModulesMarksFailedWhenPackageVerificationFails(t *testing.T) {
	store := newFakeModuleStore()
	manifest := serviceTestModuleManifest()
	store.items[manifest.ID] = modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusEnabled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: time.Now().UTC(),
	}
	runtime := &fakeProviderRuntime{}

	svc := NewModuleService(store)
	svc.providerRuntime = runtime
	svc.moduleVerifier = &fakeModuleInstaller{verifyErr: errors.New("signature invalid")}
	err := svc.StartEnabledModules(context.Background())
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.Equal(t, "MODULE_RUNTIME_RESTORE_FAILED", infraerrors.Reason(err))
	require.Empty(t, runtime.started)
	require.Equal(t, modules.ModuleStatusFailed, store.items[manifest.ID].Status)
	require.Equal(t, "signature invalid", store.items[manifest.ID].LastError)
}

type fakeModuleStore struct {
	items       map[string]modules.InstalledModule
	permissions map[string][]modules.PermissionRecord
	purgedData  map[string]bool
}

type fakeProviderRuntime struct {
	startErr       error
	stopErr        error
	started        []string
	stopped        []string
	startedModules []modules.InstalledModule
}

type fakeModuleInstaller struct {
	archivePaths   []string
	archiveBytes   [][]byte
	installed      *modules.InstalledModule
	err            error
	verified       []modules.InstalledModule
	verifiedModule *modules.InstalledModule
	verifyErr      error
}

func (i *fakeModuleInstaller) InstallArchive(_ context.Context, archivePath string) (*modules.InstalledModule, error) {
	i.archivePaths = append(i.archivePaths, archivePath)
	content, readErr := os.ReadFile(archivePath)
	if readErr == nil {
		i.archiveBytes = append(i.archiveBytes, content)
	}
	if i.err != nil {
		return nil, i.err
	}
	if i.installed == nil {
		return nil, errors.New("fake installer missing installed module")
	}
	return i.installed, nil
}

func (i *fakeModuleInstaller) VerifyInstalled(_ context.Context, module modules.InstalledModule) (*modules.InstalledModule, error) {
	i.verified = append(i.verified, module)
	if i.verifyErr != nil {
		return nil, i.verifyErr
	}
	if i.verifiedModule != nil {
		return i.verifiedModule, nil
	}
	return &module, nil
}

func (r *fakeProviderRuntime) StartProvider(_ context.Context, module modules.InstalledModule) error {
	r.started = append(r.started, module.ID)
	r.startedModules = append(r.startedModules, module)
	return r.startErr
}

func (r *fakeProviderRuntime) StopProvider(_ context.Context, moduleID string) error {
	r.stopped = append(r.stopped, moduleID)
	return r.stopErr
}

type fakeProviderAdapter struct {
	id string
}

func (a fakeProviderAdapter) ID() string {
	return a.id
}

func (a fakeProviderAdapter) Metadata(context.Context) (*modules.ProviderMetadata, error) {
	return &modules.ProviderMetadata{ID: a.id}, nil
}

func (a fakeProviderAdapter) HealthCheck(context.Context) error {
	return nil
}

func (a fakeProviderAdapter) ListModels(context.Context, modules.ListModelsRequest) (*modules.ListModelsResponse, error) {
	return &modules.ListModelsResponse{}, nil
}

func (a fakeProviderAdapter) ValidateAccount(context.Context, modules.ProviderAccount) (*modules.AccountValidationResult, error) {
	return &modules.AccountValidationResult{Valid: true}, nil
}

func (a fakeProviderAdapter) RefreshAccount(_ context.Context, account modules.ProviderAccount) (*modules.ProviderAccount, error) {
	return &account, nil
}

func (a fakeProviderAdapter) Forward(context.Context, modules.GatewayRequest) (<-chan modules.GatewayEvent, error) {
	ch := make(chan modules.GatewayEvent)
	close(ch)
	return ch, nil
}

func (a fakeProviderAdapter) TestAccount(context.Context, modules.TestAccountRequest) (*modules.TestAccountResult, error) {
	return &modules.TestAccountResult{OK: true}, nil
}

func (a fakeProviderAdapter) NormalizeError(_ context.Context, upstreamError modules.UpstreamError) (*modules.NormalizedError, error) {
	return &modules.NormalizedError{StatusCode: upstreamError.StatusCode, Message: upstreamError.Message}, nil
}

func (a fakeProviderAdapter) ChatStream(context.Context, modules.ChatRequest) (<-chan modules.ChatEvent, error) {
	ch := make(chan modules.ChatEvent)
	close(ch)
	return ch, nil
}

func (a fakeProviderAdapter) Embed(context.Context, modules.EmbeddingRequest) (*modules.EmbeddingResponse, error) {
	return &modules.EmbeddingResponse{}, nil
}

func (a fakeProviderAdapter) CountTokens(context.Context, modules.TokenCountRequest) (*modules.TokenCountResponse, error) {
	return &modules.TokenCountResponse{}, nil
}

func newFakeModuleStore() *fakeModuleStore {
	return &fakeModuleStore{
		items:       make(map[string]modules.InstalledModule),
		permissions: make(map[string][]modules.PermissionRecord),
		purgedData:  make(map[string]bool),
	}
}

func (s *fakeModuleStore) ListInstalled(context.Context) ([]modules.InstalledModule, error) {
	result := make([]modules.InstalledModule, 0, len(s.items))
	for _, item := range s.items {
		if item.Status != modules.ModuleStatusPurged {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *fakeModuleStore) GetInstalled(_ context.Context, id string) (*modules.InstalledModule, error) {
	item, ok := s.items[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return &item, nil
}

func (s *fakeModuleStore) SaveInstalled(_ context.Context, module modules.InstalledModule) error {
	s.items[module.ID] = module
	return nil
}

func (s *fakeModuleStore) SavePermissions(_ context.Context, moduleID string, permissions []modules.PermissionRecord) error {
	s.permissions[moduleID] = permissions
	return nil
}

func (s *fakeModuleStore) ListPermissions(_ context.Context, moduleID string) ([]modules.PermissionRecord, error) {
	return s.permissions[moduleID], nil
}

func (s *fakeModuleStore) ApprovePermissions(_ context.Context, moduleID string) error {
	permissions := s.permissions[moduleID]
	now := time.Now().UTC()
	for idx := range permissions {
		permissions[idx].Approved = true
		permissions[idx].ApprovedAt = &now
	}
	s.permissions[moduleID] = permissions
	return nil
}

func (s *fakeModuleStore) ApplyMigration(context.Context, string, string, string, string) error {
	return nil
}

func (s *fakeModuleStore) SetStatus(_ context.Context, id string, status modules.ModuleStatus, lastError string) error {
	item, ok := s.items[id]
	if !ok {
		return sql.ErrNoRows
	}
	item.Status = status
	item.LastError = lastError
	if status == modules.ModuleStatusEnabled {
		now := time.Now().UTC()
		item.EnabledAt = &now
	} else if status == modules.ModuleStatusDisabled ||
		status == modules.ModuleStatusFailed ||
		status == modules.ModuleStatusUninstalled ||
		status == modules.ModuleStatusPurged {
		item.EnabledAt = nil
	}
	s.items[id] = item
	return nil
}

func (s *fakeModuleStore) PurgeModuleData(_ context.Context, module modules.InstalledModule) error {
	s.purgedData[module.ID] = true
	return nil
}

func serviceTestModuleManifest() modules.Manifest {
	return modules.Manifest{
		APIVersion: modules.ManifestAPIVersionV1Alpha1,
		ID:         "lightbridge.provider.openai-api",
		Name:       "OpenAI API Provider",
		Type:       modules.ModuleTypeProvider,
		Version:    "0.1.0",
		Core: modules.CoreSpec{
			Compatible: ">=0.1.0 <0.2.0",
		},
		Backend: &modules.BackendSpec{
			Kind:     modules.BackendKindSidecar,
			Command:  "./backend/lightbridge-provider-openai",
			Protocol: modules.BackendProtocolConnect,
		},
		Frontend: &modules.FrontendSpec{
			Kind:  modules.FrontendKindViteRemoteESM,
			Entry: "./frontend/remoteEntry.js",
			Routes: []modules.UIRouteSpec{{
				Path:          "/admin/providers/openai",
				Title:         "OpenAI API",
				ExposedModule: "./OpenAIProviderSettings",
				RequiresAdmin: true,
			}},
			Menu: []modules.UIMenuSpec{{
				Title: "OpenAI API",
				Path:  "/admin/providers/openai",
				Group: "Providers",
				Order: 10,
			}},
			AccountForms: []modules.AccountFormSpec{{
				ProviderID:    "lightbridge.provider.openai-api",
				ExposedModule: "./OpenAIAccountForm",
			}},
		},
		Capabilities: []modules.Capability{
			modules.CapabilityProviderAdapter,
			modules.CapabilityUIAdminRoute,
			modules.CapabilityUIAccountForm,
		},
	}
}

func approvedPermissions(moduleID string) []modules.PermissionRecord {
	return []modules.PermissionRecord{{
		ModuleID:        moduleID,
		PermissionType:  "network",
		PermissionValue: "https://api.openai.com/*",
		Approved:        true,
	}}
}

func writeMarketplaceRegistry(t *testing.T, registry ModuleMarketplaceResult) string {
	t.Helper()
	content, err := json.Marshal(registry)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "marketplace.json")
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}
