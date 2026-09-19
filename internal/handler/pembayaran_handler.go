package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// PembayaranHandler endpoint pembayaran pada transaksi (SD-01/02/04).
type PembayaranHandler struct {
	svc *service.PembayaranService
}

func NewPembayaranHandler(svc *service.PembayaranService) *PembayaranHandler {
	return &PembayaranHandler{svc: svc}
}

// Catat POST /transaksi/{id}/pembayaran.
//
// @Summary      Catat nominal pembayaran
// @Description  Menambahkan nominal_pembayaran ke jumlah_dibayar. Status diturunkan server.
// @Description  Header Idempotency-Key mencegah double-posting.
// @Tags         Pembayaran
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id               path   int                     true  "ID transaksi"
// @Param        Idempotency-Key  header string                  false "Kunci idempotensi"
// @Param        body             body   dto.PembayaranCreateRequest true "Nominal tambahan"
// @Success      200  {object}  dto.PembayaranCreateOKResponse
// @Failure      409  {object}  dto.ErrorResponse  "STATUS_TIDAK_VALID|KELEBIHAN_BAYAR"
// @Failure      422  {object}  dto.ErrorResponse
// @Router       /transaksi/{id}/pembayaran [post]
func (h *PembayaranHandler) Catat(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body tidak terbaca")
		return
	}
	var req dto.PembayaranCreateRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
			return
		}
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	idemKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	out, cached, err := h.svc.Catat(c.Request.Context(), id, req, uid, idemKey, raw, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	if cached != nil {
		c.Data(cached.StatusCode, "application/json; charset=utf-8", cached.Body)
		return
	}
	httpx.BalasOK(c, out)
}

// Riwayat GET /transaksi/{id}/pembayaran.
//
// @Summary      Riwayat pembayaran transaksi
// @Tags         Pembayaran
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  int  true  "ID transaksi"
// @Success      200  {object}  dto.RiwayatPembayaranListOKResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Router       /transaksi/{id}/pembayaran [get]
func (h *PembayaranHandler) Riwayat(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	out, err := h.svc.RiwayatByTransaksi(c.Request.Context(), id)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	if out == nil {
		out = []dto.RiwayatPembayaranItem{}
	}
	httpx.BalasOK(c, out)
}
