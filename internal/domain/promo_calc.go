package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// HasilBonusPromo hasil penerapan satu promo (qty bonus dan/atau diskon nominal).
type HasilBonusPromo struct {
	QtyBonus    int
	NilaiDiskon decimal.Decimal
}

// HitungBonusPromo menghitung efek satu promo menurut tipenya (04 §3.1).
// Untuk tipe diskon, nilaiDasar adalah total_after_disc baris.
func HitungBonusPromo(p Promo, qty int, nilaiDasar decimal.Decimal) HasilBonusPromo {
	hasil := HasilBonusPromo{NilaiDiskon: decimal.Zero}
	switch p.TipePromo {
	case PromoBuyXGetY, PromoBonusQty:
		hasil.QtyBonus = p.HitungBonusQty(qty)
	case PromoPercentageDiscount, PromoFixedDiscount:
		if qty < p.MinQty {
			return hasil
		}
		hasil.NilaiDiskon = p.HitungDiskon(nilaiDasar)
	default:
		return hasil
	}
	return hasil
}

// HitungBonusQty menghitung qty bonus untuk buy_x_get_y dan bonus_qty.
func (p Promo) HitungBonusQty(qty int) int {
	if qty < p.MinQty {
		return 0
	}
	switch p.TipePromo {
	case PromoBuyXGetY:
		buy := 0
		get := 0
		if p.BuyQty != nil {
			buy = *p.BuyQty
		}
		if p.GetQty != nil {
			get = *p.GetQty
		}
		if buy <= 0 || get <= 0 {
			return 0
		}
		bonus := (qty / buy) * get
		if p.MaxApplications != nil {
			batas := (*p.MaxApplications) * get
			if bonus > batas {
				bonus = batas
			}
		}
		return bonus
	case PromoBonusQty:
		// max_applications diabaikan untuk tipe ini (04 §3.1).
		return p.BonusQty
	default:
		return 0
	}
}

// HitungDiskon menghitung nilai diskon percentage/fixed terhadap nilaiDasar.
// Syarat min_qty dicek pemanggil (BerlakuUntuk / HitungBonusPromo).
func (p Promo) HitungDiskon(nilaiDasar decimal.Decimal) decimal.Decimal {
	if nilaiDasar.LessThan(p.MinAmount) {
		return decimal.Zero
	}
	switch p.TipePromo {
	case PromoPercentageDiscount:
		return nilaiDasar.Mul(p.DiscountPercentage).Div(decimal.NewFromInt(100))
	case PromoFixedDiscount:
		d := p.DiscountAmount
		if d.GreaterThan(nilaiDasar) {
			return nilaiDasar
		}
		return d
	default:
		return decimal.Zero
	}
}

// BerlakuUntuk mengecek syarat aktif, periode (tanggal transaksi), SKU, min qty/amount.
func (p Promo) BerlakuUntuk(kodeItem string, qty int, nilaiDasar decimal.Decimal, tanggal time.Time) bool {
	if !p.IsActive {
		return false
	}
	t := truncDay(tanggal)
	mulai := truncDay(time.Time(p.TanggalMulai))
	akhir := truncDay(time.Time(p.TanggalBerakhir))
	if t.Before(mulai) || t.After(akhir) {
		return false
	}
	if p.KodeBarang != nil && *p.KodeBarang != kodeItem {
		return false
	}
	if qty < p.MinQty {
		return false
	}
	if nilaiDasar.LessThan(p.MinAmount) {
		return false
	}
	return true
}

// PromoTerapan satu promo yang berhasil diterapkan pada baris.
type PromoTerapan struct {
	Promo       Promo
	QtyBonus    int
	NilaiDiskon decimal.Decimal
}

// HasilPromoBaris agregat semua promo pada satu baris (04 §3.3).
type HasilPromoBaris struct {
	QtyBonus    int
	NilaiDiskon decimal.Decimal
	Diterapkan  []PromoTerapan
}

// HitungPromoBaris menerapkan promo yang memenuhi syarat.
// Bonus dijumlahkan; diskon berurutan menurut promo.ID pada nilai sisa.
func HitungPromoBaris(
	promos []Promo,
	kodeItem string,
	qty int,
	nilaiDasar decimal.Decimal,
	tanggal time.Time,
) HasilPromoBaris {
	hasil := HasilPromoBaris{NilaiDiskon: decimal.Zero, Diterapkan: make([]PromoTerapan, 0)}
	if len(promos) == 0 {
		return hasil
	}
	urut := append([]Promo(nil), promos...)
	slices.SortFunc(urut, func(a, b Promo) int {
		return cmp.Compare(a.ID, b.ID)
	})

	sisaNilai := nilaiDasar
	for _, p := range urut {
		if !p.BerlakuUntuk(kodeItem, qty, sisaNilai, tanggal) {
			continue
		}
		switch p.TipePromo {
		case PromoBuyXGetY, PromoBonusQty:
			h := HitungBonusPromo(p, qty, sisaNilai)
			if h.QtyBonus > 0 {
				hasil.QtyBonus += h.QtyBonus
				hasil.Diterapkan = append(hasil.Diterapkan, PromoTerapan{
					Promo: p, QtyBonus: h.QtyBonus,
				})
			}
		case PromoPercentageDiscount, PromoFixedDiscount:
			h := HitungBonusPromo(p, qty, sisaNilai)
			if h.NilaiDiskon.IsPositive() {
				hasil.NilaiDiskon = hasil.NilaiDiskon.Add(h.NilaiDiskon)
				sisaNilai = sisaNilai.Sub(h.NilaiDiskon)
				hasil.Diterapkan = append(hasil.Diterapkan, PromoTerapan{
					Promo: p, NilaiDiskon: h.NilaiDiskon,
				})
			}
		}
	}
	return hasil
}

func truncDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
