package service

import (
	"time"

	"app/internal/domain"
	"github.com/shopspring/decimal"
)

func decPtr(d decimal.Decimal) *decimal.Decimal {
	v := d
	return &v
}

func markupPtr(m domain.MarkupType) *domain.MarkupType {
	v := m
	return &v
}

func hargaPricingBerubah(a, b *domain.BarangMasuk) bool {
	return !a.Harga.Equal(b.Harga) ||
		!a.DiscHPP1.Equal(b.DiscHPP1) ||
		!a.DiscHPP2.Equal(b.DiscHPP2) ||
		!a.DiscHPP3.Equal(b.DiscHPP3) ||
		a.MarkupMTType != b.MarkupMTType ||
		!a.MarkupMTAmount.Equal(b.MarkupMTAmount) ||
		a.MarkupGTType != b.MarkupGTType ||
		!a.MarkupGTAmount.Equal(b.MarkupGTAmount)
}

func buatPriceChangeLog(
	sebelum, sesudah *domain.BarangMasuk,
	bulkID string,
	userID *uint64,
	ket *string,
	now time.Time,
) domain.PriceChangeLog {
	return domain.PriceChangeLog{
		BarangMasukID:     sebelum.ID,
		BarangID:          sebelum.BarangID,
		BulkOperationID:   bulkID,
		OldHarga:          decPtr(sebelum.Harga),
		OldDiscHPP1:       decPtr(sebelum.DiscHPP1),
		OldDiscHPP2:       decPtr(sebelum.DiscHPP2),
		OldDiscHPP3:       decPtr(sebelum.DiscHPP3),
		OldMarkupMTType:   markupPtr(sebelum.MarkupMTType),
		OldMarkupMTAmount: decPtr(sebelum.MarkupMTAmount),
		OldMarkupGTType:   markupPtr(sebelum.MarkupGTType),
		OldMarkupGTAmount: decPtr(sebelum.MarkupGTAmount),
		OldHargaMT:        decPtr(sebelum.HargaMT),
		OldHargaGT:        decPtr(sebelum.HargaGT),
		OldHPP:            decPtr(sebelum.HPP),
		NewHarga:          decPtr(sesudah.Harga),
		NewDiscHPP1:       decPtr(sesudah.DiscHPP1),
		NewDiscHPP2:       decPtr(sesudah.DiscHPP2),
		NewDiscHPP3:       decPtr(sesudah.DiscHPP3),
		NewMarkupMTType:   markupPtr(sesudah.MarkupMTType),
		NewMarkupMTAmount: decPtr(sesudah.MarkupMTAmount),
		NewMarkupGTType:   markupPtr(sesudah.MarkupGTType),
		NewMarkupGTAmount: decPtr(sesudah.MarkupGTAmount),
		NewHargaMT:        decPtr(sesudah.HargaMT),
		NewHargaGT:        decPtr(sesudah.HargaGT),
		NewHPP:            decPtr(sesudah.HPP),
		Keterangan:        ket,
		ChangedBy:         userID,
		ChangedAt:         now,
		CreatedAt:         now,
	}
}
