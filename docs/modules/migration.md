# Module Migration

This migration path moves legacy OpenAI reverse-proxy data into the module-based
LightBridge runtime without carrying legacy runtime code forward.

Supported sources:

- `lightbridge`: legacy LightBridge database with built-in OpenAI reverse proxy
- `sub2api`: Sub2API database, including common SQLite deployments

The migrator preserves OpenAI OAuth, API key, CodeX reverse-proxy, passthrough,
WebSocket, proxy, and account scheduling metadata as data. Runtime behavior after
migration is provided by the installed `openai` Provider module.

## Build The OpenAI Provider Package

From the module example directory:

```bash
go run ./tools/build-package.go
```

The package is written to:

```text
examples/modules/lightbridge-provider-openai/dist/lightbridge-module-openai-0.1.0.tar.zst
```

## Dry Run

```bash
go run ./backend/cmd/module-migrate \
  --source-kind lightbridge \
  --source-driver postgres \
  --source-dsn "$OLD_LIGHTBRIDGE_DSN" \
  --target-driver postgres \
  --target-dsn "$NEW_LIGHTBRIDGE_DSN" \
  --openai-module-package examples/modules/lightbridge-provider-openai/dist/lightbridge-module-openai-0.1.0.tar.zst \
  --dry-run
```

For Sub2API SQLite:

```bash
go run ./backend/cmd/module-migrate \
  --source-kind sub2api \
  --source-driver sqlite \
  --source-dsn /path/to/sub2api.db \
  --target-driver postgres \
  --target-dsn "$NEW_LIGHTBRIDGE_DSN" \
  --openai-module-package examples/modules/lightbridge-provider-openai/dist/lightbridge-module-openai-0.1.0.tar.zst \
  --dry-run
```

## Apply Migration

Remove `--dry-run` after reviewing the report:

```bash
go run ./backend/cmd/module-migrate \
  --source-kind lightbridge \
  --source-driver postgres \
  --source-dsn "$OLD_LIGHTBRIDGE_DSN" \
  --target-driver postgres \
  --target-dsn "$NEW_LIGHTBRIDGE_DSN" \
  --openai-module-package examples/modules/lightbridge-provider-openai/dist/lightbridge-module-openai-0.1.0.tar.zst \
  --module-data-dir data
```

By default the migrator installs the OpenAI Provider module, approves its
declared permissions, and marks it enabled. The next LightBridge startup will
start enabled module runtimes.

## What Is Migrated

- Legacy OpenAI accounts become `platform=openai` and `provider_id=openai`.
- OAuth credentials keep `access_token`, `refresh_token`, `id_token`, `client_id`,
  ChatGPT account/user IDs, organization IDs, and subscription metadata when
  present.
- API key credentials keep API keys and base URL metadata.
- CodeX and reverse-proxy behavior flags are preserved as account `extra`
  metadata, including `codex_cli_only`, `openai_passthrough`, and OpenAI
  WebSocket mode fields.
- Proxy rows are copied or reused by exact endpoint match.
- Every migrated account receives a `module_migration` marker for idempotent
  re-runs.

## What Is Not Migrated

Legacy reverse-proxy implementation code is not copied into the new runtime.
After migration, OpenAI behavior is owned by the installed Provider module.
