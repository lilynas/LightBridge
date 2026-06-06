package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/LightBridge/internal/modules"
	"github.com/stretchr/testify/require"
)

func TestModuleStoreListInstalled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	manifest := testModuleManifest()
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	installedAt := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	enabledAt := installedAt.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"id", "name", "type", "version", "status", "install_path", "manifest_json", "installed_at", "enabled_at", "last_error",
	}).AddRow(
		manifest.ID,
		manifest.Name,
		string(manifest.Type),
		manifest.Version,
		string(modules.ModuleStatusEnabled),
		"/data/modules/lightbridge.provider.openai-api/0.1.0",
		manifestBytes,
		installedAt,
		enabledAt,
		"",
	)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, COALESCE(last_error, '')
FROM installed_modules
WHERE status <> 'purged'
ORDER BY installed_at DESC, id ASC`)).WillReturnRows(rows)

	got, err := store.ListInstalled(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, manifest.ID, got[0].ID)
	require.Equal(t, modules.ModuleStatusEnabled, got[0].Status)
	require.NotNil(t, got[0].EnabledAt)
	require.Equal(t, manifest.Frontend.Routes[0].Path, got[0].Manifest.Frontend.Routes[0].Path)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreGetInstalledNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, name, type, version, status, install_path, manifest_json, installed_at, enabled_at, COALESCE(last_error, '')
FROM installed_modules
WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	got, err := store.GetInstalled(context.Background(), "missing")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Nil(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreSaveInstalled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	manifest := testModuleManifest()
	installedAt := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	item := modules.InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      modules.ModuleStatusInstalled,
		InstallPath: "/data/modules/lightbridge.provider.openai-api/0.1.0",
		Manifest:    manifest,
		InstalledAt: installedAt,
	}

	mock.ExpectExec(regexp.QuoteMeta(`
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
  updated_at = NOW()`)).
		WithArgs(
			item.ID,
			item.Name,
			string(item.Type),
			item.Version,
			string(item.Status),
			item.InstallPath,
			sqlmock.AnyArg(),
			installedAt,
			item.EnabledAt,
			item.LastError,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.SaveInstalled(context.Background(), item))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreSavePermissions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	moduleID := "lightbridge.provider.openai-api"
	permissions := []modules.PermissionRecord{{
		ModuleID:        moduleID,
		PermissionType:  "network",
		PermissionValue: "https://api.openai.com/*",
	}}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM module_permissions WHERE module_id = $1`)).
		WithArgs(moduleID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO module_permissions (module_id, permission_type, permission_value, approved, created_at)
VALUES ($1, $2, $3, false, NOW())
ON CONFLICT (module_id, permission_type, permission_value) DO NOTHING`)).
		WithArgs(moduleID, "network", "https://api.openai.com/*").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, store.SavePermissions(context.Background(), moduleID, permissions))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreListAndApprovePermissions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	moduleID := "lightbridge.provider.openai-api"
	createdAt := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	approvedAt := createdAt.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"module_id", "permission_type", "permission_value", "approved", "approved_at", "created_at",
	}).AddRow(moduleID, "network", "https://api.openai.com/*", true, approvedAt, createdAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT module_id, permission_type, permission_value, approved, approved_at, created_at
FROM module_permissions
WHERE module_id = $1
ORDER BY permission_type ASC, permission_value ASC`)).
		WithArgs(moduleID).
		WillReturnRows(rows)

	got, err := store.ListPermissions(context.Background(), moduleID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.True(t, got[0].Approved)
	require.NotNil(t, got[0].ApprovedAt)

	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE module_permissions
SET approved = true, approved_at = NOW()
WHERE module_id = $1`)).
		WithArgs(moduleID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.ApprovePermissions(context.Background(), moduleID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreApplyMigrationExecutesAndRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	moduleID := "lightbridge.provider.openai-api"
	migrationName := "migrations/001_create_provider_openai_config.sql"
	checksum := "0123456789abcdef"
	sqlText := "CREATE TABLE provider_openai_config (id TEXT PRIMARY KEY);"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT checksum
FROM module_migrations
WHERE module_id = $1 AND migration_name = $2`)).
		WithArgs(moduleID, migrationName).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta(sqlText)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO module_migrations (module_id, migration_name, checksum, applied_at)
