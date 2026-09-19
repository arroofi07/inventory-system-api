package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// KeteranganPenyesuaianTotal wajib di riwayat saat jumlah_dibayar berubah karena total (04 §5.3).
const KeteranganPenyesuaianTotal = "penyesuaian akibat perubahan total"

// TurunkanStatusPembayaran menurunkan status/sisa dari total dan jumlah dibayar (04 §5.2).
// Kelebihan bayar dipangkas ke total_akhir (lunas). Mengembalikan status, sisa, dibayar (setelah pangkas).
func TurunkanStatusPembayaran(totalAkhir, jumlahDibayar decimal.Decimal) (StatusPembayaran, decimal.Decimal, decimal.Decimal) {
	if totalAkhir.LessThanOrEqual(decimal.Zero) {
		return PembayaranLunas, decimal.Zero, decimal.Zero
	}
	if jumlahDibayar.GreaterThanOrEqual(totalAkhir) {
		return PembayaranLunas, decimal.Zero, totalAkhir
	}
	if jumlahDibayar.LessThanOrEqual(decimal.Zero) {
		return PembayaranHutang, totalAkhir, decimal.Zero
	}
	return PembayaranSebagian, totalAkhir.Sub(jumlahDibayar), jumlahDibayar
}

// HasilSesuaikanPembayaran snapshot sebelum/sesudah untuk jejak riwayat (SD-03).
type HasilSesuaikanPembayaran struct {
	OldJumlahDibayar decimal.Decimal
	NewJumlahDibayar decimal.Decimal
	OldSisaHutang    decimal.Decimal
	NewSisaHutang    decimal.Decimal
	OldStatus        StatusPembayaran
	NewStatus        StatusPembayaran
	// TulisRiwayat true bila jumlah_dibayar berubah.
	TulisRiwayat bool
	// NominalDelta = new - old (boleh negatif).
	NominalDelta decimal.Decimal
}

// SesuaikanPembayaran menyesuaikan field pembayaran saat total_akhir berubah (04 §5.3).
func (t *TransaksiPenjualan) SesuaikanPembayaran(totalAkhirBaru decimal.Decimal) HasilSesuaikanPembayaran {
	hasil := HasilSesuaikanPembayaran{
		OldJumlahDibayar: t.JumlahDibayar,
		OldSisaHutang:    t.SisaHutang,
		OldStatus:        t.StatusPembayaran,
	}
	switch t.StatusPembayaran {
	case PembayaranLunas:
		t.TotalAkhir = totalAkhirBaru
		t.JumlahDibayar = totalAkhirBaru
		t.SisaHutang = decimal.Zero
		t.TanggalJatuhTempo = nil
	case PembayaranHutang:
		t.TotalAkhir = totalAkhirBaru
		t.JumlahDibayar = decimal.Zero
		t.SisaHutang = totalAkhirBaru
	case PembayaranSebagian:
		t.TotalAkhir = totalAkhirBaru
		status, sisa, dibayar := TurunkanStatusPembayaran(totalAkhirBaru, t.JumlahDibayar)
		t.StatusPembayaran = status
		t.SisaHutang = sisa
		t.JumlahDibayar = dibayar
		if status == PembayaranLunas {
			t.TanggalJatuhTempo = nil
		}
	default:
		t.TotalAkhir = totalAkhirBaru
		status, sisa, dibayar := TurunkanStatusPembayaran(totalAkhirBaru, t.JumlahDibayar)
		t.StatusPembayaran = status
		t.SisaHutang = sisa
		t.JumlahDibayar = dibayar
	}
	hasil.NewJumlahDibayar = t.JumlahDibayar
	hasil.NewSisaHutang = t.SisaHutang
	hasil.NewStatus = t.StatusPembayaran
	hasil.NominalDelta = t.JumlahDibayar.Sub(hasil.OldJumlahDibayar)
	hasil.TulisRiwayat = !hasil.NominalDelta.IsZero()
	return hasil
}

// KategoriJatuhTempo klasifikasi 04 §5.5.
type KategoriJatuhTempo string

const (
	KategoriLunas               KategoriJatuhTempo = "lunas"
	KategoriTanpaJatuhTempo     KategoriJatuhTempo = "tanpa_jatuh_tempo"
	KategoriOverdue             KategoriJatuhTempo = "overdue"
	KategoriMendekatiJatuhTempo KategoriJatuhTempo = "mendekati_jatuh_tempo"
	KategoriNormal              KategoriJatuhTempo = "normal"
)

// KlasifikasiJatuhTempo menghitung kategori dan hari terlambat (04 §5.5).
// Ambang mendekati default 7 hari.
func KlasifikasiJatuhTempo(
	status StatusPembayaran,
	tjt *datatypes.Date,
	hariIni time.Time,
	ambangMendekatiHari int,
) (KategoriJatuhTempo, int) {
	if status == PembayaranLunas {
		return KategoriLunas, 0
	}
	if tjt == nil {
		return KategoriTanpaJatuhTempo, 0
	}
	if ambangMendekatiHari < 1 {
		ambangMendekatiHari = 7
	}
	hariIni = time.Date(hariIni.Year(), hariIni.Month(), hariIni.Day(), 0, 0, 0, 0, time.UTC)
	jt := time.Time(*tjt).UTC()
	jt = time.Date(jt.Year(), jt.Month(), jt.Day(), 0, 0, 0, 0, time.UTC)
	if jt.Before(hariIni) {
		hari := int(hariIni.Sub(jt).Hours() / 24)
		return KategoriOverdue, hari
	}
	batas := hariIni.AddDate(0, 0, ambangMendekatiHari)
	if !jt.After(batas) {
		return KategoriMendekatiJatuhTempo, 0
	}
	return KategoriNormal, 0
}
