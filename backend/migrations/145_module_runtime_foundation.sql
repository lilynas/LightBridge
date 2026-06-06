-- Module runtime foundation for installable LightBridge modules.
-- This migration intentionally uses raw SQL tables instead of Ent schemas so
-- the module manager can land before generated Ent code is introduced.

CREATE TABLE IF NOT EXISTS installed_modules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    version TEXT NOT NULL,
    status TEXT NOT NULL,
    install_path TEXT NOT NULL,
    manifest_json JSONB NOT NULL,
    last_error TEXT,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    enabled_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('installed', 'enabled', 'disabled', 'failed', 'uninstalled', 'purged'))
);

CREATE TABLE IF NOT EXISTS module_permissions (
    module_id TEXT NOT NULL REFERENCES installed_modules(id) ON DELETE CASCADE,
    permission_type TEXT NOT NULL,
    permission_value TEXT NOT NULL,
    approved BOOLEAN NOT NULL DEFAULT false,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (module_id, permission_type, permission_value)
);

CREATE TABLE IF NOT EXISTS module_migrations (
    module_id TEXT NOT NULL REFERENCES installed_modules(id) ON DELETE CASCADE,
    migration_name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (module_id, migration_name)
);

CREATE TABLE IF NOT EXISTS module_runtime_instances (
    module_id TEXT PRIMARY KEY REFERENCES installed_modules(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    pid INTEGER,
    socket_path TEXT,
    started_at TIMESTAMPTZ,
    stopped_at TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    last_error TEXT,
    restart_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_provider_instances (
    id BIGSERIAL PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES installed_modules(id) ON DELETE RESTRICT,
    display_name TEXT NOT NULL,
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_installed_modules_status ON installed_modules(status);
CREATE INDEX IF NOT EXISTS idx_installed_modules_type ON installed_modules(type);
CREATE INDEX IF NOT EXISTS idx_module_permissions_module ON module_permissions(module_id);
CREATE INDEX IF NOT EXISTS idx_ai_provider_instances_provider ON ai_provider_instances(provider_id);
