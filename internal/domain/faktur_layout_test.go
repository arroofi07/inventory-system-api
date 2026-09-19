package domain_test

import (
	"testing"

	"app/internal/domain"
)

func TestPilihLayoutFaktur(t *testing.T) {
	kasus := []struct {
		n, half, full, per int
		layout             domain.LayoutFaktur
		halaman            int
	}{
		{0, 10, 30, 30, domain.LayoutFakturHalf, 1},
		{1, 10, 30, 30, domain.LayoutFakturHalf, 1},
		{10, 10, 30, 30, domain.LayoutFakturHalf, 1},
		{11, 10, 30, 30, domain.LayoutFakturFull, 1},
		{30, 10, 30, 30, domain.LayoutFakturFull, 1},
		{31, 10, 30, 30, domain.LayoutFakturPaginated, 2},
		{60, 10, 30, 30, domain.LayoutFakturPaginated, 2},
		{61, 10, 30, 30, domain.LayoutFakturPaginated, 3},
	}
	for _, k := range kasus {
		layout, halaman := domain.PilihLayoutFaktur(k.n, k.half, k.full, k.per)
		if layout != k.layout || halaman != k.halaman {
			t.Fatalf("n=%d got %s/%d want %s/%d", k.n, layout, halaman, k.layout, k.halaman)
		}
	}
}

func TestHitungJumlahBarisCetakFaktur(t *testing.T) {
	n := domain.HitungJumlahBarisCetakFaktur([]domain.TransaksiDetail{
		{QtyPromo: 0},
		{QtyPromo: 2},
		{QtyPromo: 1},
	})
	if n != 5 { // 3 detail + 2 bonus baris
		t.Fatalf("got %d", n)
	}
}
