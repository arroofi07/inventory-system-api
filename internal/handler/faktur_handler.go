package handler

import (
	"net/http"
	"strconv"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
	"github.com/gin-gonic/gin"
)

// FakturHandler GET faktur JSON (SD-09/10).
type FakturHandler struct {
	svc *service.FakturService
}

func NewFakturHandler(svc *service.FakturService) *FakturHandler {
	return &FakturHandler{svc: svc}
}

// Ambil GET /transaksi/{id}/faktur.
//
// @Summary      Data faktur siap cetak
// @Description  JSON layout/alokasi/terbilang. Memicu lock sekali cetak (FAKTUR_LOCK_AKTIF).
// @Tags         Faktur
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  int  true  "ID transaksi"
// @Success      200  {object}  dto.FakturOKResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Failure      409  {object}  dto.ErrorResponse  "STATUS_TIDAK_VALID|FAKTUR_TERKUNCI"
// @Router       /transaksi/{id}/faktur [get]
func (h *FakturHandler) Ambil(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	out, err := h.svc.Ambil(c.Request.Context(), id, uid, role, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// PDF GET /transaksi/{id}/faktur/pdf.
//
// @Summary      Unduh faktur PDF
// @Description  PDF arsip dengan angka sama JSON. Memicu lock sekali cetak identik GET /faktur.
// @Tags         Faktur
// @Security     BearerAuth
// @Produce      application/pdf
// @Param        id   path  int  true  "ID transaksi"
// @Success      200  {file}  file
// @Failure      403  {object}  dto.ErrorResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Failure      409  {object}  dto.ErrorResponse  "STATUS_TIDAK_VALID|FAKTUR_TERKUNCI"
// @Router       /transaksi/{id}/faktur/pdf [get]
func (h *FakturHandler) PDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	out, err := h.svc.AmbilPDF(c.Request.Context(), id, uid, role, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasPDF(c, out.Filename, out.Body)
}

// Referensi tipe untuk generator swag (komentan @Success).
var _ = dto.FakturOKResponse{}
