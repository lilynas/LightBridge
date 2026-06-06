# LightBridge OpenAI Provider Module

This module packages the legacy sub2API/OpenAI provider behavior behind the
LightBridge provider-module protocol. It follows the same package shape as the
mock provider module and is intended to replace the built-in OpenAI provider
during provider modularization.

## Build A Release Package

From this directory:

```sh
go run ./tools/build-package.go
```

The command creates:

- `dist/lightbridge-module-openai-0.1.0.tar.zst`
- `dist/registry.json`
- `dist/ed25519.pub`

The generated registry uses the GitHub release asset URL by default. Override
the release base URL only when publishing to another release:

```sh
LIGHTBRIDGE_MODULE_RELEASE_BASE_URL=https://github.com/<owner>/<repo>/releases/download/<tag> \
  go run ./tools/build-package.go
```

Local `file://` or absolute-path registry entries are blocked by default. For
local smoke tests only:

```sh
LIGHTBRIDGE_MODULE_ALLOW_LOCAL_REGISTRY=1 \
LIGHTBRIDGE_MODULE_DOWNLOAD_URL=file:///absolute/path/to/lightbridge-module-openai-0.1.0.tar.zst \
  go run ./tools/build-package.go
```

Configure LightBridge Core from GitHub with:

```yaml
modules:
  signature_public_key_path: /absolute/path/to/ed25519.pub
  marketplace_registry_url: https://github.com/WilliamWang1721/LightBridge/releases/download/module-migration-20260606/registry.json
```

Or configure a local smoke-test registry with:

```yaml
modules:
  signature_public_key_path: /absolute/path/examples/modules/lightbridge-provider-openai/dist/ed25519.pub
  marketplace_registry_path: /absolute/path/examples/modules/lightbridge-provider-openai/dist/registry.json
```

The module ID is `openai` so upgraded legacy accounts with
`accounts.provider_id = openai` continue to resolve the provider adapter.
