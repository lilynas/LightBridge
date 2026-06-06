# Provider SDK

## Provider Module Responsibilities

A provider module owns:

- upstream protocol conversion
- provider authentication and token refresh
- provider model mapping
- account validation
- provider-specific usage extraction
- provider-specific error normalization

Core owns:

- downstream API key auth
- user/group/account selection
- balance and quota checks
- proxy assignment
- concurrency and scheduling
- logs, billing persistence, and downstream response formatting

## Minimal Provider Implementation

A minimal provider must implement:

- `Metadata`
- `HealthCheck`
- `ListModels`
- `ValidateAccount`
- `Forward`
- `TestAccount`

`RefreshAccount` may return no-op for API key providers. `NormalizeError` may return a generic upstream error until provider-specific classification is implemented.

## Gateway Forwarding Contract

Provider modules receive gateway traffic through `ProviderAdapter.Forward`.
They do not receive legacy provider-specific Core structs.

For OpenAI-compatible Chat Completions on the generic gateway path, Core sends:

| Field | Required behavior |
| --- | --- |
| `downstream_protocol` | `chat_completions` |
| `endpoint` | `/v1/chat/completions` |
| `method` | Downstream HTTP method, normally `POST` |
| `headers` | Sanitized downstream headers; `Authorization`, `Cookie`, and `Set-Cookie` are removed |
| `body` | Final request body after Core rewrites, including channel model mapping |
| `stream` | Parsed from the final request body |
| `account.provider_id` | Same value as `module.yaml.id` and `Metadata.id` |
| `account.secrets` | Approved provider credentials only |
| `metadata.model` | Final model from `body.model` |

Do not derive the provider identity from `account.config.platform`.
For module accounts, Core sets `platform=module` as a classification marker.
The provider identity is `account.provider_id`.

If a provider supports multiple downstream shapes, branch on
`downstream_protocol` and `endpoint`. Do not infer the protocol from the module
ID or account display name.

## Mock Provider Walkthrough

Use this walkthrough when validating the module runtime before building a real
OpenAI or Anthropic provider. The mock provider returns deterministic streaming
events and does not call an upstream service.

### 1. Create The Module Directory

```text
examples/modules/lightbridge-provider-mock/
  module.yaml
  backend/
    darwin-arm64/
      lightbridge-provider-mock
    linux-amd64/
      lightbridge-provider-mock
  frontend/
    remoteEntry.js
  migrations/
  checksums.txt
  signature.sig
```

For local development, build only the platform you are running. The package is
still valid when `module.yaml.backend.command` points at the single local
binary.

### 2. Write `module.yaml`

```yaml
apiVersion: lightbridge.dev/modules/v1alpha1
id: lightbridge.provider.mock
name: Mock Provider
type: provider
version: 0.1.0
core:
  compatible: ">=0.1.0 <0.2.0"
backend:
  kind: sidecar
  command: ./backend/darwin-arm64/lightbridge-provider-mock
  protocol: grpc
  healthcheck:
    rpc: HealthCheck
    timeout: 2s
frontend:
  kind: vite-remote-esm
  entry: ./frontend/remoteEntry.js
  routes:
    - path: /admin/providers/mock
      title: Mock Provider
      exposedModule: ./MockProviderSettings
  menu:
    - title: Mock Provider
      path: /admin/providers/mock
      group: Providers
  accountForms:
    - providerId: lightbridge.provider.mock
      exposedModule: ./MockAccountForm
capabilities:
  - provider.adapter
  - ui.admin.route
  - ui.account.form
permissions:
  network: []
  secrets:
    - mock_api_key
  database:
    - provider_mock_*
migrations: []
```

The provider ID in `Metadata.id`, `module.yaml.id`, account `provider_id`, and
`extra.provider_id` must be identical.

### 3. Implement The Sidecar

The first implementation uses gRPC over `LIGHTBRIDGE_MODULE_SOCKET`. Provider
modules should import the public module SDK when it exists. Until that package
is published, keep the mock provider small and mirror the JSON field names from
[Backend Plugin Protocol](backend-plugin-protocol.md).

Minimum behavior:

| RPC | Mock behavior |
| --- | --- |
| `Metadata` | Return `id = lightbridge.provider.mock`, display name, and chat/stream support. |
| `HealthCheck` | Return success if the sidecar is ready. |
| `ListModels` | Return `mock-chat` and `mock-stream`. |
| `ValidateAccount` | Accept accounts with `mock_api_key` present; reject missing secrets. |
| `Forward` | Read one `GatewayRequest`; stream `headers`, two `data` events, one `usage`, then `done`. |
| `TestAccount` | Return success with latency metadata. |
| `RefreshAccount` | Return the input account unchanged. |
| `NormalizeError` | Map unknown upstream errors to `provider_error`. |

