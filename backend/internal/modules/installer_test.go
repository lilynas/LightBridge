package modules

import (
	"archive/tar"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestPackageInstallerInstallsMockProviderExamplePackage(t *testing.T) {
	exampleDir := filepath.Clean("../../../examples/modules/lightbridge-provider-mock")
	manifestTemplate, err := os.ReadFile(filepath.Join(exampleDir, "module.template.yaml"))
	require.NoError(t, err)
	frontendEntry, err := os.ReadFile(filepath.Join(exampleDir, "frontend/remoteEntry.js"))
	require.NoError(t, err)

	platform := runtime.GOOS + "-" + runtime.GOARCH
	backendPath := fmt.Sprintf("backend/%s/lightbridge-provider-mock", platform)
	manifest := strings.ReplaceAll(string(manifestTemplate), "{{PLATFORM}}", platform)
	verifier, privateKey := newInstallerTestSigner(t)
	archivePath := buildTarZstdNamedWithModes(
		t,
		"lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst",
		signedModuleFiles(t, privateKey, map[string][]byte{
			ManifestFilename:          []byte(manifest),
			backendPath:               []byte("#!/bin/sh\n"),
			"frontend/remoteEntry.js": frontendEntry,
		}),
		map[string]int64{backendPath: 0o755},
	)
	store := newInstallerTestStore()
	installer := NewPackageInstallerWithVerifierAndCoreVersion(t.TempDir(), store, verifier, "0.1.5")

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.mock", installed.ID)
	require.Equal(t, "Mock Provider", installed.Name)
	require.Equal(t, ModuleStatusInstalled, installed.Status)
	require.Contains(t, installed.Manifest.Capabilities, CapabilityProviderAdapter)
	require.Contains(t, installed.Manifest.Capabilities, CapabilityUIAdminRoute)
	require.Contains(t, installed.Manifest.Capabilities, CapabilityUIAccountForm)
	require.NotNil(t, installed.Manifest.Backend)
	require.Equal(t, BackendProtocolGRPC, installed.Manifest.Backend.Protocol)
	require.NotNil(t, installed.Manifest.Frontend)
	require.Len(t, installed.Manifest.Frontend.Routes, 1)
	require.Equal(t, "/admin/providers/mock", installed.Manifest.Frontend.Routes[0].Path)
	require.Len(t, installed.Manifest.Frontend.Menu, 1)
	require.Len(t, installed.Manifest.Frontend.AccountForms, 1)
	require.Equal(t, "lightbridge.provider.mock", installed.Manifest.Frontend.AccountForms[0].ProviderID)
	require.Len(t, store.permissions[installed.ID], 2)
	require.Empty(t, store.migrations)
}

func TestPackageInstallerInstallArchive(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifier(baseDir, store, verifier)
	installer.now = func() time.Time {
		return time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	}
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.openai-api", installed.ID)
	require.Equal(t, ModuleStatusInstalled, installed.Status)
	require.Equal(t, InstallDir(baseDir, installed.ID, installed.Version), installed.InstallPath)
	require.FileExists(t, filepath.Join(installed.InstallPath, ManifestFilename))
	require.FileExists(t, filepath.Join(installed.InstallPath, ChecksumsFilename))
	require.FileExists(t, filepath.Join(installed.InstallPath, SignatureFilename))
	require.Equal(t, *installed, store.items[installed.ID])
	require.Len(t, store.permissions[installed.ID], 3)
	require.Len(t, store.migrations, 1)
	require.Equal(t, "lightbridge.provider.openai-api", store.migrations[0].moduleID)
	require.Equal(t, "migrations/001_create_provider_openai_config.sql", store.migrations[0].name)
	require.NotEmpty(t, store.migrations[0].checksum)
	require.Equal(t, "SELECT 1;", store.migrations[0].sql)
}

func TestPackageInstallerAcceptsCompatibleCoreVersion(t *testing.T) {
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifierAndCoreVersion(t.TempDir(), store, verifier, "0.1.5")
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("CREATE TABLE provider_openai_config (id TEXT PRIMARY KEY);"),
	})

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.openai-api", installed.ID)
	require.Len(t, store.migrations, 1)
}

