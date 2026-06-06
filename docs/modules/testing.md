# Module System Testing

## Required Test Matrix

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Core startup | no modules installed | Core starts; login/admin/API key/group pages work. |
| Gateway | no provider for group | Stable `no_provider_for_group` error. |
| Installer | checksum mismatch | install rejected; no DB enabled state. |
| Installer | signature mismatch | install rejected; audit entry written. |
| Installer | incompatible `core.compatible` | install rejected before DB registration. |
| Manifest | unsupported capability | install rejected with explicit capability error. |
| Marketplace | registry not configured | marketplace API returns an empty module list. |
| Marketplace | invalid registry entry | marketplace API rejects the registry with explicit validation error. |
| Marketplace | local path / `file://` package | package copied to temp workspace, registry SHA256 checked if present, then package installer runs. |
| Marketplace | HTTP(S) package | package downloaded within timeout, registry SHA256 checked if present, then package installer runs. |
| Marketplace | archive SHA256 mismatch | install rejected before package unpacking. |
| Migration | migration references core table | install rejected before executing SQL. |
| Migration | migration references declared private table | migration allowed and applied once. |
| Migration | module migration fails | module marked `failed`; Core remains usable. |
| Lifecycle | disable module | runtime stopped; files and DB data remain. |
| Lifecycle | uninstall module | runtime stopped; install directory removed; DB data remains. |
| Lifecycle | purge module | private tables matching declared database prefixes removed; install directory removed. |
| Runtime | sidecar healthcheck timeout | module marked `failed`; provider not scheduled. |
| Runtime | sidecar `Metadata.id` mismatch | module marked `failed`; provider not registered. |
| Runtime | enabled module package tampered before Core restart | startup restore re-verifies installed package, marks module `failed`, and does not start sidecar. |
| Runtime | installed module package tampered before enable | enable returns `MODULE_PACKAGE_VERIFY_FAILED`, marks module `failed`, and does not start sidecar. |
| Runtime | sidecar crash mid-request | request returns provider error; Core does not panic. |
| UI | remote entry missing | route shows module error page; shell remains usable. |
| Account | module account without `provider_id` | gateway returns an explicit module account contract error. |
| Account | module account provider not registered | gateway returns provider-module error and does not fall back to legacy provider code. |
| Gateway | usage attached to `data` event | usage is captured in `ForwardResult` for existing logging/billing. |
| Gateway | Chat Completions channel mapping rewrites `model` before module forward | provider receives the final mapped body, final `metadata.model`, and final `stream` value. |
| Gateway | invalid Chat Completions JSON reaches service-level module branch | request is rejected before `ProviderAdapter.Forward` is called. |
| Provider | Anthropic streaming sample | request streams successfully and usage is persisted. |

## Backend Unit Tests

Add focused tests for:

- manifest parsing and validation
- capability allowlist enforcement
- module ID and version validation
- checksum file parsing
- install state transitions
- `core.compatible` range checks
- marketplace registry file and URL decoding
- marketplace entry validation and capability allowlist enforcement
- marketplace package download from local path, `file://`, HTTP, and HTTPS
- marketplace archive SHA256 verification before package installer delegation
- installed package re-verification before `Enable` starts a sidecar
- installed package re-verification during enabled module startup restore
- module migration private-table enforcement
- provider registry resolve errors
- provider sidecar metadata ID must match module ID before registration
- HTTP provider adapter protocol endpoints
- large provider NDJSON gateway events
- module provider bridge legacy fallback guard
- module provider bridge missing `provider_id` and missing registered provider errors
- module provider bridge usage extraction from both standalone `usage` events and `data.usage`
- module provider bridge Chat Completions public entrypoint routes module accounts through `ProviderRegistry.Resolve(provider_id)`
- module provider bridge forwards the final request body after Core rewrites, not a stale handler-level `ParsedRequest.Body`
- module provider bridge rejects invalid JSON before calling the provider adapter

## Frontend Unit Tests

Add focused tests for:

- UI manifest loading
- route contribution registration
- menu contribution ordering
- remote load failure fallback
- account form contribution selection

## Integration Tests

The first full integration test should install a local mock provider module from a fixture directory, enable it, start Core, and send one streaming gateway request through the sidecar.

Recommended fixture layout:

