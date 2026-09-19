package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// LaporanHandler endpoint laporan & ekspor (SC-09 / SE-07/08).
type LaporanHandler struct {
	export  *service.ExportService
	laporan *service.LaporanService
	jobs    *service.ExportJobStore
}

func NewLaporanHandler(export *service.ExportService, laporan *service.LaporanService, jobs *service.ExportJobStore) *LaporanHandler {
	if jobs == nil {
		jobs = service.NewExportJobStore()
	}
	return &LaporanHandler{export: export, laporan: laporan, jobs: jobs}
}

// ExportPenjualan GET /laporan/penjualan/export — default status_approval=approved.
//
// @Summary      Ekspor laporan penjualan CSV
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      text/csv
// @Param        status_approval   query  string  false  "pending|approved|rejected (default approved)"
// @Param        status_pembayaran query  string  false  "lunas|hutang|sebagian"
// @Param        date_from         query  string  false  "YYYY-MM-DD"
// @Param        date_to           query  string  false  "YYYY-MM-DD"
// @Param        channel_outlet    query  string  false  "Channel outlet"
// @Param        sales_id          query  int     false  "Filter sales"
// @Param        q                 query  string  false  "Pencarian"
// @Success      200  {file}  file
// @Router       /laporan/penjualan/export [get]
func (h *LaporanHandler) ExportPenjualan(c *gin.Context) {
	var q dto.TransaksiListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	if fmt := c.Query("format"); fmt != "" && fmt != "csv" {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "FORMAT_TIDAK_DIDUKUNG", "Hanya format=csv didukung di v1 (xlsx ditunda)")
		return
	}
	qCopy := q
	if strings.TrimSpace(qCopy.StatusApproval) == "" {
		qCopy.StatusApproval = "approved"
	}
	_, total, err := h.export.HitungPenjualan(c.Request.Context(), qCopy)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	if total > service.AmbangEksporAsync {
		uid, _ := c.Get(middleware.CtxKeyUserID)
		userID, _ := uid.(uint64)
		jobID := h.jobs.Mulai(userID, "laporan-penjualan.csv", func() ([]byte, error) {
			body, _, err := h.export.EksporPenjualan(c.Request.Context(), q)
			return body, err
		})
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"job_id": jobID, "status": "pending"}})
		return
	}
	body, name, err := h.export.EksporPenjualan(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

// AmbilEkspor GET /ekspor/{job_id}.
func (h *LaporanHandler) AmbilEkspor(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxKeyUserID)
	userID, _ := uid.(uint64)
	job, ok := h.jobs.Ambil(c.Param("job_id"), userID)
	if !ok {
		httpx.BalasError(c, http.StatusNotFound, "TIDAK_DITEMUKAN", "Job ekspor tidak ditemukan")
		return
	}
	switch job.Status {
	case "pending":
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"job_id": job.ID, "status": "pending"}})
	case "failed":
		httpx.BalasError(c, http.StatusInternalServerError, "EKSPOR_GAGAL", job.Error)
	case "ready":
		httpx.BalasCSV(c, job.Filename, job.Body)
	default:
		httpx.BalasError(c, http.StatusConflict, "STATUS_TIDAK_VALID", "Status job tidak dikenal")
	}
}

