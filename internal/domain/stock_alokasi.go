package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// BatchKandidat batch tersedia untuk alokasi (saldo > 0).
type BatchKandidat struct {
	BarangMasukID uint64
	NoBatch       string
	Exp           time.Time
	TanggalMasuk  time.Time
	QtyTersedia   int
	HPPDenganPPN  decimal.Decimal
	HargaMT       decimal.Decimal
	HargaGT       decimal.Decimal
}

// AlokasiBatchHasil satu potongan alokasi ke batch.
type AlokasiBatchHasil struct {
	BarangMasukID uint64
	NoBatch       string
	Exp           time.Time
	Qty           int
	Harga         decimal.Decimal
	HPPSnapshot   decimal.Decimal
}

// AlokasiBatch memilih batch FEFO/FIFO untuk memenuhi qty (04 §2.3).
// Batch kedaluwarsa (exp < hariIni) dikecualikan. Tidak ada alokasi parsial:
// bila stok kurang, mengembalikan ErrStokTidakCukup.
func AlokasiBatch(
	batches []BatchKandidat,
	qtyDiminta int,
	metode MetodeAlokasi,
	channel ChannelOutlet,
	hariIni time.Time,
) ([]AlokasiBatchHasil, error) {
	if qtyDiminta <= 0 {
		return nil, nil
	}
	if !metode.Valid() {
		metode = AlokasiFEFO
	}

	hari := truncDay(hariIni)
	tersedia := make([]BatchKandidat, 0, len(batches))
	for _, b := range batches {
		if b.QtyTersedia <= 0 {
			continue
		}
		if truncDay(b.Exp).Before(hari) {
			continue
		}
		tersedia = append(tersedia, b)
	}

	slices.SortFunc(tersedia, func(a, b BatchKandidat) int {
		switch metode {
		case AlokasiFIFO:
			if c := a.TanggalMasuk.Compare(b.TanggalMasuk); c != 0 {
				return c
			}
			return cmp.Compare(a.BarangMasukID, b.BarangMasukID)
		default: // FEFO
			if c := a.Exp.Compare(b.Exp); c != 0 {
				return c
			}
			if c := a.TanggalMasuk.Compare(b.TanggalMasuk); c != 0 {
				return c
			}
			return cmp.Compare(a.BarangMasukID, b.BarangMasukID)
		}
	})

	sisa := qtyDiminta
	out := make([]AlokasiBatchHasil, 0)
	for _, b := range tersedia {
		if sisa == 0 {
			break
		}
		ambil := b.QtyTersedia
		if ambil > sisa {
			ambil = sisa
		}
		out = append(out, AlokasiBatchHasil{
			BarangMasukID: b.BarangMasukID,
			NoBatch:       b.NoBatch,
			Exp:           b.Exp,
			Qty:           ambil,
			Harga:         PilihHarga(b.HargaMT, b.HargaGT, channel),
			HPPSnapshot:   b.HPPDenganPPN,
		})
		sisa -= ambil
	}
	if sisa > 0 {
		return nil, ErrStokTidakCukup
	}
	return out, nil
}
