package modules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	moduleIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,126}[a-z0-9])?$`)
	semverPattern   = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:[-+][0-9A-Za-z.-]+)?$`)
)

var allowedCapabilities = map[Capability]struct{}{
	CapabilityProviderAdapter:       {},
	CapabilityUIAdminRoute:          {},
	CapabilityUIAccountForm:         {},
	CapabilityGatewayRequestFilter:  {},
	CapabilityGatewayResponseFilter: {},
	CapabilityModuleMigration:       {},
}

var allowedModuleTypes = map[ModuleType]struct{}{
	ModuleTypeProvider: {},
	ModuleTypeUI:       {},
	ModuleTypeGateway:  {},
}

func ValidateManifest(m Manifest) error {
	if strings.TrimSpace(m.APIVersion) != ManifestAPIVersionV1Alpha1 {
		return fmt.Errorf("unsupported apiVersion %q", m.APIVersion)
	}
	if !moduleIDPattern.MatchString(m.ID) || strings.Contains(m.ID, "..") {
		return fmt.Errorf("invalid module id %q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("module name is required")
	}
	if _, ok := allowedModuleTypes[m.Type]; !ok {
		return fmt.Errorf("unsupported module type %q", m.Type)
	}
	if !semverPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid module version %q", m.Version)
	}
	if err := ValidateCoreCompatibility(m.Core.Compatible, ""); err != nil {
		return err
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("at least one capability is required")
	}
	seen := make(map[Capability]struct{}, len(m.Capabilities))
	for _, capability := range m.Capabilities {
		if _, ok := allowedCapabilities[capability]; !ok {
			return fmt.Errorf("unsupported capability %q", capability)
		}
		if _, ok := seen[capability]; ok {
			return fmt.Errorf("duplicate capability %q", capability)
		}
		seen[capability] = struct{}{}
	}
	if m.Type == ModuleTypeProvider && !slices.Contains(m.Capabilities, CapabilityProviderAdapter) {
		return fmt.Errorf("provider module %q requires %s capability", m.ID, CapabilityProviderAdapter)
	}
	if slices.Contains(m.Capabilities, CapabilityProviderAdapter) {
		if err := validateProviderBackend(m.Backend); err != nil {
			return err
		}
	}
	if m.Frontend != nil {
		if err := validateFrontend(m, *m.Frontend); err != nil {
			return err
		}
	}
	for _, migration := range m.Migrations {
		if err := validateRelativePath("migration", migration); err != nil {
			return err
		}
		if !strings.HasPrefix(filepath.ToSlash(migration), "migrations/") {
			return fmt.Errorf("migration %q must be under migrations/", migration)
		}
	}
	return nil
}

func validateProviderBackend(backend *BackendSpec) error {
	if backend == nil {
		return fmt.Errorf("provider.adapter requires backend spec")
	}
	if backend.Kind != BackendKindSidecar {
		return fmt.Errorf("unsupported backend kind %q", backend.Kind)
	}
	if backend.Protocol != BackendProtocolConnect && backend.Protocol != BackendProtocolGRPC {
		return fmt.Errorf("unsupported backend protocol %q", backend.Protocol)
	}
	if strings.TrimSpace(backend.Command) == "" {
		return fmt.Errorf("backend.command is required")
	}
	if err := validateRelativePath("backend.command", backend.Command); err != nil {
		return err
	}
	return nil
}

