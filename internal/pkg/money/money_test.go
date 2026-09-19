package money_test

import (
	"testing"

	"app/internal/pkg/money"
	"github.com/shopspring/decimal"
)

func TestTerapkanDiskon(t *testing.T) {
	got := money.TerapkanDiskon(decimal.RequireFromString("20000"), decimal.RequireFromString("23.1"))
	if got.Round(2).StringFixed(2) != "15380.00" {
		t.Fatalf("got %s", got.Round(2).StringFixed(2))
	}
	sama := money.TerapkanDiskon(decimal.RequireFromString("100"), decimal.Zero)
	if !sama.Equal(decimal.RequireFromString("100")) {
		t.Fatal("disc 0 harus identitas")
	}
}
