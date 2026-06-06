package modules

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type Installer interface {
	InstallArchive(ctx context.Context, archivePath string) (*InstalledModule, error)
}

type InstalledVerifier interface {
	VerifyInstalled(ctx context.Context, module InstalledModule) (*InstalledModule, error)
}

type PackageInstaller struct {
	baseDir     string
	store       Store
	verifier    SignatureVerifier
	coreVersion string
	now         func() time.Time
}

func NewPackageInstaller(baseDir string, store Store) *PackageInstaller {
	return NewPackageInstallerWithVerifier(baseDir, store, nil)
}

func NewPackageInstallerWithVerifier(baseDir string, store Store, verifier SignatureVerifier) *PackageInstaller {
	return NewPackageInstallerWithVerifierAndCoreVersion(baseDir, store, verifier, "")
}

func NewPackageInstallerWithVerifierAndCoreVersion(baseDir string, store Store, verifier SignatureVerifier, coreVersion string) *PackageInstaller {
	if baseDir == "" {
		baseDir = "data"
	}
	return &PackageInstaller{
		baseDir:     baseDir,
		store:       store,
		verifier:    verifier,
		coreVersion: strings.TrimSpace(coreVersion),
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (i *PackageInstaller) InstallArchive(ctx context.Context, archivePath string) (*InstalledModule, error) {
	if i == nil || i.store == nil {
		return nil, fmt.Errorf("module installer is not configured")
	}
	if strings.TrimSpace(archivePath) == "" {
		return nil, fmt.Errorf("module archive path is required")
	}

	workDir, err := os.MkdirTemp("", "lightbridge-module-install-*")
	if err != nil {
		return nil, fmt.Errorf("create module install workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	if err := ExtractArchive(archivePath, workDir); err != nil {
		return nil, err
	}

	checksumsRaw, err := os.ReadFile(filepath.Join(workDir, ChecksumsFilename))
	if err != nil {
		return nil, fmt.Errorf("load module checksums: %w", err)
	}
	signatureRaw, err := os.ReadFile(filepath.Join(workDir, SignatureFilename))
	if err != nil {
		return nil, fmt.Errorf("load module signature: %w", err)
	}
	if i.verifier == nil {
		return nil, fmt.Errorf("module signature verifier is not configured")
	}
	if err := i.verifier.Verify(checksumsRaw, signatureRaw); err != nil {
		return nil, fmt.Errorf("verify module signature: %w", err)
	}

	manifest, err := LoadManifestDir(workDir)
	if err != nil {
		return nil, fmt.Errorf("load module manifest: %w", err)
	}
	if err := validateArchiveFilename(archivePath, manifest.ID, manifest.Version); err != nil {
		return nil, err
	}
	if err := ValidateCoreCompatibility(manifest.Core.Compatible, i.coreVersion); err != nil {
		return nil, err
	}
	checksums, err := ParseChecksums(checksumsRaw)
	if err != nil {
		return nil, fmt.Errorf("load module checksums: %w", err)
	}
	if err := validateManifestChecksumCoverage(*manifest, checksums); err != nil {
		return nil, err
	}
	if err := VerifyChecksums(workDir, checksums); err != nil {
		return nil, err
	}
	if err := validateManifestPackageFiles(workDir, *manifest); err != nil {
		return nil, err
	}

	targetDir := InstallDir(i.baseDir, manifest.ID, manifest.Version)
	stagingDir := targetDir + ".installing"
	if err := os.RemoveAll(stagingDir); err != nil {
		return nil, fmt.Errorf("clean module staging dir: %w", err)
	}
	if err := copyDir(workDir, stagingDir); err != nil {
		return nil, fmt.Errorf("copy module package: %w", err)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, fmt.Errorf("replace existing module dir: %w", err)
	}
	if err := os.Rename(stagingDir, targetDir); err != nil {
		return nil, fmt.Errorf("activate module dir: %w", err)
	}

	installed := InstalledModule{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Status:      ModuleStatusInstalled,
		InstallPath: targetDir,
		Manifest:    *manifest,
		InstalledAt: i.now(),
	}
	if err := i.store.SaveInstalled(ctx, installed); err != nil {
		return nil, fmt.Errorf("register installed module: %w", err)
	}
	if err := i.store.SavePermissions(ctx, installed.ID, PermissionRecordsFromSet(installed.ID, manifest.Permissions)); err != nil {
		return nil, fmt.Errorf("register module permissions: %w", err)
	}
	if err := i.applyMigrations(ctx, installed, checksums); err != nil {
		_ = i.store.SetStatus(ctx, installed.ID, ModuleStatusFailed, err.Error())
		return nil, err
	}
	return &installed, nil
}

func (i *PackageInstaller) VerifyInstalled(_ context.Context, module InstalledModule) (*InstalledModule, error) {
	if i == nil {
		return nil, fmt.Errorf("module installer is not configured")
	}
	installPath := strings.TrimSpace(module.InstallPath)
	if installPath == "" {
		return nil, fmt.Errorf("module install path is required")
	}
	if err := validatePackageTree(installPath); err != nil {
		return nil, err
	}

	checksumsRaw, err := os.ReadFile(filepath.Join(installPath, ChecksumsFilename))
	if err != nil {
		return nil, fmt.Errorf("load module checksums: %w", err)
	}
	signatureRaw, err := os.ReadFile(filepath.Join(installPath, SignatureFilename))
	if err != nil {
		return nil, fmt.Errorf("load module signature: %w", err)
	}
	if i.verifier == nil {
		return nil, fmt.Errorf("module signature verifier is not configured")
	}
	if err := i.verifier.Verify(checksumsRaw, signatureRaw); err != nil {
		return nil, fmt.Errorf("verify module signature: %w", err)
	}

	manifest, err := LoadManifestDir(installPath)
	if err != nil {
		return nil, fmt.Errorf("load module manifest: %w", err)
	}
	if manifest.ID != module.ID {
		return nil, fmt.Errorf("installed manifest id %q does not match database module id %q", manifest.ID, module.ID)
	}
	if manifest.Version != module.Version {
		return nil, fmt.Errorf("installed manifest version %q does not match database module version %q", manifest.Version, module.Version)
	}
	if err := ValidateCoreCompatibility(manifest.Core.Compatible, i.coreVersion); err != nil {
		return nil, err
	}
	checksums, err := ParseChecksums(checksumsRaw)
	if err != nil {
		return nil, fmt.Errorf("load module checksums: %w", err)
	}
	if err := validateManifestChecksumCoverage(*manifest, checksums); err != nil {
		return nil, err
	}
	if err := VerifyChecksums(installPath, checksums); err != nil {
		return nil, err
	}
	if err := validateManifestPackageFiles(installPath, *manifest); err != nil {
		return nil, err
	}

	refreshed := module
	refreshed.Name = manifest.Name
	refreshed.Type = manifest.Type
	refreshed.Version = manifest.Version
	refreshed.Manifest = *manifest
	return &refreshed, nil
}

func (i *PackageInstaller) applyMigrations(ctx context.Context, installed InstalledModule, checksums []ChecksumEntry) error {
	if len(installed.Manifest.Migrations) == 0 {
		return nil
	}
	checksumByPath := make(map[string]string, len(checksums))
	for _, entry := range checksums {
		checksumByPath[filepath.ToSlash(filepath.Clean(entry.Path))] = entry.HexDigest
	}
	for _, migration := range installed.Manifest.Migrations {
		cleanMigration := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(migration, "./")))
		content, err := os.ReadFile(filepath.Join(installed.InstallPath, filepath.FromSlash(cleanMigration)))
		if err != nil {
			return fmt.Errorf("read module migration %s: %w", migration, err)
		}
		checksum := checksumByPath[cleanMigration]
		if checksum == "" {
			return fmt.Errorf("migration %q is not covered by %s", migration, ChecksumsFilename)
		}
		if err := ValidateModuleMigrationSQL(installed.Manifest, string(content)); err != nil {
			return fmt.Errorf("validate module migration %s: %w", migration, err)
		}
		if err := i.store.ApplyMigration(ctx, installed.ID, cleanMigration, checksum, string(content)); err != nil {
			return fmt.Errorf("apply module migration %s: %w", migration, err)
		}
	}
	return nil
}

type semanticVersion struct {
	major int
	minor int
	patch int
}

var (
	semverCorePattern             = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)`)
	migrationTablePattern         = regexp.MustCompile(`(?is)\b(?:create|alter|drop)\s+table\s+(?:if\s+(?:not\s+)?exists\s+)?(?:only\s+)?(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)
	migrationIndexPattern         = regexp.MustCompile(`(?is)\bcreate\s+(?:unique\s+)?index\s+(?:concurrently\s+)?(?:if\s+not\s+exists\s+)?"?[a-zA-Z_][a-zA-Z0-9_]*"?\s+on\s+(?:only\s+)?(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)
	migrationCommentTablePattern  = regexp.MustCompile(`(?is)\bcomment\s+on\s+table\s+(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)
	migrationCommentColumnPattern = regexp.MustCompile(`(?is)\bcomment\s+on\s+column\s+(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?\."?[a-zA-Z_][a-zA-Z0-9_]*"?`)
	migrationDMLPattern           = regexp.MustCompile(`(?is)\b(?:insert\s+into|update|delete\s+from|truncate\s+(?:table\s+)?)\s+(?:only\s+)?(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)
	migrationFKPattern            = regexp.MustCompile(`(?is)\breferences\s+(?:"?([a-zA-Z_][a-zA-Z0-9_]*)"?\.)?"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)
)

func ValidateCoreCompatibility(coreRange string, coreVersion string) error {
	coreRange = strings.TrimSpace(coreRange)
	if coreRange == "" {
		return fmt.Errorf("core.compatible is required")
	}
	constraints := strings.Fields(coreRange)
	for _, token := range constraints {
		if token == "" {
			continue
		}
		_, versionText := splitVersionConstraint(token)
		if _, err := parseSemanticVersion(versionText); err != nil {
			return fmt.Errorf("invalid core.compatible constraint %q: %w", token, err)
		}
	}
	coreVersion = strings.TrimSpace(coreVersion)
	if coreVersion == "" {
		return nil
	}
	current, err := parseSemanticVersion(coreVersion)
	if err != nil {
		return fmt.Errorf("invalid core version %q: %w", coreVersion, err)
	}
	for _, token := range constraints {
		if token == "" {
			continue
		}
		op, versionText := splitVersionConstraint(token)
		want, _ := parseSemanticVersion(versionText)
		if !compareSemanticVersion(current, want, op) {
			return fmt.Errorf("module requires core %q, current core is %q", coreRange, coreVersion)
		}
	}
	return nil
}

func splitVersionConstraint(token string) (string, string) {
	for _, op := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(token, op) {
			return op, strings.TrimSpace(strings.TrimPrefix(token, op))
		}
	}
	return "=", token
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	match := semverCorePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 4 {
		return semanticVersion{}, fmt.Errorf("expected semver major.minor.patch")
	}
	var out semanticVersion
	if _, err := fmt.Sscanf(match[1]+"."+match[2]+"."+match[3], "%d.%d.%d", &out.major, &out.minor, &out.patch); err != nil {
		return semanticVersion{}, err
	}
	return out, nil
}

func compareSemanticVersion(current semanticVersion, want semanticVersion, op string) bool {
	cmp := current.compare(want)
	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	default:
		return cmp == 0
	}
}

