package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// BarangHandler adalah template CRUD SB-01:
// list (paginasi/sort/search) → detail → create → update → soft-nonaktif,
// validasi playground, audit pada tulis.
type BarangHandler struct {
	svc *service.BarangService
}

func NewBarangHandler(svc *service.BarangService) *BarangHandler {
	return &BarangHandler{svc: svc}
}

func (h *BarangHandler) Daftar(c *gin.Context) {
	var q dto.BarangListQuery
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

func (h *BarangHandler) Detail(c *gin.Context) {
	key := c.Param("kode_barang")
	id, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		item, err := h.svc.DetailByKode(c.Request.Context(), key)
		if err != nil {
			httpx.MapDomainError(c, err)
			return
		}
		httpx.BalasOK(c, item)
		return
	}
	item, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *BarangHandler) Buat(c *gin.Context) {
	var req dto.BarangCreateRequest
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

func (h *BarangHandler) Ubah(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("kode_barang"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.BarangUpdateRequest
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

func (h *BarangHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("kode_barang"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.BarangStatusRequest
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

func (h *BarangHandler) BatchTersedia(c *gin.Context) {
	kode := c.Param("kode_barang")
	var q dto.BatchTersediaQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	item, err := h.svc.BatchTersedia(c.Request.Context(), kode, q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *BarangHandler) DaftarBatch(c *gin.Context) {
	kode := c.Param("kode_barang")
	var q dto.BatchListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, err := h.svc.DaftarBatch(c.Request.Context(), kode, q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, items)
}

func (h *BarangHandler) HargaMassal(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("kode_barang"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.HargaMassalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	item, err := h.svc.HargaMassal(c.Request.Context(), id, req, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, item)
}

func (h *BarangHandler) RiwayatHarga(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("kode_barang"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var q dto.RiwayatHargaQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, err := h.svc.RiwayatHarga(c.Request.Context(), id, q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasList(c, items, meta)
}

func auditMetaDariContext(c *gin.Context) service.AuditMeta {
	meta := service.AuditMeta{}
	if v, ok := c.Get(middleware.CtxKeyUserID); ok {
		id := v.(uint64)
		meta.UserID = &id
	}
	ip := c.ClientIP()
	meta.IPAddress = &ip
	if rid := c.GetString(middleware.CtxKeyRequestID); rid != "" {
		meta.RequestID = &rid
	}
	return meta
}