Forward event sequence:

```json
{"type":"headers","status_code":200,"headers":{"content-type":["text/event-stream"]}}
{"type":"data","data":{"choices":[{"delta":{"content":"hello"}}]}}
{"type":"data","data":{"choices":[{"delta":{"content":" from mock"}}]}}
{"type":"usage","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}
{"type":"done"}
```

Implementation guardrails:

- Bind only `LIGHTBRIDGE_MODULE_SOCKET`.
- Remove a stale socket file before binding only when it equals
  `LIGHTBRIDGE_MODULE_SOCKET`.
- Do not open the LightBridge database.
- Use `LIGHTBRIDGE_CORE_BRIDGE_SOCKET` only for approved CoreBridge calls.
- Never log account secrets or downstream authorization headers.

Minimal handler shape:

```go
type MockProvider struct {
	moduleID string
}

func (p *MockProvider) Metadata(ctx context.Context, _ *Empty) (*ProviderMetadata, error) {
	return &ProviderMetadata{
		ID:          "lightbridge.provider.mock",
		DisplayName: "Mock Provider",
		CredentialTypes: []string{"api_key"},
		Extra: map[string]any{
			"downstream_protocols": []string{"openai-compatible"},
			"endpoints": []string{
				"/v1/chat/completions",
			},
		},
		Supports: map[string]bool{
			"chat":   true,
			"stream": true,
		},
	}, nil
}

func (p *MockProvider) HealthCheck(ctx context.Context, _ *Empty) (*Empty, error) {
	return &Empty{}, nil
}

func (p *MockProvider) ListModels(ctx context.Context, req *ListModelsRequest) (*ListModelsResponse, error) {
	return &ListModelsResponse{
		Models: []ModelInfo{
			{ID: "mock-chat", Capabilities: map[string]bool{"chat": true}},
			{ID: "mock-stream", Capabilities: map[string]bool{"chat": true, "stream": true}},
		},
	}, nil
}

func (p *MockProvider) ValidateAccount(ctx context.Context, account *ProviderAccount) (*AccountValidationResult, error) {
	if account.Secrets["mock_api_key"] == "" {
		return &AccountValidationResult{
			Valid:    false,
			Warnings: []string{"mock_api_key is required"},
		}, nil
	}
	return &AccountValidationResult{Valid: true}, nil
}

func (p *MockProvider) Forward(stream ProviderAdapter_ForwardServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	_ = req

	events := []*GatewayEvent{
		{Type: "headers", StatusCode: 200, Headers: map[string][]string{"content-type": []string{"text/event-stream"}}},
		{Type: "data", Data: map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "hello"}}}}},
		{Type: "data", Data: map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": " from mock"}}}}},
		{Type: "usage", Usage: &TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}},
		{Type: "done"},
	}
	for _, event := range events {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	return nil
}
```

This skeleton is intentionally illustrative. Keep the real implementation aligned
with the public module SDK types once they are published. If generated protobuf
types replace JSON codec structs, update this section and
[Backend Plugin Protocol](backend-plugin-protocol.md) together.

### 4. Build The Local Binary

```bash
cd examples/modules/lightbridge-provider-mock/backend-src
GOOS=darwin GOARCH=arm64 go build -o ../backend/darwin-arm64/lightbridge-provider-mock .
chmod +x ../backend/darwin-arm64/lightbridge-provider-mock
```

Use `GOOS=linux GOARCH=amd64` for the Linux package asset.

### 5. Generate Checksums And Signature

From the module root:

```bash
find module.yaml backend frontend migrations -type f 2>/dev/null \
  | LC_ALL=C sort \
  | xargs -I{} sh -c 'hash=$(shasum -a 256 "$1" | awk "{print \\$1}"); printf "sha256 %s %s\n" "$hash" "$1"' sh {} \
  > checksums.txt
```

Sign the raw `checksums.txt` bytes with the Ed25519 private key that matches
Core's `modules.signature_public_key_path`. The exact signing command depends
on the local key tool. The installer verifies `signature.sig` over the raw
`checksums.txt` content, not over the archive.

### 6. Pack The Module

```bash
tar --zstd -cf lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst \
  module.yaml backend frontend migrations checksums.txt signature.sig
```

If local `tar` does not support `--zstd`, use:

```bash
tar -cf - module.yaml backend frontend migrations checksums.txt signature.sig \
  | zstd -T0 -o lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst
```

### 7. Install Into Local Core

