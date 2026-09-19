package domain_test

import (
	"testing"

	"app/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

func TestTurunkanStatusPembayaran(t *testing.T) {
	kasus := []struct {
		nama                string
		totalAkhir, dibayar string
		status              domain.StatusPembayaran
		sisa                string
	}{
		{"belum bayar", "347985.00", "0.00", domain.PembayaranHutang, "347985.00"},
		{"bayar sebagian", "347985.00", "150000.00", domain.PembayaranSebagian, "197985.00"},
		{"bayar penuh", "347985.00", "347985.00", domain.PembayaranLunas, "0.00"},
		{"bayar melebihi dipangkas", "347985.00", "400000.00", domain.PembayaranLunas, "0.00"},
		{"total nol dianggap lunas", "0.00", "0.00", domain.PembayaranLunas, "0.00"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			status, sisa, dibayar := domain.TurunkanStatusPembayaran(dec(k.totalAkhir), dec(k.dibayar))
			if status != k.status {
				t.Fatalf("status %s want %s", status, k.status)
			}
			if sisa.Round(2).StringFixed(2) != k.sisa {
				t.Fatalf("sisa %s want %s", sisa.Round(2).StringFixed(2), k.sisa)
			}
			if dibayar.Add(sisa).Sub(dec(k.totalAkhir)).Abs().GreaterThanOrEqual(dec("0.01")) {
				t.Fatalf("invariant I1 dilanggar")
			}
		})
	}
}

func TestSesuaikanPembayaranSaatTotalBerubah(t *testing.T) {
	t.Run("status lunas mengikuti total baru", func(t *testing.T) {
		trx := domain.TransaksiPenjualan{
			StatusPembayaran: domain.PembayaranLunas,
			TotalAkhir:       dec("200000"),
			JumlahDibayar:    dec("200000"),
			SisaHutang:       decimal.Zero,
		}
		trx.SesuaikanPembayaran(dec("250000"))
		if trx.StatusPembayaran != domain.PembayaranLunas {
			t.Fatal(trx.StatusPembayaran)
		}
		if trx.JumlahDibayar.Round(2).StringFixed(2) != "250000.00" {
			t.Fatal(trx.JumlahDibayar)
		}
		if !trx.SisaHutang.IsZero() {
			t.Fatal(trx.SisaHutang)
		}
	})

	t.Run("sebagian naik ke lunas bila total turun di bawah nilai dibayar", func(t *testing.T) {
		jt := datatypes.Date(tgl("2026-10-04"))
		trx := domain.TransaksiPenjualan{
			StatusPembayaran:  domain.PembayaranSebagian,
			TotalAkhir:        dec("300000"),
			JumlahDibayar:     dec("150000"),
			SisaHutang:        dec("150000"),
			TanggalJatuhTempo: &jt,
		}
		trx.SesuaikanPembayaran(dec("120000"))
		if trx.StatusPembayaran != domain.PembayaranLunas {
			t.Fatal(trx.StatusPembayaran)
		}
		if trx.JumlahDibayar.Round(2).StringFixed(2) != "120000.00" {
			t.Fatal(trx.JumlahDibayar)
		}
		if !trx.SisaHutang.IsZero() || trx.TanggalJatuhTempo != nil {
			t.Fatalf("sisa/jatuh tempo: %s %v", trx.SisaHutang, trx.TanggalJatuhTempo)
		}
	})

	t.Run("hutang mengikuti total baru", func(t *testing.T) {
		trx := domain.TransaksiPenjualan{
			StatusPembayaran: domain.PembayaranHutang,
			TotalAkhir:       dec("100000"),
			JumlahDibayar:    decimal.Zero,
			SisaHutang:       dec("100000"),
		}
		trx.SesuaikanPembayaran(dec("175000"))
		if trx.StatusPembayaran != domain.PembayaranHutang || trx.SisaHutang.String() != "175000" {
			t.Fatalf("%+v", trx)
		}
	})

	t.Run("sebagian tetap sebagian bila masih kurang", func(t *testing.T) {
		trx := domain.TransaksiPenjualan{
			StatusPembayaran: domain.PembayaranSebagian,
			TotalAkhir:       dec("300000"),
			JumlahDibayar:     dec("100000"),
			SisaHutang:        dec("200000"),
		}
		trx.SesuaikanPembayaran(dec("400000"))
		if trx.StatusPembayaran != domain.PembayaranSebagian {
			t.Fatal(trx.StatusPembayaran)
		}
		if trx.JumlahDibayar.String() != "100000" || trx.SisaHutang.String() != "300000" {
			t.Fatalf("dibayar=%s sisa=%s", trx.JumlahDibayar, trx.SisaHutang)
		}
	})
}

