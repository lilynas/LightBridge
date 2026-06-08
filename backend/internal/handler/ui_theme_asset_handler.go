package handler

import (
	"github.com/Wei-Shaw/LightBridge/internal/pkg/response"
	"github.com/Wei-Shaw/LightBridge/internal/service"
	"github.com/gin-gonic/gin"
)

type UIThemeAssetHandler struct {
	service *service.UIThemeService
}

func NewUIThemeAssetHandler(s *service.UIThemeService) *UIThemeAssetHandler {
	return &UIThemeAssetHandler{service: s}
}

func (h *UIThemeAssetHandler) Serve(c *gin.Context) {
	fullPath, contentType, err := h.service.ResolveAssetPath(c.Request.Context(), c.Param("id"), c.Param("path"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.File(fullPath)
	c.Abort()
}
