package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"app/internal/pkg/money"
)

// Pemeriksaan adalah definisi satu verifikasi paritas V1–V9 (09 §5).
type Pemeriksaan struct {
	Kode       string
	Nama       string
	Toleransi  decimal.Decimal
	WajibLulus bool
}

var daftarPemeriksaan = []Pemeriksaan{
	{"V1", "Jumlah baris per tabel", decimal.Zero, true},
	{"V2", "Jumlah baris transaksi_detail", decimal.Zero, true},
	{"V3", "Total penjualan approved", decimal.Zero, true},
	{"V4", "Total piutang", decimal.Zero, true},
	{"V5", "Stok per SKU", decimal.Zero, true},
	{"V6", "Invariant database", decimal.Zero, true},
	{"V7", "Penjualan per periode", decimal.NewFromFloat(0.01), true},
	{"V8", "Penjualan per channel", decimal.NewFromFloat(0.01), true},
	{"V9", "Laba total", decimal.Zero, false},
}

// Verifikator menjalankan pemeriksaan paritas sumber vs target.
type Verifikator struct {
	Sumber *Sumber
	Target *Target
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Jalankan mengembalikan false bila ada pemeriksaan wajib yang gagal.
func (v *Verifikator) Jalankan(ctx context.Context) (ok bool, hasil []HasilVerifikasi) {
	ok = true
	runners := []func(context.Context) HasilVerifikasi{
		v.v1, v.v2, v.v3, v.v4, v.v5, v.v6, v.v7, v.v8, v.v9,
	}
	for i, run := range runners {
		h := run(ctx)
		h.Kode = daftarPemeriksaan[i].Kode
		h.Nama = daftarPemeriksaan[i].Nama
		h.WajibLulus = daftarPemeriksaan[i].WajibLulus
		if h.WajibLulus && h.Status == "GAGAL" {
			ok = false
		}
		hasil = append(hasil, h)
	}
	return ok, hasil
}

func (v *Verifikator) v1(ctx context.Context) HasilVerifikasi {
	tabel := []string{
		"users", "barang", "barang_masuk", "pelanggan", "promos",
		"transaksi_penjualan", "riwayat_pembayaran", "price_change_logs",
	}
	var parts []string
	lulus := true
	for _, t := range tabel {
		lama, errL := v.countSumber(ctx, t)
		baru, errB := countTabel(ctx, v.Target, t)
		if errL != nil || errB != nil {
			lulus = false
			parts = append(parts, fmt.Sprintf("%s:error", t))
			continue
		}
		if lama != baru {
			lulus = false
			parts = append(parts, fmt.Sprintf("%s lama=%d baru=%d", t, lama, baru))
		}
	}
	if lulus {
		return HasilVerifikasi{Status: "LULUS", Detail: "semua tabel sama"}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: strings.Join(parts, "; ")}
}

func (v *Verifikator) v2(ctx context.Context) HasilVerifikasi {
	var detailLama, single, detailBaru int64
	if v.Sumber.PunyaTabel(ctx, "transaksi_detail") {
		_ = v.Sumber.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaksi_detail`).Scan(&detailLama)
	}
	_ = v.Sumber.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM transaksi_penjualan WHERE COALESCE(is_multi_item, 0) = 0`).Scan(&single)
	_ = v.Target.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaksi_detail`).Scan(&detailBaru)
	harap := detailLama + single
	det := fmt.Sprintf("diharapkan %d, aktual %d", harap, detailBaru)
	if harap == detailBaru {
		return HasilVerifikasi{Status: "LULUS", Detail: det}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: det}
}

func (v *Verifikator) v3(ctx context.Context) HasilVerifikasi {
	lama, err1 := sumApproved(ctx, v.Sumber, "total_akhir")
	baru, err2 := sumApproved(ctx, v.Target, "total_akhir")
	if err1 != nil || err2 != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: fmt.Sprintf("query: %v %v", err1, err2)}
	}
	ok, det := bandingkanNilai(lama, baru, decimal.Zero)
	if ok {
		return HasilVerifikasi{Status: "LULUS", Detail: "Rp " + baru.StringFixed(2)}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: det}
}

func (v *Verifikator) v4(ctx context.Context) HasilVerifikasi {
	q := `SELECT COALESCE(ROUND(SUM(sisa_hutang), 2), 0) FROM transaksi_penjualan
		WHERE status_approval = 'approved' AND status_pembayaran <> 'lunas'`
	lama, err1 := scanDecRow(ctx, v.Sumber, q)
	baru, err2 := scanDecRow(ctx, v.Target, q)
	if err1 != nil || err2 != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: fmt.Sprintf("%v %v", err1, err2)}
	}
	ok, det := bandingkanNilai(lama, baru, decimal.Zero)
	if ok {
		return HasilVerifikasi{Status: "LULUS", Detail: "Rp " + baru.StringFixed(2)}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: det}
}

func (v *Verifikator) v5(ctx context.Context) HasilVerifikasi {
	keluarSingle := `0`
	if v.Sumber.PunyaKolom(ctx, "transaksi_penjualan", "kode_item") {
		keluarSingle = `COALESCE((
				SELECT SUM(tp.total_qty_keluar)
				FROM transaksi_penjualan tp
				WHERE tp.kode_item = b.kode_barang
				  AND tp.status_approval = 'approved'
				  AND COALESCE(tp.is_multi_item, 0) = 0
			), 0)`
	}
	keluarMulti := `0`
	if v.Sumber.PunyaTabel(ctx, "transaksi_detail") {
		keluarMulti = `COALESCE((
				SELECT SUM(COALESCE(td.total_qty_keluar, td.qty))
				FROM transaksi_detail td
				JOIN transaksi_penjualan tp2 ON tp2.id = td.transaksi_penjualan_id
				WHERE td.kode_item = b.kode_barang
				  AND tp2.status_approval = 'approved'
			), 0)`
	}
	q := fmt.Sprintf(`
		SELECT b.kode_barang,
			COALESCE((SELECT SUM(bm.qty) FROM barang_masuk bm WHERE bm.barang_id = b.id), 0)
			- %s - %s AS stok_lama
		FROM barang b`, keluarSingle, keluarMulti)
	lama := map[string]int64{}
	rows, err := v.Sumber.QueryContext(ctx, q)
	if err != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: "sumber: " + err.Error()}
	}
	defer rows.Close()
	for rows.Next() {
		var kode string
		var qty int64
		if err := rows.Scan(&kode, &qty); err != nil {
			return HasilVerifikasi{Status: "GAGAL", Detail: err.Error()}
		}
		lama[kode] = qty
	}

	baru := map[string]int64{}
	brows, err := v.Target.QueryContext(ctx, `SELECT kode_barang, stok_tersedia FROM barang`)
	if err != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: "target: " + err.Error()}
	}
	defer brows.Close()
	for brows.Next() {
		var kode string
		var qty int64
		if err := brows.Scan(&kode, &qty); err != nil {
			return HasilVerifikasi{Status: "GAGAL", Detail: err.Error()}
		}
		baru[kode] = qty
	}

	selisih := 0
	var contoh []string
	for kode, lv := range lama {
		if lv != baru[kode] {
			selisih++
			if len(contoh) < 5 {
				contoh = append(contoh, fmt.Sprintf("%s lama=%d baru=%d", kode, lv, baru[kode]))
			}
		}
	}
	if selisih == 0 {
		return HasilVerifikasi{Status: "LULUS", Detail: "0 SKU berselisih"}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: fmt.Sprintf("%d SKU berselisih: %s", selisih, strings.Join(contoh, "; "))}
}

func (v *Verifikator) v6(ctx context.Context) HasilVerifikasi {
	queries := []struct {
		kode string
		sql  string
	}{
		{"I1", `SELECT COUNT(*) FROM transaksi_penjualan WHERE ABS(jumlah_dibayar + sisa_hutang - total_akhir) > 0.01`},
		{"I3", `SELECT COUNT(*) FROM transaksi_detail WHERE total_qty_keluar <> qty + qty_promo`},
		{"I4", `SELECT COUNT(*) FROM transaksi_penjualan tp WHERE tp.total_qty_keluar <> COALESCE((SELECT SUM(td.total_qty_keluar) FROM transaksi_detail td WHERE td.transaksi_penjualan_id = tp.id), 0)`},
		{"I5", `SELECT COUNT(*) FROM barang b WHERE b.stok_tersedia <> COALESCE((SELECT SUM(sm.qty) FROM stock_movements sm WHERE sm.barang_id = b.id), 0)`},
	}
	var gagal []string
	for _, q := range queries {
		var n int64
		if err := v.Target.QueryRowContext(ctx, q.sql).Scan(&n); err != nil {
			return HasilVerifikasi{Status: "GAGAL", Detail: q.kode + ": " + err.Error()}
		}
		if n > 0 {
			gagal = append(gagal, fmt.Sprintf("%s=%d", q.kode, n))
		}
	}
	var i7, i8 int64
	_ = v.Target.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaksi_penjualan WHERE status_approval = 'approved' AND (no_transaksi IS NULL OR approved_at IS NULL)`).Scan(&i7)
	_ = v.Target.QueryRowContext(ctx, `SELECT COUNT(*) FROM transaksi_penjualan WHERE status_approval <> 'approved' AND no_transaksi IS NOT NULL`).Scan(&i8)
	detail := fmt.Sprintf("I7=%d I8=%d (laporkan ke SF-02 bila > 0)", i7, i8)
	if len(gagal) > 0 {
		return HasilVerifikasi{Status: "GAGAL", Detail: strings.Join(gagal, " ") + " " + detail}
	}
	return HasilVerifikasi{Status: "LULUS", Detail: "I1/I3/I4/I5 nol. " + detail}
}

