package domain

import (
	"app/internal/pkg/money"
	"github.com/shopspring/decimal"
)

// BarisFaktur nilai final per baris setelah alokasi proporsional.
type BarisFaktur struct {
	Urutan             uint16
	TotalAfterDisc     decimal.Decimal
	NilaiSetelahGlobal decimal.Decimal
	PPNBaris           decimal.Decimal
	TotalFinalBaris    decimal.Decimal
}

// AlokasiProporsionalFaktur membagi DPP setelah global + PPN ke baris
// proporsional terhadap total_after_disc. Selisih pembulatan ke baris terakhir (04 §6.4).
func AlokasiProporsionalFaktur(
	totalSetelahGlobal decimal.Decimal,
	ppnNominal decimal.Decimal,
	details []TransaksiDetail,
) []BarisFaktur {
	n := len(details)
	out := make([]BarisFaktur, n)
	if n == 0 {
		return out
	}

	sumAfter := decimal.Zero
	for _, d := range details {
		sumAfter = sumAfter.Add(d.TotalAfterDisc)
	}

	totalAkhir := totalSetelahGlobal.Add(ppnNominal)
	accNilai := decimal.Zero
	accPPN := decimal.Zero
	accFinal := decimal.Zero

	for i, d := range details {
		out[i].Urutan = d.Urutan
		out[i].TotalAfterDisc = d.TotalAfterDisc

		if sumAfter.IsZero() {
			out[i].NilaiSetelahGlobal = decimal.Zero
			out[i].PPNBaris = decimal.Zero
			out[i].TotalFinalBaris = decimal.Zero
			continue
		}

		if i < n-1 {
			proporsi := d.TotalAfterDisc.Div(sumAfter)
			nilai := money.RoundMoney(totalSetelahGlobal.Mul(proporsi))
			ppn := money.RoundMoney(ppnNominal.Mul(proporsi))
			final := money.RoundMoney(nilai.Add(ppn))
			out[i].NilaiSetelahGlobal = nilai
			out[i].PPNBaris = ppn
			out[i].TotalFinalBaris = final
			accNilai = accNilai.Add(nilai)
			accPPN = accPPN.Add(ppn)
			accFinal = accFinal.Add(final)
			continue
		}

		// Baris terakhir menanggung sisa agar sum exact.
		out[i].NilaiSetelahGlobal = money.RoundMoney(totalSetelahGlobal.Sub(accNilai))
		out[i].PPNBaris = money.RoundMoney(ppnNominal.Sub(accPPN))
		out[i].TotalFinalBaris = money.RoundMoney(totalAkhir.Sub(accFinal))
	}

	return out
}
