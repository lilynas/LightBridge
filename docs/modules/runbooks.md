# Module System Runbooks

## Marketplace Returns No Modules

1. Check `modules.marketplace_registry_path` and `modules.marketplace_registry_url`.
2. If both are set, inspect the local file path first; it takes precedence over the URL.
3. Confirm the registry root JSON has a top-level `modules` array.
4. Confirm each entry has `id`, `version`, `type`, `core`, `downloadUrl`, and at least one allowed capability.
5. If using a URL, confirm it uses `http` or `https` and returns a 2xx status within `modules.marketplace_timeout_seconds`.

## Marketplace Install Fails Before Package Verification

1. Validate the selected `module_id` and `version` exactly match a registry entry.
2. Confirm `downloadUrl` uses a supported source: local path, `file://`, `http://`, or `https://`.
3. For local path or `file://`, verify the LightBridge process user can read the file and that it is a regular file.
4. For HTTP(S), verify the package URL returns 2xx within `modules.marketplace_timeout_seconds`.
5. If registry `sha256` is set, compute SHA256 over the archive bytes and compare it with the registry value.
6. After registry SHA256 passes, continue with package-level `module.yaml`, `checksums.txt`, and `signature.sig` checks.

## Install Fails: Checksum Mismatch

1. Distinguish registry archive SHA256 from package `checksums.txt`.
2. If the marketplace error is `MODULE_MARKETPLACE_SHA256_MISMATCH`, compare the downloaded archive with registry `sha256`.
3. If the installer reports a package checksum mismatch, compare `checksums.txt` entries with files inside the unpacked module root.
2. Re-download the package.
4. Reject the package if mismatch persists.

## Install Fails: Signature Mismatch

1. Confirm the signing key is trusted by Core config.
2. Confirm `signature.sig` signs the canonical `checksums.txt`.
3. Do not bypass signature verification in production.

## Module Startup Fails

1. Check `installed_modules.status` and `module_runtime_instances.last_error`.
2. Check module stdout/stderr logs.
3. Verify executable bit on `backend/<os>-<arch>/<binary>`.
4. Verify Unix socket path is writable by the LightBridge service user.
5. Confirm the sidecar binds `LIGHTBRIDGE_MODULE_SOCKET`, not a reconstructed
   `data/modules-runtime/<module-id>.sock` path. Core can shorten long socket
   paths to the OS temp directory.
6. If the module uses CoreBridge, confirm `LIGHTBRIDGE_CORE_BRIDGE_SOCKET` is
   present and dialable before the module makes CoreBridge calls.
7. Disable the module if it blocks provider availability.

## Module Migration Fails

1. Identify the failed module migration in `module_migrations`.
2. Inspect SQL for forbidden core table changes.
3. Fix with a new migration; do not mutate an already-applied migration.
4. Re-enable the module after migration succeeds.

## Provider Failed Or Crashed

1. Check `module_runtime_instances.status` for `failed` or `crashed`.
2. Check sidecar healthcheck and metadata identity errors.
3. Check account credential validation.
4. Confirm outbound network access declared by the module matches actual upstream.
5. Disable the provider or move affected groups to another registered provider.

## Remote UI Fails To Load

1. Open `/api/v1/modules/ui-manifest`.
2. Verify `remoteEntry` URL returns JavaScript.
3. Confirm module version in URL matches installed version.
4. Check browser console for missing shared dependency.
5. Disable the module UI contribution if it breaks admin workflow.

## Unix Socket Path Looks Wrong

1. Treat the environment variable as authoritative:
   `LIGHTBRIDGE_MODULE_SOCKET` for provider RPC and
   `LIGHTBRIDGE_CORE_BRIDGE_SOCKET` for CoreBridge.
2. On macOS, long workspace paths can exceed Unix socket limits. Core falls
   back from `data/modules-runtime/<module-id>.sock` to a shortened
   `lbm-<hash>.sock` path in the OS temp directory.
3. Do not hard-code the fallback directory in modules. Bind and dial only the
   exact paths supplied by Core.
4. When a module stops, crashes, fails to start, or fails healthcheck, Core
   removes both provider and CoreBridge socket files. Stale socket files after
   shutdown indicate cleanup did not run.

## Error Code Triage

Start from the stable error code, then inspect the smallest relevant surface.

