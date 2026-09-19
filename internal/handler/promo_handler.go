package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/service"
)

type PromoHandler struct {
	svc *service.PromoService
}

func NewPromoHandler(svc *service.PromoService) *PromoHandler {
	return &PromoHandler{svc: svc}
}

func (h *PromoHandler) Daftar(c *gin.Context) {
	var q dto.PromoListQuery
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

func (h *PromoHandler) Detail(c *gin.Context) {
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

func (h *PromoHandler) Buat(c *gin.Context) {
	var req dto.PromoCreateRequest
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

func (h *PromoHandler) Ubah(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.PromoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	item, err := h.svc.Ubah(c.Request.Context(), id, req, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *PromoHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.PromoStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	item, err := h.svc.SetStatus(c.Request.Context(), id, req.IsActive, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *PromoHandler) Hapus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	if err := h.svc.Hapus(c.Request.Context(), id, auditMetaDariContext(c)); err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.Status(204)
}
