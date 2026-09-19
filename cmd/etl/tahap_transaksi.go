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

type barisDetail struct {
	KodeItem       string
	NamaItem       string
	Satuan         string
	BatchNumber    string
	ExpiryDate     sql.NullTime
	Qty            int
	QtyPromo       int
	TotalQtyKeluar int
	Harga          decimal.Decimal
	Subtotal       decimal.Decimal
	Disc1          decimal.Decimal
	Disc2          decimal.Decimal
	Disc3          decimal.Decimal
	TotalAfterDisc decimal.Decimal
	PromoTeks      [3]string
	PromoQty       [3]int
}

type sumberTransaksiHeader struct {
	ID                  uint64
	NoTransaksi         sql.NullString
	Tanggal             sql.NullTime
	Periode             sql.NullString
	KodePelanggan       sql.NullString
	NamaPelanggan       sql.NullString
	Alamat              sql.NullString
	Channel             sql.NullString
	Area                sql.NullString
	IsMulti             sql.NullInt64
	Disc1, Disc2, Disc3 sql.NullString
	PPNPersen           sql.NullString
	Total               sql.NullString
	TotalAkhir          sql.NullString
	JumlahItem          sql.NullInt64
	TotalQtyDitagih     sql.NullInt64
	TotalQtyKeluar      sql.NullInt64
	StatusApproval      sql.NullString
	ApprovedAt          sql.NullTime
	ApprovedBy          sql.NullInt64
	ApprovalNotes       sql.NullString
	Fulfillment         sql.NullString
	QtyFulfilled        sql.NullInt64
	QtyBackorder        sql.NullInt64
	StockNotes          sql.NullString
	StatusBayar         sql.NullString
	JumlahDibayar       sql.NullString
	SisaHutang          sql.NullString
	JatuhTempo          sql.NullTime
	BayarTerakhir       sql.NullTime
	KetBayar            sql.NullString
	FakturAt            sql.NullTime
	SalesID             sql.NullInt64
	Created, Updated    sql.NullTime
	// single-item
	KodeItem, NamaItem, Satuan, Batch sql.NullString
	Expiry                            sql.NullTime
	Jumlah                            sql.NullInt64
	Harga                             sql.NullString
	Promo                             [3]sql.NullString
	PromoQty                          [3]sql.NullInt64
}

func migrasiTransaksiHeader(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "transaksi_penjualan") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "transaksi_penjualan") {
		return fmt.Errorf("tabel sumber transaksi_penjualan tidak ada")
	}

	rows, err := queryHeaderTransaksi(ctx, sumber)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO transaksi_penjualan
			(id, no_transaksi, tanggal, periode, kode_pelanggan, nama_pelanggan,
			 alamat, channel_outlet, area, is_multi_item,
			 disc1_persen, disc2_persen, disc3_persen, ppn_persen,
			 total, ppn_nominal, total_akhir,
			 jumlah_item, total_qty_ditagih, total_qty_keluar,
			 status_approval, approved_at, approved_by, approval_notes,
			 fulfillment_status, qty_fulfilled, qty_backorder, stock_notes,
			 status_pembayaran, jumlah_dibayar, sisa_hutang,
			 tanggal_jatuh_tempo, tanggal_pembayaran_terakhir, keterangan_pembayaran,
			 faktur_dicetak_at, faktur_dicetak_by, sales_id, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		h, err := scanHeader(rows)
		if err != nil {
			lap.Gagal("transaksi_penjualan", 0, err)
			continue
		}
		if err := sisipkanHeader(ctx, stmt, h, lap); err != nil {
			lap.Gagal("transaksi_penjualan", h.ID, err)
			continue
		}
		lap.Berhasil("transaksi_penjualan")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "transaksi_penjualan")
	return nil
}

