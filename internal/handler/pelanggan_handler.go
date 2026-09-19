package handler

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/service"
)

type PelangganHandler struct {
	svc    *service.PelangganService
	export *service.ExportService
	impor  *service.ImportService
}

func NewPelangganHandler(
	svc *service.PelangganService,
	export *service.ExportService,
	impor *service.ImportService,
) *PelangganHandler {
	return &PelangganHandler{svc: svc, export: export, impor: impor}
}

func (h *PelangganHandler) Daftar(c *gin.Context) {
	var q dto.PelangganListQuery
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

// CariSelect2 GET /pelanggan/cari — envelope Select2 (results + pagination.more).
func (h *PelangganHandler) CariSelect2(c *gin.Context) {
	var q dto.PelangganCariQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	out, err := h.svc.CariSelect2(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(200, out)
}

func (h *PelangganHandler) Detail(c *gin.Context) {
	key := c.Param("id")
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

// RiwayatTransaksi GET /pelanggan/{kode}/riwayat-transaksi — ringkas outlet (SC-06).
func (h *PelangganHandler) RiwayatTransaksi(c *gin.Context) {
	key := c.Param("id")
	items, err := h.svc.RiwayatTransaksi(c.Request.Context(), key)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, items)
}

func (h *PelangganHandler) Buat(c *gin.Context) {
	var req dto.PelangganCreateRequest
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

func (h *PelangganHandler) Ubah(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.PelangganUpdateRequest
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

func (h *PelangganHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.PelangganStatusRequest
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

func (h *PelangganHandler) TemplateImpor(c *gin.Context) {
	body, name, err := h.export.TemplatePelanggan()
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

func (h *PelangganHandler) Export(c *gin.Context) {
	var q dto.PelangganListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, 422, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	body, name, err := h.export.EksporPelanggan(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

func (h *PelangganHandler) Impor(c *gin.Context) {
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
	hasil, err := h.impor.ImporPelanggan(c.Request.Context(), data, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, hasil)
}
