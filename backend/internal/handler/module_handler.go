package handler

import (
	"github.com/Wei-Shaw/LightBridge/internal/pkg/response"
	"github.com/Wei-Shaw/LightBridge/internal/service"
	"github.com/gin-gonic/gin"
)

type ModuleHandler struct {
	service *service.ModuleService
}

func NewModuleHandler(service *service.ModuleService) *ModuleHandler {
	return &ModuleHandler{service: service}
}

func (h *ModuleHandler) UIManifest(c *gin.Context) {
	manifest, err := h.service.UIManifest(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, manifest)
}