Use an archive install when testing without a marketplace registry:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/modules/install \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"archive_path":"/absolute/path/lightbridge-module-lightbridge.provider.mock-0.1.0.tar.zst"}'
```

Then enable the module:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/admin/modules/lightbridge.provider.mock/enable \
  -H "Authorization: Bearer <admin-token>"
```

Restart Core if the current build documents module activation as restart-bound.
After startup, verify:

```bash
curl -sS http://127.0.0.1:8080/api/v1/modules/ui-manifest \
  -H "Authorization: Bearer <admin-token>"
```

The response must include `lightbridge.provider.mock`.

### 8. Create A Module Account

Submit the shared account API payload with modular identity:

```json
{
  "name": "Mock Account",
  "platform": "module",
  "type": "module",
  "provider_id": "lightbridge.provider.mock",
  "credentials": {
    "mock_api_key": "test"
  },
  "extra": {
    "provider_id": "lightbridge.provider.mock",
    "module_id": "lightbridge.provider.mock"
  }
}
```

The account API must receive the top-level `provider_id`. Repository code writes
it to `accounts.provider_id` and keeps `extra.provider_id` in sync for old data,
exports, and module compatibility. Do not treat `platform = "module"` as a
provider identifier.

### 9. Send One Streaming Request

Use a downstream request routed to a group/account that selects the mock
provider:

```bash
curl -N http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock-stream",
    "stream": true,
    "messages": [{"role": "user", "content": "ping"}]
  }'
```

Expected downstream chunks:

```text
data: {"choices":[{"delta":{"content":"hello"}}]}
data: {"choices":[{"delta":{"content":" from mock"}}]}
data: [DONE]
```

Expected Core evidence:

- provider was resolved through `ProviderRegistry.Resolve("lightbridge.provider.mock")`
- no legacy provider branch was entered
- usage `{input_tokens:3, output_tokens:4, total_tokens:7}` was captured
- sidecar stdout/stderr logs are available in module runtime logs
- stopping the sidecar unregisters the provider without crashing Core

## MVP Sidecar Protocol

The primary runtime uses gRPC over a Unix socket. The module receives the socket path through `LIGHTBRIDGE_MODULE_SOCKET` and should bind only that socket.

Do not derive socket paths from the module ID. Core may shorten socket paths on
platforms with strict Unix socket length limits. Always read:

- `LIGHTBRIDGE_MODULE_ID`
- `LIGHTBRIDGE_MODULE_VERSION`
- `LIGHTBRIDGE_MODULE_SOCKET`
- `LIGHTBRIDGE_CORE_BRIDGE_SOCKET`

`LIGHTBRIDGE_CORE_BRIDGE_SOCKET` points to the per-module CoreBridge service for
approved Core data access. It is optional for pure stateless providers, but any
module that needs user summaries, account credentials, module config, audit
logging, or provider runtime status updates must use it instead of opening a DB
connection.

The MVP uses a gRPC JSON codec instead of generated protobuf code. This keeps the module SDK stable while the protocol is still v1alpha1. A provider module must register this service:

```text
lightbridge.modules.ProviderAdapter
```

Required RPCs:

| RPC | Request | Response |
| --- | --- | --- |
| `Metadata` | empty | `ProviderMetadata` |
| `HealthCheck` | empty | empty |
| `ListModels` | `ListModelsRequest` | `ListModelsResponse` |
| `ValidateAccount` | `ProviderAccount` | `AccountValidationResult` |
| `RefreshAccount` | `ProviderAccount` | `ProviderAccount` |
| `Forward` | client stream `GatewayRequest` | server stream `GatewayEvent` |
| `TestAccount` | `TestAccountRequest` | `TestAccountResult` |
| `NormalizeError` | `UpstreamError` | `NormalizedError` |

`Forward` is the target gateway path. Core sends one `GatewayRequest`, closes the send side, then reads `GatewayEvent` objects until the stream ends.

HTTP JSON compatibility remains available only for modules that declare `backend.protocol: connect`.

Compatibility endpoints:

| Endpoint | Request | Response |
| --- | --- | --- |
| `POST /provider/health` | empty | `204 No Content` |
| `POST /provider/metadata` | empty | `ProviderMetadata` |
| `POST /provider/models` | `ListModelsRequest` | `ListModelsResponse` |
| `POST /provider/validate-account` | `ProviderAccount` | `AccountValidationResult` |
| `POST /provider/refresh-account` | `ProviderAccount` | `ProviderAccount` |
| `POST /provider/test-account` | `TestAccountRequest` | `TestAccountResult` |
| `POST /provider/normalize-error` | `UpstreamError` | `NormalizedError` |
| `POST /provider/forward` | `GatewayRequest` | NDJSON stream of `GatewayEvent` |

