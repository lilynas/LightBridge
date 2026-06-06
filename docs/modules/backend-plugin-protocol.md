# Backend Plugin Protocol

## Runtime Model

Backend modules are sidecar processes supervised by LightBridge Core. Core starts each enabled module during startup after core migrations have completed and before provider capabilities are registered.

Primary MVP transport:

- Unix socket at `data/modules-runtime/<module-id>.sock`
- gRPC over the Unix socket
- JSON codec payloads for the first implementation, so provider modules can implement the contract without generated protobuf code
- one sidecar process per enabled module version

Compatibility transport:

- `backend.protocol: connect` keeps the older HTTP JSON-over-Unix-socket adapter available for migration and tests.
- New provider modules must use `backend.protocol: grpc`.

Do not use Go native `.so` plugins.

On macOS and other platforms with short Unix socket path limits, Core may shorten
the runtime path automatically. Modules must read the socket path from the
environment instead of reconstructing it from `data/modules-runtime`.

Runtime environment injected into sidecars:

| Variable | Purpose |
| --- | --- |
| `LIGHTBRIDGE_MODULE_ID` | Installed module ID from `module.yaml`. |
| `LIGHTBRIDGE_MODULE_VERSION` | Installed module version. |
| `LIGHTBRIDGE_MODULE_SOCKET` | Provider service Unix socket the sidecar must bind. |
| `LIGHTBRIDGE_CORE_BRIDGE_SOCKET` | CoreBridge Unix socket the module may dial for approved Core services. Present only when CoreBridge is configured. |

## Startup Sequence

1. Core loads DB/config.
2. Core applies core migrations.
3. Core loads enabled module manifests from DB and disk.
4. Core verifies checksums, signatures, compatible core version, and approved permissions.
5. Core applies module migrations.
6. Core starts sidecar processes.
7. Core waits for module healthcheck.
8. Core registers declared capabilities.

If a module fails any step, mark it `failed`, record a runtime error, and continue starting Core unless the module is explicitly marked as required by a future dependency policy.

For provider modules, identity is verified before registration. The value
returned by `ProviderAdapter.Metadata().id` must match `module.yaml.id`.
Core does not auto-fill or alias an empty metadata ID. If it is empty or
different, enabling the module fails before the adapter reaches the provider
registry.

## ProviderAdapter gRPC Contract

Provider modules implementing `provider.adapter` must bind `LIGHTBRIDGE_MODULE_SOCKET` and register the gRPC service:

```text
lightbridge.modules.ProviderAdapter
```

Core currently calls the service with gRPC's JSON content subtype. The wire method names are:

| RPC | Purpose |
| --- | --- |
| `Metadata(Empty) returns ProviderMetadata` | Returns provider ID, display name, and support metadata. |
| `HealthCheck(Empty) returns Empty` | Healthcheck used before registering the adapter. |
| `ListModels(ListModelsRequest) returns ListModelsResponse` | Lists provider models for account/config context. |
| `ValidateAccount(ProviderAccount) returns AccountValidationResult` | Validates account config and secrets. |
| `RefreshAccount(ProviderAccount) returns ProviderAccount` | Refreshes provider-owned credentials when supported. |
| `Forward(stream GatewayRequest) returns stream GatewayEvent` | Accepts a gateway request and streams normalized gateway events. |
| `TestAccount(TestAccountRequest) returns TestAccountResult` | Runs the admin account connectivity test. |
| `NormalizeError(UpstreamError) returns NormalizedError` | Converts upstream-specific errors to Core-normalized errors. |

Core wraps the gRPC sidecar client behind the internal `ProviderAdapter` interface. Gateway and scheduler code must depend on that interface only.

## Connect Compatibility Contract

Modules using `backend.protocol: connect` expose HTTP JSON endpoints on the same Unix socket:

| Endpoint | Purpose |
| --- | --- |
| `POST /provider/health` | Healthcheck used before registering the adapter. |
| `POST /provider/metadata` | Returns provider ID, display name, and support metadata. |
| `POST /provider/models` | Lists provider models for account/config context. |
| `POST /provider/validate-account` | Validates account config and secrets. |
| `POST /provider/refresh-account` | Refreshes provider-owned credentials when supported. |
| `POST /provider/forward` | Accepts a `GatewayRequest` and streams NDJSON `GatewayEvent` lines. |
| `POST /provider/test-account` | Runs the admin account connectivity test. |
| `POST /provider/normalize-error` | Converts upstream-specific errors to Core-normalized errors. |

The compatibility endpoints `/provider/chat-stream`, `/provider/embed`, and `/provider/count-tokens` can remain during migration, but new provider work should target the gRPC `Forward` RPC.

