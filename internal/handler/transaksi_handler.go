package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

// TransaksiHandler endpoint transaksi penjualan.
type TransaksiHandler struct {
	svc *service.TransaksiService
}

func NewTransaksiHandler(svc *service.TransaksiService) *TransaksiHandler {
	return &TransaksiHandler{svc: svc}
}

// Daftar GET /transaksi — filter + isolasi sales (SC-07).
//
// @Summary      Daftar transaksi
// @Description  Sales hanya melihat miliknya. Admin/afiliasi melihat semua (izin lihat_semua).
// @Description  Filter: q, status_approval, status_pembayaran, date_from, date_to, sales_id.
// @Tags         Transaksi
// @Security     BearerAuth
// @Produce      json
// @Param        q                 query  string  false  "Cari kode/nama pelanggan, id, no_transaksi"
// @Param        status_approval   query  string  false  "pending|approved|rejected"
// @Param        status_pembayaran query  string  false  "lunas|hutang|sebagian"
// @Param        date_from         query  string  false  "YYYY-MM-DD"
// @Param        date_to           query  string  false  "YYYY-MM-DD"
// @Param        kecukupan_stok    query  string  false  "cukup|kurang (hanya pending)"
// @Param        sales_id          query  int     false  "Filter sales (hanya lihat_semua)"
// @Param        page              query  int     false  "Halaman"
// @Param        per_page          query  int     false  "Per halaman"
// @Param        sort              query  string  false  "Kolom sort"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  dto.ErrorResponse
// @Failure      403  {object}  dto.ErrorResponse
// @Router       /transaksi [get]
func (h *TransaksiHandler) Daftar(c *gin.Context) {
	var q dto.TransaksiListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Parameter query tidak valid")
		return
	}
	userID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	items, meta, err := h.svc.Daftar(c.Request.Context(), q, userID, role)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasList(c, items, meta)
}

