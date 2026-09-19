package main

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"app/internal/domain"
)

func TestNormalkanChannel(t *testing.T) {
	cases := []struct {
		in   string
		want domain.ChannelOutlet
		ok   bool
	}{
		{"Modern Trade", domain.ChannelModernTrade, true},
		{"  modern   trade  ", domain.ChannelModernTrade, true},
		{"MT Independent", domain.ChannelModernTradeIndependent, true},
		{"gt kosmetik", domain.ChannelGeneralTradeKosmetik, true},
		{"subagen", domain.ChannelSubAgen, true},
		{"unknown channel", "", false},
	}
	for _, tc := range cases {
		got, ok := normalkanChannel(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%q: got (%q,%v) want (%q,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPlaceholderKeNull(t *testing.T) {
	if placeholderKeNull("-") != nil || placeholderKeNull("N/A") != nil || placeholderKeNull("  ") != nil {
		t.Fatal("placeholder harus NULL")
	}
	if placeholderKeNull("10.20.0.1-000") == nil {
		t.Fatal("NPWP nyata tidak boleh NULL")
	}
}

func TestCocokkanPromo(t *testing.T) {
	master := []promoRef{
		{ID: 1, Kode: "BXGY12", Nama: "Beli 12 Gratis 1", Tipe: domain.PromoBuyXGetY},
		{ID: 2, Kode: "DISC10", Nama: "Diskon 10%", Tipe: domain.PromoPercentageDiscount},
	}
	if p, cara := cocokkanPromo(master, "BXGY12"); cara != "kode_persis" || p.ID != 1 {
		t.Fatalf("kode_persis: %v %s", p, cara)
	}
	if p, cara := cocokkanPromo(master, "Diskon 10%"); cara != "nama_persis" || p.ID != 2 {
		t.Fatalf("nama_persis: %v %s", p, cara)
	}
	if p, cara := cocokkanPromo(master, "  beli  12 gratis 1 "); cara != "nama_dinormalkan" || p.ID != 1 {
		t.Fatalf("nama_dinormalkan: %v %s", p, cara)
	}
	if p, cara := cocokkanPromo(master, "tidak ada"); cara != "tidak_ditemukan" || p != nil {
		t.Fatalf("tidak_ditemukan: %v %s", p, cara)
	}
}

func TestBentukDetailDariHeader(t *testing.T) {
	h := sumberTransaksiHeader{
		KodeItem: sql.NullString{String: "SHS001", Valid: true},
		NamaItem: sql.NullString{String: "Shisena", Valid: true},
		Satuan:   sql.NullString{String: "PCS", Valid: true},
		Jumlah:   sql.NullInt64{Int64: 12, Valid: true},
		Harga:    sql.NullString{String: "20000", Valid: true},
		Disc1:    sql.NullString{String: "10", Valid: true},
		PromoQty: [3]sql.NullInt64{{Int64: 1, Valid: true}},
	}
	d, err := bentukDetailDariHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 1 || d[0].Qty != 12 || d[0].QtyPromo != 1 || d[0].TotalQtyKeluar != 13 {
		t.Fatalf("qty: %+v", d[0])
	}
	wantSub := decimal.RequireFromString("240000")
	if !d[0].Subtotal.Equal(wantSub) {
		t.Fatalf("subtotal %s", d[0].Subtotal)
	}
}

func TestBentukDetailDariHeader_TanpaSKU(t *testing.T) {
	_, err := bentukDetailDariHeader(sumberTransaksiHeader{})
	if err == nil {
		t.Fatal("harus gagal tanpa kode_item")
	}
}

func TestSuffixFaktur(t *testing.T) {
	if suffixFaktur("FK-001", 1) != "FK-001" {
		t.Fatal("kali 1 tidak berubah")
	}
	if suffixFaktur("FK-001", 2) != "FK-001-2" {
		t.Fatal("kali 2 harus -2")
	}
}

func TestHitungSelisihTotalAkhir_Inflate(t *testing.T) {
	total := decimal.RequireFromString("100000")
	ppn := decimal.RequireFromString("11")
	okVal := decimal.RequireFromString("111000")
	_, _, meny := hitungSelisihTotalAkhir(total, ppn, okVal)
	if meny {
		t.Fatal("nilai pas tidak inflate")
	}
	gelembung := decimal.RequireFromString("150000")
	_, sel, meny := hitungSelisihTotalAkhir(total, ppn, gelembung)
	if !meny || !sel.IsPositive() {
		t.Fatalf("harus inflate, selisih=%s", sel)
	}
}

func TestPecahRulesBuyXGetY(t *testing.T) {
	b, g, err := pecahRulesBuyXGetY(`{"buy_qty":12,"get_qty":1}`, 0, 0)
	if err != nil || b != 12 || g != 1 {
		t.Fatalf("json: %d %d %v", b, g, err)
	}
	_, _, err = pecahRulesBuyXGetY(`{"buy_qty":12}`, 0, 0)
	if err == nil {
		t.Fatal("rules tidak lengkap harus error")
	}
	b, g, err = pecahRulesBuyXGetY("", 5, 1)
	if err != nil || b != 5 || g != 1 {
		t.Fatalf("kolom: %d %d %v", b, g, err)
	}
}

func TestStatusV9(t *testing.T) {
	if statusV9(decimal.Zero) != "LULUS" {
		t.Fatal("nol harus LULUS")
	}
	if statusV9(decimal.NewFromInt(1)) != "TINJAU" {
		t.Fatal("selisih harus TINJAU, bukan GAGAL")
	}
}

func TestBandingkanNilai(t *testing.T) {
	ok, _ := bandingkanNilai(decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.Zero)
	if !ok {
		t.Fatal("sama harus lulus")
	}
	ok, _ = bandingkanNilai(decimal.RequireFromString("10.00"), decimal.RequireFromString("10.005"), decimal.NewFromFloat(0.01))
	if !ok {
		t.Fatal("dalam toleransi")
	}
}

func TestRingkasKesimpulan_LulusGagal(t *testing.T) {
	s := ringkasKesimpulan([]HasilVerifikasi{
		{Status: "LULUS", WajibLulus: true},
		{Status: "GAGAL", WajibLulus: true},
		{Status: "TINJAU", WajibLulus: false},
	})
	if !strings.Contains(s, "1 wajib GAGAL") || !strings.Contains(s, "1 TINJAU") {
		t.Fatalf("got %q", s)
	}
}

func TestCatatInflateMasukLaporan(t *testing.T) {
	lap := NewLaporan("a", "b")
	lap.CatatInflate(BarisInflate{ID: 9, NoTransaksi: "000001", Selisih: "12.00"})
	if len(lap.DaftarInflate()) != 1 {
		t.Fatal("inflate tidak tercatat")
	}
}

func TestSkipSumber(t *testing.T) {
	lap := NewLaporan("x", "y")
	if !skipSumber(nil, lap, "users") {
		t.Fatal("nil sumber harus skip")
	}
	if skipSumber(&Sumber{}, lap, "users") {
		t.Fatal("sumber ada tidak skip")
	}
}

func TestTanggalDari(t *testing.T) {
	tm := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	got := tanggalDari(sql.NullTime{Time: tm, Valid: true}, time.Time{})
	if !got.Equal(tm) {
		t.Fatal(got)
	}
}
