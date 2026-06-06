package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/LightBridge/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterModuleAssetRoutes(r *gin.Engine, dataDir string) {
	root := filepath.Join(dataDir, "modules")
	r.GET("/modules/*filepath", func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		clean := filepath.Clean(rel)
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			c.String(http.StatusBadRequest, "invalid module asset path")
			return
		}

		filePath := filepath.Join(root, clean)
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			c.String(http.StatusNotFound, "module asset not found")
			return
		}
		c.File(filePath)
	})
}

func RegisterModuleRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	modules := v1.Group("/modules")
	{
		modules.GET("/ui-manifest", h.Module.UIManifest)
	}
}
