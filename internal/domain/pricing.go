package domain

import (
	"app/internal/pkg/money"
	"github.com/shopspring/decimal"
)

// HitungHPP menghitung harga pokok dengan diskon dagang berantai.
// Setiap tingkat diterapkan pada hasil tingkat sebelumnya (bukan aditif).
func HitungHPP(harga, disc1, disc2, disc3 decimal.Decimal) decimal.Decimal {
	hasil := harga
	for _, disc := range []decimal.Decimal{disc1, disc2, disc3} {
		hasil = money.TerapkanDiskon(hasil, disc)
	}
	return hasil
}

// HitungHPPDenganPPN menambah PPN pada HPP: hpp * (1 + ppnPersen/100).
func HitungHPPDenganPPN(hpp, ppnPersen decimal.Decimal) decimal.Decimal {
	faktor := decimal.NewFromInt(1).Add(ppnPersen.Div(decimal.NewFromInt(100)))
	return hpp.Mul(faktor)
}

// HitungHargaChannel menghitung harga jual dari harga list principal + markup.
// Markup dihitung dari harga (bukan dari HPP).
func HitungHargaChannel(harga, markupAmount decimal.Decimal, tipe MarkupType) decimal.Decimal {
	switch tipe {
	case MarkupPercent:
		return harga.Add(harga.Mul(markupAmount).Div(decimal.NewFromInt(100)))
	case MarkupValue:
		return harga.Add(markupAmount)
	default:
		return harga
	}
}

// PilihHarga memilih harga_mt atau harga_gt menurut channel pelanggan.
func PilihHarga(hargaMT, hargaGT decimal.Decimal, channel ChannelOutlet) decimal.Decimal {
	if channel.IsModernTrade() {
		return hargaMT
	}
	return hargaGT
}

// HargaUntukChannel memilih harga batch menurut channel (alias PilihHarga).
func HargaUntukChannel(batch BarangMasuk, channel ChannelOutlet) decimal.Decimal {
	return PilihHarga(batch.HargaMT, batch.HargaGT, channel)
}

// DiskonBerjenjang menerapkan tiga tingkat diskon persen berantai pada nilai.
func DiskonBerjenjang(nilai, disc1, disc2, disc3 decimal.Decimal) decimal.Decimal {
	hasil := nilai
	for _, disc := range []decimal.Decimal{disc1, disc2, disc3} {
		hasil = money.TerapkanDiskon(hasil, disc)
	}
	return hasil
}
