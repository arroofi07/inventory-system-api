package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type movementSumber struct {
	BarangID      uint64
	BarangMasukID sql.NullInt64
	MovementType  string
	Qty           int
	ReferenceType sql.NullString
	ReferenceID   sql.NullInt64
	ReferenceNo   sql.NullString
	TanggalUrut   time.Time
	CreatedAt     time.Time
}

// bangunLedger merekonstruksi stock_movements dari barang_masuk + penjualan approved (09 §4 tahap 11).
func bangunLedger(ctx context.Context, _ *Sumber, target *Target, lap *Laporan) error {
	rows, err := target.QueryContext(ctx, `
		SELECT barang_id, barang_masuk_id, movement_type, qty,
		       reference_type, reference_id, reference_no, tanggal_urut, created_at
		FROM (
			SELECT
				bm.barang_id,
				bm.id AS barang_masuk_id,
				'PENERIMAAN' AS movement_type,
				bm.qty AS qty,
				'barang_masuk' AS reference_type,
				bm.id AS reference_id,
				bm.no_faktur AS reference_no,
				bm.tanggal_masuk AS tanggal_urut,
				COALESCE(bm.created_at, bm.tanggal_masuk) AS created_at
			FROM barang_masuk bm

			UNION ALL

			SELECT
				b.id AS barang_id,
				td.barang_masuk_id,
				'PENJUALAN' AS movement_type,
				-td.total_qty_keluar AS qty,
				'transaksi_penjualan' AS reference_type,
				tp.id AS reference_id,
				tp.no_transaksi AS reference_no,
				tp.tanggal AS tanggal_urut,
				COALESCE(tp.approved_at, tp.created_at) AS created_at
			FROM transaksi_detail td
			JOIN transaksi_penjualan tp ON tp.id = td.transaksi_penjualan_id
			JOIN barang b ON b.kode_barang = td.kode_item
			WHERE tp.status_approval = 'approved' AND td.total_qty_keluar <> 0
		) g
		ORDER BY g.barang_id, g.tanggal_urut, g.created_at, g.reference_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO stock_movements
			(barang_id, barang_masuk_id, movement_type, qty, saldo_setelah,
			 reference_type, reference_id, reference_no, keterangan, created_by, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,NULL,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	saldo := map[uint64]int{}
	for rows.Next() {
		var m movementSumber
		if err := rows.Scan(&m.BarangID, &m.BarangMasukID, &m.MovementType, &m.Qty,
			&m.ReferenceType, &m.ReferenceID, &m.ReferenceNo, &m.TanggalUrut, &m.CreatedAt); err != nil {
			return err
		}
		if m.Qty == 0 {
			continue
		}
		saldo[m.BarangID] += m.Qty
		if saldo[m.BarangID] < 0 {
			lap.Peringatan("stock_movements", m.BarangID, fmt.Sprintf(
				"saldo sementara negatif (%d) setelah %s ref %s",
				saldo[m.BarangID], m.MovementType, strNS(m.ReferenceNo)))
		}
		var bm any
		if m.BarangMasukID.Valid {
			bm = m.BarangMasukID.Int64
		}
		if _, err := stmt.ExecContext(ctx,
			m.BarangID, bm, m.MovementType, m.Qty, saldo[m.BarangID],
			strNS(m.ReferenceType), nullInt(m.ReferenceID), trimPtr(strNS(m.ReferenceNo)),
			"Dimigrasikan dari sistem Laravel", m.CreatedAt,
		); err != nil {
			lap.Gagal("stock_movements", m.BarangID, err)
			continue
		}
		lap.Berhasil("stock_movements")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "stock_movements")
	return nil
}

// isiStokTersedia mengisi barang.stok_tersedia dari SUM ledger (tahap 12).
func isiStokTersedia(ctx context.Context, _ *Sumber, target *Target, lap *Laporan) error {
	res, err := target.ExecContext(ctx, `
		UPDATE barang b
		SET b.stok_tersedia = COALESCE((
			SELECT SUM(sm.qty) FROM stock_movements sm WHERE sm.barang_id = b.id
		), 0)`)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	for i := int64(0); i < n; i++ {
		lap.Berhasil("barang.stok_tersedia")
	}
	if n == 0 {
		lap.Peringatan("barang.stok_tersedia", 0, "tidak ada baris barang yang di-update")
	}
	return nil
}

// aturCounterNoTransaksi set last_number = MAX(no_transaksi numerik) (tahap 13).
func aturCounterNoTransaksi(ctx context.Context, _ *Sumber, target *Target, lap *Laporan) error {
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT last_number FROM no_transaksi_seq WHERE id = 1 FOR UPDATE`); err != nil {
		return err
	}
	var maxNum sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT MAX(CAST(no_transaksi AS UNSIGNED))
		FROM transaksi_penjualan
		WHERE no_transaksi REGEXP '^[0-9]+$'`).Scan(&maxNum); err != nil {
		return err
	}
	last := int64(0)
	if maxNum.Valid {
		last = maxNum.Int64
	}
	if _, err := tx.ExecContext(ctx, `UPDATE no_transaksi_seq SET last_number = ? WHERE id = 1`, last); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	lap.Berhasil("no_transaksi_seq")
	lap.Peringatan("no_transaksi_seq", 0, fmt.Sprintf("last_number=%d", last))
	return nil
}
