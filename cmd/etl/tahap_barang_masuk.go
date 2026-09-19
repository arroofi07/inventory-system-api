package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"app/internal/domain"
	"app/internal/pkg/money"
)

// Migrasi barang_masuk — SF-03.
// Hitung HPP; pertahankan harga_mt/harga_gt tersimpan; duplikat unique di-suffix.
func migrasiBarangMasuk(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "barang_masuk") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "barang_masuk") {
		return fmt.Errorf("tabel sumber barang_masuk tidak ada")
	}

	q := fmt.Sprintf(`
		SELECT
			id,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s
		FROM barang_masuk
		ORDER BY id`,
		sumber.ExprKolom(ctx, "barang_masuk", "barang_id", "barang_id", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "no_faktur", "no_faktur", "''"),
		sumber.ExprKolom(ctx, "barang_masuk", "no_batch", "no_batch", "''"),
		sumber.ExprKolom(ctx, "barang_masuk", "exp", "exp", "NULL"),
		sumber.ExprKolom(ctx, "barang_masuk", "qty", "qty", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "harga", "harga", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "disc_hpp_1", "disc_hpp_1", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "disc_hpp_2", "disc_hpp_2", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "disc_hpp_3", "disc_hpp_3", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "markup_mt_type", "markup_mt_type", "'percent'"),
		sumber.ExprKolom(ctx, "barang_masuk", "markup_mt_amount", "markup_mt_amount", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "markup_gt_type", "markup_gt_type", "'percent'"),
		sumber.ExprKolom(ctx, "barang_masuk", "markup_gt_amount", "markup_gt_amount", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "harga_mt", "harga_mt", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "harga_gt", "harga_gt", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "aging_month", "aging_month", "0"),
		sumber.ExprKolom(ctx, "barang_masuk", "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, "barang_masuk", "updated_at", "updated_at", "NULL"),
	)

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO barang_masuk
			(id, barang_id, no_faktur, no_batch, exp, tanggal_masuk, qty,
			 harga, disc_hpp_1, disc_hpp_2, disc_hpp_3, hpp, hpp_dengan_ppn,
			 markup_mt_type, markup_mt_amount, markup_gt_type, markup_gt_amount,
			 harga_mt, harga_gt, aging_month, created_by, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	seen := map[string]int{}
	ppn := decimal.NewFromInt(ppnDefault)

	for rows.Next() {
		var (
			id, barangID                      uint64
			noFaktur, noBatch, mtType, gtType sql.NullString
			exp, created, updated             sql.NullTime
			qty, aging                        sql.NullInt64
			hargaS, d1s, d2s, d3s             sql.NullString
			mtAmtS, gtAmtS, mtS, gtS          sql.NullString
		)
		if err := rows.Scan(&id, &barangID, &noFaktur, &noBatch, &exp, &qty,
			&hargaS, &d1s, &d2s, &d3s, &mtType, &mtAmtS, &gtType, &gtAmtS,
			&mtS, &gtS, &aging, &created, &updated); err != nil {
			lap.Gagal("barang_masuk", 0, err)
			continue
		}
		qtyN := intNS(qty, 0)
		if qtyN <= 0 {
			lap.Gagal("barang_masuk", id, fmt.Errorf("qty tidak valid: %d", qtyN))
			continue
		}
		if !exp.Valid {
			lap.Gagal("barang_masuk", id, fmt.Errorf("exp kosong"))
			continue
		}
		harga := decNS(hargaS)
		d1, d2, d3 := decNS(d1s), decNS(d2s), decNS(d3s)
		hpp := money.RoundMoney(domain.HitungHPP(harga, d1, d2, d3))
		hppPPN := money.RoundMoney(domain.HitungHPPDenganPPN(hpp, ppn))

		mtHitung := money.RoundMoney(domain.HitungHargaChannel(harga, decNS(mtAmtS), markupTipe(strNS(mtType))))
		gtHitung := money.RoundMoney(domain.HitungHargaChannel(harga, decNS(gtAmtS), markupTipe(strNS(gtType))))
		hargaMT := money.RoundMoney(decNS(mtS))
		hargaGT := money.RoundMoney(decNS(gtS))
		if !mtHitung.Equal(hargaMT) || !gtHitung.Equal(hargaGT) {
			lap.Peringatan("barang_masuk", id, fmt.Sprintf(
				"harga channel tersimpan tidak cocok hasil hitung: mt %s vs %s, gt %s vs %s. Nilai tersimpan dipertahankan.",
				hargaMT, mtHitung, hargaGT, gtHitung,
			))
		}

		faktur := wajibIsi(strNS(noFaktur), fmt.Sprintf("FK-%d", id))
		batch := wajibIsi(strNS(noBatch), "0")
		key := fmt.Sprintf("%d|%s|%s", barangID, batch, faktur)
		seen[key]++
		if seen[key] > 1 {
			faktur = fmt.Sprintf("%s-%d", faktur, seen[key])
			lap.Peringatan("barang_masuk", id, fmt.Sprintf(
				"duplikat (barang_id, no_batch, no_faktur); no_faktur diberi akhiran menjadi %s", faktur))
		}

		tanggalMasuk := tanggalDari(created, exp.Time)
		if _, err := stmt.ExecContext(ctx,
			id, barangID, faktur, batch, exp.Time.Format("2006-01-02"),
			tanggalMasuk.Format("2006-01-02"), qtyN,
			money.RoundMoney(harga), d1, d2, d3, hpp, hppPPN,
			string(markupTipe(strNS(mtType))), decNS(mtAmtS),
			string(markupTipe(strNS(gtType))), decNS(gtAmtS),
			hargaMT, hargaGT, intNS(aging, 0),
			waktuAtauNil(created), waktuAtauNil(updated),
		); err != nil {
			lap.Gagal("barang_masuk", id, err)
			continue
		}
		lap.Berhasil("barang_masuk")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "barang_masuk")
	return nil
}

func suffixFaktur(noFaktur string, kaliKe int) string {
	if kaliKe <= 1 {
		return noFaktur
	}
	return fmt.Sprintf("%s-%d", strings.TrimSpace(noFaktur), kaliKe)
}
