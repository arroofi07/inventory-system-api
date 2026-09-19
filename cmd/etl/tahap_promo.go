package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"app/internal/domain"
	"app/internal/pkg/money"
)

// Migrasi promo master — SF-03. Pecah rules JSON menjadi buy_qty/get_qty.
func migrasiPromo(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "promos") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "promos") {
		lap.Peringatan("promos", 0, "tabel sumber promos tidak ada")
		return nil
	}

	cols := []string{
		sumber.ExprKolom(ctx, "promos", "kode_promo", "kode_promo", "''"),
		sumber.ExprKolom(ctx, "promos", "nama_promo", "nama_promo", "''"),
		sumber.ExprKolom(ctx, "promos", "deskripsi", "deskripsi", "NULL"),
		sumber.ExprKolom(ctx, "promos", "tipe_promo", "tipe_promo", "''"),
		sumber.ExprKolom(ctx, "promos", "rules", "rules", "NULL"),
		sumber.ExprKolom(ctx, "promos", "buy_qty", "buy_qty", "NULL"),
		sumber.ExprKolom(ctx, "promos", "get_qty", "get_qty", "NULL"),
		sumber.ExprKolom(ctx, "promos", "bonus_qty", "bonus_qty", "0"),
		sumber.ExprKolom(ctx, "promos", "discount_percentage", "discount_percentage", "0"),
		sumber.ExprKolom(ctx, "promos", "discount_amount", "discount_amount", "0"),
		sumber.ExprKolom(ctx, "promos", "min_qty", "min_qty", "1"),
		sumber.ExprKolom(ctx, "promos", "min_amount", "min_amount", "0"),
		sumber.ExprKolom(ctx, "promos", "max_applications", "max_applications", "NULL"),
		sumber.ExprKolom(ctx, "promos", "kode_barang", "kode_barang", "NULL"),
		sumber.ExprKolom(ctx, "promos", "tanggal_mulai", "tanggal_mulai", "NULL"),
		sumber.ExprKolom(ctx, "promos", "tanggal_berakhir", "tanggal_berakhir", "NULL"),
		sumber.ExprKolom(ctx, "promos", "is_active", "is_active", "1"),
		sumber.ExprKolom(ctx, "promos", "syarat_ketentuan", "syarat_ketentuan", "NULL"),
		sumber.ExprKolom(ctx, "promos", "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, "promos", "updated_at", "updated_at", "NULL"),
	}
	q := "SELECT id, " + strings.Join(cols, ", ") + " FROM promos ORDER BY id"

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO promos
			(id, kode_promo, nama_promo, deskripsi, tipe_promo,
			 buy_qty, get_qty, bonus_qty, discount_percentage, discount_amount,
			 min_qty, min_amount, max_applications, kode_barang,
			 tanggal_mulai, tanggal_berakhir, is_active, syarat_ketentuan,
			 created_by, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id                              uint64
			kode, nama, tipe                sql.NullString
			desk, rules, kodeBarang, syarat sql.NullString
			buy, get, bonus, minQty, maxApp sql.NullInt64
			discPct, discAmt, minAmt        sql.NullString
			mulai, akhir, created, updated  sql.NullTime
			aktif                           sql.NullInt64
		)
		if err := rows.Scan(&id, &kode, &nama, &desk, &tipe, &rules, &buy, &get, &bonus,
			&discPct, &discAmt, &minQty, &minAmt, &maxApp, &kodeBarang,
			&mulai, &akhir, &aktif, &syarat, &created, &updated); err != nil {
			lap.Gagal("promos", 0, err)
			continue
		}
		tp, ok := tipePromoValid(strNS(tipe))
		if !ok {
			lap.Gagal("promos", id, fmt.Errorf("tipe_promo tidak dikenal: %q", strNS(tipe)))
			continue
		}
		var buyQty, getQty any
		if tp == domain.PromoBuyXGetY {
			bq, gq, errRules := pecahRulesBuyXGetY(strNS(rules), intNS(buy, 0), intNS(get, 0))
			if errRules != nil {
				lap.Gagal("promos", id, errRules)
				continue
			}
			buyQty, getQty = bq, gq
		}

		if !mulai.Valid || !akhir.Valid {
			lap.Gagal("promos", id, fmt.Errorf("periode promo tidak lengkap"))
			continue
		}

		kodeBrg := placeholderKeNull(strNS(kodeBarang))
		if _, err := stmt.ExecContext(ctx,
			id, wajibIsi(strNS(kode), fmt.Sprintf("PRM-%d", id)), wajibIsi(strNS(nama), "-"),
			trimPtr(strNS(desk)), string(tp), buyQty, getQty, intNS(bonus, 0),
			money.RoundMoney(decNS(discPct)), money.RoundMoney(decNS(discAmt)),
			intNS(minQty, 1), money.RoundMoney(decNS(minAmt)),
			nullInt(maxApp), kodeBrg,
			mulai.Time.Format("2006-01-02"), akhir.Time.Format("2006-01-02"),
			boolNS(aktif, true), trimPtr(strNS(syarat)),
			waktuAtauNil(created), waktuAtauNil(updated),
		); err != nil {
			lap.Gagal("promos", id, err)
			continue
		}
		lap.Berhasil("promos")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "promos")
	return nil
}

func nullInt(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

func pecahRulesBuyXGetY(rulesJSON string, buyCol, getCol int) (buy, get int, err error) {
	if buyCol > 0 && getCol > 0 {
		return buyCol, getCol, nil
	}
	var rules struct {
		BuyQty int `json:"buy_qty"`
		GetQty int `json:"get_qty"`
	}
	if strings.TrimSpace(rulesJSON) != "" {
		_ = json.Unmarshal([]byte(rulesJSON), &rules)
	}
	if rules.BuyQty <= 0 || rules.GetQty <= 0 {
		return 0, 0, fmt.Errorf("promo buy_x_get_y dengan rules tidak lengkap: %q", rulesJSON)
	}
	return rules.BuyQty, rules.GetQty, nil
}
