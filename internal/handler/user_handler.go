package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Daftar(c *gin.Context) {
	var q dto.UserListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, err := h.svc.Daftar(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasList(c, items, meta)
}

func (h *UserHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	item, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *UserHandler) Buat(c *gin.Context) {
	var req dto.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	item, err := h.svc.Buat(c.Request.Context(), req, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCreated(c, item)
}

func (h *UserHandler) Ubah(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	aktorID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	item, err := h.svc.Ubah(c.Request.Context(), id, req, aktorID, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *UserHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.UserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	aktorID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	item, err := h.svc.SetStatus(c.Request.Context(), id, req.IsActive, aktorID, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *UserHandler) Hapus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	aktorID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	if err := h.svc.Hapus(c.Request.Context(), id, aktorID, auditMetaDariContext(c)); err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.Status(204)
}
