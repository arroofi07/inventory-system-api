package domain_test

import (
	"testing"
	"time"

	"app/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

func TestHitungPromoBarisBonusDanDiskonBerurutan(t *testing.T) {
	t.Parallel()
	tanggal := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	buy := 10
	get := 1
	pBonus := domain.Promo{
		ID: 2, KodePromo: "B", TipePromo: domain.PromoBuyXGetY,
		BuyQty: &buy, GetQty: &get, MinQty: 1, IsActive: true,
		TanggalMulai: datatypes.Date(tanggal.AddDate(0, -1, 0)), TanggalBerakhir: datatypes.Date(tanggal.AddDate(0, 1, 0)),
	}
	pDiskon := domain.Promo{
		ID: 1, KodePromo: "D", TipePromo: domain.PromoPercentageDiscount,
		DiscountPercentage: decimal.RequireFromString("10"), MinQty: 1, IsActive: true,
		TanggalMulai: datatypes.Date(tanggal.AddDate(0, -1, 0)), TanggalBerakhir: datatypes.Date(tanggal.AddDate(0, 1, 0)),
	}
	// ID lebih kecil (diskon) diterapkan dulu pada nilai sisa.
	h := domain.HitungPromoBaris(
		[]domain.Promo{pBonus, pDiskon},
		"SHS001", 10, decimal.RequireFromString("180000"), tanggal,
	)
	if h.QtyBonus != 1 {
		t.Fatalf("qty bonus=%d", h.QtyBonus)
	}
	wantDiskon := decimal.RequireFromString("18000") // 10% dari 180000
	if !h.NilaiDiskon.Equal(wantDiskon) {
		t.Fatalf("diskon=%s want %s", h.NilaiDiskon, wantDiskon)
	}
	if len(h.Diterapkan) != 2 || h.Diterapkan[0].Promo.ID != 1 {
		t.Fatalf("urut id: %+v", h.Diterapkan)
	}
}
