package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
)

type ModuleStore struct {
	db *sql.DB
}

func NewModuleStore(db *sql.DB) *ModuleStore {
	return &ModuleStore{db: db}
}

func (s *ModuleStore) ListInstalled(ctx context.Context) ([]modules.InstalledModule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, COALESCE(last_error, '')
FROM installed_modules
WHERE status <> 'purged'
ORDER BY installed_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []modules.InstalledModule
	for rows.Next() {
		item, err := scanInstalledModule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ModuleStore) GetInstalled(ctx context.Context, id string) (*modules.InstalledModule, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, COALESCE(last_error, '')
FROM installed_modules
WHERE id = $1`, id)
	item, err := scanInstalledModule(row)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ModuleStore) SaveInstalled(ctx context.Context, module modules.InstalledModule) error {
	manifestBytes, err := json.Marshal(module.Manifest)
	if err != nil {
		return fmt.Errorf("marshal module manifest: %w", err)
	}
	installedAt := module.InstalledAt
	if installedAt.IsZero() {
		installedAt = time.Now().UTC()
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO installed_modules (id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, last_error, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NOW())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  type = EXCLUDED.type,
  version = EXCLUDED.version,
  status = EXCLUDED.status,
  install_path = EXCLUDED.install_path,
  manifest_json = EXCLUDED.manifest_json,
  enabled_at = EXCLUDED.enabled_at,
  last_error = EXCLUDED.last_error,
  updated_at = NOW()`,
		module.ID,
		module.Name,
		string(module.Type),
		module.Version,
		string(module.Status),
		module.InstallPath,
		manifestBytes,
		installedAt,
		module.EnabledAt,
		module.LastError,
	)
	return err
}

func (s *ModuleStore) SavePermissions(ctx context.Context, moduleID string, permissions []modules.PermissionRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM module_permissions WHERE module_id = $1`, moduleID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if permission.PermissionType == "" || permission.PermissionValue == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO module_permissions (module_id, permission_type, permission_value, approved, created_at)
VALUES ($1, $2, $3, false, NOW())
ON CONFLICT (module_id, permission_type, permission_value) DO NOTHING`,
			moduleID,
			permission.PermissionType,
			permission.PermissionValue,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *ModuleStore) ListPermissions(ctx context.Context, moduleID string) ([]modules.PermissionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT module_id, permission_type, permission_value, approved, approved_at, created_at
FROM module_permissions
WHERE module_id = $1
ORDER BY permission_type ASC, permission_value ASC`, moduleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []modules.PermissionRecord
	for rows.Next() {
		var record modules.PermissionRecord
		var approvedAt sql.NullTime
		if err := rows.Scan(
			&record.ModuleID,
			&record.PermissionType,
			&record.PermissionValue,
			&record.Approved,
			&approvedAt,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		if approvedAt.Valid {
			record.ApprovedAt = &approvedAt.Time
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ModuleStore) ApprovePermissions(ctx context.Context, moduleID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE module_permissions
SET approved = true, approved_at = NOW()
WHERE module_id = $1`, moduleID)
	return err
}

func (s *ModuleStore) ApplyMigration(ctx context.Context, moduleID string, migrationName string, checksum string, sqlText string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var existingChecksum string
	err = tx.QueryRowContext(ctx, `
SELECT checksum
FROM module_migrations
WHERE module_id = $1 AND migration_name = $2`, moduleID, migrationName).Scan(&existingChecksum)
	if err == nil {
		if existingChecksum != checksum {
			return fmt.Errorf("module migration %s checksum changed", migrationName)
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO module_migrations (module_id, migration_name, checksum, applied_at)
VALUES ($1, $2, $3, NOW())`, moduleID, migrationName, checksum)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ModuleStore) SetStatus(ctx context.Context, id string, status modules.ModuleStatus, lastError string) error {
	var enabledAt any
	if status == modules.ModuleStatusEnabled {
		enabledAt = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE installed_modules
SET status = $2,
    enabled_at = CASE WHEN $2 = 'enabled' THEN $3 WHEN $2 IN ('disabled', 'failed', 'uninstalled', 'purged') THEN NULL ELSE enabled_at END,
    last_error = NULLIF($4, ''),
    updated_at = NOW()
WHERE id = $1`, id, string(status), enabledAt, lastError)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *ModuleStore) PurgeModuleData(ctx context.Context, module modules.InstalledModule) error {
	prefixes := modules.DatabasePermissionPrefixes(module.Manifest.Permissions.Database)
	if len(prefixes) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_type = 'BASE TABLE'`)
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			_ = rows.Close()
			return err
		}
		if modules.DatabaseTableAllowedByPrefixes(table, prefixes) {
			tables = append(tables, table)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS `+quotePostgresIdent(table)+` CASCADE`); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *ModuleStore) UpdateRuntimeInstance(ctx context.Context, update modules.RuntimeInstanceUpdate) error {
	if update.ModuleID == "" {
		return fmt.Errorf("module runtime update requires module id")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO module_runtime_instances (
  module_id, status, pid, socket_path, started_at, stopped_at, last_heartbeat_at, last_error, restart_count, updated_at
)
VALUES (
  $1, $2, $3, NULLIF($4, ''), $5, $6, $7, NULLIF($8, ''),
  CASE WHEN $9 THEN 1 ELSE 0 END,
  NOW()
)
ON CONFLICT (module_id) DO UPDATE SET
  status = EXCLUDED.status,
  pid = EXCLUDED.pid,
  socket_path = COALESCE(EXCLUDED.socket_path, module_runtime_instances.socket_path),
  started_at = COALESCE(EXCLUDED.started_at, module_runtime_instances.started_at),
  stopped_at = COALESCE(EXCLUDED.stopped_at, module_runtime_instances.stopped_at),
  last_heartbeat_at = COALESCE(EXCLUDED.last_heartbeat_at, module_runtime_instances.last_heartbeat_at),
  last_error = EXCLUDED.last_error,
  restart_count = module_runtime_instances.restart_count + CASE WHEN $9 THEN 1 ELSE 0 END,
  updated_at = NOW()`,
		update.ModuleID,
		string(update.Status),
		update.PID,
		update.SocketPath,
		update.StartedAt,
		update.StoppedAt,
		update.LastHeartbeatAt,
		update.LastError,
		update.IncrementRestartCount,
	)
	return err
}

func quotePostgresIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

type installedModuleScanner interface {
	Scan(dest ...any) error
}

func scanInstalledModule(scanner installedModuleScanner) (*modules.InstalledModule, error) {
	var item modules.InstalledModule
	var manifestBytes []byte
	var enabledAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Type,
		&item.Version,
		&item.Status,
		&item.InstallPath,
		&manifestBytes,
		&item.InstalledAt,
		&enabledAt,
		&item.LastError,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	if enabledAt.Valid {
		item.EnabledAt = &enabledAt.Time
	}
	if err := json.Unmarshal(manifestBytes, &item.Manifest); err != nil {
		return nil, fmt.Errorf("unmarshal module manifest: %w", err)
	}
	return &item, nil
}