func validateFrontend(manifest Manifest, frontend FrontendSpec) error {
	if frontend.Kind != FrontendKindViteRemoteESM {
		return fmt.Errorf("unsupported frontend kind %q", frontend.Kind)
	}
	frontendEntry := strings.TrimSpace(frontend.Entry)
	if frontendEntry == "" {
		return fmt.Errorf("frontend.entry is required")
	}
	if frontendEntry != frontend.Entry {
		return fmt.Errorf("frontend.entry %q must not contain surrounding whitespace", frontend.Entry)
	}
	if err := validateRelativePath("frontend.entry", frontend.Entry); err != nil {
		return err
	}
	if !strings.HasSuffix(filepath.ToSlash(frontendEntry), ".js") {
		return fmt.Errorf("frontend.entry %q must be a JavaScript remote entry", frontend.Entry)
	}
	if len(frontend.Routes) > 0 || len(frontend.Menu) > 0 {
		if !slices.Contains(manifest.Capabilities, CapabilityUIAdminRoute) {
			return fmt.Errorf("frontend routes and menu require %s capability", CapabilityUIAdminRoute)
		}
	}
	seenRoutePaths := make(map[string]struct{}, len(frontend.Routes))
	for _, route := range frontend.Routes {
		routePath := strings.TrimSpace(route.Path)
		routeTitle := strings.TrimSpace(route.Title)
		exposedModule := strings.TrimSpace(route.ExposedModule)
		if routeTitle == "" {
			return fmt.Errorf("frontend route %q title is required", route.Path)
		}
		if routeTitle != route.Title {
			return fmt.Errorf("frontend route %q title must not contain surrounding whitespace", route.Path)
		}
		if routePath != route.Path {
			return fmt.Errorf("frontend route path %q must not contain surrounding whitespace", route.Path)
		}
		if !strings.HasPrefix(routePath, "/admin/") {
			return fmt.Errorf("frontend route path %q must start with /admin/", route.Path)
		}
		if _, ok := seenRoutePaths[routePath]; ok {
			return fmt.Errorf("duplicate frontend route path %q", routePath)
		}
		seenRoutePaths[routePath] = struct{}{}
		if exposedModule != route.ExposedModule {
			return fmt.Errorf("frontend route %q exposedModule must not contain surrounding whitespace", route.Path)
		}
		if !strings.HasPrefix(exposedModule, "./") {
			return fmt.Errorf("frontend route %q exposedModule must start with ./", route.Path)
		}
	}
	for _, menu := range frontend.Menu {
		menuPath := strings.TrimSpace(menu.Path)
		menuTitle := strings.TrimSpace(menu.Title)
		if menuTitle == "" {
			return fmt.Errorf("frontend menu %q title is required", menu.Path)
		}
		if menuTitle != menu.Title {
			return fmt.Errorf("frontend menu %q title must not contain surrounding whitespace", menu.Path)
		}
		if menuPath != menu.Path {
			return fmt.Errorf("frontend menu path %q must not contain surrounding whitespace", menu.Path)
		}
		if !strings.HasPrefix(menuPath, "/admin/") {
			return fmt.Errorf("frontend menu path %q must start with /admin/", menu.Path)
		}
	}
	if len(frontend.AccountForms) > 0 {
		if !slices.Contains(manifest.Capabilities, CapabilityUIAccountForm) {
			return fmt.Errorf("frontend accountForms require %s capability", CapabilityUIAccountForm)
		}
		if !slices.Contains(manifest.Capabilities, CapabilityProviderAdapter) {
			return fmt.Errorf("frontend accountForms require %s capability", CapabilityProviderAdapter)
		}
	}
	for _, form := range frontend.AccountForms {
		providerID := strings.TrimSpace(form.ProviderID)
		exposedModule := strings.TrimSpace(form.ExposedModule)
		if providerID == "" {
			return fmt.Errorf("account form providerId is required")
		}
		if providerID != form.ProviderID {
			return fmt.Errorf("account form providerId %q must not contain surrounding whitespace", form.ProviderID)
		}
		if providerID != manifest.ID {
			return fmt.Errorf("account form providerId %q must match module id %q", providerID, manifest.ID)
		}
		if exposedModule != form.ExposedModule {
			return fmt.Errorf("account form exposedModule must not contain surrounding whitespace")
		}
		if !strings.HasPrefix(exposedModule, "./") {
			return fmt.Errorf("account form exposedModule must start with ./")
		}
	}
	return nil
}

func validateRelativePath(label, path string) error {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		return fmt.Errorf("%s %q must be relative", label, path)
	}
	slash := filepath.ToSlash(clean)
	if slash == "." || strings.HasPrefix(slash, "../") || slash == ".." || strings.Contains(slash, "/../") {
		return fmt.Errorf("%s %q escapes module root", label, path)
	}
	return nil
}

func IsAllowedCapability(capability Capability) bool {
	_, ok := allowedCapabilities[capability]
	return ok
}