VALUES ($1, $2, $3, NOW())`)).
		WithArgs(moduleID, migrationName, checksum).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, store.ApplyMigration(context.Background(), moduleID, migrationName, checksum, sqlText))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreApplyMigrationSkipsSameChecksum(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	moduleID := "lightbridge.provider.openai-api"
	migrationName := "migrations/001_create_provider_openai_config.sql"
	checksum := "0123456789abcdef"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT checksum
FROM module_migrations
WHERE module_id = $1 AND migration_name = $2`)).
		WithArgs(moduleID, migrationName).
		WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(checksum))
	mock.ExpectCommit()

	require.NoError(t, store.ApplyMigration(context.Background(), moduleID, migrationName, checksum, "SELECT 1;"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreApplyMigrationRejectsChecksumChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	moduleID := "lightbridge.provider.openai-api"
	migrationName := "migrations/001_create_provider_openai_config.sql"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT checksum
FROM module_migrations
WHERE module_id = $1 AND migration_name = $2`)).
		WithArgs(moduleID, migrationName).
		WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow("old-checksum"))
	mock.ExpectRollback()

	err = store.ApplyMigration(context.Background(), moduleID, migrationName, "new-checksum", "SELECT 1;")
	require.ErrorContains(t, err, "checksum changed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreSetStatusNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE installed_modules
SET status = $2,
    enabled_at = CASE WHEN $2 = 'enabled' THEN $3 WHEN $2 IN ('disabled', 'failed', 'uninstalled', 'purged') THEN NULL ELSE enabled_at END,
    last_error = NULLIF($4, ''),
    updated_at = NOW()
WHERE id = $1`)).
		WithArgs("missing", string(modules.ModuleStatusDisabled), nil, "").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.SetStatus(context.Background(), "missing", modules.ModuleStatusDisabled, "")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStorePurgeModuleDataDropsOnlyDeclaredPrefixTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	manifest := testModuleManifest()
	manifest.Permissions.Database = []string{"provider_openai_*"}
	module := modules.InstalledModule{
		ID:       manifest.ID,
		Name:     manifest.Name,
		Type:     manifest.Type,
		Version:  manifest.Version,
		Status:   modules.ModuleStatusDisabled,
		Manifest: manifest,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_type = 'BASE TABLE'`)).
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).
			AddRow("accounts").
			AddRow("provider_openai_config").
			AddRow("provider_openai_tokens"))
	mock.ExpectExec(regexp.QuoteMeta(`DROP TABLE IF EXISTS "provider_openai_config" CASCADE`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DROP TABLE IF EXISTS "provider_openai_tokens" CASCADE`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	require.NoError(t, store.PurgeModuleData(context.Background(), module))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStorePurgeModuleDataSkipsWithoutDatabasePermission(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	manifest := testModuleManifest()
	manifest.Permissions.Database = nil

	require.NoError(t, store.PurgeModuleData(context.Background(), modules.InstalledModule{Manifest: manifest}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModuleStoreUpdateRuntimeInstance(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := NewModuleStore(db)
	startedAt := time.Date(2026, 5, 31, 8, 0, 0, 0, time.UTC)
	pid := 1234
	update := modules.RuntimeInstanceUpdate{
		ModuleID:              "lightbridge.provider.openai-api",
		Status:                modules.RuntimeStatusRunning,
		PID:                   &pid,
		SocketPath:            "/data/modules-runtime/lightbridge.provider.openai-api.sock",
		StartedAt:             &startedAt,
		LastHeartbeatAt:       &startedAt,
		IncrementRestartCount: true,
	}

	mock.ExpectExec(regexp.QuoteMeta(`
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
  updated_at = NOW()`)).
		WithArgs(
			update.ModuleID,
			string(update.Status),
			update.PID,
			update.SocketPath,
			update.StartedAt,
			update.StoppedAt,
			update.LastHeartbeatAt,
			update.LastError,
			update.IncrementRestartCount,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.UpdateRuntimeInstance(context.Background(), update))
	require.NoError(t, mock.ExpectationsWereMet())
}

func testModuleManifest() modules.Manifest {
	return modules.Manifest{
		APIVersion: modules.ManifestAPIVersionV1Alpha1,
		ID:         "lightbridge.provider.openai-api",
		Name:       "OpenAI API Provider",
		Type:       modules.ModuleTypeProvider,
		Version:    "0.1.0",
		Core: modules.CoreSpec{
			Compatible: ">=0.1.0 <0.2.0",
		},
		Backend: &modules.BackendSpec{
			Kind:     modules.BackendKindSidecar,
			Command:  "./backend/lightbridge-provider-openai",
			Protocol: modules.BackendProtocolConnect,
		},
		Frontend: &modules.FrontendSpec{
			Kind:  modules.FrontendKindViteRemoteESM,
			Entry: "./frontend/remoteEntry.js",
			Routes: []modules.UIRouteSpec{{
				Path:          "/admin/providers/openai",
				Title:         "OpenAI API",
				ExposedModule: "./OpenAIProviderSettings",
				RequiresAdmin: true,
			}},
			Menu: []modules.UIMenuSpec{{
				Title: "OpenAI API",
				Path:  "/admin/providers/openai",
				Group: "Providers",
				Order: 10,
			}},
			AccountForms: []modules.AccountFormSpec{{
				ProviderID:    "lightbridge.provider.openai-api",
				ExposedModule: "./OpenAIAccountForm",
			}},
		},
		Capabilities: []modules.Capability{
			modules.CapabilityProviderAdapter,
			modules.CapabilityUIAdminRoute,
			modules.CapabilityUIAccountForm,
		},
	}
}
