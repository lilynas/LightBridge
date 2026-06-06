# LightBridge Module Architecture

This directory defines the engineering contract for the LightBridge microkernel module system. It is the source of truth for the first provider-module MVP.

## MVP Goal

LightBridge Core remains a usable gateway shell:

- setup wizard, config loading, PostgreSQL/Redis, logging, audit
- username/password login, JWT, admin/user basics
- API key auth, groups, common accounts, proxy records
- scheduling state, usage records, billing/log persistence
- module installation, enable/disable, upgrade, rollback

Core must not contain provider business code in the final provider-module target. AI providers are downloaded modules that register capabilities through stable extension points.

## Non-Goals For MVP

- Do not move 2FA, Passkey, billing, payment, or ops monitoring in the first phase.
- Do not use Go native `.so` plugins.
- Do not implement strong runtime sandboxing in the first phase. Permissions are declared, approved, and audited.
- Do not support hot activation of backend modules. Installing can happen at runtime; full activation is restart-bound unless explicitly documented otherwise.

## First Supported Extension Points

Only these capabilities are allowed in the first implementation:

| Capability | Purpose |
| --- | --- |
| `provider.adapter` | Register an AI provider adapter through the backend plugin protocol. |
| `ui.admin.route` | Add an admin UI route from a frontend remote. |
| `ui.account.form` | Provide provider-specific account creation/edit UI. |
| `gateway.request_filter` | Inspect or rewrite normalized gateway requests before provider selection. |
| `gateway.response_filter` | Inspect or rewrite provider response events before output. |
| `module.migration` | Apply module-owned database migrations. |

Any new extension point must be added to this document set before implementation.

## Implementation Order

1. Define module manifest schema, status enums, and package verification.
2. Add module database tables and repository/service APIs.
3. Add admin module APIs and module install/enable/disable lifecycle.
4. Add marketplace registry loading from local JSON or HTTP(S), plus package download/copy from local path, `file://`, HTTP, or HTTPS.
5. Add sidecar supervisor with Unix socket lifecycle.
6. Add ProviderAdapter sidecar contract and core-side HTTP adapter.
7. Add provider registry and adapt gateway/account-test entrypoints to resolve module accounts dynamically.
8. Add UI manifest endpoint and frontend dynamic route/menu loader.
9. Create an Anthropic provider module sample and run a streaming request through it.

## Current Provider MVP Contract

The core provider module bridge is account-driven:

- new module accounts use `platform = "module"` and `type = "module"`
- API payloads must include `provider_id`
- MVP provider identity is single-source: `module.yaml.id`,
  `frontend.accountForms[].providerId`, sidecar `ProviderMetadata.id`, account
  `provider_id`, and `extra.provider_id` must be identical
- persistence writes `accounts.provider_id` and keeps `extra.provider_id`/`extra.module_id` as a compatibility bridge
- `platform = "module"` is only a classification marker and is never a provider ID fallback
- a module account with no registered provider fails explicitly instead of entering legacy provider branches

`accounts.provider_id` is the canonical provider routing field. `extra.provider_id`
remains readable for old data and import/export compatibility, but new code must
write the top-level field and keep the compatibility extra in sync.

## Installer and Lifecycle Guarantees

Module installation now enforces the parts of the package protocol that prevent hidden core coupling:

- `core.compatible` is checked against the running `BuildInfo.Version`.
- `module.yaml`, `checksums.txt`, and `signature.sig` are required.
- `signature.sig` must be a valid Ed25519 signature over the raw `checksums.txt` bytes.
- manifest-referenced backend, frontend, and migration files must exist in the package and be covered by `checksums.txt`.
- module migrations must only touch tables allowed by `permissions.database`.

Marketplace installation is a thin index layer over the same installer:

- `modules.marketplace_registry_path` or `modules.marketplace_registry_url` points Core at a JSON registry.
- registry entries must declare `id`, `version`, `type`, `core`, `downloadUrl`, and allowed MVP capabilities.
- `downloadUrl` supports local paths, `file://`, `http://`, and `https://`.
- registry `sha256`, when present, verifies the downloaded archive bytes before package installation.
- package `module.yaml`, `checksums.txt`, and `signature.sig` remain mandatory and are verified after registry validation.