func TestAlokasiProporsionalFaktur(t *testing.T) {
	baris := domain.AlokasiProporsionalFaktur(dec("313500"), dec("34485"), []domain.TransaksiDetail{
		{Urutan: 1, TotalAfterDisc: dec("180000")},
		{Urutan: 2, TotalAfterDisc: dec("150000")},
	})

	jumlah := decimal.Zero
	for _, b := range baris {
		jumlah = jumlah.Add(b.TotalFinalBaris)
	}
	if jumlah.Sub(dec("347985")).Abs().GreaterThanOrEqual(dec("0.01")) {
		t.Fatalf("jumlah baris %s != 347985", jumlah)
	}
	if baris[0].TotalFinalBaris.Round(2).StringFixed(2) != "189810.00" {
		t.Fatalf("baris0 %s (sisanya ke baris terakhir)", baris[0].TotalFinalBaris.Round(2).StringFixed(2))
	}
	if baris[1].TotalFinalBaris.Round(2).StringFixed(2) != "158175.00" {
		t.Fatalf("baris1 %s", baris[1].TotalFinalBaris.Round(2).StringFixed(2))
	}

	sumNilai := baris[0].NilaiSetelahGlobal.Add(baris[1].NilaiSetelahGlobal)
	sumPPN := baris[0].PPNBaris.Add(baris[1].PPNBaris)
	if !sumNilai.Equal(dec("313500")) || !sumPPN.Equal(dec("34485")) {
		t.Fatalf("nilai %s ppn %s", sumNilai, sumPPN)
	}
}

func TestAlokasiProporsionalTotalNol(t *testing.T) {
	baris := domain.AlokasiProporsionalFaktur(decimal.Zero, decimal.Zero, []domain.TransaksiDetail{
		{Urutan: 1, TotalAfterDisc: decimal.Zero},
	})
	if baris[0].TotalFinalBaris.Round(2).StringFixed(2) != "0.00" {
		t.Fatal(baris[0].TotalFinalBaris)
	}
	if len(domain.AlokasiProporsionalFaktur(dec("1"), dec("1"), nil)) != 0 {
		t.Fatal("tanpa detail")
	}
}

func TestHitungProvit(t *testing.T) {
	t.Run("HPP dikalikan qty keluar termasuk bonus", func(t *testing.T) {
		hasil := domain.HitungProvit(domain.InputProvit{
			TotalFinalBaris: dec("189809.98"),
			HPPSnapshot:     dec("15893.85"),
			TotalQtyKeluar:  11,
		})
		if hasil.HPPTotal.Round(2).StringFixed(2) != "174832.35" {
			t.Fatal(hasil.HPPTotal.Round(2).StringFixed(2))
		}
		if hasil.Provit.Round(2).StringFixed(2) != "14977.63" {
			t.Fatal(hasil.Provit.Round(2).StringFixed(2))
		}
		if hasil.MarginPersen.Round(2).StringFixed(2) != "7.89" {
			t.Fatal(hasil.MarginPersen.Round(2).StringFixed(2))
		}
	})

	t.Run("memakai qty ditagih melebih-lebihkan laba", func(t *testing.T) {
		benar := domain.HitungProvit(domain.InputProvit{
			TotalFinalBaris: dec("189809.98"), HPPSnapshot: dec("15893.85"), TotalQtyKeluar: 11,
		})
		salah := domain.HitungProvit(domain.InputProvit{
			TotalFinalBaris: dec("189809.98"), HPPSnapshot: dec("15893.85"), TotalQtyKeluar: 10,
		})
		if benar.Provit.Round(2).StringFixed(2) != "14977.63" {
			t.Fatal(benar.Provit)
		}
		if salah.Provit.Round(2).StringFixed(2) != "30871.48" {
			t.Fatal(salah.Provit)
		}
	})

	t.Run("margin nol bila nilai jual nol", func(t *testing.T) {
		h := domain.HitungProvit(domain.InputProvit{TotalFinalBaris: decimal.Zero, HPPSnapshot: dec("1"), TotalQtyKeluar: 1})
		if !h.MarginPersen.IsZero() {
			t.Fatal(h.MarginPersen)
		}
	})
}
