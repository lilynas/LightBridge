# LightBridge OpenAI Provider Module

This module packages the legacy sub2API/OpenAI provider behavior behind the
LightBridge provider-module protocol. It follows the same package shape as the
mock provider module and is intended to replace the built-in OpenAI provider
during provider modularization.

## Build A Local Package

From this directory:

```sh
go run ./tools/build-package.go
```

The command creates:

- `dist/lightbridge-module-openai-0.1.0.tar.zst`
- `dist/registry.json`
- `dist/ed25519.pub`

Configure LightBridge Core with:

```yaml
modules:
  signature_public_key_path: /absolute/path/examples/modules/lightbridge-provider-openai/dist/ed25519.pub
  marketplace_registry_path: /absolute/path/examples/modules/lightbridge-provider-openai/dist/registry.json
```

The module ID is `openai` so upgraded legacy accounts with
`accounts.provider_id = openai` continue to resolve the provider adapter.