func (v *Verifikator) v7(ctx context.Context) HasilVerifikasi {
	return v.agregatSelisih(ctx, "periode", daftarPemeriksaan[6].Toleransi)
}

func (v *Verifikator) v8(ctx context.Context) HasilVerifikasi {
	return v.agregatSelisih(ctx, "channel_outlet", daftarPemeriksaan[7].Toleransi)
}

func (v *Verifikator) agregatSelisih(ctx context.Context, kolom string, tol decimal.Decimal) HasilVerifikasi {
	q := fmt.Sprintf(`
		SELECT %s, COALESCE(ROUND(SUM(total_akhir), 2), 0)
		FROM transaksi_penjualan WHERE status_approval = 'approved'
		GROUP BY %s`, kolom, kolom)
	lama, err := scanAgregat(ctx, v.Sumber, q)
	if err != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: err.Error()}
	}
	baru, err := scanAgregat(ctx, v.Target, q)
	if err != nil {
		return HasilVerifikasi{Status: "GAGAL", Detail: err.Error()}
	}
	n := 0
	var contoh []string
	for k, lv := range lama {
		sel := baru[k].Sub(lv).Abs()
		if sel.GreaterThan(tol) {
			n++
			if len(contoh) < 5 {
				contoh = append(contoh, fmt.Sprintf("%s selisih=%s", k, sel.StringFixed(2)))
			}
		}
	}
	if n == 0 {
		return HasilVerifikasi{Status: "LULUS", Detail: "0 grup berselisih"}
	}
	return HasilVerifikasi{Status: "GAGAL", Detail: fmt.Sprintf("%d grup: %s", n, strings.Join(contoh, "; "))}
}

