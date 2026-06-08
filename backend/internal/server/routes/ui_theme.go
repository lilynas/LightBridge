package routes

import (
	"github.com/Wei-Shaw/LightBridge/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterUIThemeAssetRoutes(r *gin.Engine, h *handler.Handlers) {
	if h == nil || h.UIThemeAsset == nil {
		return
	}
	r.GET("/ui-themes/:id/*path", h.UIThemeAsset.Serve)
}
