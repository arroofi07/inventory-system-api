package domain

import "github.com/shopspring/decimal"

// InputProvit input perhitungan laba per baris (04 §7.1).
type InputProvit struct {
	TotalFinalBaris decimal.Decimal
	HPPSnapshot     decimal.Decimal
	TotalQtyKeluar  int
}

// HasilProvit hasil laba per baris.
type HasilProvit struct {
	HPPTotal     decimal.Decimal
	Provit       decimal.Decimal
	MarginPersen decimal.Decimal
}

// HitungProvit menghitung laba memakai total_qty_keluar (termasuk bonus), bukan qty ditagih.
func HitungProvit(in InputProvit) HasilProvit {
	hppTotal := in.HPPSnapshot.Mul(decimal.NewFromInt(int64(in.TotalQtyKeluar)))
	provit := in.TotalFinalBaris.Sub(hppTotal)
	margin := decimal.Zero
	if in.TotalFinalBaris.IsPositive() {
		margin = provit.Div(in.TotalFinalBaris).Mul(decimal.NewFromInt(100))
	}
	return HasilProvit{
		HPPTotal:     hppTotal,
		Provit:       provit,
		MarginPersen: margin,
	}
}
