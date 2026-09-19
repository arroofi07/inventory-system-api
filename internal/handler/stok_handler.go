package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// StokHandler endpoint ledger & penyesuaian (SC-13).
type StokHandler struct {
	svc *service.StockService
}

func NewStokHandler(svc *service.StockService) *StokHandler {
	return &StokHandler{svc: svc}
}

// DaftarPergerakan GET /stok/pergerakan.
//
// @Summary      Kartu stok / pergerakan ledger
// @Tags         Stok
// @Security     BearerAuth
// @Produce      json
// @Param        kode_barang    query  string  true   "Kode SKU"
// @Param        date_from      query  string  false  "YYYY-MM-DD"
// @Param        date_to        query  string  false  "YYYY-MM-DD"
// @Param        movement_type  query  string  false  "PENERIMAAN|PENJUALAN|PENYESUAIAN|..."
// @Param        page           query  int     false  "Halaman"
// @Param        per_page       query  int     false  "Per halaman"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  dto.ErrorResponse
// @Failure      403  {object}  dto.ErrorResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Router       /stok/pergerakan [get]
func (h *StokHandler) DaftarPergerakan(c *gin.Context) {
	var q dto.PergerakanStokListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	items, meta, ringkas, err := h.svc.DaftarPergerakan(c.Request.Context(), q)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": meta, "ringkasan": ringkas})
}

// Penyesuaian POST /stok/penyesuaian.
//
// @Summary      Penyesuaian stok manual
// @Tags         Stok
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  dto.PenyesuaianStokRequest  true  "Delta qty + alasan"
// @Success      200   {object}  map[string]interface{}
// @Failure      401   {object}  dto.ErrorResponse
// @Failure      403   {object}  dto.ErrorResponse
// @Failure      422   {object}  dto.ErrorResponse
// @Router       /stok/penyesuaian [post]
func (h *StokHandler) Penyesuaian(c *gin.Context) {
	var req dto.PenyesuaianStokRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.Penyesuaian(c.Request.Context(), req, uid, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Rekonsiliasi GET /stok/rekonsiliasi — bandingkan ledger vs stok_tersedia.
//
// @Summary      Rekonsiliasi ledger vs stok_tersedia
// @Tags         Stok
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /stok/rekonsiliasi [get]
func (h *StokHandler) Rekonsiliasi(c *gin.Context) {
	out, err := h.svc.Rekonsiliasi(c.Request.Context())
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}
