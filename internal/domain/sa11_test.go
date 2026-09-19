package domain_test

import (
	"testing"
	"time"

	"app/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

func ptrInt(v int) *int          { return &v }
func ptrStr(v string) *string    { return &v }
func tgl(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
func tglDate(s string) datatypes.Date { return datatypes.Date(tgl(s)) }

func TestHitungBonusQty(t *testing.T) {
	t.Run("buy_x_get_y dengan batas penerapan", func(t *testing.T) {
		p := domain.Promo{
			TipePromo:       domain.PromoBuyXGetY,
			BuyQty:          ptrInt(12),
			GetQty:          ptrInt(1),
			MinQty:          12,
			MaxApplications: ptrInt(3),
		}
		kasus := map[int]int{11: 0, 12: 1, 35: 2, 48: 3, 120: 3}
		for qty, want := range kasus {
			if got := p.HitungBonusQty(qty); got != want {
				t.Fatalf("qty %d: got %d want %d", qty, got, want)
			}
			h := domain.HitungBonusPromo(p, qty, decimal.Zero)
			if h.QtyBonus != want {
				t.Fatalf("HitungBonusPromo qty %d: got %d want %d", qty, h.QtyBonus, want)
			}
		}
	})

	t.Run("bonus_qty mengabaikan max_applications", func(t *testing.T) {
		p := domain.Promo{
			TipePromo:       domain.PromoBonusQty,
			BonusQty:        5,
			MinQty:          10,
			MaxApplications: ptrInt(3),
		}
		if p.HitungBonusQty(9) != 0 || p.HitungBonusQty(10) != 5 || p.HitungBonusQty(1000) != 5 {
			t.Fatal("bonus_qty salah")
		}
	})

	t.Run("fixed_discount tidak melebihi nilai barang", func(t *testing.T) {
		p := domain.Promo{
			TipePromo:      domain.PromoFixedDiscount,
			DiscountAmount: dec("50000"),
			MinQty:         1,
		}
		if p.HitungDiskon(dec("30000")).String() != "30000" {
			t.Fatalf("got %s", p.HitungDiskon(dec("30000")))
		}
		if p.HitungDiskon(dec("80000")).String() != "50000" {
			t.Fatalf("got %s", p.HitungDiskon(dec("80000")))
		}
	})

	t.Run("percentage_discount", func(t *testing.T) {
		p := domain.Promo{
			TipePromo:          domain.PromoPercentageDiscount,
			DiscountPercentage: dec("10"),
			MinQty:             1,
		}
		got := domain.HitungBonusPromo(p, 5, dec("200000")).NilaiDiskon
		if got.String() != "20000" {
			t.Fatalf("got %s", got)
		}
	})
}

func TestPromoBerlakuUntuk(t *testing.T) {
	p := domain.Promo{
		IsActive:        true,
		TanggalMulai:    tglDate("2026-09-01"),
		TanggalBerakhir: tglDate("2026-09-30"),
		KodeBarang:      ptrStr("SHS001"),
		MinQty:          10,
		MinAmount:       dec("100000"),
	}

	t.Run("periode dinilai dari tanggal transaksi", func(t *testing.T) {
		if p.BerlakuUntuk("SHS001", 10, dec("200000"), tgl("2026-08-31")) {
			t.Fatal("sebelum mulai")
		}
		if !p.BerlakuUntuk("SHS001", 10, dec("200000"), tgl("2026-09-01")) {
			t.Fatal("hari mulai")
		}
		if !p.BerlakuUntuk("SHS001", 10, dec("200000"), tgl("2026-09-30")) {
			t.Fatal("hari akhir")
		}
		if p.BerlakuUntuk("SHS001", 10, dec("200000"), tgl("2026-10-01")) {
			t.Fatal("setelah akhir")
		}
	})

	t.Run("SKU lain tidak mendapat promo", func(t *testing.T) {
		if p.BerlakuUntuk("MSK014", 10, dec("200000"), tgl("2026-09-15")) {
			t.Fatal("SKU lain")
		}
	})

	t.Run("syarat minimum", func(t *testing.T) {
		if p.BerlakuUntuk("SHS001", 9, dec("200000"), tgl("2026-09-15")) {
			t.Fatal("qty kurang")
		}
		if p.BerlakuUntuk("SHS001", 10, dec("99999"), tgl("2026-09-15")) {
			t.Fatal("amount kurang")
		}
	})
}

func TestAlokasiBatchFEFO(t *testing.T) {
	hari := tgl("2026-03-01")
	batches := []domain.BatchKandidat{
		{BarangMasukID: 1, NoBatch: "B-001", Exp: tgl("2026-06-30"), TanggalMasuk: tgl("2025-11-05"), QtyTersedia: 15, HargaMT: dec("100"), HargaGT: dec("90"), HPPDenganPPN: dec("50")},
		{BarangMasukID: 2, NoBatch: "B-002", Exp: tgl("2027-01-31"), TanggalMasuk: tgl("2025-12-01"), QtyTersedia: 50, HargaMT: dec("100"), HargaGT: dec("90"), HPPDenganPPN: dec("50")},
		{BarangMasukID: 3, NoBatch: "B-003", Exp: tgl("2026-03-31"), TanggalMasuk: tgl("2026-01-10"), QtyTersedia: 20, HargaMT: dec("100"), HargaGT: dec("90"), HPPDenganPPN: dec("50")},
		{BarangMasukID: 4, NoBatch: "B-004", Exp: tgl("2026-09-30"), TanggalMasuk: tgl("2026-02-20"), QtyTersedia: 30, HargaMT: dec("100"), HargaGT: dec("90"), HPPDenganPPN: dec("50")},
	}

	got, err := domain.AlokasiBatch(batches, 45, domain.AlokasiFEFO, domain.ChannelModernTrade, hari)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len %d", len(got))
	}
	want := []struct {
		id  uint64
		qty int
	}{{3, 20}, {1, 15}, {4, 10}}
	for i, w := range want {
		if got[i].BarangMasukID != w.id || got[i].Qty != w.qty {
			t.Fatalf("i=%d got id=%d qty=%d want id=%d qty=%d", i, got[i].BarangMasukID, got[i].Qty, w.id, w.qty)
		}
		if got[i].Harga.String() != "100" {
			t.Fatalf("harga MT %s", got[i].Harga)
		}
	}
}

func TestAlokasiBatchFIFO(t *testing.T) {
	hari := tgl("2026-03-01")
	batches := []domain.BatchKandidat{
		{BarangMasukID: 1, NoBatch: "B-001", Exp: tgl("2026-06-30"), TanggalMasuk: tgl("2025-11-05"), QtyTersedia: 15, HargaGT: dec("90")},
		{BarangMasukID: 3, NoBatch: "B-003", Exp: tgl("2026-03-31"), TanggalMasuk: tgl("2026-01-10"), QtyTersedia: 20, HargaGT: dec("90")},
	}
	got, err := domain.AlokasiBatch(batches, 20, domain.AlokasiFIFO, domain.ChannelGeneralTrade, hari)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].BarangMasukID != 1 || got[0].Qty != 15 || got[1].BarangMasukID != 3 || got[1].Qty != 5 {
		t.Fatalf("%+v", got)
	}
}

