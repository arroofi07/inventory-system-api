package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"app/internal/domain"
	"app/internal/pkg/money"
)

const ppnDefault = 11

// skipSumber true bila tahap butuh DB Laravel tetapi sedang framework-only.
func skipSumber(sumber *Sumber, lap *Laporan, nama string) bool {
	if sumber != nil {
		return false
	}
	lap.Peringatan(nama, 0, "framework-only: tahap dilewati (butuh sumber Laravel)")
	return true
}

func trimPtr(s string) any {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return v
}

func placeholderKeNull(s string) any {
	v := strings.TrimSpace(s)
	switch strings.ToUpper(v) {
	case "", "-", "N/A", "NA", "0", "NULL", "--":
		return nil
	default:
		return v
	}
}

func wajibIsi(s, fallback string) string {
	v := strings.TrimSpace(s)
	if v == "" {
		return fallback
	}
	return v
}

func parseDec(s string) decimal.Decimal {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func decNS(ns sql.NullString) decimal.Decimal {
	if !ns.Valid {
		return decimal.Zero
	}
	return parseDec(ns.String)
}

func intNS(ns sql.NullInt64, def int) int {
	if !ns.Valid {
		return def
	}
	return int(ns.Int64)
}

func boolNS(ns sql.NullInt64, def bool) bool {
	if !ns.Valid {
		return def
	}
	return ns.Int64 != 0
}

func strNS(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func waktuAtauNil(nt sql.NullTime) any {
	if !nt.Valid {
		return nil
	}
	return nt.Time
}

func tanggalDari(nt sql.NullTime, fallback time.Time) time.Time {
	if nt.Valid {
		return nt.Time
	}
	if !fallback.IsZero() {
		return fallback
	}
	return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
}

func periodeDari(tanggal time.Time, mentah string) string {
	p := strings.TrimSpace(mentah)
	if len(p) >= 7 {
		return p[:7]
	}
	return tanggal.Format("2006-01")
}

func markupTipe(s string) domain.MarkupType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "value", "nominal", "amount":
		return domain.MarkupValue
	default:
		return domain.MarkupPercent
	}
}

func roleValid(s string) (domain.Role, bool) {
	r := domain.Role(strings.TrimSpace(s))
	switch r {
	case domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleAfiliasi, domain.RoleSales:
		return r, true
	default:
		return "", false
	}
}

func jenisKelaminAtauNil(s string) any {
	v := strings.ToUpper(strings.TrimSpace(s))
	if v == "L" || v == "P" {
		return v
	}
	return nil
}

func statusPembayaran(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "lunas", "hutang", "sebagian":
		return v
	default:
		return "hutang"
	}
}

func statusApproval(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "pending", "approved", "rejected":
		return v
	default:
		return "pending"
	}
}

func fulfillmentStatus(s string) string {
	v := strings.TrimSpace(s)
	switch v {
	case "awaiting_approval", "full", "partial", "backorder", "stock_shortage":
		return v
	default:
		return "awaiting_approval"
	}
}

func tipePromoValid(s string) (domain.TipePromo, bool) {
	t := domain.TipePromo(strings.TrimSpace(s))
	switch t {
	case domain.PromoBuyXGetY, domain.PromoBonusQty, domain.PromoPercentageDiscount, domain.PromoFixedDiscount:
		return t, true
	default:
		return "", false
	}
}

func resetAutoIncrement(ctx context.Context, t *Target, tabel string) {
	var maxID sql.NullInt64
	q := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM `%s`", tabel)
	if err := t.QueryRowContext(ctx, q).Scan(&maxID); err != nil {
		return
	}
	_, _ = t.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", tabel, maxID.Int64+1))
}

func hitungSelisihTotalAkhir(total, ppnPersen, totalAkhir decimal.Decimal) (seharusnya, selisih decimal.Decimal, menyimpang bool) {
	seharusnya = money.RoundMoney(total.Mul(decimal.NewFromInt(1).Add(ppnPersen.Div(decimal.NewFromInt(100)))))
	selisih = money.RoundMoney(totalAkhir.Sub(seharusnya))
	menyimpang = selisih.Abs().GreaterThan(decimal.NewFromFloat(0.01))
	return seharusnya, selisih, menyimpang
}

func bandingkanNilai(lama, baru, toleransi decimal.Decimal) (lulus bool, detail string) {
	selisih := baru.Sub(lama).Abs()
	detail = fmt.Sprintf("lama=%s baru=%s selisih=%s", lama.StringFixed(2), baru.StringFixed(2), selisih.StringFixed(2))
	return selisih.LessThanOrEqual(toleransi), detail
}

func statusV9(selisih decimal.Decimal) string {
	if selisih.IsZero() {
		return "LULUS"
	}
	return "TINJAU"
}
