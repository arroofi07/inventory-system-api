package main

import (
	"context"
	"database/sql"
	"fmt"
)

// Migrasi barang — SF-03. stok_tersedia = 0 (diisi tahap 12).
// Satuan dari modus transaksi, default PCS. Alokasi FEFO.
func migrasiBarang(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "barang") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "barang") {
		return fmt.Errorf("tabel sumber barang tidak ada")
	}

	satuanExpr := `'PCS'`
	if sumber.PunyaTabel(ctx, "transaksi_penjualan") && sumber.PunyaKolom(ctx, "transaksi_penjualan", "kode_item") {
		satuanExpr = `COALESCE((
			SELECT tp.satuan
			FROM transaksi_penjualan tp
			WHERE tp.kode_item = b.kode_barang AND tp.satuan IS NOT NULL AND TRIM(tp.satuan) <> ''
			GROUP BY tp.satuan
			ORDER BY COUNT(*) DESC
			LIMIT 1
		), 'PCS')`
	}

	q := fmt.Sprintf(`
		SELECT
			b.id,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s
		FROM barang b
		ORDER BY b.id`,
		sumber.ExprKolom(ctx, "barang", "kode_barang", "kode_barang", "''"),
		sumber.ExprKolom(ctx, "barang", "nama_item", "nama_item", "''"),
		sumber.ExprKolom(ctx, "barang", "brand", "brand", "''"),
		sumber.ExprKolom(ctx, "barang", "min_stock", "min_stock", "0"),
		sumber.ExprKolom(ctx, "barang", "reorder_point", "reorder_point", "0"),
		sumber.ExprKolom(ctx, "barang", "is_active", "is_active", "1"),
		sumber.ExprKolom(ctx, "barang", "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, "barang", "updated_at", "updated_at", "NULL"),
	)

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO barang
			(id, kode_barang, nama_item, brand, satuan, stok_tersedia,
			 min_stock, reorder_point, metode_alokasi, expiry_alert_days,
			 is_active, created_at, updated_at)
		VALUES (?,?,?,?,?,0,?,?, 'FEFO', 30, ?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Satuan dihitung per baris di Go agar aman bila subquery gagal di sebagian SKU.
	satuanStmt := satuanExpr

	for rows.Next() {
		var (
			id                      uint64
			kode, nama, brand       string
			minStok, reorder, aktif sql.NullInt64
			created, updated        sql.NullTime
		)
		if err := rows.Scan(&id, &kode, &nama, &brand, &minStok, &reorder, &aktif, &created, &updated); err != nil {
			lap.Gagal("barang", 0, err)
			continue
		}
		kode = wajibIsi(kode, fmt.Sprintf("SKU-%d", id))
		satuan := "PCS"
		if satuanStmt != `'PCS'` {
			var s sql.NullString
			err := sumber.QueryRowContext(ctx, `
				SELECT tp.satuan FROM transaksi_penjualan tp
				WHERE tp.kode_item = ? AND tp.satuan IS NOT NULL AND TRIM(tp.satuan) <> ''
				GROUP BY tp.satuan ORDER BY COUNT(*) DESC LIMIT 1`, kode).Scan(&s)
			if err == nil && s.Valid && s.String != "" {
				satuan = s.String
			}
		}

		if _, err := stmt.ExecContext(ctx,
			id, kode, wajibIsi(nama, kode), wajibIsi(brand, "-"),
			satuan, intNS(minStok, 0), intNS(reorder, 0), boolNS(aktif, true),
			waktuAtauNil(created), waktuAtauNil(updated),
		); err != nil {
			lap.Gagal("barang", id, err)
			continue
		}
		lap.Berhasil("barang")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "barang")
	return nil
}