func TestPackageInstallerRejectsIncompatibleCoreVersion(t *testing.T) {
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifierAndCoreVersion(t.TempDir(), store, verifier, "0.2.0")
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("CREATE TABLE provider_openai_config (id TEXT PRIMARY KEY);"),
	})

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "requires core")
	require.Empty(t, store.items)
	require.Empty(t, store.migrations)
}

func TestPackageInstallerRejectsChecksumMismatch(t *testing.T) {
	backend := []byte("#!/bin/sh\n")
	frontend := []byte("export default {}")
	migration := []byte("SELECT 1;")
	backendSum := sha256.Sum256(backend)
	frontendSum := sha256.Sum256(frontend)
	migrationSum := sha256.Sum256(migration)
	verifier, privateKey := newInstallerTestSigner(t)
	archivePath := buildTarZstd(t, signExistingChecksums(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		ChecksumsFilename: []byte(
			"sha256 0000000000000000000000000000000000000000000000000000000000000000 module.yaml\n" +
				fmt.Sprintf("sha256 %x backend/linux-amd64/lightbridge-provider-openai\n", backendSum) +
				fmt.Sprintf("sha256 %x frontend/remoteEntry.js\n", frontendSum) +
				fmt.Sprintf("sha256 %x migrations/001_create_provider_openai_config.sql\n", migrationSum),
		),
		"backend/linux-amd64/lightbridge-provider-openai":  backend,
		"frontend/remoteEntry.js":                          frontend,
		"migrations/001_create_provider_openai_config.sql": migration,
	}))
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestPackageInstallerRejectsManifestFileMissingChecksum(t *testing.T) {
	manifestSum := sha256.Sum256([]byte(validManifestYAML))
	verifier, privateKey := newInstallerTestSigner(t)
	archivePath := buildTarZstd(t, signExistingChecksums(t, privateKey, map[string][]byte{
		ManifestFilename:  []byte(validManifestYAML),
		ChecksumsFilename: []byte(fmt.Sprintf("sha256 %x %s\n", manifestSum, ManifestFilename)),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	}))
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "backend.command")
	require.ErrorContains(t, err, "not covered by checksums.txt")
}

func TestPackageInstallerRejectsNonExecutableBackendCommand(t *testing.T) {
	verifier, privateKey := newInstallerTestSigner(t)
	files := signedModuleFiles(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	archivePath := buildTarZstdWithModes(t, files, map[string]int64{
		"backend/linux-amd64/lightbridge-provider-openai": 0o644,
	})
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "backend.command")
	require.ErrorContains(t, err, "must be executable")
}

func TestPackageInstallerRejectsUnexpectedExecutableFile(t *testing.T) {
	verifier, privateKey := newInstallerTestSigner(t)
	files := signedModuleFiles(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"frontend/debug-tool.sh":                           []byte("#!/bin/sh\n"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	archivePath := buildTarZstdWithModes(t, files, map[string]int64{
		"frontend/debug-tool.sh": 0o755,
	})
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "unexpected executable file")
	require.ErrorContains(t, err, "frontend/debug-tool.sh")
}

func TestPackageInstallerMarksFailedWhenMigrationFails(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	store.migrationErr = errors.New("migration failed")
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifier(baseDir, store, verifier)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "apply module migration")
	item := store.items["lightbridge.provider.openai-api"]
	require.Equal(t, ModuleStatusFailed, item.Status)
	require.Contains(t, item.LastError, "migration failed")
}

func TestPackageInstallerRejectsMissingSignature(t *testing.T) {
	_, privateKey := newInstallerTestSigner(t)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	extracted := t.TempDir()
	require.NoError(t, ExtractArchive(archivePath, extracted))
	require.NoError(t, os.Remove(filepath.Join(extracted, SignatureFilename)))
	archivePath = buildTarZstdFromDir(t, extracted)
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), SignatureVerifierFunc(func([]byte, []byte) error {
		return nil
	}))

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "load module signature")
}

func TestPackageInstallerRejectsBadSignature(t *testing.T) {
	verifier, privateKey := newInstallerTestSigner(t)
	files := signedModuleFiles(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	files[SignatureFilename] = []byte("bad-signature")
	archivePath := buildTarZstd(t, files)
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "verify module signature")
}

func TestPackageInstallerRejectsMissingSignatureVerifier(t *testing.T) {
	_, privateKey := newInstallerTestSigner(t)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	installer := NewPackageInstaller(t.TempDir(), newInstallerTestStore())

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, "module signature verifier is not configured")
}