func (v semanticVersion) compare(other semanticVersion) int {
	switch {
	case v.major != other.major:
		return v.major - other.major
	case v.minor != other.minor:
		return v.minor - other.minor
	default:
		return v.patch - other.patch
	}
}

func ValidateModuleMigrationSQL(manifest Manifest, sqlText string) error {
	if strings.TrimSpace(sqlText) == "" {
		return fmt.Errorf("module migration is empty")
	}
	allowed := DatabasePermissionPrefixes(manifest.Permissions.Database)
	if len(allowed) == 0 {
		return fmt.Errorf("module migration requires database permission prefix")
	}
	for _, table := range migrationReferencedTables(sqlText) {
		if table == "" {
			continue
		}
		if !DatabaseTableAllowedByPrefixes(table, allowed) {
			return fmt.Errorf("migration references table %q outside declared module database prefixes", table)
		}
	}
	return nil
}

func DatabasePermissionPrefixes(values []string) []string {
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		value = strings.TrimSuffix(value, "*")
		value = strings.TrimSuffix(value, ".")
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func migrationReferencedTables(sqlText string) []string {
	seen := make(map[string]struct{})
	appendMatches := func(pattern *regexp.Regexp) {
		for _, match := range pattern.FindAllStringSubmatch(sqlText, -1) {
			if len(match) < 3 {
				continue
			}
			table := strings.ToLower(strings.TrimSpace(match[2]))
			if table == "" {
				continue
			}
			seen[table] = struct{}{}
		}
	}
	appendMatches(migrationTablePattern)
	appendMatches(migrationIndexPattern)
	appendMatches(migrationCommentTablePattern)
	appendMatches(migrationCommentColumnPattern)
	appendMatches(migrationDMLPattern)
	appendMatches(migrationFKPattern)
	result := make([]string, 0, len(seen))
	for table := range seen {
		result = append(result, table)
	}
	return result
}

func DatabaseTableAllowedByPrefixes(table string, prefixes []string) bool {
	table = strings.ToLower(strings.TrimSpace(table))
	for _, prefix := range prefixes {
		if strings.HasSuffix(prefix, "_") {
			if strings.HasPrefix(table, prefix) {
				return true
			}
			continue
		}
		if table == prefix || strings.HasPrefix(table, prefix+"_") {
			return true
		}
	}
	return false
}

func PermissionRecordsFromSet(moduleID string, permissions PermissionSet) []PermissionRecord {
	var records []PermissionRecord
	appendRecords := func(permissionType string, values []string) {
		for _, value := range values {
			cleanValue := strings.TrimSpace(value)
			if cleanValue == "" {
				continue
			}
			records = append(records, PermissionRecord{
				ModuleID:        moduleID,
				PermissionType:  permissionType,
				PermissionValue: cleanValue,
			})
		}
	}
	appendRecords("network", permissions.Network)
	appendRecords("secrets", permissions.Secrets)
	appendRecords("database", permissions.Database)
	appendRecords("ui", permissions.UI)
	appendRecords("gateway", permissions.Gateway)
	return records
}

func ExtractArchive(archivePath, targetDir string) error {
	if !strings.HasSuffix(archivePath, ".tar.zst") {
		return fmt.Errorf("module archive must be a .tar.zst package")
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open module archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("open zstd module archive: %w", err)
	}
	defer zstdReader.Close()

	cleanTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(zstdReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read module archive: %w", err)
		}
		if err := extractTarEntry(tarReader, header, cleanTarget); err != nil {
			return err
		}
	}
}