func sisipkanHeader(ctx context.Context, stmt *sql.Stmt, h sumberTransaksiHeader, lap *Laporan) error {
	if !h.Tanggal.Valid {
		return fmt.Errorf("tanggal kosong")
	}
	kodePlg := strings.TrimSpace(strNS(h.KodePelanggan))
	if kodePlg == "" {
		return fmt.Errorf("kode_pelanggan kosong")
	}

	total := money.RoundMoney(decNS(h.Total))
	ppnP := decNS(h.PPNPersen)
	if ppnP.IsZero() {
		ppnP = decimal.NewFromInt(ppnDefault)
	}
	totalAkhir := money.RoundMoney(decNS(h.TotalAkhir))
	seharusnya, selisih, inflate := hitungSelisihTotalAkhir(total, ppnP, totalAkhir)
	if inflate {
		no := strNS(h.NoTransaksi)
		lap.CatatInflate(BarisInflate{
			ID: h.ID, NoTransaksi: no,
			Total: total.StringFixed(2), PPNPersen: ppnP.StringFixed(2),
			TotalAkhir: totalAkhir.StringFixed(2), Seharusnya: seharusnya.StringFixed(2),
			Selisih: selisih.StringFixed(2),
		})
	}
	ppnNom := money.RoundMoney(totalAkhir.Sub(total))

	st := statusApproval(strNS(h.StatusApproval))
	noTrx := strings.TrimSpace(strNS(h.NoTransaksi))
	var noTrxVal any
	if noTrx != "" {
		noTrxVal = noTrx
	}
	if st == "approved" && (noTrxVal == nil || !h.ApprovedAt.Valid) {
		return fmt.Errorf("approved tanpa no_transaksi/approved_at (I7) — bersihkan di sumber")
	}

	dibayar := money.RoundMoney(decNS(h.JumlahDibayar))
	sisa := money.RoundMoney(decNS(h.SisaHutang))
	if dibayar.Add(sisa).Sub(totalAkhir).Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
		return fmt.Errorf("invariant bayar pecah: dibayar(%s)+sisa(%s) != total_akhir(%s)",
			dibayar, sisa, totalAkhir)
	}

	ch := strNS(h.Channel)
	if n, ok := normalkanChannel(ch); ok {
		ch = string(n)
	}

	var sales any
	if h.SalesID.Valid && h.SalesID.Int64 > 0 {
		sales = h.SalesID.Int64
	}
	var approver any
	if h.ApprovedBy.Valid && h.ApprovedBy.Int64 > 0 {
		approver = h.ApprovedBy.Int64
	}

	_, err := stmt.ExecContext(ctx,
		h.ID, noTrxVal, h.Tanggal.Time.Format("2006-01-02"),
		periodeDari(h.Tanggal.Time, strNS(h.Periode)),
		kodePlg, wajibIsi(strNS(h.NamaPelanggan), "-"), wajibIsi(strNS(h.Alamat), "-"),
		wajibIsi(ch, "-"), wajibIsi(strNS(h.Area), "-"),
		decNS(h.Disc1), decNS(h.Disc2), decNS(h.Disc3), ppnP,
		total, ppnNom, totalAkhir,
		intNS(h.JumlahItem, 0), intNS(h.TotalQtyDitagih, 0), intNS(h.TotalQtyKeluar, 0),
		st, waktuAtauNil(h.ApprovedAt), approver, trimPtr(strNS(h.ApprovalNotes)),
		fulfillmentStatus(strNS(h.Fulfillment)), intNS(h.QtyFulfilled, 0), intNS(h.QtyBackorder, 0),
		trimPtr(strNS(h.StockNotes)),
		statusPembayaran(strNS(h.StatusBayar)), dibayar, sisa,
		tanggalSQL(h.JatuhTempo), waktuAtauNil(h.BayarTerakhir), trimPtr(strNS(h.KetBayar)),
		waktuAtauNil(h.FakturAt), sales, waktuAtauNil(h.Created), waktuAtauNil(h.Updated),
	)
	return err
}

func tanggalSQL(nt sql.NullTime) any {
	if !nt.Valid {
		return nil
	}
	return nt.Time.Format("2006-01-02")
}

