package terbilang

import (
	"strings"

	"github.com/shopspring/decimal"
)

var satuan = []string{
	"", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan",
	"sepuluh", "sebelas", "dua belas", "tiga belas", "empat belas", "lima belas",
	"enam belas", "tujuh belas", "delapan belas", "sembilan belas",
}

// Rupiah mengubah nilai uang ke terbilang Indonesia, dibulatkan ke rupiah terdekat.
func Rupiah(nilai decimal.Decimal) string {
	n := nilai.Round(0).IntPart()
	if n < 0 {
		n = -n
	}
	kata := angka(n)
	if kata == "" {
		kata = "nol"
	}
	return kata + " rupiah"
}

func angka(n int64) string {
	if n < 0 {
		return ""
	}
	if n < 20 {
		return satuan[n]
	}
	if n < 100 {
		puluh := n / 10
		sisa := n % 10
		if sisa == 0 {
			return satuan[puluh] + " puluh"
		}
		return satuan[puluh] + " puluh " + satuan[sisa]
	}
	if n < 200 {
		sisa := n % 100
		if sisa == 0 {
			return "seratus"
		}
		return "seratus " + angka(sisa)
	}
	if n < 1000 {
		ratus := n / 100
		sisa := n % 100
		if sisa == 0 {
			return satuan[ratus] + " ratus"
		}
		return satuan[ratus] + " ratus " + angka(sisa)
	}
	if n < 2000 {
		sisa := n % 1000
		if sisa == 0 {
			return "seribu"
		}
		return "seribu " + angka(sisa)
	}
	if n < 1_000_000 {
		ribu := n / 1000
		sisa := n % 1000
		prefix := angka(ribu) + " ribu"
		if sisa == 0 {
			return prefix
		}
		return prefix + " " + angka(sisa)
	}
	if n < 1_000_000_000 {
		juta := n / 1_000_000
		sisa := n % 1_000_000
		prefix := angka(juta) + " juta"
		if sisa == 0 {
			return prefix
		}
		return prefix + " " + angka(sisa)
	}
	if n < 1_000_000_000_000 {
		miliar := n / 1_000_000_000
		sisa := n % 1_000_000_000
		prefix := angka(miliar) + " miliar"
		if sisa == 0 {
			return prefix
		}
		return prefix + " " + angka(sisa)
	}
	triliun := n / 1_000_000_000_000
	sisa := n % 1_000_000_000_000
	prefix := angka(triliun) + " triliun"
	if sisa == 0 {
		return prefix
	}
	return strings.TrimSpace(prefix + " " + angka(sisa))
}