// Penjualan GET /laporan/penjualan — default approved (SE-10).
//
// @Summary      Laporan penjualan (filter periode/channel/sales)
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Param        date_from      query string false "YYYY-MM-DD"
// @Param        date_to        query string false "YYYY-MM-DD"
// @Param        channel_outlet query string false "Channel"
// @Param        sales_id       query int    false "Sales ID"
// @Param        page           query int    false "Halaman"
// @Param        per_page       query int    false "Per halaman"
// @Success      200  {object}  map[string]interface{}
// @Router       /laporan/penjualan [get]
func (h *LaporanHandler) Penjualan(c *gin.Context) {
	var q dto.TransaksiListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, ringkas, err := h.export.LaporanPenjualan(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": meta, "ringkasan": ringkas})
}

// Stok GET /laporan/stok.
//
// @Summary      Laporan stok per SKU
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Param        q           query  string  false  "Cari"
// @Param        brand       query  string  false  "Brand"
// @Param        status_stok query  string  false  "NORMAL|RENDAH|HABIS"
// @Param        page        query  int     false  "Halaman"
// @Param        per_page    query  int     false  "Per halaman"
// @Success      200  {object}  map[string]interface{}
// @Router       /laporan/stok [get]
func (h *LaporanHandler) Stok(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan stok belum diinisialisasi")
		return
	}
	var q dto.LaporanStokQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, ringkas, err := h.laporan.Stok(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": meta, "ringkasan": ringkas})
}

// ExportStok GET /laporan/stok/export.
//
// @Summary      Ekspor laporan stok CSV
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      text/csv
// @Router       /laporan/stok/export [get]
func (h *LaporanHandler) ExportStok(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan stok belum diinisialisasi")
		return
	}
	var q dto.LaporanStokQuery
	_ = c.ShouldBindQuery(&q)
	body, name, err := h.laporan.EksporStok(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

// BarangKeluar GET /laporan/barang-keluar.
//
// @Summary      Laporan barang keluar (+ laba bila diizinkan)
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Router       /laporan/barang-keluar [get]
func (h *LaporanHandler) BarangKeluar(c *gin.Context) {
	h.barangKeluar(c)
}

// Laba GET /laporan/laba — sama data, wajib izin laporan.laba.
//
// @Summary      Laporan laba per baris penjualan
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Router       /laporan/laba [get]
func (h *LaporanHandler) Laba(c *gin.Context) {
	h.barangKeluar(c)
}

func (h *LaporanHandler) barangKeluar(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan belum diinisialisasi")
		return
	}
	var q dto.LaporanBarangKeluarQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	items, meta, ringkas, err := h.laporan.BarangKeluar(c.Request.Context(), q, role)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": meta, "ringkasan": ringkas})
}

// ExportBarangKeluar GET /laporan/barang-keluar/export.
//
// @Summary      Ekspor barang keluar CSV
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      text/csv
// @Router       /laporan/barang-keluar/export [get]
func (h *LaporanHandler) ExportBarangKeluar(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan belum diinisialisasi")
		return
	}
	var q dto.LaporanBarangKeluarQuery
	_ = c.ShouldBindQuery(&q)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	body, name, err := h.laporan.EksporBarangKeluar(c.Request.Context(), q, role)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}

// ChannelAnalytics GET /laporan/channel-analytics.
//
// @Summary      Analitik channel, territory, produk terlaris, tren
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      json
// @Param        date_from query string false "YYYY-MM-DD"
// @Param        date_to   query string false "YYYY-MM-DD"
// @Param        limit     query int    false "Limit produk terlaris"
// @Success      200  {object}  map[string]interface{}
// @Router       /laporan/channel-analytics [get]
func (h *LaporanHandler) ChannelAnalytics(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan belum diinisialisasi")
		return
	}
	var q dto.ChannelAnalyticsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	data, err := h.laporan.ChannelAnalytics(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// ExportChannelAnalytics GET /laporan/channel-analytics/export.
//
// @Summary      Ekspor channel analytics CSV
// @Tags         Laporan
// @Security     BearerAuth
// @Produce      text/csv
// @Router       /laporan/channel-analytics/export [get]
func (h *LaporanHandler) ExportChannelAnalytics(c *gin.Context) {
	if h.laporan == nil {
		httpx.BalasError(c, http.StatusNotImplemented, "BELUM_SIAP", "Laporan belum diinisialisasi")
		return
	}
	var q dto.ChannelAnalyticsQuery
	_ = c.ShouldBindQuery(&q)
	body, name, err := h.laporan.EksporChannelAnalytics(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCSV(c, name, body)
}