func migrasiTransaksiDetail(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "transaksi_detail") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "transaksi_penjualan") {
		return fmt.Errorf("tabel sumber transaksi_penjualan tidak ada")
	}

	rows, err := queryHeaderTransaksi(ctx, sumber)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO transaksi_detail
			(transaksi_penjualan_id, urutan, kode_item, nama_item, satuan,
			 barang_masuk_id, batch_number, expiry_date,
			 qty, qty_promo, total_qty_keluar,
			 harga, subtotal, disc1_persen, disc2_persen, disc3_persen,
			 total_after_disc, hpp_snapshot, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		h, err := scanHeader(rows)
		if err != nil {
			lap.Gagal("transaksi_detail", 0, err)
			continue
		}
		details, err := kumpulkanDetail(ctx, sumber, h)
		if err != nil {
			lap.Gagal("transaksi_detail", h.ID, err)
			continue
		}
		qtyDitagih, qtyKeluar := 0, 0
		okSemua := true
		for i, d := range details {
			if d.Qty <= 0 {
				lap.Gagal("transaksi_detail", h.ID, fmt.Errorf("urutan %d qty <= 0", i+1))
				okSemua = false
				break
			}
			bmID, hpp, cocok := cariBatchTarget(ctx, target, d.KodeItem, d.BatchNumber, d.ExpiryDate)
			if !cocok && d.BatchNumber != "" && d.BatchNumber != "0" {
				lap.Peringatan("transaksi_detail", h.ID, fmt.Sprintf(
					"batch %q untuk SKU %s tidak ditemukan, barang_masuk_id dibiarkan NULL",
					d.BatchNumber, d.KodeItem))
			}
			if hpp.IsZero() {
				hpp = hppTerbaru(ctx, target, d.KodeItem, tanggalDari(h.Tanggal, h.Created.Time))
			}
			var bm any
			if bmID > 0 {
				bm = bmID
			}
			var exp any
			if d.ExpiryDate.Valid {
				exp = d.ExpiryDate.Time.Format("2006-01-02")
			}
			var batch any
			if strings.TrimSpace(d.BatchNumber) != "" {
				batch = d.BatchNumber
			}
			if _, err := stmt.ExecContext(ctx,
				h.ID, i+1, d.KodeItem, d.NamaItem, d.Satuan,
				bm, batch, exp,
				d.Qty, d.QtyPromo, d.TotalQtyKeluar,
				money.RoundMoney(d.Harga), money.RoundMoney(d.Subtotal),
				d.Disc1, d.Disc2, d.Disc3, money.RoundMoney(d.TotalAfterDisc),
				money.RoundMoney(hpp), waktuAtauNil(h.Created), waktuAtauNil(h.Updated),
			); err != nil {
				lap.Gagal("transaksi_detail", h.ID, err)
				okSemua = false
				break
			}
			qtyDitagih += d.Qty
			qtyKeluar += d.TotalQtyKeluar
			lap.Berhasil("transaksi_detail")
		}
		if !okSemua || len(details) == 0 {
			if len(details) == 0 {
				lap.Gagal("transaksi_detail", h.ID, fmt.Errorf("tidak ada baris detail"))
			}
			continue
		}
		_, _ = target.ExecContext(ctx, `
			UPDATE transaksi_penjualan
			SET jumlah_item = ?, total_qty_ditagih = ?, total_qty_keluar = ?
			WHERE id = ?`, len(details), qtyDitagih, qtyKeluar, h.ID)
	}
	return rows.Err()
}

func kumpulkanDetail(ctx context.Context, sumber *Sumber, h sumberTransaksiHeader) ([]barisDetail, error) {
	multi := h.IsMulti.Valid && h.IsMulti.Int64 != 0
	if multi && sumber.PunyaTabel(ctx, "transaksi_detail") {
		return ambilDetailMultiItem(ctx, sumber, h)
	}
	return bentukDetailDariHeader(h)
}

func bentukDetailDariHeader(t sumberTransaksiHeader) ([]barisDetail, error) {
	kode := strings.TrimSpace(strNS(t.KodeItem))
	if kode == "" {
		return nil, fmt.Errorf("transaksi single-item tanpa kode_item")
	}
	qty := intNS(t.Jumlah, 0)
	qtyPromo := intNS(t.PromoQty[0], 0) + intNS(t.PromoQty[1], 0) + intNS(t.PromoQty[2], 0)
	harga := decNS(t.Harga)
	subtotal := harga.Mul(decimal.NewFromInt(int64(qty)))
	d1, d2, d3 := decNS(t.Disc1), decNS(t.Disc2), decNS(t.Disc3)
	totalAfter := domain.DiskonBerjenjang(subtotal, d1, d2, d3)
	d := barisDetail{
		KodeItem: kode, NamaItem: wajibIsi(strNS(t.NamaItem), kode),
		Satuan:      wajibIsi(strNS(t.Satuan), "PCS"),
		BatchNumber: strings.TrimSpace(strNS(t.Batch)), ExpiryDate: t.Expiry,
		Qty: qty, QtyPromo: qtyPromo, TotalQtyKeluar: qty + qtyPromo,
		Harga: harga, Subtotal: subtotal, Disc1: d1, Disc2: d2, Disc3: d3,
		TotalAfterDisc: totalAfter,
	}
	for i := 0; i < 3; i++ {
		d.PromoTeks[i] = strNS(t.Promo[i])
		d.PromoQty[i] = intNS(t.PromoQty[i], 0)
	}
	return []barisDetail{d}, nil
}