// Pratinjau menghitung transaksi tanpa menyimpan.
//
// @Summary      Pratinjau transaksi
// @Description  Menghitung total, PPN, bonus promo, dan alokasi batch tanpa menulis DB.
// @Description  Mode multi: kirim items[]. Mode single: kirim objek item di header request.
// @Description  Promo dirujuk via kode_promos (bukan FK wajib). Harga final ditentukan server.
// @Description  Field respons baris: jumlah (qty ditagih) dan total_qty_keluar (jumlah+bonus).
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.TransaksiPratinjauRequest  true  "Payload pratinjau"
// @Success      200   {object}  dto.PratinjauTransaksiOKResponse
// @Failure      401   {object}  dto.ErrorResponse
// @Failure      403   {object}  dto.ErrorResponse
// @Failure      404   {object}  dto.ErrorResponse  "TIDAK_DITEMUKAN"
// @Failure      422   {object}  dto.ErrorResponse  "VALIDASI_GAGAL"
// @Router       /transaksi/pratinjau [post]
func (h *TransaksiHandler) Pratinjau(c *gin.Context) {
	var req dto.TransaksiPratinjauRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	out, err := h.svc.Pratinjau(c.Request.Context(), req)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// CekStok memeriksa ketersediaan stok per SKU/batch (soft-check, SC-05).
//
// @Summary      Cek ketersediaan stok
// @Description  Soft-check qty tersedia dari ledger per SKU/batch yang diminta.
// @Description  Bukan jaminan alokasi; jaminan hanya di approval gate.
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.TransaksiCekStokRequest  true  "Daftar SKU/qty"
// @Success      200   {object}  dto.TransaksiCekStokOKResponse
// @Failure      401   {object}  dto.ErrorResponse
// @Failure      403   {object}  dto.ErrorResponse
// @Failure      404   {object}  dto.ErrorResponse
// @Failure      422   {object}  dto.ErrorResponse
// @Router       /transaksi/cek-stok [post]
func (h *TransaksiHandler) CekStok(c *gin.Context) {
	var req dto.TransaksiCekStokRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	out, err := h.svc.CekStok(c.Request.Context(), req)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Buat menyimpan transaksi pending.
//
// @Summary      Buat transaksi (pending)
// @Description  Menyimpan order status pending tanpa no_transaksi dan tanpa mengurangi stok.
// @Description  Snapshot pelanggan, HPP, dan promo tersimpan per baris transaksi_detail.
// @Description  Harga harus cocok dengan channel batch; stok harus cukup (validasi, belum potong).
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.TransaksiCreateRequest  true  "Payload store"
// @Success      201   {object}  dto.TransaksiOKResponse
// @Failure      401   {object}  dto.ErrorResponse
// @Failure      403   {object}  dto.ErrorResponse
// @Failure      409   {object}  dto.ErrorResponse  "HARGA_TIDAK_SESUAI / STOK_TIDAK_CUKUP / KELEBIHAN_BAYAR"
// @Failure      422   {object}  dto.ErrorResponse  "VALIDASI_GAGAL"
// @Router       /transaksi [post]
func (h *TransaksiHandler) Buat(c *gin.Context) {
	var req dto.TransaksiCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	salesID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.Buat(c.Request.Context(), req, salesID, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasCreated(c, out)
}

// TambahItems menambah baris ke transaksi pending dan menghitung ulang dari nol.
//
// @Summary      Tambah item ke transaksi pending
// @Description  Hanya transaksi pending milik sales. Setelah add, single-item boleh menjadi multi-item.
// @Description  Total dihitung ulang dari nol (bukan inkremental). Stok belum berkurang.
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "ID transaksi"
// @Param        body  body      dto.TransaksiAddItemsRequest  true  "Item baru"
// @Success      200   {object}  dto.TransaksiOKResponse
// @Failure      401   {object}  dto.ErrorResponse
// @Failure      403   {object}  dto.ErrorResponse
// @Failure      404   {object}  dto.ErrorResponse
// @Failure      404   {object}  dto.ErrorResponse  "TIDAK_DITEMUKAN"
// @Failure      409   {object}  dto.ErrorResponse  "STATUS_TIDAK_VALID / STOK_TIDAK_CUKUP / HARGA_TIDAK_SESUAI"
// @Failure      422   {object}  dto.ErrorResponse  "VALIDASI_GAGAL"
// @Router       /transaksi/{id}/items [post]
func (h *TransaksiHandler) TambahItems(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.TransaksiAddItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	salesID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.TambahItems(c.Request.Context(), id, req, salesID, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Detail mengembalikan transaksi lengkap dengan baris detail.
//
// @Summary      Detail transaksi
// @Description  Mengembalikan header (snapshot pelanggan) + items dari transaksi_detail.
// @Description  Setiap baris memuat jumlah (qty ditagih) dan total_qty_keluar.
// @Description  Sales hanya melihat transaksi miliknya.
// @Tags         Transaksi
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "ID transaksi"
// @Success      200  {object}  dto.TransaksiOKResponse
// @Failure      401  {object}  dto.ErrorResponse
// @Failure      403  {object}  dto.ErrorResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Router       /transaksi/{id} [get]
func (h *TransaksiHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	userID := c.MustGet(middleware.CtxKeyUserID).(uint64)
	role := c.MustGet(middleware.CtxKeyRole).(domain.Role)
	out, err := h.svc.Detail(c.Request.Context(), id, userID, role)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// KetersediaanStok tabel kecukupan + pending bersaing (SC-09).
//
// @Summary      Ketersediaan stok transaksi
// @Tags         Transaksi
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  int  true  "ID transaksi"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  dto.ErrorResponse
// @Router       /transaksi/{id}/ketersediaan-stok [get]
func (h *TransaksiHandler) KetersediaanStok(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	out, err := h.svc.KetersediaanStok(c.Request.Context(), id)
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Approve menyetujui transaksi pending (SC-10).
//
// @Summary      Setujui transaksi
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "ID transaksi"
// @Param        body  body  dto.ApproveRequest    false "Catatan opsional"
// @Success      200   {object}  map[string]interface{}
// @Failure      409   {object}  dto.ErrorResponse  "STOK_TIDAK_CUKUP"
// @Router       /transaksi/{id}/approve [post]
func (h *TransaksiHandler) Approve(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.ApproveRequest
	_ = c.ShouldBindJSON(&req)
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.Approve(c.Request.Context(), id, req, uid, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// Reject menolak transaksi pending (SC-11).
//
// @Summary      Tolak transaksi
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  int                 true  "ID transaksi"
// @Param        body  body  dto.RejectRequest   true  "Catatan wajib"
// @Success      200   {object}  dto.TransaksiOKResponse
// @Failure      422   {object}  dto.ErrorResponse
// @Router       /transaksi/{id}/reject [post]
func (h *TransaksiHandler) Reject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "ID tidak valid")
		return
	}
	var req dto.RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.Reject(c.Request.Context(), id, req, uid, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}

// BulkApprove menyetujui banyak transaksi (SC-12).
//
// @Summary      Bulk approve
// @Tags         Transaksi
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  dto.BulkApproveRequest  true  "Daftar ID"
// @Success      200   {object}  map[string]interface{}
// @Router       /transaksi/approvals/bulk [post]
func (h *TransaksiHandler) BulkApprove(c *gin.Context) {
	var req dto.BulkApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Body JSON tidak valid")
		return
	}
	uid := c.MustGet(middleware.CtxKeyUserID).(uint64)
	out, err := h.svc.BulkApprove(c.Request.Context(), req, uid, auditMetaDariContext(c))
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}
	httpx.BalasOK(c, out)
}
