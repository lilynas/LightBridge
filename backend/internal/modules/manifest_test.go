package modules

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const validManifestYAML = `
apiVersion: lightbridge.dev/modules/v1alpha1
id: lightbridge.provider.openai-api
name: OpenAI API Provider
type: provider
version: 0.1.0
core:
  compatible: ">=0.1.0 <0.2.0"
backend:
  kind: sidecar
  command: ./backend/linux-amd64/lightbridge-provider-openai
  protocol: connect
  socket: data/modules-runtime/lightbridge.provider.openai-api.sock
  healthcheck:
    rpc: HealthCheck
    timeout: 2s
frontend:
  kind: vite-remote-esm
  entry: ./frontend/remoteEntry.js
  routes:
    - path: /admin/providers/openai
      title: OpenAI API
      exposedModule: ./OpenAIProviderSettings
      requiresAdmin: true
  menu:
    - title: OpenAI API
      path: /admin/providers/openai
      group: Providers
  accountForms:
    - providerId: lightbridge.provider.openai-api
      exposedModule: ./OpenAIAccountForm
capabilities:
  - provider.adapter
  - ui.admin.route
  - ui.account.form
permissions:
  network:
    - https://api.openai.com/*
  secrets:
    - openai_api_key
  database:
    - provider_openai_*
migrations:
  - migrations/001_create_provider_openai_config.sql
`

func TestLoadManifestBytesValid(t *testing.T) {
	manifest, err := LoadManifestBytes([]byte(validManifestYAML))
	require.NoError(t, err)
	require.Equal(t, "lightbridge.provider.openai-api", manifest.ID)
	require.Equal(t, ModuleTypeProvider, manifest.Type)
	require.Equal(t, 2_000_000_000, int(manifest.Backend.Healthcheck.Timeout.Duration))
	require.Len(t, manifest.Frontend.Routes, 1)
}

func TestLoadManifestRejectsUnsupportedCapability(t *testing.T) {
	content := strings.Replace(validManifestYAML, "provider.adapter", "auth.passkey", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "unsupported capability")
}

func TestLoadManifestRejectsInvalidCoreCompatibleConstraint(t *testing.T) {
	content := strings.Replace(validManifestYAML, ">=0.1.0 <0.2.0", ">=0.1", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "invalid core.compatible constraint")
}

func TestLoadManifestRejectsMissingPermissions(t *testing.T) {
	start := strings.Index(validManifestYAML, "permissions:\n")
	end := strings.Index(validManifestYAML, "migrations:\n")
	require.NotEqual(t, -1, start)
	require.NotEqual(t, -1, end)
	content := validManifestYAML[:start] + validManifestYAML[end:]
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "permissions is required")
}

func TestLoadManifestRejectsProviderWithoutBackend(t *testing.T) {
	content := strings.Replace(validManifestYAML, "  - provider.adapter\n", "", 1)
	content = strings.Replace(content, "  - ui.admin.route\n", "  - provider.adapter\n  - ui.admin.route\n", 1)
	start := strings.Index(content, "backend:\n")
	end := strings.Index(content, "frontend:\n")
	require.NotEqual(t, -1, start)
	require.NotEqual(t, -1, end)
	content = content[:start] + content[end:]
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "provider.adapter requires backend spec")
}

func TestLoadManifestRejectsProviderWithoutProviderAdapterCapability(t *testing.T) {
	content := strings.Replace(validManifestYAML, "  - provider.adapter\n", "", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "provider module")
	require.ErrorContains(t, err, "requires provider.adapter capability")
}

func TestLoadManifestRejectsFrontendRoutesWithoutCapability(t *testing.T) {
	content := strings.Replace(validManifestYAML, "  - ui.admin.route\n", "", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend routes and menu require ui.admin.route capability")
}

func TestLoadManifestRejectsFrontendEntryWithoutJavaScriptExtension(t *testing.T) {
	content := strings.Replace(validManifestYAML, "./frontend/remoteEntry.js", "./frontend/remoteEntry.css", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend.entry")
	require.ErrorContains(t, err, "JavaScript remote entry")
}

func TestLoadManifestRejectsFrontendEntryWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "./frontend/remoteEntry.js", "\"./frontend/remoteEntry.js \"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend.entry")
	require.ErrorContains(t, err, "must not contain surrounding whitespace")
}

func TestLoadManifestRejectsAccountFormsWithoutCapability(t *testing.T) {
	content := strings.Replace(validManifestYAML, "  - ui.account.form\n", "", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend accountForms require ui.account.form capability")
}

func TestLoadManifestRejectsAccountFormProviderIDMismatch(t *testing.T) {
	content := strings.Replace(validManifestYAML, "providerId: lightbridge.provider.openai-api", "providerId: lightbridge.provider.openai-alias", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "must match module id")
}

func TestLoadManifestRejectsAccountFormProviderIDWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "providerId: lightbridge.provider.openai-api", "providerId: \"lightbridge.provider.openai-api \"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "account form providerId")
	require.ErrorContains(t, err, "must not contain surrounding whitespace")
}

func TestLoadManifestRejectsDuplicateFrontendRoutePath(t *testing.T) {
	content := strings.Replace(validManifestYAML,
		"  menu:\n",
		"    - path: /admin/providers/openai\n      title: OpenAI API Duplicate\n      exposedModule: ./OpenAIProviderDuplicate\n  menu:\n",
		1,
	)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "duplicate frontend route path")
}

func TestLoadManifestRejectsFrontendRouteOutsideAdmin(t *testing.T) {
	content := strings.Replace(validManifestYAML, "path: /admin/providers/openai", "path: /providers/openai", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "must start with /admin/")
}

