// Package modules contains the core contracts for installable LightBridge
// modules. It intentionally has no dependency on handler/service packages so it
// can be shared by installers, runtime supervisors, and tests.
package modules

import "time"

const (
	ManifestAPIVersionV1Alpha1 = "lightbridge.dev/modules/v1alpha1"
)

type ModuleType string

const (
	ModuleTypeProvider ModuleType = "provider"
	ModuleTypeUI       ModuleType = "ui"
	ModuleTypeGateway  ModuleType = "gateway"
)

type ModuleStatus string

const (
	ModuleStatusInstalled   ModuleStatus = "installed"
	ModuleStatusEnabled     ModuleStatus = "enabled"
	ModuleStatusDisabled    ModuleStatus = "disabled"
	ModuleStatusFailed      ModuleStatus = "failed"
	ModuleStatusUninstalled ModuleStatus = "uninstalled"
	ModuleStatusPurged      ModuleStatus = "purged"
)

type RuntimeStatus string

const (
	RuntimeStatusStarting RuntimeStatus = "starting"
	RuntimeStatusRunning  RuntimeStatus = "running"
	RuntimeStatusStopped  RuntimeStatus = "stopped"
	RuntimeStatusFailed   RuntimeStatus = "failed"
	RuntimeStatusCrashed  RuntimeStatus = "crashed"
)

type Capability string

const (
	CapabilityProviderAdapter       Capability = "provider.adapter"
	CapabilityUIAdminRoute          Capability = "ui.admin.route"
	CapabilityUIAccountForm         Capability = "ui.account.form"
	CapabilityGatewayRequestFilter  Capability = "gateway.request_filter"
	CapabilityGatewayResponseFilter Capability = "gateway.response_filter"
	CapabilityModuleMigration       Capability = "module.migration"
)

type BackendKind string

const (
	BackendKindSidecar BackendKind = "sidecar"
)

type BackendProtocol string

const (
	BackendProtocolConnect BackendProtocol = "connect"
	BackendProtocolGRPC    BackendProtocol = "grpc"
)

type FrontendKind string

const (
	FrontendKindViteRemoteESM FrontendKind = "vite-remote-esm"
)

type Manifest struct {
	APIVersion   string        `json:"apiVersion" yaml:"apiVersion"`
	ID           string        `json:"id" yaml:"id"`
	Name         string        `json:"name" yaml:"name"`
	Type         ModuleType    `json:"type" yaml:"type"`
	Version      string        `json:"version" yaml:"version"`
	Description  string        `json:"description,omitempty" yaml:"description,omitempty"`
	Core         CoreSpec      `json:"core" yaml:"core"`
	Backend      *BackendSpec  `json:"backend,omitempty" yaml:"backend,omitempty"`
	Frontend     *FrontendSpec `json:"frontend,omitempty" yaml:"frontend,omitempty"`
	Capabilities []Capability  `json:"capabilities" yaml:"capabilities"`
	Permissions  PermissionSet `json:"permissions" yaml:"permissions"`
	Migrations   []string      `json:"migrations,omitempty" yaml:"migrations,omitempty"`
}

type CoreSpec struct {
	Compatible string `json:"compatible" yaml:"compatible"`
}

type BackendSpec struct {
	Kind        BackendKind      `json:"kind" yaml:"kind"`
	Command     string           `json:"command" yaml:"command"`
	Protocol    BackendProtocol  `json:"protocol" yaml:"protocol"`
	Socket      string           `json:"socket,omitempty" yaml:"socket,omitempty"`
	Healthcheck *HealthcheckSpec `json:"healthcheck,omitempty" yaml:"healthcheck,omitempty"`
}

type HealthcheckSpec struct {
	RPC     string       `json:"rpc,omitempty" yaml:"rpc,omitempty"`
	Timeout DurationSpec `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type DurationSpec struct {
	time.Duration
}

type FrontendSpec struct {
	Kind         FrontendKind      `json:"kind" yaml:"kind"`
	Entry        string            `json:"entry" yaml:"entry"`
	Routes       []UIRouteSpec     `json:"routes,omitempty" yaml:"routes,omitempty"`
	Menu         []UIMenuSpec      `json:"menu,omitempty" yaml:"menu,omitempty"`
	AccountForms []AccountFormSpec `json:"accountForms,omitempty" yaml:"accountForms,omitempty"`
}

type UIRouteSpec struct {
	Path          string `json:"path" yaml:"path"`
	Title         string `json:"title" yaml:"title"`
	ExposedModule string `json:"exposedModule" yaml:"exposedModule"`
	RequiresAdmin bool   `json:"requiresAdmin,omitempty" yaml:"requiresAdmin,omitempty"`
}

type UIMenuSpec struct {
	Title string `json:"title" yaml:"title"`
	Path  string `json:"path" yaml:"path"`
	Group string `json:"group,omitempty" yaml:"group,omitempty"`
	Order int    `json:"order,omitempty" yaml:"order,omitempty"`
}

type AccountFormSpec struct {
	ProviderID    string `json:"providerId" yaml:"providerId"`
	ExposedModule string `json:"exposedModule" yaml:"exposedModule"`
}

type PermissionSet struct {
	Network  []string `json:"network,omitempty" yaml:"network,omitempty"`
	Secrets  []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Database []string `json:"database,omitempty" yaml:"database,omitempty"`
	UI       []string `json:"ui,omitempty" yaml:"ui,omitempty"`
	Gateway  []string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

type PermissionRecord struct {
	ModuleID        string     `json:"module_id"`
	PermissionType  string     `json:"permission_type"`
	PermissionValue string     `json:"permission_value"`
	Approved        bool       `json:"approved"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type InstalledModule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        ModuleType   `json:"type"`
	Version     string       `json:"version"`
	Status      ModuleStatus `json:"status"`
	InstallPath string       `json:"install_path"`
	Manifest    Manifest     `json:"manifest"`
	InstalledAt time.Time    `json:"installed_at"`
	EnabledAt   *time.Time   `json:"enabled_at,omitempty"`
	LastError   string       `json:"last_error,omitempty"`
}

type RuntimeInstanceUpdate struct {
	ModuleID              string
	Status                RuntimeStatus
	PID                   *int
	SocketPath            string
	StartedAt             *time.Time
	StoppedAt             *time.Time
	LastHeartbeatAt       *time.Time
	LastError             string
	IncrementRestartCount bool
}
