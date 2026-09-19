package terbilang_test

import (
	"fmt"
	"testing"

	"app/internal/pkg/terbilang"
	"github.com/shopspring/decimal"
)

func TestTerbilang(t *testing.T) {
	kasus := map[int64]string{
		0:          "nol rupiah",
		1:          "satu rupiah",
		11:         "sebelas rupiah",
		15:         "lima belas rupiah",
		19:         "sembilan belas rupiah",
		20:         "dua puluh rupiah",
		21:         "dua puluh satu rupiah",
		100:        "seratus rupiah",
		101:        "seratus satu rupiah",
		200:        "dua ratus rupiah",
		1000:       "seribu rupiah",
		1001:       "seribu satu rupiah",
		1100:       "seribu seratus rupiah",
		2000:       "dua ribu rupiah",
		11000:      "sebelas ribu rupiah",
		100000:     "seratus ribu rupiah",
		1000000:    "satu juta rupiah",
		1500000:    "satu juta lima ratus ribu rupiah",
		347985:     "tiga ratus empat puluh tujuh ribu sembilan ratus delapan puluh lima rupiah",
		1000000000: "satu miliar rupiah",
	}
	for angka, want := range kasus {
		t.Run(fmt.Sprint(angka), func(t *testing.T) {
			got := terbilang.Rupiah(decimal.NewFromInt(angka))
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestTerbilangMembulatkanSen(t *testing.T) {
	if terbilang.Rupiah(dec("99.60")) != "seratus rupiah" {
		t.Fatal(terbilang.Rupiah(dec("99.60")))
	}
	if terbilang.Rupiah(dec("99.40")) != "sembilan puluh sembilan rupiah" {
		t.Fatal(terbilang.Rupiah(dec("99.40")))
	}
}

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}
