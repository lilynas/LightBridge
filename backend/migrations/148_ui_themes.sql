CREATE TABLE IF NOT EXISTS ui_themes (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    version VARCHAR(32) NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    entry_css TEXT NOT NULL,
    preview TEXT NOT NULL DEFAULT '',
    manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ui_themes_active ON ui_themes(active);