Lifecycle semantics are intentionally conservative:

- `Disable`: stop runtime contributions only; keep files and data.
- `Uninstall`: stop runtime, delete the installed package directory, keep database data and migration records.
- `Purge`: explicit destructive action; delete module private tables matching declared database prefixes, delete package files, then mark the module as `purged`.

The file deletion guard only removes the expected install directory:

```text
<modules.data_dir>/modules/<module-id>/<version>
```

Any empty, symlinked, mismatched, or escaping path is rejected.

## Required Reading

- [Package Spec](package-spec.md)
- [Backend Plugin Protocol](backend-plugin-protocol.md)
- [Frontend Extension Protocol](frontend-extension-protocol.md)
- [Provider SDK](provider-sdk.md)
- [Migrations and Data](migrations-and-data.md)
- [Security and Permissions](security-permissions.md)
- [Testing](testing.md)
- [Runbooks](runbooks.md)

## New Engineer Start Path

Read and execute in this order:

1. Read this overview and confirm the MVP scope: provider modules only, not
   2FA, Passkey, billing, or ops.
2. Read [Package Spec](package-spec.md) and inspect one module package with the
   manual package review checklist.
3. Read [Backend Plugin Protocol](backend-plugin-protocol.md) and identify the
   ProviderAdapter and CoreBridge fields used by the feature you are changing.
4. Read [Provider SDK](provider-sdk.md) and build or update the mock provider
   before touching a real OpenAI/Anthropic provider.
5. Read [Frontend Extension Protocol](frontend-extension-protocol.md) before
   adding any admin route, sidebar item, or provider account form.
6. Read [Testing](testing.md) and add/adjust the smallest focused test for the
   behavior you are changing.
7. Use [Runbooks](runbooks.md) when install, runtime, migration, provider, or UI
   behavior fails.

Do not start by editing gateway provider branches. First identify the
`provider_id`, module account contract, provider registry lookup, and sidecar
adapter path that should own the behavior.

## Development Milestones And Evidence

| Milestone | Work | Evidence |
| --- | --- | --- |
| Module schema | Manifest parser, module tables, status enums | invalid manifest tests and state transition tests pass. |
| Installer | checksum, signature, compatibility, migration validation | valid mock package installs; mismatch packages reject. |
| Supervisor | sidecar start/stop/crash cleanup, sockets, logs | crash test marks module `crashed` without Core panic. |
| Provider protocol | ProviderAdapter/CoreBridge contracts | mock provider streams `headers/data/usage/done`. |
| Provider registry | `provider_id` resolves adapters | module account never enters legacy provider branches. |
| UI manifest | dynamic route/menu/account form contributions | broken remote renders module error page only. |
| Sample provider | mock first, Anthropic next | streaming request succeeds and usage is captured. |

## Core Tables And API Paths

The provider-module MVP uses these Core-owned tables:

| Table | Purpose |
| --- | --- |
| `installed_modules` | Registered package, manifest, version, status, install path. |
| `module_permissions` | Declared and approved permissions per module/version. |
| `module_migrations` | Applied module migration names. |
| `module_runtime_instances` | Sidecar process/runtime status, sockets, health, logs metadata. |
| `ai_provider_instances` | AI provider instance/config records. Do not use payment-oriented `provider_instances`. |
| `accounts` | Shared account table. Module accounts use `platform=module`, `type=module`, and `provider_id`/`extra.provider_id`. |

The first admin API surface is:

| API | Purpose |
| --- | --- |
| `GET /api/v1/admin/modules/marketplace` | List marketplace modules from local/remote registry. |
| `GET /api/v1/admin/modules/installed` | List installed modules and status. |
| `POST /api/v1/admin/modules/install` | Install from marketplace selection or local archive path. |
| `POST /api/v1/admin/modules/:id/enable` | Approve/start module contributions when permissions are satisfied. |
| `POST /api/v1/admin/modules/:id/disable` | Stop backend/UI contributions and keep data/files. |
| `POST /api/v1/admin/modules/:id/uninstall` | Stop module and delete installed package files; keep data. |
| `POST /api/v1/admin/modules/:id/purge` | Destructive cleanup of package files and declared module-private data. |
| `GET /api/v1/modules/ui-manifest` | Return enabled frontend route/menu/account-form contributions. |

