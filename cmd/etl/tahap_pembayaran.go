package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"app/internal/pkg/money"
)

// Migrasi riwayat_pembayaran — SF-04.
// nominal_pembayaran = max(new - old, 0). Selisih negatif dilaporkan.
func migrasiRiwayatPembayaran(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "riwayat_pembayaran") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "riwayat_pembayaran") {
		lap.Peringatan("riwayat_pembayaran", 0, "tabel sumber tidak ada")
		return nil
	}

	tabel := "riwayat_pembayaran"
	q := fmt.Sprintf(`
		SELECT
			id,
			%s, %s, %s, %s, %s,
			%s, %s,
			%s, %s, %s, %s, %s
		FROM riwayat_pembayaran
		ORDER BY id`,
		sumber.ExprKolom(ctx, tabel, "transaksi_penjualan_id", "transaksi_penjualan_id", "0"),
		sumber.ExprKolom(ctx, tabel, "old_jumlah_dibayar", "old_jumlah_dibayar", "0"),
		sumber.ExprKolom(ctx, tabel, "new_jumlah_dibayar", "new_jumlah_dibayar", "0"),
		sumber.ExprKolom(ctx, tabel, "old_sisa_hutang", "old_sisa_hutang", "0"),
		sumber.ExprKolom(ctx, tabel, "new_sisa_hutang", "new_sisa_hutang", "0"),
		sumber.ExprKolom(ctx, tabel, "old_status", "old_status", "NULL"),
		sumber.ExprKolom(ctx, tabel, "new_status", "new_status", "NULL"),
		sumber.ExprKolom(ctx, tabel, "keterangan", "keterangan", "NULL"),
		sumber.ExprKolom(ctx, tabel, "changed_by", "changed_by", "NULL"),
		sumber.ExprKolom(ctx, tabel, "changed_at", "changed_at", "NULL"),
		sumber.ExprKolom(ctx, tabel, "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, tabel, "metode_pembayaran", "metode_pembayaran", "NULL"),
	)

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO riwayat_pembayaran
			(id, transaksi_penjualan_id,
			 old_jumlah_dibayar, new_jumlah_dibayar, old_sisa_hutang, new_sisa_hutang,
			 old_status, new_status, nominal_pembayaran, metode_pembayaran,
			 tanggal_pembayaran, keterangan, changed_by, changed_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id, trxID                        uint64
			oldByr, newByr, oldSisa, newSisa sql.NullString
			oldSt, newSt, ket, metode        sql.NullString
			changedBy                        sql.NullInt64
			changedAt, created               sql.NullTime
		)
		if err := rows.Scan(&id, &trxID, &oldByr, &newByr, &oldSisa, &newSisa,
			&oldSt, &newSt, &ket, &changedBy, &changedAt, &created, &metode); err != nil {
			lap.Gagal("riwayat_pembayaran", 0, err)
			continue
		}
		oldD := money.RoundMoney(decNS(oldByr))
		newD := money.RoundMoney(decNS(newByr))
		nom := newD.Sub(oldD)
		if nom.IsNegative() {
			lap.Peringatan("riwayat_pembayaran", id, fmt.Sprintf(
				"koreksi turun nominal %s — tidak dihitung sebagai penerimaan", nom.StringFixed(2)))
			nom = decimal.Zero
		}

		if !changedAt.Valid {
			lap.Gagal("riwayat_pembayaran", id, fmt.Errorf("changed_at kosong"))
			continue
		}
		var by any
		if changedBy.Valid && changedBy.Int64 > 0 {
			by = changedBy.Int64
		}

		if _, err := stmt.ExecContext(ctx,
			id, trxID, oldD, newD, money.RoundMoney(decNS(oldSisa)), money.RoundMoney(decNS(newSisa)),
			statusBayarAtauNil(strNS(oldSt)), statusBayarAtauNil(strNS(newSt)), nom, trimPtr(strNS(metode)),
			changedAt.Time.Format("2006-01-02"), trimPtr(strNS(ket)), by, changedAt.Time, waktuAtauNil(created),
		); err != nil {
			lap.Gagal("riwayat_pembayaran", id, err)
			continue
		}
		lap.Berhasil("riwayat_pembayaran")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "riwayat_pembayaran")
	return nil
}

func statusBayarAtauNil(s string) any {
	if stringsTrimEmpty(s) {
		return nil
	}
	return statusPembayaran(s)
}

func stringsTrimEmpty(s string) bool {
	return len(s) == 0
}