Legacy compatibility endpoints kept during migration:

| Endpoint | Request | Response |
| --- | --- | --- |
| `POST /provider/chat-stream` | `ChatRequest` | NDJSON stream of `ChatEvent` |
| `POST /provider/embed` | `EmbeddingRequest` | `EmbeddingResponse` |
| `POST /provider/count-tokens` | `TokenCountRequest` | `TokenCountResponse` |

For the HTTP compatibility path, each NDJSON line from `/provider/forward` is a `GatewayEvent`. The core-side reader accepts one event line up to 32 MiB. Providers should still keep event lines small by streaming incremental chunks instead of buffering full upstream responses.

```json
{"type":"headers","status_code":200,"headers":{"content-type":["text/event-stream"]}}
{"type":"data","data":{"choices":[{"delta":{"content":"hello"}}]}}
{"type":"usage","usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}
{"type":"done"}
```

Valid `GatewayEvent.type` values are `headers`, `data`, `usage`, `error`, and `done`. Provider modules own upstream conversion and usage extraction; Core owns API key auth, account selection, quota/billing checks, concurrency, logging, and downstream HTTP/SSE formatting.

`usage` may be sent as a standalone event or embedded on a `data` event. Core reads `GatewayEvent.usage` before dispatching on the event type.

## Metadata Requirements

`Metadata` must return:

- provider ID matching `module.yaml`
- supported downstream protocols
- supported endpoints
- supported credential types
- account form schema or frontend account form contribution
- model capability hints

Example capability summary:

```json
{
  "id": "lightbridge.provider.openai-api",
  "displayName": "OpenAI API",
  "supports": {
    "chat": true,
    "stream": true,
    "responses": true,
    "embeddings": true,
    "images": true,
    "tools": true,
    "vision": true
  }
}
```

## Core Integration Rule

Gateway code must not branch on provider names. The only allowed flow is:

```go
adapter, err := providerRegistry.Resolve(providerID)
if err != nil {
    return err
}
events, err := adapter.Forward(ctx, request)
```

Do not add `if provider == "openai"` or `switch platform` branches for new moduleized providers.

## Account Contract

Provider module accounts use the shared core `accounts` table but are marked with the modular provider identity:

- `accounts.type = "module"`
- `accounts.provider_id = <provider id>`
- `accounts.extra.provider_id = <provider id>` for old-data and import/export compatibility
- `accounts.extra.module_id = <module id>`

During the first migration step the core still keeps legacy `platform` for existing scheduling and grouping flows. New module-created accounts should set:

- `platform = "module"`
- `type = "module"`
- `provider_id = <provider id>` in the API payload
- `extra.provider_id = <provider id>`
- `extra.module_id = <module id>`

The generated Ent model includes `Account.ProviderID`. Repository code must call
`SetProviderID`, read `m.ProviderID`, and preserve `extra.provider_id` only as a
compatibility bridge. Admin list filters and schedulable account queries match
legacy `platform`, top-level `provider_id`, and legacy `extra.provider_id`.

The gateway treats `type = "module"`, `platform = "module"`, or non-empty `extra.module_id` as the signal to resolve through `ProviderRegistry` instead of legacy platform branches. If an account is marked as a module account and its provider is not registered, Core returns an explicit provider-module error. It must not fall back to the legacy Claude/Anthropic path.

The provider sidecar receives `ProviderAccount` with:

- `id`: core account ID as a string
- `provider_id`: resolved provider ID
- `display_name`: account name
- `config`: non-secret account configuration copied from `extra`
- `secrets`: account credentials copied from `credentials`
- `metadata.platform` and `metadata.type`: legacy bridge values

Core strips downstream `Authorization`, `Cookie`, and `Set-Cookie` headers before forwarding `GatewayRequest` to provider sidecars. Core also strips sensitive hop/auth headers from provider responses before writing downstream output.

## CoreBridge Usage

Provider modules must access Core-owned data through CoreBridge only. The gRPC
service name is:

```text
lightbridge.modules.CoreBridge
```

Available methods:

| RPC | Purpose |
| --- | --- |
| `GetUserSummary` | Read a limited user profile for routing, audit context, or provider policy. |
| `GetAccountCredentials` | Read approved account config and secrets for the module/provider. |
| `WriteAuditLog` | Append module audit events for credential reads, status changes, and admin actions. |
| `GetModuleConfig` | Read module-level configuration. |
| `UpdateProviderRuntimeStatus` | Report provider health/status from the sidecar. |

