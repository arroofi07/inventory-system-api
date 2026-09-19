package domain

import (
	"app/internal/pkg/money"
	"github.com/shopspring/decimal"
)

// ItemInput satu baris untuk perhitungan transaksi.
type ItemInput struct {
	Qty         int
	Harga       decimal.Decimal
	Disc1       decimal.Decimal
	Disc2       decimal.Decimal
	Disc3       decimal.Decimal
	DiskonPromo decimal.Decimal // nilai nominal diskon promo per baris
}

// TransaksiInput input kalkulasi header + detail.
type TransaksiInput struct {
	Items       []ItemInput
	Disc1Global decimal.Decimal
	Disc2Global decimal.Decimal
	Disc3Global decimal.Decimal
	PPNPersen   decimal.Decimal
}

// ItemHasil hasil per baris (presisi penuh; Round saat persist).
type ItemHasil struct {
	Subtotal           decimal.Decimal
	TotalAfterDisc     decimal.Decimal
	NilaiSetelahPromo  decimal.Decimal
}

// TransaksiHasil hasil agregat transaksi.
type TransaksiHasil struct {
	Items       []ItemHasil
	GrandTotal  decimal.Decimal // SUM nilai setelah promo, sebelum diskon global
	Total       decimal.Decimal // DPP setelah diskon global
	PPNNominal  decimal.Decimal
	TotalAkhir  decimal.Decimal
}

// HitungTotalTransaksi menerapkan urutan: baris (qty×harga → disc 1–3 → promo)
// → jumlah → disc global 1–3 → DPP → PPN → total_akhir.
// Perhitungan antara memakai presisi penuh; pembulatan 2 desimal saat penyimpanan.
func HitungTotalTransaksi(in TransaksiInput) TransaksiHasil {
	hasil := TransaksiHasil{
		Items:      make([]ItemHasil, 0, len(in.Items)),
		GrandTotal: decimal.Zero,
	}

	for _, it := range in.Items {
		qty := decimal.NewFromInt(int64(it.Qty))
		subtotal := qty.Mul(it.Harga)
		afterDisc := DiskonBerjenjang(subtotal, it.Disc1, it.Disc2, it.Disc3)
		setelahPromo := afterDisc.Sub(it.DiskonPromo)
		if setelahPromo.IsNegative() {
			setelahPromo = decimal.Zero
		}

		hasil.Items = append(hasil.Items, ItemHasil{
			Subtotal:          subtotal,
			TotalAfterDisc:    afterDisc,
			NilaiSetelahPromo: setelahPromo,
		})
		hasil.GrandTotal = hasil.GrandTotal.Add(setelahPromo)
	}

	setelahGlobal := DiskonBerjenjang(hasil.GrandTotal, in.Disc1Global, in.Disc2Global, in.Disc3Global)
	hasil.Total = setelahGlobal
	hasil.PPNNominal = setelahGlobal.Mul(in.PPNPersen).Div(decimal.NewFromInt(100))
	hasil.TotalAkhir = setelahGlobal.Add(hasil.PPNNominal)
	return hasil
}

// BulatkanHasilTransaksi membulatkan field tersimpan ke 2 desimal (half-up).
func BulatkanHasilTransaksi(h TransaksiHasil) TransaksiHasil {
	out := TransaksiHasil{
		Items:      make([]ItemHasil, len(h.Items)),
		GrandTotal: money.RoundMoney(h.GrandTotal),
		Total:      money.RoundMoney(h.Total),
		PPNNominal: money.RoundMoney(h.PPNNominal),
		TotalAkhir: money.RoundMoney(h.TotalAkhir),
	}
	for i, it := range h.Items {
		out.Items[i] = ItemHasil{
			Subtotal:          money.RoundMoney(it.Subtotal),
			TotalAfterDisc:    money.RoundMoney(it.TotalAfterDisc),
			NilaiSetelahPromo: money.RoundMoney(it.NilaiSetelahPromo),
		}
	}
	// Jaga konsistensi total_akhir = total + ppn setelah pembulatan (toleransi diuji terpisah).
	out.TotalAkhir = money.RoundMoney(out.Total.Add(out.PPNNominal))
	return out
}
