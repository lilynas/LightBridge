package modules

import "context"

type Store interface {
	ListInstalled(ctx context.Context) ([]InstalledModule, error)
	GetInstalled(ctx context.Context, id string) (*InstalledModule, error)
	SaveInstalled(ctx context.Context, module InstalledModule) error
	SavePermissions(ctx context.Context, moduleID string, permissions []PermissionRecord) error
	ListPermissions(ctx context.Context, moduleID string) ([]PermissionRecord, error)
	ApprovePermissions(ctx context.Context, moduleID string) error
	ApplyMigration(ctx context.Context, moduleID string, migrationName string, checksum string, sql string) error
	SetStatus(ctx context.Context, id string, status ModuleStatus, lastError string) error
}

type DataPurger interface {
	PurgeModuleData(ctx context.Context, module InstalledModule) error
}

type RuntimeStore interface {
	UpdateRuntimeInstance(ctx context.Context, update RuntimeInstanceUpdate) error
}
