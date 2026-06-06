# Module Package Specification

## Package Layout

Module packages are distributed as:

```text
lightbridge-module-<module-id>-<version>.tar.zst
```

Installed modules live under:

```text
<modules.data_dir>/modules/<module-id>/<version>/
```

Required package layout:

```text
lightbridge-provider-openai/
  module.yaml
  backend/
    linux-amd64/
      lightbridge-provider-openai
    linux-arm64/
      lightbridge-provider-openai
  frontend/
    remoteEntry.js
    assets/
  migrations/
    001_create_provider_openai_config.sql
  checksums.txt
  signature.sig
```

`frontend/` and `migrations/` are optional. `module.yaml`, `checksums.txt`, and `signature.sig` are always required.

## Manifest Example

```yaml
apiVersion: lightbridge.dev/modules/v1alpha1
id: lightbridge.provider.openai-api
name: OpenAI API Provider
type: provider
version: 0.1.0
description: OpenAI-compatible upstream provider using API keys.
core:
  compatible: ">=0.1.0 <0.2.0"
backend:
  kind: sidecar
  command: ./backend/linux-amd64/lightbridge-provider-openai
  protocol: grpc
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
```

## Required Manifest Fields

| Field | Required | Notes |
| --- | --- | --- |
| `apiVersion` | Yes | Must be `lightbridge.dev/modules/v1alpha1` for the first implementation. |
| `id` | Yes | Reverse-DNS style module ID. Use lowercase letters, digits, dots, and hyphens. |
| `name` | Yes | Human-readable display name. |
| `type` | Yes | MVP supports `provider`, `ui`, and `gateway`. Provider modules use `provider`. |
| `version` | Yes | SemVer string without leading `v`. |
| `core.compatible` | Yes | Version range checked against core build version. |
| `backend.protocol` | Yes for backend modules | New provider modules must use `grpc`; `connect` is retained only for HTTP JSON compatibility modules. |
| `capabilities` | Yes | Must be a subset of the allowed MVP capabilities. |
| `permissions` | Yes | May be empty, but must be present. |

## Module States

| State | Meaning |
| --- | --- |
| `installed` | Files are unpacked and manifest is registered; not active. |
| `enabled` | Module is selected for startup registration. |
| `disabled` | Module data remains, but no backend/UI capabilities are active. |
| `failed` | Startup, migration, healthcheck, or verification failed. |
| `uninstalled` | Module files removed; DB config and private data retained. |
| `purged` | Module files and module-owned data removed after explicit confirmation. |

## State Transitions

Only these MVP transitions are valid:

| From | Action | To | Notes |
| --- | --- | --- | --- |
| none | install succeeds | `installed` | Package files and manifest are registered. |
| none | install fails | none or `failed` | No enabled state may be written. |
| `installed` | enable succeeds | `enabled` | Permissions must be approved before sidecar starts. |
| `installed` | enable fails | `failed` | Record install path and error for debugging. |
| `enabled` | disable | `disabled` | Stop runtime/UI contributions; keep files and data. |
| `failed` | disable | `disabled` | Allows admin to stop retrying a broken module. |
| `disabled` | enable succeeds | `enabled` | Re-verify package and healthcheck before registration. |
| `installed`/`disabled`/`failed` | uninstall | `uninstalled` | Delete package files only; keep DB data. |
| `enabled` | uninstall | `uninstalled` | Stop runtime first, then delete package files. |
| any non-`purged` | purge | `purged` | Explicit destructive action; delete module private data and package files. |

Invalid transitions must return explicit lifecycle errors. Do not silently coerce
`purged` back to `installed`; reinstalling a purged module is a fresh install of
a new package copy.

## Compatibility

`core.compatible` is checked during package installation against the running core `BuildInfo.Version`.

The first implementation supports whitespace-separated SemVer constraints:

```text
>=0.1.0 <0.2.0
```

Supported operators are `>=`, `>`, `<=`, `<`, and `=`. Versions may include a leading `v`; prerelease/build suffixes are parsed by their `major.minor.patch` prefix for the MVP. Empty `core.compatible` is invalid.

## Checksums

`checksums.txt` uses one line per file:

```text
sha256 <module-yaml-sha256> module.yaml
sha256 <backend-sha256> backend/linux-amd64/lightbridge-provider-openai
sha256 <frontend-sha256> frontend/remoteEntry.js
sha256 <migration-sha256> migrations/001_create_provider_openai_config.sql
```

Paths are relative to the module root and must use `/`.

## Signature

`signature.sig` signs the exact `checksums.txt` bytes. The current installer verifies this signature with an Ed25519 public key configured on the core service:

```yaml
modules:
  data_dir: ./data
  signature_public_key_path: /etc/lightbridge/modules/ed25519.pub
  marketplace_registry_path: /etc/lightbridge/modules/registry.json
  marketplace_registry_url: ""
  marketplace_timeout_seconds: 20
```

The public key may be raw 32-byte content, hex, base64, or a PEM block containing raw Ed25519 public-key bytes. If no verifier is configured, or the signature does not match `checksums.txt`, installation fails before manifest files are registered.

## Marketplace Registry

The first marketplace implementation reads a static JSON registry from either `modules.marketplace_registry_path` or `modules.marketplace_registry_url`. If both are set, the local file path wins. If neither is set, `GET /api/v1/admin/modules/marketplace` returns an empty module list.

Registry URLs must use `http` or `https`. Published package `downloadUrl`
values must also use `http` or `https`. Local paths are reserved for smoke
tests and development-only registries.

