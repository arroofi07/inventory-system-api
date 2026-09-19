package handler

import (
	"github.com/gin-gonic/gin"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// DashboardHandler GET /dashboard (SE-05).
type DashboardHandler struct {
	svc *service.DashboardService
}

func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// Ambil GET /dashboard.
//
// @Summary      Dashboard sesuai role
// @Tags         Dashboard
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  dto.DashboardOKResponse
// @Router       /dashboard [get]
func (h *DashboardHandler) Ambil(c *gin.Context) {
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	out, err := h.svc.Ambil(c.Request.Context(), uid, role)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

var _ = dto.DashboardOKResponse{}
