package main

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	moduleID      = "lightbridge.provider.mock"
	moduleName    = "Mock Provider"
	moduleVersion = "0.1.0"
)

func main() {
	root, err := moduleRoot()
	must(err)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	distDir := filepath.Join(root, "dist")
	stageDir, err := os.MkdirTemp("", "lightbridge-provider-mock-package-*")
	must(err)
	defer func() { _ = os.RemoveAll(stageDir) }()
	archivePath := filepath.Join(distDir, fmt.Sprintf("lightbridge-module-%s-%s.tar.zst", moduleID, moduleVersion))

	must(os.MkdirAll(distDir, 0o755))
	must(os.MkdirAll(filepath.Join(stageDir, "backend", platform), 0o755))
	must(os.MkdirAll(filepath.Join(stageDir, "frontend"), 0o755))

	binaryPath := filepath.Join(stageDir, "backend", platform, "lightbridge-provider-mock")
	must(run(filepath.Join(root, "backend"), "go", "build", "-o", binaryPath, "."))
	must(os.Chmod(binaryPath, 0o755))
	must(copyFile(filepath.Join(root, "frontend", "remoteEntry.js"), filepath.Join(stageDir, "frontend", "remoteEntry.js"), 0o644))
	must(writeManifest(root, stageDir, platform))

	checksums, err := buildChecksums(stageDir)
	must(err)
	must(os.WriteFile(filepath.Join(stageDir, "checksums.txt"), checksums, 0o644))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	must(err)
	signature := ed25519.Sign(privateKey, checksums)
	must(os.WriteFile(filepath.Join(stageDir, "signature.sig"), []byte(hex.EncodeToString(signature)+"\n"), 0o644))
	must(os.WriteFile(filepath.Join(distDir, "ed25519.pub"), []byte(hex.EncodeToString(publicKey)+"\n"), 0o644))

	must(writeArchive(stageDir, archivePath))
	archiveSHA, err := sha256File(archivePath)
	must(err)
	must(writeRegistry(distDir, archivePath, archiveSHA))

	fmt.Printf("package: %s\n", archivePath)
	fmt.Printf("registry: %s\n", filepath.Join(distDir, "registry.json"))
	fmt.Printf("public key: %s\n", filepath.Join(distDir, "ed25519.pub"))
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(wd) == "tools" {
		return filepath.Dir(wd), nil
	}
	return wd, nil
}

func writeManifest(root, stageDir, platform string) error {
	content, err := os.ReadFile(filepath.Join(root, "module.template.yaml"))
	if err != nil {
		return err
	}
	rendered := strings.ReplaceAll(string(content), "{{PLATFORM}}", platform)
	return os.WriteFile(filepath.Join(stageDir, "module.yaml"), []byte(rendered), 0o644)
}

func buildChecksums(stageDir string) ([]byte, error) {
	paths := []string{
		"module.yaml",
		"frontend/remoteEntry.js",
	}
	err := filepath.WalkDir(filepath.Join(stageDir, "backend"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stageDir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out bytes.Buffer
	for _, rel := range paths {
		sum, err := sha256File(filepath.Join(stageDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, "sha256 %s %s\n", sum, rel)
	}
	return out.Bytes(), nil
}

func writeRegistry(distDir, archivePath, archiveSHA string) error {
	absArchive, err := filepath.Abs(archivePath)
	if err != nil {
		return err
	}
	registry := map[string]any{
		"modules": []map[string]any{
			{
				"id":          moduleID,
				"version":     moduleVersion,
				"type":        "provider",
				"name":        moduleName,
				"description": "Deterministic provider module used to verify the marketplace, runtime, and UI extension loop.",
				"downloadUrl": "file://" + absArchive,
				"sha256":      archiveSHA,
				"core":        ">=0.1.0 <0.2.0",
				"capabilities": []string{
					"provider.adapter",
					"ui.admin.route",
					"ui.account.form",
				},
				"permissions": map[string][]string{
					"network":  {},
					"secrets":  {"mock_api_key"},
					"database": {"provider_mock_*"},
				},
			},
		},
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(distDir, "registry.json"), append(data, '\n'), 0o644)
}

func writeArchive(stageDir, archivePath string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder, err := zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(3)))
	if err != nil {
		return err
	}
	defer encoder.Close()
	tarWriter := tar.NewWriter(encoder)
	defer tarWriter.Close()

	var paths []string
	if err := filepath.WalkDir(stageDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(stageDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		paths = append(paths, rel)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		path := filepath.Join(stageDir, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tarWriter, file); err != nil {
			_ = file.Close()
			return err
		}
		_ = file.Close()
	}
	return nil
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
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func run(dir string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
