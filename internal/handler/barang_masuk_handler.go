package handler

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/service"
)

type BarangMasukHandler struct {
	svc    *service.BarangMasukService
	export *service.ExportService
	impor  *service.ImportService
}

func NewBarangMasukHandler(
	svc *service.BarangMasukService,
	export *service.ExportService,
	impor *service.ImportService,
) *BarangMasukHandler {
	return &BarangMasukHandler{svc: svc, export: export, impor: impor}
}

func (h *BarangMasukHandler) Daftar(c *gin.Context) {
	var q dto.BarangMasukListQuery
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

func (h *BarangMasukHandler) Detail(c *gin.Context) {
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

func (h *BarangMasukHandler) Buat(c *gin.Context) {
	var req dto.BarangMasukCreateRequest
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

func (h *BarangMasukHandler) Ubah(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.BarangMasukUpdateRequest
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

func (h *BarangMasukHandler) Hapus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	if err := h.svc.Hapus(c.Request.Context(), id, auditMetaDariContext(c)); err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, gin.H{"ok": true})
}

func (h *BarangMasukHandler) TemplateImpor(c *gin.Context) {
	body, name, err := h.export.TemplateBarangMasuk()
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

func (h *BarangMasukHandler) Export(c *gin.Context) {
	var q dto.BarangMasukListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	body, name, err := h.export.EksporBarangMasuk(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

func (h *BarangMasukHandler) Impor(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "File CSV wajib (field file)")
		return
	}
	f, err := fh.Open()
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	hasil, err := h.impor.ImporBarangMasuk(c.Request.Context(), data, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, hasil)
}
