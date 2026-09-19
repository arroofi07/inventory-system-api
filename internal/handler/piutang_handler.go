package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/service"
)

// PiutangHandler endpoint piutang & notifikasi (SD-05/06).
type PiutangHandler struct {
	svc *service.PiutangService
}

func NewPiutangHandler(svc *service.PiutangService) *PiutangHandler {
	return &PiutangHandler{svc: svc}
}

func (h *PiutangHandler) balasList(c *gin.Context, items []dto.PiutangItem, meta dto.PageMeta, ringkas dto.PiutangRingkasan) {
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": meta, "ringkasan": ringkas})
}

// Daftar GET /piutang.
//
// @Summary      Daftar piutang
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      json
// @Param        q                     query  string  false  "Cari"
// @Param        status_pembayaran     query  string  false  "lunas|hutang|sebagian"
// @Param        kategori_jatuh_tempo  query  string  false  "overdue|mendekati_jatuh_tempo|normal|tanpa_jatuh_tempo|lunas"
// @Param        brand                 query  string  false  "Filter brand"
// @Param        date_from             query  string  false  "YYYY-MM-DD"
// @Param        date_to               query  string  false  "YYYY-MM-DD"
// @Param        date_type             query  string  false  "tanggal_transaksi|tanggal_jatuh_tempo|tanggal_pembayaran_terakhir"
// @Success      200  {object}  map[string]interface{}
// @Router       /piutang [get]
func (h *PiutangHandler) Daftar(c *gin.Context) {
	var q dto.PiutangListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, ringkas, err := h.svc.Daftar(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	h.balasList(c, items, meta, ringkas)
}

// Overdue GET /piutang/overdue.
//
// @Summary      Piutang overdue
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /piutang/overdue [get]
func (h *PiutangHandler) Overdue(c *gin.Context) {
	var q dto.PiutangListQuery
	_ = c.ShouldBindQuery(&q)
	items, meta, ringkas, err := h.svc.Overdue(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	h.balasList(c, items, meta, ringkas)
}

// Export GET /piutang/export.
//
// @Summary      Ekspor piutang CSV
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      text/csv
// @Router       /piutang/export [get]
func (h *PiutangHandler) Export(c *gin.Context) {
	var q dto.PiutangListQuery
	_ = c.ShouldBindQuery(&q)
	body, name, err := h.svc.Ekspor(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

// Pelanggan GET /piutang/pelanggan/:kode.
//
// @Summary      Piutang per pelanggan
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      json
// @Param        kode  path  string  true  "Kode pelanggan"
// @Router       /piutang/pelanggan/{kode} [get]
func (h *PiutangHandler) Pelanggan(c *gin.Context) {
	var q dto.PiutangListQuery
	_ = c.ShouldBindQuery(&q)
	items, meta, ringkas, err := h.svc.Pelanggan(c.Request.Context(), c.Param("kode"), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	h.balasList(c, items, meta, ringkas)
}

// RiwayatPelanggan GET /piutang/pelanggan/:kode/riwayat.
//
// @Summary      Riwayat pembayaran pelanggan
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      json
// @Param        kode  path  string  true  "Kode pelanggan"
// @Router       /piutang/pelanggan/{kode}/riwayat [get]
func (h *PiutangHandler) RiwayatPelanggan(c *gin.Context) {
	out, err := h.svc.RiwayatPelanggan(c.Request.Context(), c.Param("kode"))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	if out == nil {
		out = []dto.RiwayatPembayaranItem{}
	}
	httpx.BalasOK(c, out)
}

// Notifikasi GET /notifikasi/piutang.
//
// @Summary      Notifikasi count piutang
// @Tags         Piutang
// @Security     BearerAuth
// @Produce      json
// @Router       /notifikasi/piutang [get]
func (h *PiutangHandler) Notifikasi(c *gin.Context) {
	out, err := h.svc.Notifikasi(c.Request.Context())
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Penerimaan GET /laporan/penerimaan.
//
// @Summary      Rekap penerimaan kas periode
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Param        date_from  query  string  true  "YYYY-MM-DD"
// @Param        date_to    query  string  true  "YYYY-MM-DD"
// @Router       /laporan/penerimaan [get]
func (h *PiutangHandler) Penerimaan(c *gin.Context) {
	var q dto.PenerimaanLaporanQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	out, err := h.svc.Penerimaan(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}