Implementation and docs must use the same names. If a route or table is renamed
in code, update this section and all linked protocol docs in the same change.

### Admin API Payloads

Marketplace install:

```json
{
  "module_id": "lightbridge.provider.mock",
  "version": "0.1.0"
}
```

Local archive install:

```json
{
  "archive_path": "/absolute/path/lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst"
}
```

Enable response:

```json
{
  "module_id": "lightbridge.provider.mock",
  "version": "0.1.0",
  "status": "enabled",
  "runtime_status": "running"
}
```

Disable/uninstall/purge response:

```json
{
  "module_id": "lightbridge.provider.mock",
  "version": "0.1.0",
  "status": "disabled"
}
```

Error response shape:

```json
{
  "error": {
    "code": "MODULE_SIGNATURE_INVALID",
    "message": "module signature verification failed",
    "module_id": "lightbridge.provider.mock",
    "version": "0.1.0"
  }
}
```

Admin APIs must return stable `error.code` values listed in
[testing.md](testing.md). The `message` may be localized or clarified, but tests
should assert the stable code.

## Documentation Acceptance Matrix

Before marking the module documentation complete, verify:

| Requirement | Required Evidence |
| --- | --- |
| New engineer can find the entry point | `DEV_GUIDE.md` links to this directory and this README has the start path. |
| Engineer can inspect a package | `package-spec.md` has layout, manifest, checksum/signature, state machine, and manual checklist. |
| Engineer can implement a provider | `provider-sdk.md` has mock provider steps from manifest to streaming request. |
| Engineer can implement backend RPC | `backend-plugin-protocol.md` has RPC names and request/response field tables. |
| Engineer can implement frontend remote | `frontend-extension-protocol.md` has UI manifest types, loader flow, SDK surface, and failure behavior. |
| Engineer can write safe migrations | `migrations-and-data.md` has allowed/rejected SQL and lifecycle data rules. |
| Engineer understands MVP security | `security-permissions.md` states declaration/approval/audit and no strong sandbox claim. |
| Engineer can test the MVP | `testing.md` covers no-provider, install provider, sidecar crash, and remote failure scenarios. |
| Engineer can troubleshoot failures | `runbooks.md` maps stable error codes to inspection steps. |

If implementation changes any API path, status name, table name, RPC field,
error code, or lifecycle behavior, update the affected document in the same
change. Documentation that contradicts implementation is treated as a failed
acceptance criterion.

## Consistency Index

Use the same meaning everywhere:

| Term | Meaning | Must Appear In |
| --- | --- | --- |
| `module_id` | Installable module identity from `module.yaml.id`. | module tables, permissions, runtime, UI manifest, CoreBridge audit. |
| `provider_id` | Provider adapter identity used by gateway/account routing. Must equal provider module ID in MVP. | account payloads, provider registry, ProviderAdapter metadata, GatewayRequest account. |
| `platform=module` | Compatibility marker for shared account table. Not a provider ID. | new module account rows only. |
| `type=module` | Compatibility marker for module-owned account credentials. | new module account rows only. |
| `enabled` | Module selected for active runtime/UI contributions. | `installed_modules.status`, admin APIs, tests. |
| `running` | Runtime status after sidecar healthcheck and metadata identity verification succeed. | `module_runtime_instances`, provider registry scheduling. |
| `failed` | Lifecycle or runtime failure that needs admin action or retry. | install/enable/runtime errors. |
| `crashed` | A previously running sidecar exited unexpectedly. | `module_runtime_instances`, runtime logs, provider registry unregister. |

Do not reuse legacy provider names such as `openai`, `anthropic`, `gemini`, or
`antigravity` as new provider registration entrypoints. They may exist only as
legacy compatibility mapping while the corresponding module provider is being
introduced.
