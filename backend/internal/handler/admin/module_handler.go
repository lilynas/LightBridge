package admin

import (
	"strings"

	"github.com/Wei-Shaw/LightBridge/internal/modules"
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

func (h *ModuleHandler) ListInstalled(c *gin.Context) {
	modules, err := h.service.ListInstalled(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, modules)
}

func (h *ModuleHandler) ProviderAdapters(c *gin.Context) {
	adapters, err := h.service.ProviderAdapters(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adapters)
}

func (h *ModuleHandler) ProviderAccountForms(c *gin.Context) {
	forms, err := h.service.ProviderAccountForms(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, forms)
}

func (h *ModuleHandler) Marketplace(c *gin.Context) {
	marketplace, err := h.service.Marketplace(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, marketplace)
}

type installModuleRequest struct {
	ArchivePath string `json:"archive_path"`
	ModuleID    string `json:"module_id"`
	Version     string `json:"version"`
}

func (h *ModuleHandler) Install(c *gin.Context) {
	var req installModuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	var (
		module *modules.InstalledModule
		err    error
	)
	if strings.TrimSpace(req.ArchivePath) != "" {
		module, err = h.service.InstallArchive(c.Request.Context(), strings.TrimSpace(req.ArchivePath))
	} else if strings.TrimSpace(req.ModuleID) != "" || strings.TrimSpace(req.Version) != "" {
		module, err = h.service.InstallFromMarketplace(c.Request.Context(), strings.TrimSpace(req.ModuleID), strings.TrimSpace(req.Version))
	} else {
		response.BadRequest(c, "archive_path or module_id and version are required")
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

func (h *ModuleHandler) InstallArchive(c *gin.Context) {
	h.Install(c)
}

type changeModuleVersionRequest struct {
	Version string `json:"version"`
}

func (h *ModuleHandler) Upgrade(c *gin.Context) {
	var req changeModuleVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	module, err := h.service.UpgradeFromMarketplace(c.Request.Context(), c.Param("id"), strings.TrimSpace(req.Version))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

func (h *ModuleHandler) Rollback(c *gin.Context) {
	var req changeModuleVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	module, err := h.service.RollbackFromMarketplace(c.Request.Context(), c.Param("id"), strings.TrimSpace(req.Version))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

func (h *ModuleHandler) Permissions(c *gin.Context) {
	status, err := h.service.Permissions(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *ModuleHandler) ApprovePermissions(c *gin.Context) {
	status, err := h.service.ApprovePermissions(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *ModuleHandler) Enable(c *gin.Context) {
	module, err := h.service.Enable(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

func (h *ModuleHandler) Disable(c *gin.Context) {
	module, err := h.service.Disable(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

func (h *ModuleHandler) Uninstall(c *gin.Context) {
	module, err := h.service.Uninstall(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}

type purgeModuleRequest struct {
	Confirm bool `json:"confirm"`
}

func (h *ModuleHandler) Purge(c *gin.Context) {
	var req purgeModuleRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		response.BadRequest(c, "purge confirmation is required")
		return
	}
	module, err := h.service.Purge(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, module)
}
