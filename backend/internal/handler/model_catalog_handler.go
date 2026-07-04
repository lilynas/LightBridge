package handler

import (
	"strconv"
	"strings"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/response"
	"github.com/WilliamWang1721/LightBridge/internal/server/middleware"
	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelCatalogHandler struct {
	modelCatalogService *service.ModelCatalogService
	apiKeyService       *service.APIKeyService
}

func NewModelCatalogHandler(modelCatalogService *service.ModelCatalogService, apiKeyService *service.APIKeyService) *ModelCatalogHandler {
	return &ModelCatalogHandler{modelCatalogService: modelCatalogService, apiKeyService: apiKeyService}
}

// List returns a user-visible model catalog with source/account details removed.
// GET /api/v1/model-catalog?group_id=123&view=merged
func (h *ModelCatalogHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.modelCatalogService == nil || h.apiKeyService == nil {
		response.Success(c, service.ModelCatalogView{Models: []service.ModelCatalogModel{}})
		return
	}

	availableGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowed := make(map[int64]struct{}, len(availableGroups))
	for _, g := range availableGroups {
		allowed[g.ID] = struct{}{}
	}

	groupID, ok := parseUserModelCatalogGroupID(c)
	if !ok {
		return
	}
	if groupID != nil {
		if _, allowedGroup := allowed[*groupID]; !allowedGroup {
			response.Success(c, service.ModelCatalogView{Models: []service.ModelCatalogModel{}})
			return
		}
	}

	view, err := h.modelCatalogService.ListCatalog(c.Request.Context(), groupID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	filterUserModelCatalog(view, allowed)
	response.Success(c, view)
}

func parseUserModelCatalogGroupID(c *gin.Context) (*int64, bool) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return nil, false
	}
	return &id, true
}

func filterUserModelCatalog(view *service.ModelCatalogView, allowed map[int64]struct{}) {
	if view == nil {
		return
	}
	filteredModels := make([]service.ModelCatalogModel, 0, len(view.Models))
	for _, model := range view.Models {
		groups := make([]service.ModelCatalogGroupRef, 0, len(model.Groups))
		for _, group := range model.Groups {
			if _, ok := allowed[group.ID]; ok {
				groups = append(groups, group)
			}
		}
		if len(groups) == 0 {
			continue
		}
		model.Groups = groups
		model.Sources = nil
		filteredModels = append(filteredModels, model)
	}
	view.Models = filteredModels
}