```text
backend/internal/modules/testdata/
  packages/
    valid-mock-provider/
      module.yaml
      backend/
        test-os-test-arch/
          lightbridge-provider-mock
      frontend/
        remoteEntry.js
      checksums.txt
      signature.sig
    checksum-mismatch/
    signature-mismatch/
    unsupported-capability/
    migration-core-table/
  registry/
    valid-local-path.json
    invalid-entry.json
    sha256-mismatch.json
  remotes/
    missing-remote-entry/
```

Mock provider sidecar behavior:

- `Metadata` returns `id = lightbridge.provider.mock`.
- `HealthCheck` succeeds unless the test sets `LIGHTBRIDGE_TEST_HEALTHCHECK_FAIL=1`.
- `Forward` emits `headers`, two `data` events, `usage`, then `done`.
- `Forward` exits mid-request when the test sets `LIGHTBRIDGE_TEST_CRASH_ON_FORWARD=1`.
- `TestAccount` rejects accounts missing `mock_api_key`.

## Critical End-To-End Scenarios

These scenarios are the minimum gate before claiming provider-module MVP works.

### 1. Core No-Provider Startup

Goal: Core starts with no installed modules and no provider business code loaded
from modules.

Expected evidence:

- login/admin/API key/group pages remain reachable
- `/api/v1/admin/modules/installed` returns `[]`
- `/api/v1/modules/ui-manifest` returns `[]`
- gateway request for a group with no account returns `no_provider_for_group`

Suggested focused command:

```bash
cd backend
go test ./internal/service -run 'TestProviderModuleBridge.*NoProvider|TestGateway.*NoProvider' -count=1 -timeout=90s
```

If those tests do not exist yet, add them before broadening the test run.

### 2. Install And Use Mock Provider

Goal: install a signed local mock provider package and route one streaming
request through the sidecar.

Expected evidence:

- installer accepts package checksums/signature
- module reaches `enabled`
- provider registry resolves `lightbridge.provider.mock`
- sidecar `Metadata.id`, UI account form `providerId`, and account
  `provider_id` all equal `lightbridge.provider.mock`
- module account uses `platform=module`, `type=module`, and `provider_id`
- gateway `Forward` receives `downstream_protocol=chat_completions`,
  `endpoint=/v1/chat/completions`, and the final request body after channel
  model mapping
- downstream streaming request returns two data chunks and `[DONE]`
- usage event is persisted or captured in `ForwardResult`

Suggested focused command:

```bash
cd backend
go test ./internal/modules -run 'TestPackageInstallerInstallsMockProviderExamplePackage|TestGRPCProviderAdapterTalksToMockProviderExampleSidecar' -count=1 -timeout=90s
go test ./internal/service -run 'TestProviderModuleBridgeGatewayForwardAsChatCompletionsUsesRegisteredProvider|TestProviderModuleBridgeForwardUsesRegisteredProvider' -count=1 -timeout=90s
```

### 3. Sidecar Crash

Goal: a provider sidecar crash does not crash Core or fall back to legacy
provider logic.

Expected evidence:

- current request returns a provider-module error
- runtime instance is marked `failed` or `crashed`
- stdout/stderr are captured
- `ProviderRegistry.Resolve` no longer schedules the crashed provider
- no legacy provider branch is entered

Suggested focused command:

```bash
cd backend
go test ./internal/modules -run 'Test.*Sidecar.*Crash|Test.*ProviderRuntime.*Crash' -count=1 -timeout=90s
```

### 4. Frontend Remote Failure

Goal: a missing or broken frontend remote does not break the Core shell.

Expected evidence:

- UI manifest still returns the module contribution
- route registration succeeds
- navigating to the route renders module error page
- sidebar/admin shell remain usable
- disabling the module removes or marks the contribution without deleting data

Suggested frontend command:

```bash
cd frontend
pnpm test -- --run ui-manifest remote-failure account-form-contribution
```

If the frontend test runner is not yet configured for these names, create a
fixture-driven unit test for the manifest loader before testing the full admin
route.

## Expected Error Codes

Use stable error codes in tests and runbooks. Error messages may change for
clarity, but codes should not change without updating this document.