func ambilDetailMultiItem(ctx context.Context, sumber *Sumber, h sumberTransaksiHeader) ([]barisDetail, error) {
	fk := sumber.KolomPertama(ctx, "transaksi_detail", "transaksi_penjualan_id", "transaksi_id")
	if fk == "" {
		return nil, fmt.Errorf("transaksi_detail tanpa kolom FK transaksi")
	}
	qtyCol := sumber.KolomPertama(ctx, "transaksi_detail", "qty", "jumlah")
	if qtyCol == "" {
		qtyCol = "qty"
	}
	q := fmt.Sprintf(`
		SELECT
			%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
			%s, %s, %s, %s, %s, %s
		FROM transaksi_detail WHERE `+fk+` = ? ORDER BY id`,
		sumber.ExprKolom(ctx, "transaksi_detail", "kode_item", "kode_item", "''"),
		sumber.ExprKolom(ctx, "transaksi_detail", "nama_item", "nama_item", "''"),
		sumber.ExprKolom(ctx, "transaksi_detail", "satuan", "satuan", "'PCS'"),
		sumber.ExprKolom(ctx, "transaksi_detail", "batch_number", "batch_number", "NULL"),
		sumber.ExprKolom(ctx, "transaksi_detail", "expiry_date", "expiry_date", "NULL"),
		sumber.ExprKolom(ctx, "transaksi_detail", qtyCol, "qty", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "qty_promo", "qty_promo", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "harga", "harga", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "subtotal", "subtotal", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "disc1_persen", "disc1_persen", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "disc2_persen", "disc2_persen", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "disc3_persen", "disc3_persen", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "total_after_disc", "total_after_disc", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo1", "promo1", "NULL"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo2", "promo2", "NULL"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo3", "promo3", "NULL"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo_qty1", "promo_qty1", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo_qty2", "promo_qty2", "0"),
		sumber.ExprKolom(ctx, "transaksi_detail", "promo_qty3", "promo_qty3", "0"),
	)

	drows, err := sumber.QueryContext(ctx, q, h.ID)
	if err != nil {
		return nil, err
	}
	defer drows.Close()

	var out []barisDetail
	for drows.Next() {
		var (
			kode, nama, satuan, batch   sql.NullString
			exp                         sql.NullTime
			qty, qtyP                   sql.NullInt64
			harga, sub, d1, d2, d3, tad sql.NullString
			p1, p2, p3                  sql.NullString
			pq1, pq2, pq3               sql.NullInt64
		)
		if err := drows.Scan(&kode, &nama, &satuan, &batch, &exp, &qty, &qtyP,
			&harga, &sub, &d1, &d2, &d3, &tad, &p1, &p2, &p3, &pq1, &pq2, &pq3); err != nil {
			return nil, err
		}
		qtyN := intNS(qty, 0)
		promoN := intNS(qtyP, 0)
		if promoN == 0 {
			promoN = intNS(pq1, 0) + intNS(pq2, 0) + intNS(pq3, 0)
		}
		hargaD := decNS(harga)
		subD := decNS(sub)
		if subD.IsZero() {
			subD = hargaD.Mul(decimal.NewFromInt(int64(qtyN)))
		}
		tadD := decNS(tad)
		if tadD.IsZero() {
			tadD = domain.DiskonBerjenjang(subD, decNS(d1), decNS(d2), decNS(d3))
		}
		out = append(out, barisDetail{
			KodeItem: strings.TrimSpace(strNS(kode)), NamaItem: wajibIsi(strNS(nama), strNS(kode)),
			Satuan: wajibIsi(strNS(satuan), "PCS"), BatchNumber: strings.TrimSpace(strNS(batch)),
			ExpiryDate: exp, Qty: qtyN, QtyPromo: promoN, TotalQtyKeluar: qtyN + promoN,
			Harga: hargaD, Subtotal: subD, Disc1: decNS(d1), Disc2: decNS(d2), Disc3: decNS(d3),
			TotalAfterDisc: tadD,
			PromoTeks:      [3]string{strNS(p1), strNS(p2), strNS(p3)},
			PromoQty:       [3]int{intNS(pq1, 0), intNS(pq2, 0), intNS(pq3, 0)},
		})
	}
	return out, drows.Err()
}

func cariBatchTarget(ctx context.Context, t *Target, kode, batch string, exp sql.NullTime) (id uint64, hpp decimal.Decimal, ok bool) {
	batch = strings.TrimSpace(batch)
	if batch == "" || batch == "0" {
		return 0, decimal.Zero, false
	}
	var hppS sql.NullString
	q := `
		SELECT bm.id, bm.hpp FROM barang_masuk bm
		JOIN barang b ON b.id = bm.barang_id
		WHERE b.kode_barang = ? AND bm.no_batch = ?`
	args := []any{kode, batch}
	if exp.Valid {
		q += " AND bm.exp = ?"
		args = append(args, exp.Time.Format("2006-01-02"))
	}
	q += " ORDER BY bm.id ASC LIMIT 1"
	err := t.QueryRowContext(ctx, q, args...).Scan(&id, &hppS)
	if err != nil {
		return 0, decimal.Zero, false
	}
	return id, decNS(hppS), true
}

func hppTerbaru(ctx context.Context, t *Target, kode string, sebelum timeOrNow) decimal.Decimal {
	var hppS sql.NullString
	err := t.QueryRowContext(ctx, `
		SELECT bm.hpp FROM barang_masuk bm
		JOIN barang b ON b.id = bm.barang_id
		WHERE b.kode_barang = ? AND bm.tanggal_masuk <= ?
		ORDER BY bm.tanggal_masuk DESC, bm.id DESC LIMIT 1`,
		kode, sebelum.Format("2006-01-02")).Scan(&hppS)
	if err != nil {
		return decimal.Zero
	}
	return decNS(hppS)
}

type timeOrNow interface {
	Format(string) string
}

func queryHeaderTransaksi(ctx context.Context, sumber *Sumber) (*sql.Rows, error) {
	tp := "transaksi_penjualan"
	cols := []string{
		sumber.ExprKolom(ctx, tp, "no_transaksi", "no_transaksi", "NULL"),
		sumber.ExprKolom(ctx, tp, "tanggal", "tanggal", "NULL"),
		sumber.ExprKolom(ctx, tp, "periode", "periode", "NULL"),
		sumber.ExprKolom(ctx, tp, "kode_pelanggan", "kode_pelanggan", "''"),
		sumber.ExprKolom(ctx, tp, "nama_pelanggan", "nama_pelanggan", "''"),
		sumber.ExprKolom(ctx, tp, "alamat", "alamat", "''"),
		sumber.ExprKolom(ctx, tp, "channel_outlet", "channel_outlet", "''"),
		sumber.ExprKolom(ctx, tp, "area", "area", "''"),
		sumber.ExprKolom(ctx, tp, "is_multi_item", "is_multi_item", "0"),
		sumber.ExprKolom(ctx, tp, "disc1_persen", "disc1_persen", "0"),
		sumber.ExprKolom(ctx, tp, "disc2_persen", "disc2_persen", "0"),
		sumber.ExprKolom(ctx, tp, "disc3_persen", "disc3_persen", "0"),
		sumber.ExprKolom(ctx, tp, "ppn_persen", "ppn_persen", "11"),
		sumber.ExprKolom(ctx, tp, "total", "total", "0"),
		sumber.ExprKolom(ctx, tp, "total_akhir", "total_akhir", "0"),
		sumber.ExprKolom(ctx, tp, "jumlah_item", "jumlah_item", "0"),
		sumber.ExprKolom(ctx, tp, "total_qty_ditagih", "total_qty_ditagih", "0"),
		sumber.ExprKolom(ctx, tp, "total_qty_keluar", "total_qty_keluar", "0"),
		sumber.ExprKolom(ctx, tp, "status_approval", "status_approval", "'pending'"),
		sumber.ExprKolom(ctx, tp, "approved_at", "approved_at", "NULL"),
		sumber.ExprKolom(ctx, tp, "approved_by", "approved_by", "NULL"),
		sumber.ExprKolom(ctx, tp, "approval_notes", "approval_notes", "NULL"),
		sumber.ExprKolom(ctx, tp, "fulfillment_status", "fulfillment_status", "'awaiting_approval'"),
		sumber.ExprKolom(ctx, tp, "qty_fulfilled", "qty_fulfilled", "0"),
		sumber.ExprKolom(ctx, tp, "qty_backorder", "qty_backorder", "0"),
		sumber.ExprKolom(ctx, tp, "stock_notes", "stock_notes", "NULL"),
		sumber.ExprKolom(ctx, tp, "status_pembayaran", "status_pembayaran", "'hutang'"),
		sumber.ExprKolom(ctx, tp, "jumlah_dibayar", "jumlah_dibayar", "0"),
		sumber.ExprKolom(ctx, tp, "sisa_hutang", "sisa_hutang", "0"),
		sumber.ExprKolom(ctx, tp, "tanggal_jatuh_tempo", "tanggal_jatuh_tempo", "NULL"),
		sumber.ExprKolom(ctx, tp, "tanggal_pembayaran_terakhir", "tanggal_pembayaran_terakhir", "NULL"),
		sumber.ExprKolom(ctx, tp, "keterangan_pembayaran", "keterangan_pembayaran", "NULL"),
		sumber.ExprKolom(ctx, tp, "faktur_dicetak_at", "faktur_dicetak_at", "NULL"),
		sumber.ExprKolom(ctx, tp, "sales_id", "sales_id", "NULL"),
		sumber.ExprKolom(ctx, tp, "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, tp, "updated_at", "updated_at", "NULL"),
		sumber.ExprKolom(ctx, tp, "kode_item", "kode_item", "NULL"),
		sumber.ExprKolom(ctx, tp, "nama_item", "nama_item", "NULL"),
		sumber.ExprKolom(ctx, tp, "satuan", "satuan", "NULL"),
		sumber.ExprKolom(ctx, tp, "batch_number", "batch_number", "NULL"),
		sumber.ExprKolom(ctx, tp, "expiry_date", "expiry_date", "NULL"),
		sumber.ExprKolom(ctx, tp, "jumlah", "jumlah", "0"),
		sumber.ExprKolom(ctx, tp, "harga", "harga", "0"),
		sumber.ExprKolom(ctx, tp, "promo1", "promo1", "NULL"),
		sumber.ExprKolom(ctx, tp, "promo2", "promo2", "NULL"),
		sumber.ExprKolom(ctx, tp, "promo3", "promo3", "NULL"),
		sumber.ExprKolom(ctx, tp, "promo_qty1", "promo_qty1", "0"),
		sumber.ExprKolom(ctx, tp, "promo_qty2", "promo_qty2", "0"),
		sumber.ExprKolom(ctx, tp, "promo_qty3", "promo_qty3", "0"),
	}
	q := "SELECT id, " + strings.Join(cols, ", ") + " FROM transaksi_penjualan ORDER BY id"
	return sumber.QueryContext(ctx, q)
}

func scanHeader(rows *sql.Rows) (sumberTransaksiHeader, error) {
	var h sumberTransaksiHeader
	err := rows.Scan(
		&h.ID,
		&h.NoTransaksi, &h.Tanggal, &h.Periode,
		&h.KodePelanggan, &h.NamaPelanggan, &h.Alamat, &h.Channel, &h.Area,
		&h.IsMulti,
		&h.Disc1, &h.Disc2, &h.Disc3, &h.PPNPersen,
		&h.Total, &h.TotalAkhir,
		&h.JumlahItem, &h.TotalQtyDitagih, &h.TotalQtyKeluar,
		&h.StatusApproval, &h.ApprovedAt, &h.ApprovedBy, &h.ApprovalNotes,
		&h.Fulfillment, &h.QtyFulfilled, &h.QtyBackorder, &h.StockNotes,
		&h.StatusBayar, &h.JumlahDibayar, &h.SisaHutang,
		&h.JatuhTempo, &h.BayarTerakhir, &h.KetBayar,
		&h.FakturAt, &h.SalesID, &h.Created, &h.Updated,
		&h.KodeItem, &h.NamaItem, &h.Satuan, &h.Batch, &h.Expiry, &h.Jumlah, &h.Harga,
		&h.Promo[0], &h.Promo[1], &h.Promo[2],
		&h.PromoQty[0], &h.PromoQty[1], &h.PromoQty[2],
	)
	return h, err
}

type promoRef struct {
	ID   uint64
	Kode string
	Nama string
	Tipe domain.TipePromo
}

func migrasiTransaksiDetailPromo(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "transaksi_detail_promo") {
		return nil
	}
	master, err := muatPromoTarget(ctx, target)
	if err != nil {
		return err
	}
	stat := map[string]int{}

	drows, err := target.QueryContext(ctx, `
		SELECT td.id, td.transaksi_penjualan_id, td.urutan
		FROM transaksi_detail td ORDER BY td.transaksi_penjualan_id, td.urutan`)
	if err != nil {
		return err
	}
	defer drows.Close()

	type dref struct {
		ID, TrxID uint64
		Urutan    int
	}
	var daftar []dref
	for drows.Next() {
		var r dref
		if err := drows.Scan(&r.ID, &r.TrxID, &r.Urutan); err != nil {
			return err
		}
		daftar = append(daftar, r)
	}
	if err := drows.Err(); err != nil {
		return err
	}

	hrows, err := queryHeaderTransaksi(ctx, sumber)
	if err != nil {
		return err
	}
	defer hrows.Close()
	byID := map[uint64]sumberTransaksiHeader{}
	for hrows.Next() {
		h, err := scanHeader(hrows)
		if err != nil {
			continue
		}
		byID[h.ID] = h
	}

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO transaksi_detail_promo
			(transaksi_detail_id, promo_id, kode_promo, nama_promo, tipe_promo, qty_bonus, nilai_diskon, created_at)
		VALUES (?,?,?,?,?,?,0,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, d := range daftar {
		h, ok := byID[d.TrxID]
		if !ok {
			continue
		}
		details, err := kumpulkanDetail(ctx, sumber, h)
		if err != nil || d.Urutan < 1 || d.Urutan > len(details) {
			continue
		}
		det := details[d.Urutan-1]
		for i := 0; i < 3; i++ {
			teks := det.PromoTeks[i]
			if strings.TrimSpace(teks) == "" {
				continue
			}
			p, cara := cocokkanPromo(master, teks)
			stat[cara]++
			if p == nil {
				lap.Peringatan("transaksi_detail_promo", d.ID, fmt.Sprintf(
					"nama promo %q tidak cocok master; qty_promo tetap di detail", teks))
				continue
			}
			if _, err := stmt.ExecContext(ctx, d.ID, p.ID, p.Kode, p.Nama, string(p.Tipe), det.PromoQty[i], waktuAtauNil(h.Created)); err != nil {
				lap.Gagal("transaksi_detail_promo", d.ID, err)
				continue
			}
			lap.Berhasil("transaksi_detail_promo")
		}
	}
	lap.Peringatan("transaksi_detail_promo", 0, fmt.Sprintf(
		"pencocokan: kode_persis=%d nama_persis=%d nama_dinormalkan=%d tidak_ditemukan=%d",
		stat["kode_persis"], stat["nama_persis"], stat["nama_dinormalkan"], stat["tidak_ditemukan"]))
	return nil
}

func muatPromoTarget(ctx context.Context, t *Target) ([]promoRef, error) {
	rows, err := t.QueryContext(ctx, `SELECT id, kode_promo, nama_promo, tipe_promo FROM promos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []promoRef
	for rows.Next() {
		var p promoRef
		var tipe string
		if err := rows.Scan(&p.ID, &p.Kode, &p.Nama, &tipe); err != nil {
			return nil, err
		}
		p.Tipe = domain.TipePromo(tipe)
		out = append(out, p)
	}
	return out, rows.Err()
}

func cocokkanPromo(master []promoRef, teks string) (*promoRef, string) {
	bersih := strings.TrimSpace(teks)
	if bersih == "" {
		return nil, "kosong"
	}
	for i := range master {
		if master[i].Kode == bersih {
			return &master[i], "kode_persis"
		}
	}
	for i := range master {
		if master[i].Nama == bersih {
			return &master[i], "nama_persis"
		}
	}
	norm := normalkanTeks(bersih)
	for i := range master {
		if normalkanTeks(master[i].Nama) == norm {
			return &master[i], "nama_dinormalkan"
		}
	}
	return nil, "tidak_ditemukan"
}

func normalkanTeks(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
