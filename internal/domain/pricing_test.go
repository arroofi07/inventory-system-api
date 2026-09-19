package domain_test

import (
	"fmt"
	"testing"
	"time"

	"app/internal/domain"
	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestHitungHPP(t *testing.T) {
	kasus := []struct {
		nama                string
		harga               string
		disc1, disc2, disc3 string
		hppDiharapkan       string
		hppPPNDiharapkan    string
	}{
		{
			nama:             "tiga tingkat diskon",
			harga:            "20000",
			disc1:            "23.1",
			disc2:            "2",
			disc3:            "5",
			hppDiharapkan:    "14318.78",
			hppPPNDiharapkan: "15893.85",
		},
		{
			nama:             "satu tingkat diskon",
			harga:            "4000",
			disc1:            "5",
			disc2:            "0",
			disc3:            "0",
			hppDiharapkan:    "3800.00",
			hppPPNDiharapkan: "4218.00",
		},
		{
			nama:             "tanpa diskon",
			harga:            "10000",
			disc1:            "0",
			disc2:            "0",
			disc3:            "0",
			hppDiharapkan:    "10000.00",
			hppPPNDiharapkan: "11100.00",
		},
		{
			nama:             "diskon 100 persen menghasilkan nol",
			harga:            "10000",
			disc1:            "100",
			disc2:            "0",
			disc3:            "0",
			hppDiharapkan:    "0.00",
			hppPPNDiharapkan: "0.00",
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			hpp := domain.HitungHPP(dec(k.harga), dec(k.disc1), dec(k.disc2), dec(k.disc3))
			if got := hpp.Round(2).StringFixed(2); got != k.hppDiharapkan {
				t.Fatalf("hpp: got %s want %s", got, k.hppDiharapkan)
			}
			hppPPN := domain.HitungHPPDenganPPN(hpp, dec("11"))
			if got := hppPPN.Round(2).StringFixed(2); got != k.hppPPNDiharapkan {
				t.Fatalf("hpp+ppn: got %s want %s", got, k.hppPPNDiharapkan)
			}
		})
	}
}

func TestHPPBukanAditif(t *testing.T) {
	berantai := domain.HitungHPP(dec("20000"), dec("23.1"), dec("2"), dec("5"))
	aditif := dec("20000").Mul(dec("1").Sub(dec("30.1").Div(dec("100"))))

	if berantai.Round(2).StringFixed(2) != "14318.78" {
		t.Fatalf("berantai %s", berantai.Round(2).StringFixed(2))
	}
	if aditif.Round(2).StringFixed(2) != "13980.00" {
		t.Fatalf("aditif %s", aditif.Round(2).StringFixed(2))
	}
	if berantai.Round(2).Equal(aditif.Round(2)) {
		t.Fatal("HPP wajib berantai, bukan menjumlahkan persentase")
	}
}

func TestHitungHargaChannel(t *testing.T) {
	harga := dec("20000")
	mt := domain.HitungHargaChannel(harga, dec("15"), domain.MarkupPercent)
	gt := domain.HitungHargaChannel(harga, dec("2500"), domain.MarkupValue)
	fallback := domain.HitungHargaChannel(harga, dec("99"), domain.MarkupType("lain"))

	if mt.Round(2).StringFixed(2) != "23000.00" {
		t.Fatalf("MT got %s", mt.Round(2).StringFixed(2))
	}
	if gt.Round(2).StringFixed(2) != "22500.00" {
		t.Fatalf("GT got %s", gt.Round(2).StringFixed(2))
	}
	if !fallback.Equal(harga) {
		t.Fatalf("tipe markup tidak dikenal harus mengembalikan harga: got %s", fallback)
	}
}

func TestHitungAgingMonth(t *testing.T) {
	masuk := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC)
	if got := domain.HitungAgingMonth(masuk, exp); got != 9 {
		t.Fatalf("aging want 9 got %d", got)
	}
	if got := domain.HitungAgingMonth(masuk, masuk); got != 0 {
		t.Fatalf("exp sama want 0 got %d", got)
	}
}