func TestPackageInstallerRejectsNonCanonicalArchiveFilename(t *testing.T) {
	verifier, privateKey := newInstallerTestSigner(t)
	archivePath := buildTarZstdNamed(t, "module.tar.zst", signedModuleFiles(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	}))
	installer := NewPackageInstallerWithVerifier(t.TempDir(), newInstallerTestStore(), verifier)

	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.Nil(t, installed)
	require.ErrorContains(t, err, `module archive filename must be "lightbridge-module-lightbridge.provider.openai-api-0.1.0.tar.zst"`)
}

func TestPackageInstallerVerifyInstalledRefreshesManifestFromDisk(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifierAndCoreVersion(baseDir, store, verifier, "0.1.5")
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("CREATE TABLE provider_openai_config (id TEXT PRIMARY KEY);"),
	})
	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	installed.Name = "Stale DB Name"
	installed.Manifest.Name = "Stale DB Name"

	verified, err := installer.VerifyInstalled(context.Background(), *installed)
	require.NoError(t, err)
	require.Equal(t, "OpenAI API Provider", verified.Name)
	require.Equal(t, "OpenAI API Provider", verified.Manifest.Name)
	require.Equal(t, installed.InstallPath, verified.InstallPath)
}

func TestPackageInstallerVerifyInstalledRejectsChecksumMismatch(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifier(baseDir, store, verifier)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(installed.InstallPath, "frontend/remoteEntry.js"), []byte("tampered"), 0o644))

	verified, err := installer.VerifyInstalled(context.Background(), *installed)
	require.Nil(t, verified)
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestPackageInstallerVerifyInstalledRejectsPackageSymlink(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifier(baseDir, store, verifier)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	remoteEntryPath := filepath.Join(installed.InstallPath, "frontend/remoteEntry.js")
	require.NoError(t, os.Remove(remoteEntryPath))
	require.NoError(t, os.Symlink(filepath.Join(installed.InstallPath, ManifestFilename), remoteEntryPath))

	verified, err := installer.VerifyInstalled(context.Background(), *installed)
	require.Nil(t, verified)
	require.ErrorContains(t, err, "must not be a symlink")
	require.ErrorContains(t, err, "frontend/remoteEntry.js")
}

func TestPackageInstallerVerifyInstalledRejectsManifestIdentityMismatch(t *testing.T) {
	baseDir := t.TempDir()
	store := newInstallerTestStore()
	verifier, privateKey := newInstallerTestSigner(t)
	installer := NewPackageInstallerWithVerifier(baseDir, store, verifier)
	archivePath := buildModuleArchive(t, privateKey, map[string][]byte{
		ManifestFilename: []byte(validManifestYAML),
		"backend/linux-amd64/lightbridge-provider-openai":  []byte("#!/bin/sh\n"),
		"frontend/remoteEntry.js":                          []byte("export default {}"),
		"migrations/001_create_provider_openai_config.sql": []byte("SELECT 1;"),
	})
	installed, err := installer.InstallArchive(context.Background(), archivePath)
	require.NoError(t, err)
	installed.ID = "lightbridge.provider.other"

	verified, err := installer.VerifyInstalled(context.Background(), *installed)
	require.Nil(t, verified)
	require.ErrorContains(t, err, "does not match database module id")
}

func TestExtractArchiveRejectsEscapingPath(t *testing.T) {
	archivePath := buildTarZstd(t, map[string][]byte{
		"../evil.txt": []byte("bad"),
	})

	err := ExtractArchive(archivePath, t.TempDir())
	require.ErrorContains(t, err, "escapes package root")
}

func TestExtractArchiveRejectsSymlinkEntry(t *testing.T) {
	archivePath := buildTarZstdWithHeaders(t, []*tar.Header{
		{
			Name:     "module.yaml",
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		},
	})

	err := ExtractArchive(archivePath, t.TempDir())
	require.ErrorContains(t, err, "unsupported tar type")
}

func TestExtractArchiveRejectsTzstAlias(t *testing.T) {
	err := ExtractArchive(filepath.Join(t.TempDir(), "lightbridge-module-lightbridge.provider.openai-api-0.1.0.tzst"), t.TempDir())
	require.ErrorContains(t, err, "module archive must be a .tar.zst package")
}

