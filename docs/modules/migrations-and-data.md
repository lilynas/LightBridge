# Module Migrations and Data

## Migration Tables

Core owns these module tables:

- `installed_modules`
- `module_permissions`
- `module_migrations`
- `module_runtime_instances`
- `ai_provider_instances`

Do not reuse payment provider tables for AI providers.

## Module Migration Rules

- Module migrations are applied after core migrations and before sidecar startup.
- Module migrations must be idempotent where practical.
- Applied migrations are tracked by `(module_id, migration_name)`.
- Existing applied module migrations are immutable by checksum.
- Non-transactional migrations must be explicitly marked by filename suffix if supported later.
- Installation statically validates referenced tables against `permissions.database`.

The first validator recognizes these SQL table references:

- `CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`
- `CREATE INDEX ... ON <table>`
- `COMMENT ON TABLE <table>` and `COMMENT ON COLUMN <table>.<column>`
- `INSERT INTO`, `UPDATE`, `DELETE FROM`, `TRUNCATE`
- `REFERENCES <table>`

This validator is intentionally conservative and is not a full SQL parser. Complex migrations that cannot pass this rule should be split into reviewed core migrations plus module-private migrations.

## Table Ownership

Module-owned tables must use a module-specific prefix:

```text
provider_openai_*
provider_anthropic_*
auth_2fa_*
auth_passkey_*
```

Provider modules must not alter core tables directly. If a core schema extension is required, add a core migration first and expose the data through CoreBridge or a documented extension column.

The module database permission value is treated as a table prefix. For example:

```yaml
permissions:
  database:
    - provider_openai_*
```

allows `provider_openai_config` and `provider_openai_tokens`, but does not allow `accounts`, `users`, `usage_logs`, or any other core table.

## Account Data

Core stores common account data:

- `provider_id`
- `credential_type`
- encrypted credentials
- proxy assignment
- group assignment
- schedulable/status/runtime fields

Provider modules store only provider-private configuration in module-owned tables or module config JSON.

## Disable, Uninstall, Purge

| Operation | Files | DB data | Runtime |
| --- | --- | --- | --- |
| `disable` | keep | keep | stop and unregister |
| `uninstall` | remove | keep | stop and unregister |
| `purge` | remove | remove module-private data | stop and unregister |

`uninstall` removes only the installed package directory. It keeps module migration records and any module-private tables so reinstalling the module can recover its configuration.

`purge` must require explicit confirmation and must never delete core audit logs. The current purge implementation enumerates `public` schema base tables and drops only table names that match manifest `permissions.database` prefixes.
## Migration Authoring Checklist

Use this checklist before adding any module migration:

- Migration filename is ordered and immutable, for example
  `001_create_provider_mock_config.sql`.
- Every table name is module-private and matches one declared
  `permissions.database` prefix.
- New tables use a narrow prefix such as `provider_mock_*`, not broad prefixes
  such as `provider_*`.
- Migration SQL does not `ALTER`, `DROP`, `INSERT`, `UPDATE`, `DELETE`,
  `TRUNCATE`, `COMMENT`, or `REFERENCES` any Core table.
- If the module needs a Core field such as `accounts.provider_id`, add that
  field through a Core migration first. Do not patch the Core table from a
  module migration.
- Migration can run more than once safely through the module migration registry;
  do not rely on manual reapplication.

Allowed example:

```sql
CREATE TABLE provider_mock_settings (
  id BIGSERIAL PRIMARY KEY,
  module_id TEXT NOT NULL,
  account_id BIGINT NOT NULL,
  config_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX provider_mock_settings_account_id
  ON provider_mock_settings(account_id);
```

Rejected examples:

```sql
ALTER TABLE accounts ADD COLUMN provider_specific_token TEXT;
DROP TABLE usage_logs;
CREATE TABLE provider_settings (...);
CREATE INDEX provider_mock_accounts_idx ON accounts(id);
```

The first two mutate Core tables. The third uses a prefix that is too broad and
does not match a specific module permission. The fourth creates an index on a
Core table.

## Data Lifecycle Rules

Module lifecycle states must preserve user data unless the user explicitly
requests destructive cleanup:

| Operation | Files | Module Private Tables | Core Account Rows | Secrets/Tokens |
| --- | --- | --- | --- | --- |
| `disable` | keep | keep | keep | keep |
| `enable` | keep | keep | keep | keep |
| `uninstall` | delete installed package directory | keep | keep | keep |
| `purge` | delete installed package directory | delete declared private tables | keep unless a future explicit data policy says otherwise | keep unless explicitly owned by module private tables |

Provider accounts, OAuth tokens, passkeys, API keys, user settings, and usage
logs are not deleted by `disable` or `uninstall`. `purge` deletes only module
private data that matches the declared database prefixes.

## Migration Review Evidence

Every module migration change should leave these review artifacts:

- module ID and version
- migration filename
- declared `permissions.database` prefixes
- list of SQL table references detected by the installer
- install result or rejection error code
- rollback/purge expectation

If a migration is rejected, prefer a new migration file or a Core migration. Do
not edit an already-published migration to make the error disappear.
