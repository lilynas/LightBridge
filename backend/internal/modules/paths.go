package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const maxUnixSocketPathLen = 100

func InstallDir(baseDir, moduleID, version string) string {
	return filepath.Join(baseDir, "modules", moduleID, version)
}

func RuntimeSocketPath(baseDir, moduleID string) string {
	return runtimeSocketPath(baseDir, moduleID, ".sock")
}

func CoreBridgeSocketPath(baseDir, moduleID string) string {
	return runtimeSocketPath(baseDir, moduleID, ".core.sock")
}

func runtimeSocketPath(baseDir, moduleID, suffix string) string {
	socketName := strings.ReplaceAll(moduleID, "/", ".") + suffix
	candidate := filepath.Join(baseDir, "modules-runtime", socketName)
	if len(candidate) <= maxUnixSocketPathLen {
		return candidate
	}

	sum := sha256.Sum256([]byte(baseDir + "\x00" + moduleID + "\x00" + suffix))
	shortName := "lbm-" + hex.EncodeToString(sum[:])[:16] + suffix
	return filepath.Join(os.TempDir(), shortName)
}