func validateArchiveFilename(archivePath, moduleID, version string) error {
	expected := fmt.Sprintf("lightbridge-module-%s-%s.tar.zst", moduleID, version)
	if filepath.Base(archivePath) != expected {
		return fmt.Errorf("module archive filename must be %q", expected)
	}
	return nil
}

func validateManifestPackageFiles(root string, manifest Manifest) error {
	if err := validatePackageTree(root); err != nil {
		return err
	}
	allowedExecutableFiles := make(map[string]struct{}, 1)
	if manifest.Backend != nil {
		if err := requirePackageFile(root, "backend.command", manifest.Backend.Command); err != nil {
			return err
		}
		if err := requireExecutablePackageFile(root, "backend.command", manifest.Backend.Command); err != nil {
			return err
		}
		allowedExecutableFiles[canonicalPackagePath(manifest.Backend.Command)] = struct{}{}
	}
	if manifest.Frontend != nil {
		if err := requirePackageFile(root, "frontend.entry", manifest.Frontend.Entry); err != nil {
			return err
		}
	}
	for _, migration := range manifest.Migrations {
		if err := requirePackageFile(root, "migration", migration); err != nil {
			return err
		}
	}
	if err := validateUnexpectedExecutableFiles(root, allowedExecutableFiles); err != nil {
		return err
	}
	return nil
}

