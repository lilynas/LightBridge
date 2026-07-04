package admin

import (
	"strconv"
	"strings"

	"github.com/WilliamWang1721/LightBridge/internal/pkg/response"
	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelCatalogHandler struct {
	service *service.ModelCatalogService
}

func NewModelCatalogHandler(svc *service.ModelCatalogService) *ModelCatalogHandler {
	return &ModelCatalogHandler{service: svc}
}

// List returns the admin model catalog. Admins can see account/source details.
// GET /api/v1/admin/model-catalog?group_id=123&view=merged
func (h *ModelCatalogHandler) List(c *gin.Context) {
	if h == nil || h.service == nil {
		response.Success(c, service.ModelCatalogView{Models: []service.ModelCatalogModel{}})
		return
	}
	groupID, ok := parseOptionalModelCatalogGroupID(c)
	if !ok {
		return
	}
	view, err := h.service.ListCatalog(c.Request.Context(), groupID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func parseOptionalModelCatalogGroupID(c *gin.Context) (*int64, bool) {
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