func TestPilihHarga(t *testing.T) {
	batch := domain.BarangMasuk{HargaMT: dec("23000"), HargaGT: dec("22500")}
	kasus := []struct {
		channel    domain.ChannelOutlet
		diharapkan string
	}{
		{domain.ChannelModernTrade, "23000"},
		{domain.ChannelModernTradeIndependent, "23000"},
		{domain.ChannelGeneralTrade, "22500"},
		{domain.ChannelGeneralTradeKosmetik, "22500"},
		{domain.ChannelSubAgen, "22500"},
		{domain.ChannelOutlet(""), "22500"},
		{domain.ChannelOutlet("MODERN TRADE"), "23000"},
		{domain.ChannelOutlet("  modern trade  "), "23000"},
	}

	for _, k := range kasus {
		t.Run(fmt.Sprintf("%q", k.channel), func(t *testing.T) {
			got := domain.HargaUntukChannel(batch, k.channel).String()
			if got != k.diharapkan {
				t.Fatalf("got %s want %s", got, k.diharapkan)
			}
			got2 := domain.PilihHarga(batch.HargaMT, batch.HargaGT, k.channel).String()
			if got2 != k.diharapkan {
				t.Fatalf("PilihHarga got %s want %s", got2, k.diharapkan)
			}
		})
	}
}

func TestDiskonBerjenjang(t *testing.T) {
	// qty=2, harga=100000 → subtotal 200000; disc 10,5,2 → 167580
	got := domain.DiskonBerjenjang(dec("200000"), dec("10"), dec("5"), dec("2"))
	if got.Round(2).StringFixed(2) != "167580.00" {
		t.Fatalf("got %s", got.Round(2).StringFixed(2))
	}
}

