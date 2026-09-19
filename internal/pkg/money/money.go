package money

import "github.com/shopspring/decimal"

var (
	seratus = decimal.NewFromInt(100)
	satu    = decimal.NewFromInt(1)
)

// TerapkanDiskon menerapkan diskon persen berantai: nilai * (1 - disc/100).
func TerapkanDiskon(nilai, discPersen decimal.Decimal) decimal.Decimal {
	if discPersen.IsZero() {
		return nilai
	}
	faktor := satu.Sub(discPersen.Div(seratus))
	return nilai.Mul(faktor)
}

// RoundMoney membulatkan half-up ke 2 desimal (nilai tersimpan).
func RoundMoney(d decimal.Decimal) decimal.Decimal {
	return d.Round(2)
}
