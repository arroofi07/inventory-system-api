package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"app/internal/domain"
	"app/internal/dto"
	appvalidator "app/internal/pkg/validator"
	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Details []dto.FieldError `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

func BalasError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorEnvelope{
		Error: ErrorBody{Code: code, Message: message},
	})
}

func BalasValidasi(c *gin.Context, fields []dto.FieldError) {
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorEnvelope{
		Error: ErrorBody{
			Code:    "VALIDASI_GAGAL",
			Message: "Data tidak valid",
			Details: fields,
		},
	})
}

func BalasOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func BalasCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func BalasList(c *gin.Context, data any, meta dto.PageMeta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

// BalasCSV mengirim file CSV dengan Content-Disposition attachment.
func BalasCSV(c *gin.Context, filename string, body []byte) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", body)
}

// BalasPDF mengirim berkas PDF unduhan.
func BalasPDF(c *gin.Context, filename string, body []byte) {
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", body)
}

func MapDomainError(c *gin.Context, err error) {
	var ve *appvalidator.Errors
	if errors.As(err, &ve) {
		fields := make([]dto.FieldError, 0, len(ve.Fields))
		for _, f := range ve.Fields {
			fields = append(fields, dto.FieldError{Field: f.Field, Message: f.Message})
		}
		BalasValidasi(c, fields)
		return
	}

	switch {
	case errors.Is(err, domain.ErrKredensialSalah):
		BalasError(c, http.StatusUnauthorized, "KREDENSIAL_SALAH", "Email atau password salah")
	case errors.Is(err, domain.ErrAkunNonaktif):
		BalasError(c, http.StatusForbidden, "AKUN_NONAKTIF", "Akun tidak aktif")
	case errors.Is(err, domain.ErrRefreshTokenTidakValid), errors.Is(err, domain.ErrRefreshTokenKedaluwarsa):
		BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI", "Refresh token tidak valid atau kedaluwarsa")
	case errors.Is(err, domain.ErrTerlaluBanyakPercobaan):
		BalasError(c, http.StatusTooManyRequests, "TERLALU_BANYAK_PERCOBAAN", "Terlalu banyak percobaan login. Coba lagi nanti.")
	case errors.Is(err, domain.ErrTidakDiizinkan):
		BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI", "Token tidak valid atau sudah kedaluwarsa")
	case errors.Is(err, domain.ErrTidakDitemukan):
		BalasError(c, http.StatusNotFound, "TIDAK_DITEMUKAN", "Data tidak ditemukan")
	case errors.Is(err, domain.ErrDuplikat):
		BalasError(c, http.StatusConflict, "DUPLIKAT", "Data sudah ada")
	case errors.Is(err, domain.ErrStatusTidakValid):
		BalasError(c, http.StatusConflict, "STATUS_TIDAK_VALID", "Operasi tidak diizinkan pada status/data saat ini")
	case errors.Is(err, domain.ErrKelebihanBayar):
		BalasError(c, http.StatusConflict, "KELEBIHAN_BAYAR", "Nominal melebihi total akhir")
	case errors.Is(err, domain.ErrFakturTerkunci):
		BalasError(c, http.StatusConflict, "FAKTUR_TERKUNCI", "Faktur sudah dicetak dan terkunci")
	case errors.Is(err, domain.ErrStokTidakCukup):
		var stokErr *domain.ErrStokApproval
		if errors.As(err, &stokErr) {
			details := make([]dto.StokKurangItem, 0, len(stokErr.Details))
			for _, d := range stokErr.Details {
				details = append(details, dto.StokKurangItem{
					KodeItem: d.KodeItem, NamaItem: d.NamaItem, Diminta: d.Diminta, Tersedia: d.Tersedia,
				})
			}
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "STOK_TIDAK_CUKUP",
					"message": "Stok tidak cukup untuk menyetujui transaksi",
					"details": details,
				},
			})
			return
		}
		BalasError(c, http.StatusConflict, "STOK_TIDAK_CUKUP", "Stok tidak cukup")
	case errors.Is(err, domain.ErrHargaTidakSesuai):
		BalasError(c, http.StatusConflict, "HARGA_TIDAK_SESUAI", "Harga tidak sesuai harga channel batch")
	case errors.Is(err, domain.ErrImporValidasi):
		var ie *domain.ErrImporCSV
		if errors.As(err, &ie) {
			details := make([]dto.FieldError, 0, len(ie.Galat))
			for _, g := range ie.Galat {
				field := g.Field
				if g.Baris > 0 {
					if field != "" {
						field = fmt.Sprintf("baris_%d.%s", g.Baris, field)
					} else {
						field = fmt.Sprintf("baris_%d", g.Baris)
					}
				}
				details = append(details, dto.FieldError{Field: field, Message: g.Message})
			}
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorEnvelope{
				Error: ErrorBody{
					Code:    "IMPOR_VALIDASI_GAGAL",
					Message: "Impor ditolak — perbaiki baris yang galat lalu unggah ulang seluruh berkas",
					Details: details,
				},
			})
			return
		}
		BalasError(c, http.StatusUnprocessableEntity, "IMPOR_VALIDASI_GAGAL", "Impor ditolak")
	default:
		BalasError(c, http.StatusInternalServerError, "KESALAHAN_INTERNAL", "Terjadi kesalahan internal")
	}
}