func TestHitungTotalTransaksi(t *testing.T) {
	t.Run("satu baris tiga tingkat diskon", func(t *testing.T) {
		hasil := domain.HitungTotalTransaksi(domain.TransaksiInput{
			Items: []domain.ItemInput{{
				Qty: 2, Harga: dec("100000"),
				Disc1: dec("10"), Disc2: dec("5"), Disc3: dec("2"),
			}},
			PPNPersen: dec("11"),
		})
		if hasil.Items[0].Subtotal.Round(2).StringFixed(2) != "200000.00" {
			t.Fatalf("subtotal %s", hasil.Items[0].Subtotal.Round(2).StringFixed(2))
		}
		if hasil.Items[0].TotalAfterDisc.Round(2).StringFixed(2) != "167580.00" {
			t.Fatalf("after disc %s", hasil.Items[0].TotalAfterDisc.Round(2).StringFixed(2))
		}
	})

	t.Run("PPN di atas DPP", func(t *testing.T) {
		hasil := domain.HitungTotalTransaksi(domain.TransaksiInput{
			Items:     []domain.ItemInput{{Qty: 5, Harga: dec("50000"), Disc1: dec("10")}},
			PPNPersen: dec("11"),
		})
		if hasil.Total.Round(2).StringFixed(2) != "225000.00" {
			t.Fatalf("total %s", hasil.Total.Round(2).StringFixed(2))
		}
		if hasil.PPNNominal.Round(2).StringFixed(2) != "24750.00" {
			t.Fatalf("ppn %s", hasil.PPNNominal.Round(2).StringFixed(2))
		}
		if hasil.TotalAkhir.Round(2).StringFixed(2) != "249750.00" {
			t.Fatalf("akhir %s", hasil.TotalAkhir.Round(2).StringFixed(2))
		}
	})

	t.Run("multi-item dengan diskon global", func(t *testing.T) {
		hasil := domain.HitungTotalTransaksi(domain.TransaksiInput{
			Items: []domain.ItemInput{
				{Qty: 10, Harga: dec("20000"), Disc1: dec("10")},
				{Qty: 5, Harga: dec("30000")},
			},
			Disc1Global: dec("5"),
			PPNPersen:   dec("11"),
		})
		if hasil.GrandTotal.Round(2).StringFixed(2) != "330000.00" {
			t.Fatalf("grand %s", hasil.GrandTotal.Round(2).StringFixed(2))
		}
		if hasil.Total.Round(2).StringFixed(2) != "313500.00" {
			t.Fatalf("total %s", hasil.Total.Round(2).StringFixed(2))
		}
		if hasil.PPNNominal.Round(2).StringFixed(2) != "34485.00" {
			t.Fatalf("ppn %s", hasil.PPNNominal.Round(2).StringFixed(2))
		}
		if hasil.TotalAkhir.Round(2).StringFixed(2) != "347985.00" {
			t.Fatalf("akhir %s", hasil.TotalAkhir.Round(2).StringFixed(2))
		}
	})

	t.Run("promo melebihi nilai baris diklem ke nol", func(t *testing.T) {
		hasil := domain.HitungTotalTransaksi(domain.TransaksiInput{
			Items: []domain.ItemInput{{
				Qty: 1, Harga: dec("10000"), DiskonPromo: dec("50000"),
			}},
			PPNPersen: dec("11"),
		})
		if !hasil.Items[0].NilaiSetelahPromo.IsZero() {
			t.Fatalf("nilai setelah promo %s", hasil.Items[0].NilaiSetelahPromo)
		}
		if !hasil.GrandTotal.IsZero() || !hasil.TotalAkhir.IsZero() {
			t.Fatalf("grand/akhir harus nol: %s / %s", hasil.GrandTotal, hasil.TotalAkhir)
		}
	})

	t.Run("tanpa item", func(t *testing.T) {
		hasil := domain.HitungTotalTransaksi(domain.TransaksiInput{PPNPersen: dec("11")})
		if len(hasil.Items) != 0 || !hasil.TotalAkhir.IsZero() {
			t.Fatalf("tanpa item: %+v", hasil)
		}
	})

	t.Run("invariant total_akhir sama dengan total plus ppn", func(t *testing.T) {
		for i := 0; i < 500; i++ {
			hasil := domain.HitungTotalTransaksi(inputAcak(i))
			bulat := domain.BulatkanHasilTransaksi(hasil)
			selisih := bulat.TotalAkhir.Sub(bulat.Total.Add(bulat.PPNNominal)).Abs()
			if !selisih.LessThan(dec("0.01")) && !selisih.Equal(dec("0")) {
				// Setelah BulatkanHasilTransaksi, TotalAkhir diset = Total+PPN sehingga selisih 0.
				t.Fatalf("iterasi %d: selisih %s", i, selisih)
			}
			// Juga uji hasil mentah sebelum bulat (toleransi 0.01).
			mentah := hasil.TotalAkhir.Sub(hasil.Total.Add(hasil.PPNNominal)).Abs()
			if mentah.GreaterThanOrEqual(dec("0.01")) {
				t.Fatalf("iterasi %d mentah: selisih %s", i, mentah)
			}
		}
	})
}

// inputAcak menghasilkan kombinasi deterministik untuk uji invariant.
func inputAcak(seed int) domain.TransaksiInput {
	r := uint64(seed)*1103515245 + 12345
	next := func(mod int) int {
		r = r*1103515245 + 12345
		return int(r%uint64(mod)) + 1
	}
	n := next(3)
	items := make([]domain.ItemInput, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, domain.ItemInput{
			Qty:   next(20),
			Harga: decimal.NewFromInt(int64(next(500) * 100)),
			Disc1: decimal.NewFromInt(int64(next(15) - 1)),
			Disc2: decimal.NewFromInt(int64(next(10) - 1)),
			Disc3: decimal.NewFromInt(int64(next(5) - 1)),
		})
	}
	return domain.TransaksiInput{
		Items:       items,
		Disc1Global: decimal.NewFromInt(int64(next(8) - 1)),
		Disc2Global: decimal.NewFromInt(int64(next(5) - 1)),
		Disc3Global: decimal.Zero,
		PPNPersen:   dec("11"),
	}
}