func TestAlokasiBatchStokKurang(t *testing.T) {
	_, err := domain.AlokasiBatch([]domain.BatchKandidat{
		{BarangMasukID: 1, Exp: tgl("2027-01-01"), TanggalMasuk: tgl("2026-01-01"), QtyTersedia: 5},
	}, 10, domain.AlokasiFEFO, domain.ChannelGeneralTrade, tgl("2026-03-01"))
	if err != domain.ErrStokTidakCukup {
		t.Fatalf("err %v", err)
	}
}

func TestAlokasiBatchAbaikanKedaluwarsa(t *testing.T) {
	_, err := domain.AlokasiBatch([]domain.BatchKandidat{
		{BarangMasukID: 1, Exp: tgl("2026-02-01"), TanggalMasuk: tgl("2025-01-01"), QtyTersedia: 100},
		{BarangMasukID: 2, Exp: tgl("2027-01-01"), TanggalMasuk: tgl("2026-01-01"), QtyTersedia: 5},
	}, 10, domain.AlokasiFEFO, domain.ChannelGeneralTrade, tgl("2026-03-01"))
	if err != domain.ErrStokTidakCukup {
		t.Fatalf("batch kedaluwarsa tidak boleh dipakai: %v", err)
	}
}

func TestAlokasiBatchEdge(t *testing.T) {
	got, err := domain.AlokasiBatch(nil, 0, domain.AlokasiFEFO, domain.ChannelGeneralTrade, tgl("2026-03-01"))
	if err != nil || got != nil {
		t.Fatalf("qty 0: %v %v", got, err)
	}
	// metode tidak valid → default FEFO
	batches := []domain.BatchKandidat{
		{BarangMasukID: 9, Exp: tgl("2026-04-01"), TanggalMasuk: tgl("2026-01-01"), QtyTersedia: 3, HargaGT: dec("1")},
	}
	got, err = domain.AlokasiBatch(batches, 3, domain.MetodeAlokasi("X"), domain.ChannelGeneralTrade, tgl("2026-03-01"))
	if err != nil || len(got) != 1 || got[0].Qty != 3 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPromoEdgeCases(t *testing.T) {
	if domain.HitungBonusPromo(domain.Promo{TipePromo: "x"}, 10, dec("1")).QtyBonus != 0 {
		t.Fatal("tipe tidak dikenal")
	}
	p := domain.Promo{TipePromo: domain.PromoBuyXGetY, MinQty: 1} // buy/get nil
	if p.HitungBonusQty(100) != 0 {
		t.Fatal("buy/get kosong")
	}
	if (domain.Promo{TipePromo: domain.PromoPercentageDiscount, MinQty: 1}).HitungBonusQty(5) != 0 {
		t.Fatal("bukan tipe bonus")
	}
	disc := domain.Promo{TipePromo: domain.PromoFixedDiscount, DiscountAmount: dec("10"), MinQty: 5}
	if !domain.HitungBonusPromo(disc, 1, dec("100")).NilaiDiskon.IsZero() {
		t.Fatal("qty di bawah min untuk diskon")
	}
	inactive := domain.Promo{
		IsActive: false, TanggalMulai: tglDate("2026-01-01"), TanggalBerakhir: tglDate("2026-12-31"), MinQty: 1,
	}
	if inactive.BerlakuUntuk("A", 1, dec("1"), tgl("2026-06-01")) {
		t.Fatal("nonaktif")
	}
	pct := domain.Promo{TipePromo: domain.PromoPercentageDiscount, DiscountPercentage: dec("10"), MinAmount: dec("50000")}
	if !pct.HitungDiskon(dec("1000")).IsZero() {
		t.Fatal("di bawah min_amount")
	}
	if (domain.Promo{TipePromo: domain.PromoBuyXGetY}).HitungDiskon(dec("100")).String() != "0" {
		t.Fatal("bukan tipe diskon")
	}
}

func TestSesuaikanPembayaranStatusTakDikenal(t *testing.T) {
	trx := domain.TransaksiPenjualan{
		StatusPembayaran: domain.StatusPembayaran("???"),
		JumlahDibayar:    dec("50"),
	}
	trx.SesuaikanPembayaran(dec("100"))
	if trx.StatusPembayaran != domain.PembayaranSebagian || trx.SisaHutang.String() != "50" {
		t.Fatalf("%+v", trx)
	}
}

func TestAlokasiBatchSkipSaldoNol(t *testing.T) {
	got, err := domain.AlokasiBatch([]domain.BatchKandidat{
		{BarangMasukID: 1, Exp: tgl("2027-01-01"), TanggalMasuk: tgl("2026-01-01"), QtyTersedia: 0},
		{BarangMasukID: 2, Exp: tgl("2027-06-01"), TanggalMasuk: tgl("2026-02-01"), QtyTersedia: 7, HargaGT: dec("1")},
	}, 7, domain.AlokasiFEFO, domain.ChannelGeneralTrade, tgl("2026-03-01"))
	if err != nil || len(got) != 1 || got[0].BarangMasukID != 2 {
		t.Fatalf("%v %v", got, err)
	}
}
