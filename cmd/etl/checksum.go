package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// tabelChecksum adalah tabel yang ikut dalam fingerprint idempotensi.
// Urutan diurutkan alfabetis saat hash agar stabil.
var tabelChecksum = []string{
	"barang",
	"barang_masuk",
	"pelanggan",
	"price_change_logs",
	"promos",
	"riwayat_pembayaran",
	"stock_movements",
	"transaksi_detail",
	"transaksi_detail_promo",
	"transaksi_penjualan",
	"users",
}

// HitungChecksumTarget menghitung SHA-256 dari COUNT(*) + CHECKSUM TABLE
// untuk tabel bisnis. Dijalankan 2× setelah truncate+ETL harus menghasilkan
// nilai yang sama (09 §2 / SF-01 AC idempoten).
func HitungChecksumTarget(ctx context.Context, t *Target) (string, error) {
	names := append([]string(nil), tabelChecksum...)
	sort.Strings(names)

	var b strings.Builder
	for _, tabel := range names {
		var n int64
		q := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tabel)
		if err := t.QueryRowContext(ctx, q).Scan(&n); err != nil {
			return "", fmt.Errorf("count %s: %w", tabel, err)
		}

		// CHECKSUM TABLE mengembalikan checksum numerik MySQL per tabel.
		var name sql.NullString
		var cs sql.NullInt64
		row := t.QueryRowContext(ctx, "CHECKSUM TABLE `"+tabel+"`")
		if err := row.Scan(&name, &cs); err != nil {
			return "", fmt.Errorf("checksum %s: %w", tabel, err)
		}
		csVal := int64(0)
		if cs.Valid {
			csVal = cs.Int64
		}
		fmt.Fprintf(&b, "%s:%d:%d\n", tabel, n, csVal)
	}

	// Sertakan counter nomor transaksi.
	var lastNum sql.NullInt64
	_ = t.QueryRowContext(ctx, "SELECT last_number FROM no_transaksi_seq WHERE id = 1").Scan(&lastNum)
	fmt.Fprintf(&b, "no_transaksi_seq:%d\n", lastNum.Int64)

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

// BandingkanChecksum mengembalikan error bila dua checksum berbeda.
func BandingkanChecksum(a, b string) error {
	if a == "" || b == "" {
		return fmt.Errorf("checksum kosong (a=%q b=%q)", a, b)
	}
	if a != b {
		return fmt.Errorf("checksum berbeda: run1=%s run2=%s", a, b)
	}
	return nil
}