Core overwrites `module_id` on every CoreBridge request with the supervised
module ID. Modules may send an empty `module_id`; they must not use a spoofed
`module_id` as an authorization mechanism or expect it to pass through.

Any call that reads credentials must be paired with `WriteAuditLog` when the
CoreBridge implementation does not do it automatically. The audit metadata
should include account ID, credential reference/key, purpose, and request ID if
one exists in the gateway or admin flow.

## Gateway Event Semantics

Provider sidecars stream `GatewayEvent` objects from gRPC `Forward`. The HTTP compatibility path streams newline-delimited `GatewayEvent` objects from `/provider/forward`.

- `headers`: core copies allowed headers to the downstream response, applies `status_code`, and marks the upstream as accepted for early lock release.
- `data`: for non-stream responses, core writes `data` bytes directly. For stream responses, core writes `data: <data>\n\n`.
- `usage`: core converts `TokenUsage` into `ForwardResult.Usage` so existing usage logging and billing can run.
- `error`: core writes a sanitized downstream error if headers are not already written and returns provider error.
- `done`: for stream responses, core writes `data: [DONE]\n\n`.

The sidecar should send provider-normalized downstream chunks in `data`, not raw upstream protocol frames unless it intentionally exposes the same downstream protocol.

## First Sample Provider

Use Anthropic API as the first real provider module because it exercises streaming, messages, model listing, usage extraction, and error normalization without the full complexity of ChatGPT Web/OAuth sessions.

Acceptance for the sample provider:

- module installs and enables
- provider appears in admin provider list
- account form loads from module contribution
- account validation succeeds
- `/v1/messages` streaming request succeeds through the sidecar
- usage event is persisted by Core
- sidecar crash marks runtime `crashed` and unregisters the provider without crashing Core

## Real Provider Constraints

Build real providers after the mock provider path is green. The first real
providers should be API-key providers because they avoid browser-session and
OAuth refresh complexity while proving the module boundary.

### OpenAI API Provider

Module ID:

```text
lightbridge.provider.openai-api
```

Responsibilities:

- Convert normalized OpenAI-compatible downstream requests to OpenAI upstream
  requests.
- Support `/v1/chat/completions` first; add `/v1/responses` only after the
  module can stream chat completions reliably.
- Keep OpenAI API key handling inside `ProviderAccount.secrets`.
- Convert OpenAI stream chunks into `GatewayEvent.type = "data"` without
  leaking upstream authorization headers.
- Extract usage from final chunks or non-stream response body and emit
  `GatewayEvent.type = "usage"`.
- Normalize OpenAI rate limit, quota, auth, and model-not-found errors.

Do not:

- Add OpenAI-specific router, sidebar, account form, or model mapping in Core.
- Add `if provider == "openai"` or `switch platform` branches.
- Store OpenAI-specific config in Core tables except through common account
  fields and module-private tables.

Minimum model list:

```json
[
  {"id": "gpt-4o", "capabilities": {"chat": true, "stream": true, "vision": true, "tools": true}},
  {"id": "gpt-4.1", "capabilities": {"chat": true, "stream": true, "tools": true}}
]
```

### Anthropic API Provider

Module ID:

```text
lightbridge.provider.anthropic-api
```

Responsibilities:

- Convert normalized downstream messages into Anthropic Messages API requests.
- Support `/v1/messages` and streaming first.
- Map Anthropic `message_start`, `content_block_delta`, `message_delta`, and
  `message_stop` events into normalized `GatewayEvent` objects.
- Extract input/output token usage from Anthropic response events.
- Normalize auth, rate limit, overload, model-not-found, and invalid-request
  errors.

Downstream behavior:

- Core remains responsible for downstream HTTP/SSE formatting.
- The provider module returns normalized chunks in `GatewayEvent.data`.
- Usage may be attached to a `data` event or emitted as a standalone `usage`
  event, but the final request result must expose usage to Core logging.

Do not:

- Add Anthropic-specific quota or retry rules in Core.
- Reuse legacy Claude/Anthropic account paths for module accounts.
- Fall back from `provider_id = lightbridge.provider.anthropic-api` to legacy
  `platform = anthropic`.

### Web Providers Are Later Phase

ChatGPT Web, Claude Web, Gemini OAuth, and Antigravity-style browser/session
providers are not the first sample provider. They require extra module-owned
work:

- OAuth/session refresh
- browser/session risk controls
- sticky account/session routing
- provider-private token storage
- more detailed account validation

They still must use the same ProviderAdapter and CoreBridge contracts. Do not
add web-provider exceptions to Core.