func (v *Verifikator) v9(ctx context.Context) HasilVerifikasi {
	baru, err := scanDecRow(ctx, v.Target, `
		SELECT COALESCE(ROUND(SUM(td.total_after_disc * (1 + tp.ppn_persen / 100) - td.hpp_snapshot * td.total_qty_keluar), 2), 0)
		FROM transaksi_detail td
		JOIN transaksi_penjualan tp ON tp.id = td.transaksi_penjualan_id
		WHERE tp.status_approval = 'approved'`)
	if err != nil {
		return HasilVerifikasi{Status: "TINJAU", Detail: "gagal hitung laba baru: " + err.Error()}
	}
	lama, errLama := v.labaLama(ctx)
	if errLama != nil {
		return HasilVerifikasi{
			Status: "TINJAU",
			Detail: fmt.Sprintf("provit_baru=%s; rumus lama gagal (%v) — butuh persetujuan keuangan", baru.StringFixed(2), errLama),
		}
	}
	selisih := money.RoundMoney(baru.Sub(lama))
	return HasilVerifikasi{
		Status: statusV9(selisih),
		Detail: fmt.Sprintf("provit_lama=%s provit_baru=%s selisih=%s (HPP deterministik, lihat 11 §4)",
			lama.StringFixed(2), baru.StringFixed(2), selisih.StringFixed(2)),
	}
}

func (v *Verifikator) labaLama(ctx context.Context) (decimal.Decimal, error) {
	q := `
		SELECT COALESCE(ROUND(SUM(COALESCE(total_akhir, 0)), 2), 0)
		FROM transaksi_penjualan WHERE status_approval = 'approved'`
	return scanDecRow(ctx, v.Sumber, q)
}

func (v *Verifikator) countSumber(ctx context.Context, tabel string) (int64, error) {
	if !v.Sumber.PunyaTabel(ctx, tabel) {
		return 0, nil
	}
	return countTabel(ctx, v.Sumber, tabel)
}

func countTabel(ctx context.Context, q queryRower, tabel string) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+tabel+"`").Scan(&n)
	return n, err
}

func sumApproved(ctx context.Context, q queryRower, kolom string) (decimal.Decimal, error) {
	return scanDecRow(ctx, q, `SELECT COALESCE(ROUND(SUM(`+kolom+`), 2), 0) FROM transaksi_penjualan WHERE status_approval = 'approved'`)
}

func scanDecRow(ctx context.Context, q queryRower, sqlText string) (decimal.Decimal, error) {
	var s string
	if err := q.QueryRowContext(ctx, sqlText).Scan(&s); err != nil {
		return decimal.Zero, err
	}
	return parseDec(s), nil
}

func scanAgregat(ctx context.Context, q queryRower, sqlText string) (map[string]decimal.Decimal, error) {
	rows, err := q.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var k, val string
		if err := rows.Scan(&k, &val); err != nil {
			return nil, err
		}
		out[k] = parseDec(val)
	}
	return out, rows.Err()
}

func ringkasKesimpulan(hasil []HasilVerifikasi) string {
	wajibLulus, wajibGagal, tinjau, skip := 0, 0, 0, 0
	for _, h := range hasil {
		switch h.Status {
		case "LULUS":
			if h.WajibLulus {
				wajibLulus++
			}
		case "GAGAL":
			if h.WajibLulus {
				wajibGagal++
			}
		case "TINJAU":
			tinjau++
		case "SKIP":
			skip++
		}
	}
	if skip > 0 && wajibLulus == 0 && wajibGagal == 0 {
		return fmt.Sprintf("%d pemeriksaan di-SKIP. Kerangka ETL siap.", skip)
	}
	return fmt.Sprintf(
		"%d wajib LULUS, %d wajib GAGAL, %d TINJAU. Cutover dilarang bila wajib GAGAL > 0.",
		wajibLulus, wajibGagal, tinjau,
	)
}
