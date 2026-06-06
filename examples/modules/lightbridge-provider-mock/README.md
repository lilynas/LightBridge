# LightBridge Mock Provider Module

This example is the first end-to-end module package for the provider-module MVP.
It is intentionally deterministic: it does not call any upstream provider, but it
exercises the marketplace, installer, permission approval, provider sidecar,
dynamic admin route, sidebar menu, and module account form.

## Build A Local Package

From this directory:

```sh
go run ./tools/build-package.go
```

The command creates:

- `dist/lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst`
- `dist/registry.json`
- `dist/ed25519.pub`

Configure LightBridge Core with:

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

The generated `dist/registry.json` and `dist/ed25519.pub` are local smoke-test
artifacts. They are intentionally regenerated instead of committed.

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
