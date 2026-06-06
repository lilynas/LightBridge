# Security and Permissions

## Permission Model

The first implementation uses declared permissions, admin approval, and audit logs. It is not a strong sandbox.

Supported permission groups:

- `network`
- `secrets`
- `database`
- `ui`
- `gateway`

Modules must declare all permissions in `module.yaml`. Core must show them before enablement.

## Secret Access

Modules may read credentials/secrets only through CoreBridge. The current
CoreBridge credential method is `GetAccountCredentials`; modules must not read
Core database tables or files directly.

Core binds module identity on the server side for every CoreBridge request. A
module can leave `module_id` empty in the request; if it sends a different
module ID, Core overwrites it with the supervised module ID before permission
checks and dispatch.

Every credential read must record:

- module ID
- provider/account ID
- credential reference or secret key
- request ID
- timestamp

Secrets must not be written to module stdout/stderr, runtime status, or UI manifest.
Provider runtime status updates must also avoid embedding secret values in
`message`, `last_error`, or metadata fields.

## Network Access

`permissions.network` is an allowlist declaration for review and audit. It is not enforceable by Core alone in MVP. Future enforcement can use an outbound proxy, container sandbox, or OS firewall rules.

## Database Access

Modules do not receive a database connection. All core data access goes through CoreBridge. Module-owned migrations can create private tables, but runtime data access should still prefer CoreBridge APIs unless a future module DB API is explicitly designed.

`permissions.database` is enforced during install and purge:

- During install, migration SQL may only reference tables whose names match the declared database prefixes.
- During purge, Core drops only module-private `public` tables matching those same prefixes.
- Core tables are never allowed through module migration SQL just because a module has a broad name or a UI/backend capability.

Use narrow prefixes such as `provider_openai_*`, not generic prefixes such as `provider_*`.

## UI Trust Boundary

Frontend remotes run in the admin browser context. Only install modules from trusted sources. Core must show module ID, version, download source, signature status, and permissions before enabling frontend contributions.

## Permission Approval Flow

Permissions are approved per installed module version. A later upgrade must be
reviewed again if it changes any permission group.

```text
install package
  -> parse module.yaml
  -> verify checksums/signature
  -> show permissions to admin
  -> admin approves or rejects
  -> persist module_permissions
  -> enable/start module
```

Approval record requirements:

| Field | Purpose |
| --- | --- |
| `module_id` | Module requesting access. |
| `module_version` | Version being approved. |
| `permission_type` | `network`, `secrets`, `database`, `ui`, or `gateway`. |
| `permission_value` | Exact declared value, such as `https://api.openai.com/*`. |
| `approved` | Whether an admin approved this value. |
| `approved_by` | Admin user ID when available. |
| `approved_at` | Approval timestamp. |

Core should reject enablement when a module declares permissions that are not
approved. Install may register the module in `installed` state while waiting for
approval, but the module must not start until approval is complete.

## Secret Read Flow

Modules read secrets only through CoreBridge:

```text
sidecar
  -> CoreBridge.GetAccountCredentials(account_id, purpose, request_id)
  -> Core validates module identity from supervisor
  -> Core verifies account belongs to the requested provider/module
  -> Core verifies approved `secrets` permission
  -> Core writes audit event
  -> Core returns allowed config/secrets
```

Required checks:

- caller identity comes from the supervised runtime, not from caller-supplied
  `module_id`
- `account.provider_id` or `account.extra.provider_id` matches the caller module
  provider ID
- `account.extra.module_id`, when present, matches the caller module ID
- returned secret keys are filtered to the exact names approved in
  `permissions.secrets`
- audit log is written before or atomically with the returned credential

Secret read audit event shape:

```json
{
  "type": "module.secret.read",
  "module_id": "lightbridge.provider.mock",
  "account_id": "123",
  "provider_id": "lightbridge.provider.mock",
  "secret_keys": ["api_key"],
  "purpose": "ValidateAccount",
  "credential_ref": "runtime-account",
  "result": "allowed"
}
```

Never log secret values. Logs and audit records may contain key names, account
IDs, module IDs, and sanitized purpose strings only.

## Permission Groups

| Group | MVP Enforcement | Notes |
| --- | --- | --- |
| `network` | declaration, approval, audit | Core does not enforce outbound network isolation in MVP. |
| `secrets` | approval before CoreBridge returns values | Secret keys are allowlisted by name. |
| `database` | migration validation and purge scope | Modules do not receive DB connections. |
| `ui` | manifest validation and admin disclosure | Frontend remotes run in admin browser context. |
| `gateway` | capability registration review | Filters/hooks must be declared before enablement. |

Do not claim network, filesystem, or process-level strong sandboxing until a
separate sandbox implementation exists and is tested.

## Security Review Checklist

Before enabling a module in production:

- package signature is valid against a trusted Ed25519 public key
- `core.compatible` matches the running Core version
- permissions are narrow and match the provider's real needs
- `permissions.database` prefixes are module-specific
- sidecar binary is built from a trusted source
- frontend remote is from the same signed package
- secret keys are named and approved explicitly
- module does not require direct DB, Redis, filesystem, or Core internal imports
- uninstall and purge behavior is understood by the admin