func validatePackageTree(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect module package root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("module package root %q must not be a symlink", root)
	}
	if !info.IsDir() {
		return fmt.Errorf("module package root %q must be a directory", root)
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink == 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return fmt.Errorf("module package entry %q must not be a symlink", canonicalPackagePath(rel))
	})
}

func validateManifestChecksumCoverage(manifest Manifest, entries []ChecksumEntry) error {
	covered := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		covered[filepath.ToSlash(filepath.Clean(entry.Path))] = struct{}{}
	}
	required := []struct {
		label string
		path  string
	}{
		{label: "module manifest", path: ManifestFilename},
	}
	if manifest.Backend != nil {
		required = append(required, struct {
			label string
			path  string
		}{label: "backend.command", path: manifest.Backend.Command})
	}
	if manifest.Frontend != nil {
		required = append(required, struct {
			label string
			path  string
		}{label: "frontend.entry", path: manifest.Frontend.Entry})
	}
	for _, migration := range manifest.Migrations {
		required = append(required, struct {
			label string
			path  string
		}{label: "migration", path: migration})
	}
	for _, item := range required {
		cleanPath := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(item.path, "./")))
		if _, ok := covered[cleanPath]; !ok {
			return fmt.Errorf("%s %q is not covered by %s", item.label, item.path, ChecksumsFilename)
		}
	}
	return nil
}