| Code | Scenario |
| --- | --- |
| `MODULE_MANIFEST_INVALID` | `module.yaml` is missing required fields or has invalid values. |
| `MODULE_UNSUPPORTED_CAPABILITY` | Manifest declares a capability outside the MVP allowlist. |
| `MODULE_CORE_INCOMPATIBLE` | `core.compatible` does not match running Core version. |
| `MODULE_CHECKSUM_MISMATCH` | Expanded package file hash does not match `checksums.txt`. |
| `MODULE_SIGNATURE_INVALID` | `signature.sig` does not verify over raw `checksums.txt`. |
| `MODULE_PACKAGE_VERIFY_FAILED` | Installed package fails pre-runtime verification during enable. |
| `MODULE_MARKETPLACE_NOT_CONFIGURED` | Marketplace registry path and URL are both empty. |
| `MODULE_MARKETPLACE_ENTRY_INVALID` | Registry entry is missing required fields or declares invalid capabilities. |
| `MODULE_MARKETPLACE_SHA256_MISMATCH` | Downloaded archive bytes do not match registry `sha256`. |
| `MODULE_MIGRATION_FORBIDDEN_TABLE` | Migration references a table outside declared database prefixes. |
| `MODULE_MIGRATION_FAILED` | Allowed module migration fails during execution. |
| `MODULE_RUNTIME_START_FAILED` | Sidecar process fails to start. |
| `MODULE_RUNTIME_HEALTHCHECK_FAILED` | Sidecar healthcheck fails or times out. |
| `MODULE_RUNTIME_CRASHED` | Sidecar exits unexpectedly after startup. |
| `PROVIDER_MODULE_ACCOUNT_CONTRACT` | Module account is missing required `provider_id` or has invalid modular identity. |
| `PROVIDER_MODULE_NOT_REGISTERED` | Account references a module provider that is not registered. |
| `PROVIDER_MODULE_UNHEALTHY` | Provider exists but is not schedulable because runtime is `failed`, `crashed`, or unregistered. |
| `UI_MODULE_REMOTE_FAILED` | Remote entry or exposed module fails to load. |

When an implementation already has different stable names, update this table and
the runbooks in the same change. Do not leave tests asserting undocumented error
codes.

## Test Execution Rules

Use focused tests while the module system is being split. Avoid broad package
commands until the narrow tests are stable.

- Prefer `go test ./internal/modules -run '<specific-regex>'`.
- Prefer `go test ./internal/service -run 'TestProviderModuleBridge'`.
- Run `go generate ./ent` only when Ent schema or generated model fields change.
- If a command has no output for more than 30 seconds, stop it and record the
  blocker instead of starting another broad Go command.
- Do not run multiple package-loading/codegen commands in parallel.
- Do not treat a manually edited generated file as final if codegen is still
  failing; record it as a temporary bridge.

Recommended timeout wrapper for local debugging:

```bash
perl -e 'setpgrp(0,0); $SIG{ALRM}=sub{kill -TERM, -$$; exit 124}; alarm 90; exec @ARGV' \
  go test ./internal/modules -run 'TestProviderRuntime' -count=1
```

## Acceptance Record Template

Every module MVP verification pass should leave a short record with this shape:

```text
date:
core version:
branch/worktree:

commands:
  - <command>
    result: pass|fail|timeout
    notes:

package:
  module_id:
  version:
  archive:
  checksum result:
  signature result:

runtime:
  status:
  socket path:
  healthcheck:
  crash behavior:

provider:
  provider_id:
  account contract:
  registry resolve:
  streaming result:
  usage captured:

frontend:
  ui manifest:
  route contribution:
  menu contribution:
  account form:
  remote failure fallback:

remaining blockers:
  - <blocker>
```

This record is not a substitute for tests. It is the handoff evidence that lets
the next engineer continue without rediscovering which parts were already
verified.

## Current Verification Commands

Use narrow commands first:

```bash
cd backend
go test ./internal/modules -count=1 -timeout=90s
go test ./internal/repository -run 'TestModuleStore' -count=1 -timeout=90s
go test ./internal/service -run 'TestProviderModuleBridge' -count=1 -timeout=90s
go generate ./ent
```

If `go test ./internal/repository`, `go test ./internal/service`, or `go generate ./ent` produces no output for more than 30 seconds, stop the process and record the command as a local package-loading/codegen blocker. Do not start another broad Go command in parallel.

Tests and implementation must treat `accounts.provider_id` as the canonical
module provider ID. `extra.provider_id` remains a compatibility bridge for old
rows and exported data, so tests should cover both direct column reads/writes
and fallback reads from `extra.provider_id`.