func TestValidateCoreCompatibility(t *testing.T) {
	require.NoError(t, ValidateCoreCompatibility(">=0.1.0 <0.2.0", "0.1.5"))
	require.NoError(t, ValidateCoreCompatibility(">=0.1.0 <0.2.0", "v0.1.5"))
	require.ErrorContains(t, ValidateCoreCompatibility(">=0.1.0 <0.2.0", "0.2.0"), "requires core")
	require.ErrorContains(t, ValidateCoreCompatibility("", "0.1.5"), "core.compatible is required")
	require.ErrorContains(t, ValidateCoreCompatibility(">=0.1", "0.1.5"), "invalid core.compatible constraint")
}

func TestValidateModuleMigrationSQL(t *testing.T) {
	manifest := serviceTestManifestWithDatabasePermission("provider_openai_*")

	require.NoError(t, ValidateModuleMigrationSQL(manifest, `
CREATE TABLE provider_openai_config (id TEXT PRIMARY KEY);
CREATE INDEX provider_openai_config_id_idx ON provider_openai_config (id);
COMMENT ON TABLE provider_openai_config IS 'OpenAI provider config';
COMMENT ON COLUMN provider_openai_config.id IS 'config id';
INSERT INTO provider_openai_config (id) VALUES ('default');
`))
	require.ErrorContains(t, ValidateModuleMigrationSQL(manifest, `
ALTER TABLE accounts ADD COLUMN provider_openai_debug TEXT;
`), `table "accounts" outside declared module database prefixes`)
	require.ErrorContains(t, ValidateModuleMigrationSQL(manifest, `
CREATE TABLE provider_openai_tokens (
  id TEXT PRIMARY KEY,
  account_id TEXT REFERENCES accounts(id)
);
`), `table "accounts" outside declared module database prefixes`)
}

func serviceTestManifestWithDatabasePermission(databasePermission string) Manifest {
	manifest := Manifest{
		APIVersion: ManifestAPIVersionV1Alpha1,
		ID:         "lightbridge.provider.openai-api",
		Name:       "OpenAI API Provider",
		Type:       ModuleTypeProvider,
		Version:    "0.1.0",
		Core: CoreSpec{
			Compatible: ">=0.1.0 <0.2.0",
		},
		Capabilities: []Capability{CapabilityProviderAdapter},
		Permissions: PermissionSet{
			Database: []string{databasePermission},
		},
	}
	return manifest
}

func buildModuleArchive(t *testing.T, privateKey ed25519.PrivateKey, files map[string][]byte) string {
	t.Helper()
	return buildTarZstd(t, signedModuleFiles(t, privateKey, files))
}

func signedModuleFiles(t *testing.T, privateKey ed25519.PrivateKey, files map[string][]byte) map[string][]byte {
	t.Helper()
	signed := make(map[string][]byte, len(files)+2)
	checksums := ""
	for path, content := range files {
		sum := sha256.Sum256(content)
		checksums += fmt.Sprintf("sha256 %x %s\n", sum, filepath.ToSlash(path))
		signed[path] = content
	}
	signed[ChecksumsFilename] = []byte(checksums)
	signed[SignatureFilename] = ed25519.Sign(privateKey, []byte(checksums))
	return signed
}

func signExistingChecksums(t *testing.T, privateKey ed25519.PrivateKey, files map[string][]byte) map[string][]byte {
	t.Helper()
	checksums, ok := files[ChecksumsFilename]
	require.True(t, ok, "test module package must include checksums.txt")
	signed := make(map[string][]byte, len(files)+1)
	for path, content := range files {
		signed[path] = content
	}
	signed[SignatureFilename] = ed25519.Sign(privateKey, checksums)
	return signed
}

func buildTarZstd(t *testing.T, files map[string][]byte) string {
	t.Helper()
	return buildTarZstdNamed(t, "lightbridge-module-lightbridge.provider.openai-api-0.1.0.tar.zst", files)
}

func buildTarZstdWithModes(t *testing.T, files map[string][]byte, modes map[string]int64) string {
	t.Helper()
	return buildTarZstdNamedWithModes(t, "lightbridge-module-lightbridge.provider.openai-api-0.1.0.tar.zst", files, modes)
}

