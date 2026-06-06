# LightBridge Mock Provider Module

This example is the first end-to-end module package for the provider-module MVP.
It is intentionally deterministic: it does not call any upstream provider, but it
exercises the marketplace, installer, permission approval, provider sidecar,
dynamic admin route, sidebar menu, and module account form.

## Build A Release Package

From this directory:

```sh
go run ./tools/build-package.go
```

The command creates:

- `dist/lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst`
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
LIGHTBRIDGE_MODULE_DOWNLOAD_URL=file:///absolute/path/to/lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst \
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
  signature_public_key_path: /absolute/path/examples/modules/lightbridge-provider-mock/dist/ed25519.pub
  marketplace_registry_path: /absolute/path/examples/modules/lightbridge-provider-mock/dist/registry.json
```

Then use `/admin/modules`:

1. Review and install `Mock Provider` from Marketplace.
2. Approve the requested `mock_api_key` secret permission.
3. Enable the module.
4. Open the contributed `/admin/providers/mock` route or create a module
   account with provider `lightbridge.provider.mock`.

The sidecar streams a fixed gateway response: headers, two data events, usage,
and done.

## Delivery Verification

Run these checks from the repository root after changing the module package,
installer, runtime, UI manifest, or provider bridge:

```sh
cd examples/modules/lightbridge-provider-mock
go run ./tools/build-package.go

cd ../../../backend
go test ./internal/modules -run 'TestPackageInstallerInstallsMockProviderExamplePackage|TestGRPCProviderAdapterTalksToMockProviderExampleSidecar' -count=1 -timeout=90s
```

The generated `dist/registry.json` uses a remote release URL unless local
registry mode is explicitly enabled. The package and public key are regenerated
for each build.

For a full manual smoke test, configure Core with the generated registry and
public key, restart Core, then verify:

- `/api/v1/admin/modules/marketplace` lists `lightbridge.provider.mock`.
- `/admin/modules` can review, install, approve, and enable the module.
- `/api/v1/modules/ui-manifest` contains `/admin/providers/mock` and
  `/modules/lightbridge.provider.mock/0.1.0/frontend/remoteEntry.js`.
- `/modules/lightbridge.provider.mock/0.1.0/frontend/remoteEntry.js` returns
  JavaScript.
- creating a module account writes `platform=module`, `type=module`, and
  `provider_id=lightbridge.provider.mock`.
- a chat completions request through that account streams `hello`, ` from mock`,
  usage, and done.
