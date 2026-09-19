package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Migrasi price_change_logs — tahap 10 / SF-05.
func migrasiPriceChangeLogs(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "price_change_logs") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "price_change_logs") {
		lap.Peringatan("price_change_logs", 0, "tabel sumber tidak ada")
		return nil
	}

	t := "price_change_logs"
	cols := []string{
		sumber.ExprKolom(ctx, t, "barang_masuk_id", "barang_masuk_id", "0"),
		sumber.ExprKolom(ctx, t, "barang_id", "barang_id", "0"),
		sumber.ExprKolom(ctx, t, "bulk_operation_id", "bulk_operation_id", "NULL"),
		sumber.ExprKolom(ctx, t, "old_harga", "old_harga", "NULL"),
		sumber.ExprKolom(ctx, t, "old_disc_hpp_1", "old_disc_hpp_1", "NULL"),
		sumber.ExprKolom(ctx, t, "old_disc_hpp_2", "old_disc_hpp_2", "NULL"),
		sumber.ExprKolom(ctx, t, "old_disc_hpp_3", "old_disc_hpp_3", "NULL"),
		sumber.ExprKolom(ctx, t, "old_markup_mt_type", "old_markup_mt_type", "NULL"),
		sumber.ExprKolom(ctx, t, "old_markup_mt_amount", "old_markup_mt_amount", "NULL"),
		sumber.ExprKolom(ctx, t, "old_markup_gt_type", "old_markup_gt_type", "NULL"),
		sumber.ExprKolom(ctx, t, "old_markup_gt_amount", "old_markup_gt_amount", "NULL"),
		sumber.ExprKolom(ctx, t, "old_harga_mt", "old_harga_mt", "NULL"),
		sumber.ExprKolom(ctx, t, "old_harga_gt", "old_harga_gt", "NULL"),
		sumber.ExprKolom(ctx, t, "old_hpp", "old_hpp", "NULL"),
		sumber.ExprKolom(ctx, t, "new_harga", "new_harga", "NULL"),
		sumber.ExprKolom(ctx, t, "new_disc_hpp_1", "new_disc_hpp_1", "NULL"),
		sumber.ExprKolom(ctx, t, "new_disc_hpp_2", "new_disc_hpp_2", "NULL"),
		sumber.ExprKolom(ctx, t, "new_disc_hpp_3", "new_disc_hpp_3", "NULL"),
		sumber.ExprKolom(ctx, t, "new_markup_mt_type", "new_markup_mt_type", "NULL"),
		sumber.ExprKolom(ctx, t, "new_markup_mt_amount", "new_markup_mt_amount", "NULL"),
		sumber.ExprKolom(ctx, t, "new_markup_gt_type", "new_markup_gt_type", "NULL"),
		sumber.ExprKolom(ctx, t, "new_markup_gt_amount", "new_markup_gt_amount", "NULL"),
		sumber.ExprKolom(ctx, t, "new_harga_mt", "new_harga_mt", "NULL"),
		sumber.ExprKolom(ctx, t, "new_harga_gt", "new_harga_gt", "NULL"),
		sumber.ExprKolom(ctx, t, "new_hpp", "new_hpp", "NULL"),
		sumber.ExprKolom(ctx, t, "keterangan", "keterangan", "NULL"),
		sumber.ExprKolom(ctx, t, "changed_by", "changed_by", "NULL"),
		sumber.ExprKolom(ctx, t, "changed_at", "changed_at", "NULL"),
		sumber.ExprKolom(ctx, t, "created_at", "created_at", "NULL"),
	}
	q := "SELECT id, " + strings.Join(cols, ", ") + " FROM price_change_logs ORDER BY id"

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO price_change_logs
			(id, barang_masuk_id, barang_id, bulk_operation_id,
			 old_harga, old_disc_hpp_1, old_disc_hpp_2, old_disc_hpp_3,
			 old_markup_mt_type, old_markup_mt_amount, old_markup_gt_type, old_markup_gt_amount,
			 old_harga_mt, old_harga_gt, old_hpp,
			 new_harga, new_disc_hpp_1, new_disc_hpp_2, new_disc_hpp_3,
			 new_markup_mt_type, new_markup_mt_amount, new_markup_gt_type, new_markup_gt_amount,
			 new_harga_mt, new_harga_gt, new_hpp,
			 keterangan, changed_by, changed_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id, bmID, barangID                                      uint64
			bulk                                                    sql.NullString
			oh, od1, od2, od3, omt, oma, ogt, oga, omtH, ogtH, ohpp sql.NullString
			nh, nd1, nd2, nd3, nmt, nma, ngt, nga, nmtH, ngtH, nhpp sql.NullString
			ket                                                     sql.NullString
			changedBy                                               sql.NullInt64
			changedAt, created                                      sql.NullTime
		)
		if err := rows.Scan(&id, &bmID, &barangID, &bulk,
			&oh, &od1, &od2, &od3, &omt, &oma, &ogt, &oga, &omtH, &ogtH, &ohpp,
			&nh, &nd1, &nd2, &nd3, &nmt, &nma, &ngt, &nga, &nmtH, &ngtH, &nhpp,
			&ket, &changedBy, &changedAt, &created); err != nil {
			lap.Gagal("price_change_logs", 0, err)
			continue
		}
		if !changedAt.Valid {
			lap.Gagal("price_change_logs", id, fmt.Errorf("changed_at kosong"))
			continue
		}
		bulkID := strNS(bulk)
		if bulkID == "" {
			bulkID = uuid.NewString()
		}
		var by any
		if changedBy.Valid && changedBy.Int64 > 0 {
			by = changedBy.Int64
		}
		if _, err := stmt.ExecContext(ctx,
			id, bmID, barangID, bulkID,
			nullDec(oh), nullDec(od1), nullDec(od2), nullDec(od3),
			trimPtr(strNS(omt)), nullDec(oma), trimPtr(strNS(ogt)), nullDec(oga),
			nullDec(omtH), nullDec(ogtH), nullDec(ohpp),
			nullDec(nh), nullDec(nd1), nullDec(nd2), nullDec(nd3),
			trimPtr(strNS(nmt)), nullDec(nma), trimPtr(strNS(ngt)), nullDec(nga),
			nullDec(nmtH), nullDec(ngtH), nullDec(nhpp),
			trimPtr(strNS(ket)), by, changedAt.Time, waktuAtauNil(created),
		); err != nil {
			lap.Gagal("price_change_logs", id, err)
			continue
		}
		lap.Berhasil("price_change_logs")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "price_change_logs")
	return nil
}

func nullDec(ns sql.NullString) any {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	return decNS(ns)
}