func buildTarZstdNamed(t *testing.T, filename string, files map[string][]byte) string {
	t.Helper()
	return buildTarZstdNamedWithModes(t, filename, files, nil)
}

func buildTarZstdNamedWithModes(t *testing.T, filename string, files map[string][]byte, modes map[string]int64) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), filename)
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	zstdWriter, err := zstd.NewWriter(file)
	require.NoError(t, err)
	defer func() { _ = zstdWriter.Close() }()

	tarWriter := tar.NewWriter(zstdWriter)
	defer func() { _ = tarWriter.Close() }()

	for path, content := range files {
		mode := int64(0o644)
		if filepath.ToSlash(path) == "backend/linux-amd64/lightbridge-provider-openai" {
			mode = 0o755
		}
		if modes != nil {
			if customMode, ok := modes[filepath.ToSlash(path)]; ok {
				mode = customMode
			}
		}
		header := &tar.Header{
			Name: filepath.ToSlash(path),
			Mode: mode,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			require.NoError(t, err)
		}
		_, err := tarWriter.Write(content)
		require.NoError(t, err)
	}
	return archivePath
}

func buildTarZstdWithHeaders(t *testing.T, headers []*tar.Header) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "lightbridge-module-lightbridge.provider.openai-api-0.1.0.tar.zst")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	zstdWriter, err := zstd.NewWriter(file)
	require.NoError(t, err)
	defer func() { _ = zstdWriter.Close() }()

	tarWriter := tar.NewWriter(zstdWriter)
	defer func() { _ = tarWriter.Close() }()

	for _, header := range headers {
		require.NoError(t, tarWriter.WriteHeader(header))
	}
	return archivePath
}

func buildTarZstdFromDir(t *testing.T, root string) string {
	t.Helper()
	files := make(map[string][]byte)
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	}))
	return buildTarZstd(t, files)
}

func newInstallerTestSigner(t *testing.T) (*Ed25519SignatureVerifier, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier, err := NewEd25519SignatureVerifier(publicKey)
	require.NoError(t, err)
	return verifier, privateKey
}

type installerTestStore struct {
	items        map[string]InstalledModule
	permissions  map[string][]PermissionRecord
	migrations   []installerTestMigration
	migrationErr error
}

type installerTestMigration struct {
	moduleID string
	name     string
	checksum string
	sql      string
}

func newInstallerTestStore() *installerTestStore {
	return &installerTestStore{
		items:       make(map[string]InstalledModule),
		permissions: make(map[string][]PermissionRecord),
	}
}

func (s *installerTestStore) ListInstalled(context.Context) ([]InstalledModule, error) {
	result := make([]InstalledModule, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result, nil
}

func (s *installerTestStore) GetInstalled(_ context.Context, id string) (*InstalledModule, error) {
	item, ok := s.items[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &item, nil
}

func (s *installerTestStore) SaveInstalled(_ context.Context, module InstalledModule) error {
	s.items[module.ID] = module
	return nil
}

func (s *installerTestStore) SavePermissions(_ context.Context, moduleID string, permissions []PermissionRecord) error {
	s.permissions[moduleID] = permissions
	return nil
}

func (s *installerTestStore) ListPermissions(_ context.Context, moduleID string) ([]PermissionRecord, error) {
	return s.permissions[moduleID], nil
}

func (s *installerTestStore) ApprovePermissions(_ context.Context, moduleID string) error {
	permissions := s.permissions[moduleID]
	for idx := range permissions {
		permissions[idx].Approved = true
	}
	s.permissions[moduleID] = permissions
	return nil
}

func (s *installerTestStore) ApplyMigration(_ context.Context, moduleID string, migrationName string, checksum string, sql string) error {
	if s.migrationErr != nil {
		return s.migrationErr
	}
	s.migrations = append(s.migrations, installerTestMigration{
		moduleID: moduleID,
		name:     migrationName,
		checksum: checksum,
		sql:      sql,
	})
	return nil
}

func (s *installerTestStore) SetStatus(_ context.Context, id string, status ModuleStatus, lastError string) error {
	item, ok := s.items[id]
	if !ok {
		return os.ErrNotExist
	}
	item.Status = status
	item.LastError = lastError
	s.items[id] = item
	return nil
}
