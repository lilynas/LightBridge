package modulemigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
)

type sqlModuleStore struct {
	db *sql.DB
}

func newSQLModuleStore(db *sql.DB) *sqlModuleStore {
	return &sqlModuleStore{db: db}
}

func (s *sqlModuleStore) ListInstalled(ctx context.Context) ([]modules.InstalledModule, error) {
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
	return result, rows.Err()
}

func (s *sqlModuleStore) GetInstalled(ctx context.Context, id string) (*modules.InstalledModule, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, COALESCE(last_error, '')
FROM installed_modules
WHERE id = $1`, id)
	return scanInstalledModule(row)
}

func (s *sqlModuleStore) SaveInstalled(ctx context.Context, module modules.InstalledModule) error {
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
		string(manifestBytes),
		installedAt,
		module.EnabledAt,
		module.LastError,
	)
	return err
}

func (s *sqlModuleStore) SavePermissions(ctx context.Context, moduleID string, permissions []modules.PermissionRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM module_permissions WHERE module_id = $1`, moduleID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO module_permissions (module_id, permission_type, permission_value, approved, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (module_id, permission_type, permission_value) DO NOTHING`,
			moduleID,
			permission.PermissionType,
			permission.PermissionValue,
			permission.Approved,
			permission.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlModuleStore) ListPermissions(ctx context.Context, moduleID string) ([]modules.PermissionRecord, error) {
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
		var item modules.PermissionRecord
		var approvedAt sql.NullTime
		if err := rows.Scan(&item.ModuleID, &item.PermissionType, &item.PermissionValue, &item.Approved, &approvedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if approvedAt.Valid {
			item.ApprovedAt = &approvedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *sqlModuleStore) ApprovePermissions(ctx context.Context, moduleID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE module_permissions
SET approved = true, approved_at = NOW()
WHERE module_id = $1`, moduleID)
	return err
}

func (s *sqlModuleStore) ApplyMigration(ctx context.Context, moduleID string, migrationName string, checksum string, sqlText string) error {
	var existingChecksum string
	err := s.db.QueryRowContext(ctx, `
SELECT checksum FROM module_migrations
WHERE module_id = $1 AND migration_name = $2`, moduleID, migrationName).Scan(&existingChecksum)
	if err == nil {
		if existingChecksum != checksum {
			return fmt.Errorf("module migration checksum mismatch for %s/%s", moduleID, migrationName)
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if _, err := s.db.ExecContext(ctx, sqlText); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO module_migrations (module_id, migration_name, checksum, applied_at)
VALUES ($1, $2, $3, NOW())`, moduleID, migrationName, checksum)
	return err
}

func (s *sqlModuleStore) SetStatus(ctx context.Context, id string, status modules.ModuleStatus, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE installed_modules
SET status = $1, last_error = NULLIF($2, ''), updated_at = NOW()
WHERE id = $3`, string(status), lastError, id)
	return err
}

type installedModuleScanner interface {
	Scan(dest ...any) error
}

func scanInstalledModule(scanner installedModuleScanner) (*modules.InstalledModule, error) {
	var item modules.InstalledModule
	var manifestRaw string
	var moduleType string
	var moduleStatus string
	var enabledAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&moduleType,
		&item.Version,
		&moduleStatus,
		&item.InstallPath,
		&manifestRaw,
		&item.InstalledAt,
		&enabledAt,
		&item.LastError,
	); err != nil {
		return nil, err
	}
	if enabledAt.Valid {
		item.EnabledAt = &enabledAt.Time
	}
	item.Type = modules.ModuleType(moduleType)
	item.Status = modules.ModuleStatus(moduleStatus)
	if err := json.Unmarshal([]byte(manifestRaw), &item.Manifest); err != nil {
		return nil, fmt.Errorf("decode module manifest: %w", err)
	}
	return &item, nil
}