func requirePackageFile(root, label, rel string) error {
	if err := validateRelativePath(label, rel); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.Clean(rel))
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q is missing from module package", label, rel)
	}
	if info.IsDir() {
		return fmt.Errorf("%s %q must be a file", label, rel)
	}
	return nil
}

func requireExecutablePackageFile(root, label, rel string) error {
	path := filepath.Join(root, filepath.Clean(rel))
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q is missing from module package", label, rel)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s %q must be executable", label, rel)
	}
	return nil
}

func validateUnexpectedExecutableFiles(root string, allowed map[string]struct{}) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o111 == 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		cleanRel := canonicalPackagePath(rel)
		if _, ok := allowed[cleanRel]; ok {
			return nil
		}
		return fmt.Errorf("unexpected executable file %q in module package", cleanRel)
	})
}

func canonicalPackagePath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimPrefix(path, "./")))
}

func extractTarEntry(reader io.Reader, header *tar.Header, targetDir string) error {
	if header == nil {
		return fmt.Errorf("module archive contains an empty tar header")
	}
	cleanName := filepath.Clean(header.Name)
	if cleanName == "." || filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return fmt.Errorf("module archive entry %q escapes package root", header.Name)
	}
	targetPath := filepath.Join(targetDir, cleanName)
	if !strings.HasPrefix(targetPath, targetDir+string(filepath.Separator)) && targetPath != targetDir {
		return fmt.Errorf("module archive entry %q escapes package root", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetPath, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(header.Mode)
		if mode == 0 {
			mode = 0o644
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		defer func() { _ = dst.Close() }()
		_, err = io.Copy(dst, reader)
		return err
	default:
		return fmt.Errorf("module archive entry %q uses unsupported tar type %d", header.Name, header.Typeflag)
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}