| Error Code | First Place To Check | Next Action |
| --- | --- | --- |
| `MODULE_MANIFEST_INVALID` | `module.yaml` | Compare manifest with `package-spec.md`; validate required fields and ID/version format. |
| `MODULE_UNSUPPORTED_CAPABILITY` | `module.yaml.capabilities` | Remove unsupported capability or add a documented extension point before implementation. |
| `MODULE_CORE_INCOMPATIBLE` | `core.compatible` and Core build version | Update module compatibility range only if the protocol is actually compatible. |
| `MODULE_CHECKSUM_MISMATCH` | `checksums.txt` and expanded files | Recompute file hash; reject package if mismatch persists. |
| `MODULE_SIGNATURE_INVALID` | `signature.sig`, `checksums.txt`, trusted public key | Verify signature over raw `checksums.txt`; do not bypass in production. |
| `MODULE_PACKAGE_VERIFY_FAILED` | installed module directory | Re-run package checks for `module.yaml`, `checksums.txt`, `signature.sig`, referenced files, and `core.compatible`; reinstall from a trusted package if mismatch persists. |
| `MODULE_MARKETPLACE_ENTRY_INVALID` | registry JSON entry | Check `id`, `version`, `type`, `core`, `downloadUrl`, and capability allowlist. |
| `MODULE_MARKETPLACE_SHA256_MISMATCH` | downloaded archive bytes | Compare archive SHA256 with registry `sha256`; package-level checks are separate. |
| `MODULE_MIGRATION_FORBIDDEN_TABLE` | migration SQL table references | Move Core-table changes to Core migration or narrow module table prefixes. |
| `MODULE_RUNTIME_START_FAILED` | sidecar executable and runtime env | Check executable bit, command path, socket dir permissions, stdout/stderr. |
| `MODULE_RUNTIME_HEALTHCHECK_FAILED` | provider healthcheck RPC | Check sidecar bind path, handler registration, timeout, and startup latency. |
| `MODULE_RUNTIME_CRASHED` | runtime logs | Reproduce with same env vars; confirm Core marked runtime `crashed` and unregistered the provider. |
| `PROVIDER_MODULE_ACCOUNT_CONTRACT` | account row/payload | Ensure `platform=module`, `type=module`, `provider_id`, `extra.provider_id`, and `extra.module_id`. |
| `PROVIDER_MODULE_NOT_REGISTERED` | provider registry and enabled modules | Confirm module is enabled, runtime is `running`, and sidecar returned matching `Metadata.id`. |
| `PROVIDER_MODULE_UNHEALTHY` | runtime instance state | Fix `failed` or `crashed` runtime first; do not route to legacy provider branches. |
| `UI_MODULE_REMOTE_FAILED` | UI manifest and remote asset URL | Verify `remoteEntry` returns JavaScript and exposed module exists. |

When error names in code differ from this table, update
[testing.md](testing.md), this runbook, and the relevant implementation tests in
the same change.

## Mock Provider Fails During Local Verification

1. Confirm `module.yaml.id`, `Metadata.id`, account `provider_id`, and
   `extra.provider_id` are identical.
2. Confirm `module.yaml.backend.command` points at the binary for the current
   OS/arch and the file is executable.
3. Confirm the sidecar binds `LIGHTBRIDGE_MODULE_SOCKET`.
4. Confirm `HealthCheck` succeeds before registration.
5. Confirm `Forward` sends events in this order: `headers`, one or more `data`,
   optional `usage`, `done`.
6. Confirm every `GatewayEvent` is valid JSON and no event line exceeds the
   documented reader limit.
7. Confirm account credentials contain the expected mock secret key.
8. Confirm the gateway resolved the provider through `ProviderRegistry.Resolve`
   and did not enter a legacy provider branch.

## Admin UI Shows Provider But Account Form Fails

1. Open `/api/v1/modules/ui-manifest`.
2. Confirm `accountForms[].providerId` matches the provider ID exactly.
3. Confirm `accountForms[].exposedModule` starts with `./`.
4. Confirm `remoteEntry` URL returns JavaScript and includes the exposed account
   form module.
5. Confirm the form emits `submit` with `credential_type`, `credentials`, and
   optional `module_config`.
6. Confirm the shell wraps the form payload into the common account API with
   `platform=module`, `type=module`, `provider_id`, `extra.provider_id`, and
   `extra.module_id`.

## Provider Works In Admin Test But Gateway Fails

1. Confirm the admin test and gateway request use the same account ID/provider
   ID.
2. Confirm routing selected a module account, not a legacy account.
3. Inspect the normalized `GatewayRequest.endpoint` and
   `downstream_protocol`.
4. Confirm Core stripped downstream `Authorization`, `Cookie`, and `Set-Cookie`
   before sidecar dispatch.
5. Confirm provider emitted a `headers` event before `data` events.
6. Confirm provider errors are returned as `GatewayEvent.type = "error"` with a
   sanitized `NormalizedError`.
7. Confirm usage extraction does not block stream completion.

## Ent `provider_id` Codegen Is Out Of Sync

Symptoms:

- `ent/schema/account.go` contains `field.String("provider_id")`
- generated `ent/account.go` has no `ProviderID` field
- generated `ent/migrate/schema.go` has no `provider_id` column
- `ent/runtime/runtime.go` panics during init with an index error around account
  validators

Resolution order:

1. Keep the schema field. Do not remove `provider_id`; it is the modular
   provider identity bridge.
2. Stop any stuck `go generate ./ent` or broad `go test` process before
   starting another one.
3. Run Ent generation with a timeout:

```bash
cd backend
perl -e 'setpgrp(0,0); $SIG{ALRM}=sub{kill -TERM, -$$; exit 124}; alarm 180; exec @ARGV' \
  go run -mod=mod entgo.io/ent/cmd/ent generate \
  --feature sql/upsert,intercept,sql/execquery,sql/lock \
  --idtype int64 ./ent/schema
```

4. If dependency download fails, record the network/codegen blocker and keep
   using `extra.provider_id` as the compatibility source of truth.
5. Do not add calls to generated `SetProviderID` or `Account.ProviderID` until
   codegen has produced those symbols.
6. Once codegen succeeds, verify:

```bash
grep -n 'ProviderID\\|provider_id' backend/ent/account.go backend/ent/migrate/schema.go backend/ent/runtime/runtime.go
```

7. Re-run the focused provider bridge tests before touching gateway behavior.

Do not manually edit generated Ent files as a long-term fix. A temporary manual
bridge must be documented in the handoff and removed after codegen works.

## Upgrade Introduces New Permissions

1. Diff old and new `module.yaml.permissions`.
2. Treat any added `network`, `secrets`, `database`, `ui`, or `gateway` value as
   requiring admin review.
3. Keep the module installed but disabled until the new permission values are
   approved.
4. Record the approving admin, module version, permission type, and value.
5. Start the sidecar only after approval records exist for all declared
   permissions.
