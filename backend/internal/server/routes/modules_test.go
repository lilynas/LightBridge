package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModuleAssetRoutesServeInstalledFrontendAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	assetPath := filepath.Join(dataDir, "modules", "lightbridge.provider.mock", "0.1.0", "frontend", "remoteEntry.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(assetPath), 0o755))
	require.NoError(t, os.WriteFile(assetPath, []byte("export default {}"), 0o644))

	router := gin.New()
	RegisterModuleAssetRoutes(router, dataDir)

	req := httptest.NewRequest(http.MethodGet, "/modules/lightbridge.provider.mock/0.1.0/frontend/remoteEntry.js", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "export default {}", w.Body.String())
}

func TestModuleAssetRoutesRejectTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "secret.txt"), []byte("secret"), 0o644))

	router := gin.New()
	RegisterModuleAssetRoutes(router, dataDir)

	req := httptest.NewRequest(http.MethodGet, "/modules/%2e%2e/secret.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModuleAssetRoutesDoNotServeDirectories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "modules", "lightbridge.provider.mock", "0.1.0"), 0o755))

	router := gin.New()
	RegisterModuleAssetRoutes(router, dataDir)

	req := httptest.NewRequest(http.MethodGet, "/modules/lightbridge.provider.mock/0.1.0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}