## Gateway Request Contract

`GatewayRequest` is normalized by Core before entering a provider:

- `downstream_protocol`
- `endpoint`
- `headers`
- `body`
- `stream`
- `user_context`
- `account`
- `group_context`
- `proxy_context`
- `metadata`

Core strips downstream `Authorization`, `Cookie`, and `Set-Cookie` before forwarding headers to a provider sidecar. Modules must not receive raw admin/user JWTs or unrelated secrets. Provider credentials are passed only through `GatewayRequest.account.secrets`.

For OpenAI-compatible Chat Completions requests handled by the generic
`GatewayService` path, module accounts are resolved before any legacy provider
conversion is attempted:

```text
/v1/chat/completions
  -> API key, billing, moderation, concurrency
  -> account selection
  -> account.platform=module or account.type=module
  -> ProviderRegistry.Resolve(account.provider_id)
  -> ProviderAdapter.Forward(GatewayRequest)
```

The `GatewayRequest` sent to the provider uses the final body Core is about to
forward, not the earlier body parsed by the handler. This matters when a channel
model mapping rewrites `model` before forwarding. In that case:

- `body` must contain the mapped request body.
- `stream` must be parsed from that final body.
- `metadata.model` must match the final body `model`.
- `downstream_protocol` is `chat_completions`.
- `endpoint` is `/v1/chat/completions`.

The OpenAI-specific `OpenAIGatewayService` scheduler only selects
`platform=openai` accounts. Module provider accounts are intentionally outside
that OpenAI-only scheduler and enter through the generic `GatewayService`
module bridge.

### Provider RPC Message Fields

The v1alpha1 SDK uses JSON field names. Unknown fields must be ignored by
providers so Core can add optional metadata later without breaking old modules.

`ProviderMetadata`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | Provider ID. Must match `module.yaml.id`. |
| `display_name` | string | yes | Human-readable provider name. |
| `supports` | object | yes | Boolean capability map: `chat`, `stream`, `responses`, `tools`, `vision`, `embeddings`, `images`. |
| `credential_types` | string[] | no | Example: `["api_key"]`, `["oauth"]`. |
| `extra` | object | no | Non-secret provider metadata such as upstream family or static endpoint hints. |

`ListModelsRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `credential_ref` | string | no | Core credential reference when listing account-scoped models. |
| `config` | object | no | Provider-specific non-secret config. |

`ModelInfo`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | Downstream model ID exposed by Core. |
| `display_name` | string | no | UI label. |
| `capabilities` | object | no | Capability booleans such as `stream`, `tools`, `vision`. |
| `metadata` | object | no | Non-secret model metadata, including upstream ID or context window when known. |

`ProviderAccount`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | Core account ID. |
| `provider_id` | string | yes | Resolved provider ID. |
| `display_name` | string | no | Account name. |
| `credential_ref` | string | no | Optional Core credential reference. |
| `config` | object | no | Non-secret provider config copied from account `extra`. |
| `secrets` | object | no | Approved credentials copied from account `credentials`. |
| `metadata` | object | no | Legacy bridge values such as `platform` and `type`. |

`AccountValidationResult`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `valid` | boolean | yes | Whether the account can be used. |
| `warnings` | string[] | no | Sanitized admin-facing warnings. |
| `metadata` | object | no | Non-secret validation details. |

`TestAccountRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `account` | `ProviderAccount` | yes | Account to test. |
| `mode` | string | no | `health`, `models`, `stream`, or provider-specific mode. |

`TestAccountResult`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `ok` | boolean | yes | Test result. |
| `message` | string | no | Sanitized admin-facing message. |
| `latency` | duration | no | End-to-end provider test latency. |
| `metadata` | object | no | Non-secret test details. |

`GatewayRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `downstream_protocol` | string | yes | Stable downstream protocol identifier. Example: `chat_completions`, `anthropic-compatible`, `gemini`. Do not infer it from `account.platform`; module accounts use `platform=module`. |
| `endpoint` | string | yes | Downstream endpoint path. |
| `method` | string | no | HTTP method when relevant. |
| `headers` | map string to string[] | no | Sanitized header allowlist. |
| `body` | object or string | yes | Final downstream body after Core-level rewrites such as channel model mapping. |
| `stream` | boolean | yes | Whether the final downstream body requested streaming. |
| `user_context` | object | no | Limited user context, never raw JWT. |
| `account` | `ProviderAccount` | yes | Selected provider account. |
| `group_context` | object | no | Selected group/routing context. |
| `proxy_context` | object | no | Selected outbound proxy. |
| `metadata` | object | no | Request ID, trace context, and non-secret normalized fields such as final `model`. |

`GatewayEvent`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `type` | string | yes | One of `headers`, `data`, `usage`, `error`, `done`. |
| `status_code` | number | for `headers` | HTTP status. |
| `headers` | map string to string[] | for `headers` | Sanitized upstream response headers. |
| `data` | object or string | for `data` | Downstream chunk. |
| `usage` | `TokenUsage` | for `usage`, optional on `data` | Usage extracted by provider. |
| `error` | `NormalizedError` | for `error` | Sanitized provider error. |
| `metadata` | object | no | Non-secret trace data. |

`TokenUsage`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `input_tokens` | number | no | Prompt/input tokens. |
| `output_tokens` | number | no | Completion/output tokens. |
| `total_tokens` | number | no | Total tokens. |

`UpstreamError`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `status_code` | number | no | Suggested downstream HTTP status. |
| `code` | string | no | Provider/core error code. |
| `message` | string | no | Sanitized message. |
| `headers` | object | no | Sanitized upstream response headers. |
| `body` | object | no | Sanitized upstream error body. |

`NormalizedError`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `retryable` | boolean | no | Whether Core may retry another account. |
| `status_code` | number | no | Suggested downstream HTTP status. |
| `code` | string | no | Stable provider/core error code. |
| `message` | string | no | Sanitized message. |
| `provider_raw` | object | no | Non-secret provider details useful for classification. |

Providers must not return upstream API keys, OAuth tokens, cookies, raw
authorization headers, or full request bodies inside `message` or `metadata`.

## Gateway Event Contract

Provider responses are streamed as events:

| Event | Purpose |
| --- | --- |
| `headers` | HTTP status and response headers for downstream output. |
| `data` | Streaming or buffered response bytes. |
| `usage` | Provider usage data normalized enough for core billing/logging. |
| `error` | Normalized provider error. |
| `done` | End of stream. |

Core owns downstream HTTP/SSE formatting and usage persistence.

Each event is one JSON object followed by `\n`. The core-side reader allows one event line up to 32 MiB; providers should still emit small incremental chunks. `usage` may be its own event or attached to a `data` event.

Module provider accounts must use `type = "module"` and `platform = "module"`
for newly created accounts. Core resolves the provider from `provider_id`, then
`extra.provider_id`, and only then the platform fallback. For provider-module
MVP work, `module.yaml.id`, `ProviderMetadata.id`, `accounts.provider_id`, and
`extra.provider_id` must be identical. If an account is marked as modular but
its provider is not registered, Core returns an explicit provider-module error
and does not enter legacy provider branches.

## CoreBridge RPC

Modules must not connect directly to the database. CoreBridge exposes only approved services:

- `GetUserSummary`
- `GetAccountCredentials`
- `WriteAuditLog`
- `GetModuleConfig`
- `UpdateProviderRuntimeStatus`

Core starts a per-module CoreBridge gRPC service before launching the sidecar and
passes the socket path through `LIGHTBRIDGE_CORE_BRIDGE_SOCKET`. The service name
is:

```text
lightbridge.modules.CoreBridge
```

Core binds module identity server-side. The request structs include
`module_id` for wire compatibility, but modules must not rely on caller-supplied
`module_id`; Core overwrites it with the supervised module ID before dispatching
to the bridge implementation.

Every credential read must produce an audit record with module ID, account ID,
credential reference or key, and request ID when available.

### CoreBridge Message Fields

CoreBridge also uses JSON field names. The supervised runtime binds caller
identity server-side; any request `module_id` is informational and must be
overwritten before authorization decisions.

`GetUserSummaryRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Ignored for auth; Core overwrites it. |
| `user_id` | string | no | Core user ID. |
| `email` | string | no | User email lookup key when supported. |

`UserSummary`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Supervised module ID. |
| `user_id` | string | yes | Core user ID. |
| `username` | string | no | Username when allowed. |
| `email` | string | no | User email when allowed. |
| `role` | string | no | Limited user role. |
| `groups` | string[] | no | Group IDs or names exposed by Core. |
| `enabled` | boolean | yes | Whether the user is enabled. |
| `metadata` | object | no | Non-secret user metadata. |

`GetAccountCredentialsRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Ignored for auth; Core overwrites it. |
| `account_id` | string | yes | Core account ID. |
| `provider_id` | string | yes | Provider ID expected by the caller. |
| `credential_ref` | string | no | Optional credential reference. |
| `purpose` | string | yes | `ValidateAccount`, `Forward`, `RefreshAccount`, or another sanitized purpose. |

`CoreBridgeAccountCredentials`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Supervised module ID. |
| `account_id` | string | yes | Core account ID. |
| `provider_id` | string | yes | Resolved provider ID. |
| `display_name` | string | no | Account display name. |
| `credential_ref` | string | no | Credential reference used for the read. |
| `config` | object | no | Non-secret provider config. |
| `secrets` | object | no | Approved secrets. |
| `metadata` | object | no | Non-secret account metadata. |
| `expires_at` | timestamp | no | Credential/account expiry when known. |

`WriteAuditLogRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Ignored for auth; Core overwrites it. |
| `actor_user_id` | string | no | Core user ID when the event is user initiated. |
| `action` | string | yes | Example: `module.secret.read`, `module.provider.status`. |
| `resource_type` | string | no | Resource category. |
| `resource_id` | string | no | Resource involved in the event. |
| `severity` | string | no | Severity classification. |
| `message` | string | no | Sanitized message. |
| `metadata` | object | no | Non-secret details. |
| `occurred_at` | timestamp | no | Event time supplied by the module. |

`GetModuleConfigRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Ignored for auth; Core overwrites it. |
| `key` | string | no | Optional config key. Empty means module-visible config. |

`ModuleConfig`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | yes | Supervised module ID. |
| `key` | string | no | Config key. |
| `config` | object | no | Module-visible config values. |
| `value` | any | no | Single config value when a key was requested. |

`UpdateProviderRuntimeStatusRequest`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `module_id` | string | no | Ignored for auth; Core overwrites it. |
| `provider_id` | string | yes | Provider reporting status. |
| `status` | string | yes | One of `starting`, `running`, `stopped`, `failed`, or `crashed`. |
| `message` | string | no | Sanitized status message. |
| `last_error` | string | no | Sanitized last runtime error. |
| `last_heartbeat_at` | timestamp | no | Provider heartbeat time. |
| `metadata` | object | no | Non-secret runtime metadata. |

CoreBridge forbidden behavior:

- no arbitrary SQL or DB handle
- no Redis handle
- no filesystem access API
- no access to other modules' private tables
- no raw user JWT/session/API key exposure
- no secret reads without approved `secrets` permission
- no caller-controlled `module_id` authorization

If a module needs a Core capability not listed here, add a documented
CoreBridge method and tests before using it. Do not tunnel new behavior through
generic `metadata` fields.

## Supervisor Rules

- Capture stdout/stderr into module runtime logs.
- Apply startup and healthcheck timeout.
- Mark module `failed` after startup, healthcheck, or metadata identity failure.
- Mark module `crashed` when a previously running sidecar exits unexpectedly.
- Stop module processes before closing DB/Redis during shutdown.
- Close CoreBridge before deleting runtime sockets during stop, crash cleanup, or
  failed healthcheck cleanup.

## Runtime Defaults

These defaults keep the MVP deterministic. If implementation uses different
values, update this table and the tests in the same change.

| Setting | Default | Notes |
| --- | --- | --- |
| startup timeout | manifest `backend.healthcheck.timeout`, fallback 10s | Time from process spawn to successful healthcheck and metadata identity verification. |
| healthcheck timeout | manifest `backend.healthcheck.timeout`, fallback 10s | Applied to the startup healthcheck loop and metadata call. |
| shutdown grace | none in MVP | Runtime currently terminates provider sidecars directly on disable/stop. |
| restart attempts | none in MVP | Unexpected exit is recorded as `crashed`; admin can enable the module again after inspection. |
| restart backoff | none in MVP | Do not add retry loops without updating supervisor tests and runbooks. |
| socket directory | `<modules.data_dir>/modules-runtime` | May be shortened on macOS path-length limits. |
| stdout/stderr capture | enabled | Redact secrets before exposing in admin UI. |
| max log preview | no admin preview in MVP | Logs are written to runtime log files. Add a bounded log API before exposing them in UI. |

Runtime status values:

| Status | Meaning |
| --- | --- |
| `starting` | Process spawned; healthcheck pending. |
| `running` | Healthcheck and metadata identity verification passed; provider adapter is registered. |
| `failed` | Startup, healthcheck, metadata identity, or install/enable validation failed. |
| `stopped` | Runtime stopped because module was disabled, uninstalled, purged, or Core is shutting down. |
| `crashed` | Sidecar exited unexpectedly after successful startup. |

Core must unregister provider capabilities before marking a runtime `stopped`,
`failed`, or `crashed`. Gateway routing must only schedule module providers
present in the provider registry.
