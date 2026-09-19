package dto

import (
	"fmt"
	"strings"

	"app/internal/pkg/money"
	"github.com/shopspring/decimal"
)

// FormatUang mengembalikan string desimal 2 digit untuk API.
func FormatUang(d decimal.Decimal) string {
	return money.RoundMoney(d).StringFixed(2)
}

// ParseUang mengurai string uang API ("20000.00") ke decimal.
func ParseUang(s string, field string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, fmt.Errorf("%s wajib diisi", field)
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s tidak valid", field)
	}
	return d, nil
}

// ParseUangOpsional kosong → 0.
func ParseUangOpsional(s string, field string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	return ParseUang(s, field)
}