func TestLoadManifestRejectsFrontendRoutePathWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "path: /admin/providers/openai", "path: \" /admin/providers/openai\"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend route path")
	require.ErrorContains(t, err, "must not contain surrounding whitespace")
}

func TestLoadManifestRejectsFrontendRouteWithoutTitle(t *testing.T) {
	content := strings.Replace(validManifestYAML, "title: OpenAI API", "title: \"\"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend route")
	require.ErrorContains(t, err, "title is required")
}

func TestLoadManifestRejectsFrontendRouteTitleWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "title: OpenAI API", "title: \" OpenAI API\"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend route")
	require.ErrorContains(t, err, "title must not contain surrounding whitespace")
}

func TestLoadManifestRejectsFrontendExposedModuleWithoutDotSlash(t *testing.T) {
	content := strings.Replace(validManifestYAML, "exposedModule: ./OpenAIProviderSettings", "exposedModule: OpenAIProviderSettings", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "exposedModule must start with ./")
}

func TestLoadManifestRejectsFrontendExposedModuleWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "exposedModule: ./OpenAIProviderSettings", "exposedModule: \"./OpenAIProviderSettings \"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend route")
	require.ErrorContains(t, err, "exposedModule must not contain surrounding whitespace")
}

func TestLoadManifestRejectsFrontendMenuOutsideAdmin(t *testing.T) {
	content := strings.Replace(validManifestYAML, "path: /admin/providers/openai\n      group: Providers", "path: /providers/openai\n      group: Providers", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend menu path")
	require.ErrorContains(t, err, "must start with /admin/")
}

func TestLoadManifestRejectsFrontendMenuPathWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "path: /admin/providers/openai\n      group: Providers", "path: \"/admin/providers/openai \"\n      group: Providers", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend menu path")
	require.ErrorContains(t, err, "must not contain surrounding whitespace")
}

func TestLoadManifestRejectsFrontendMenuWithoutTitle(t *testing.T) {
	content := strings.Replace(validManifestYAML, "    - title: OpenAI API\n      path: /admin/providers/openai\n      group: Providers", "    - title: \"\"\n      path: /admin/providers/openai\n      group: Providers", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend menu")
	require.ErrorContains(t, err, "title is required")
}

func TestLoadManifestRejectsFrontendMenuTitleWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "    - title: OpenAI API\n      path: /admin/providers/openai\n      group: Providers", "    - title: \"OpenAI API \"\n      path: /admin/providers/openai\n      group: Providers", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "frontend menu")
	require.ErrorContains(t, err, "title must not contain surrounding whitespace")
}

func TestLoadManifestRejectsAccountFormExposedModuleWithoutDotSlash(t *testing.T) {
	content := strings.Replace(validManifestYAML, "exposedModule: ./OpenAIAccountForm", "exposedModule: OpenAIAccountForm", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "account form exposedModule must start with ./")
}

func TestLoadManifestRejectsAccountFormExposedModuleWithSurroundingWhitespace(t *testing.T) {
	content := strings.Replace(validManifestYAML, "exposedModule: ./OpenAIAccountForm", "exposedModule: \" ./OpenAIAccountForm\"", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "account form exposedModule")
	require.ErrorContains(t, err, "must not contain surrounding whitespace")
}

func TestLoadManifestRejectsEscapingPaths(t *testing.T) {
	content := strings.Replace(validManifestYAML, "./backend/linux-amd64/lightbridge-provider-openai", "../evil", 1)
	_, err := LoadManifestBytes([]byte(content))
	require.ErrorContains(t, err, "escapes module root")
}

func TestParseAndVerifyChecksums(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("manifest"), 0o644))
	sum := sha256.Sum256([]byte("manifest"))
	content := []byte(fmt.Sprintf("sha256 %x module.yaml\n", sum))
	entries, err := ParseChecksums(content)
	require.NoError(t, err)
	require.NoError(t, VerifyChecksums(dir, entries))
}

func TestVerifyChecksumsRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("manifest"), 0o644))
	entries, err := ParseChecksums([]byte("sha256 0000000000000000000000000000000000000000000000000000000000000000 module.yaml\n"))
	require.NoError(t, err)
	require.ErrorContains(t, VerifyChecksums(dir, entries), "checksum mismatch")
}

func TestInstallAndSocketPaths(t *testing.T) {
	require.Equal(t,
		filepath.Join("data", "modules", "lightbridge.provider.openai-api", "0.1.0"),
		InstallDir("data", "lightbridge.provider.openai-api", "0.1.0"),
	)
	require.Equal(t,
		filepath.Join("data", "modules-runtime", "lightbridge.provider.openai-api.sock"),
		RuntimeSocketPath("data", "lightbridge.provider.openai-api"),
	)
}

func TestRuntimeSocketPathFallsBackWhenBasePathIsTooLong(t *testing.T) {
	baseDir := filepath.Join(os.TempDir(), strings.Repeat("lightbridge-long-path-", 8))
	socketPath := RuntimeSocketPath(baseDir, "lightbridge.provider.openai-api")
	coreBridgeSocketPath := CoreBridgeSocketPath(baseDir, "lightbridge.provider.openai-api")

	require.LessOrEqual(t, len(socketPath), maxUnixSocketPathLen)
	require.LessOrEqual(t, len(coreBridgeSocketPath), maxUnixSocketPathLen)
	tempDir := filepath.Clean(os.TempDir())
	require.DirExists(t, tempDir)
	require.Equal(t, filepath.Dir(socketPath), tempDir)
	require.Equal(t, filepath.Dir(coreBridgeSocketPath), tempDir)
	require.NotEqual(t, socketPath, coreBridgeSocketPath)
}