Package `downloadUrl` values may use:

| Scheme | Meaning |
| --- | --- |
| local path | Copy a package already available to the Core process for smoke tests only. |
| `file://` | Copy a package from the local filesystem for smoke tests only. |
| `http://` / `https://` | Download a package with `modules.marketplace_timeout_seconds`. |

Example registry:

```json
{
  "modules": [
    {
      "id": "openai",
      "version": "0.1.0",
      "type": "provider",
      "name": "OpenAI Provider",
      "description": "OpenAI provider module adapted from the legacy Sub2API OpenAI implementation.",
      "downloadUrl": "https://github.com/WilliamWang1721/LightBridge/releases/download/module-migration-20260606/lightbridge-module-openai-0.1.0.tar.zst",
      "sha256": "9a8d4f6f0a5f4c2d8b1e4f6a4d1d8f0e9c7b6a5d4c3b2a190817263544332211",
      "signature": "optional-registry-metadata-signature",
      "core": ">=0.1.0 <0.2.0",
      "capabilities": ["provider.adapter", "ui.admin.route", "ui.account.form"],
      "permissions": {
        "network": ["https://api.openai.com/*"],
        "secrets": ["openai_api_key"],
        "database": ["provider_openai_*"]
      }
    }
  ]
}
```

The registry entry is intentionally a lightweight index, not a replacement for the package manifest. `signature` is currently exposed as metadata for future registry-level signing policy; the enforced package trust root remains `signature.sig` over `checksums.txt`.

## Install Verification Order

Marketplace installation uses this order:

1. Load the registry from `modules.marketplace_registry_path` or `modules.marketplace_registry_url`.
2. Select the exact `id` and `version`.
3. Validate registry fields: `id`, `version`, `type`, `core`, `downloadUrl`, at least one capability, and the MVP capability allowlist.
4. Download or copy the package to a temporary install workspace.
5. If registry `sha256` is present, verify the downloaded archive bytes against it.
6. Delegate to the package installer.
7. The package installer verifies `module.yaml`, `checksums.txt`, `signature.sig`, manifest-referenced files, core compatibility, permissions, and migration table boundaries.

Local archive installation through `POST /api/v1/admin/modules/install` with `archive_path` skips the registry steps and starts at package installer verification.

## File Deletion Guard

`uninstall` and `purge` delete only the exact install directory derived from the manifest and service config:

```text
<modules.data_dir>/modules/<module-id>/<version>
```

Core rejects deletion if the stored `install_path` is empty, not the expected path, outside the modules root, a symlink, or not a directory.

## Manual Package Review Checklist

Use this checklist before publishing a module release or when debugging an
install rejection.

### Archive

- File name matches `lightbridge-module-<module-id>-<version>.tar.zst`.
- Archive expands into module files without absolute paths.
- Archive entries do not contain `..`, symlinks to outside paths, device files,
  or unexpected executable files.
- `module.yaml`, `checksums.txt`, and `signature.sig` exist at the module root.

### Manifest

- `apiVersion` is `lightbridge.dev/modules/v1alpha1`.
- `id` is lowercase reverse-DNS style and matches the release filename.
- `version` is SemVer and matches the release filename.
- `type` is valid for MVP; provider modules use `provider`.
- `core.compatible` is non-empty and uses supported SemVer operators.
- `capabilities` is non-empty and every capability is in the MVP allowlist.
- `permissions` exists even when all permission groups are empty.
- `backend.command`, `frontend.entry`, and each `migrations[]` path exists when
  declared.
- provider modules declare `provider.adapter`.
- provider modules using `frontend.accountForms` also declare
  `ui.account.form`; each `accountForms[].providerId` equals `module.yaml.id`.
- sidecar `ProviderMetadata.id`, account `provider_id`, and
  `frontend.accountForms[].providerId` use the same value as `module.yaml.id`
  in the MVP.

### Checksums And Signature

- Every manifest-referenced file appears in `checksums.txt`.
- Each checksum line has exactly `sha256 <hex> <relative-path>`.
- Paths in `checksums.txt` are relative, use `/`, and do not escape the module
  root.
- File hashes match the expanded package contents.
- `signature.sig` verifies over the raw `checksums.txt` bytes using Core's
  configured Ed25519 public key.

### Migrations

- Migration files are immutable after publication.
- Migration SQL only references tables matching `permissions.database`.
- Private table prefixes are narrow, such as `provider_openai_*`.
- Migration SQL does not alter, drop, insert into, update, delete from, truncate,
  or reference core tables.

### Frontend

- `frontend.entry` points to a JavaScript remote entry.
- Admin route paths start with `/admin/`.
- `routes[].exposedModule` and account form exposed modules start with `./`.
- Account form `providerId` equals the package `id`; aliases are not supported
  in the provider MVP.
- Remote components only depend on the public extension SDK.

### Runtime

- Sidecar binary exists for the target OS/arch and is executable.
- Sidecar reads `LIGHTBRIDGE_MODULE_SOCKET`; it does not reconstruct socket
  paths from module ID.
- Sidecar does not open Core DB connections.
- Sidecar does not log secrets, raw authorization headers, cookies, or full
  request bodies.

### Lifecycle

- `disable` has been tested to stop runtime contributions without deleting data.
- `uninstall` has been tested to delete only the installed package directory.
- `purge` has been tested only against module-private tables declared in
  `permissions.database`.
