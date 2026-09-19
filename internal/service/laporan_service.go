package service

import (
	"context"
	"fmt"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/csvx"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LaporanService laporan stok & barang keluar/laba (SE-07/08).
type LaporanService struct {
	db  *gorm.DB
	lap *repository.LaporanRepo
}

func NewLaporanService(db *gorm.DB, lap *repository.LaporanRepo) *LaporanService {
	return &LaporanService{db: db, lap: lap}
}

func (s *LaporanService) Stok(ctx context.Context, q dto.LaporanStokQuery) ([]dto.BarisLaporanStok, dto.PageMeta, dto.LaporanStokRingkasan, error) {
	db := s.db.WithContext(ctx)
	rows, total, err := s.lap.ListStok(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanStokRingkasan{}, err
	}
	ringkas, err := s.lap.RingkasanStok(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanStokRingkasan{}, err
	}
	q.Normalize()
	out := make([]dto.BarisLaporanStok, 0, len(rows))
	for _, r := range rows {
		b := domain.Barang{StokTersedia: r.StokTersedia, MinStock: r.MinStock}
		var exp *string
		if r.BatchTerdekatExp != nil {
			s := r.BatchTerdekatExp.UTC().Format("2006-01-02")
			exp = &s
		}
		nilai := r.NilaiStokHPP
		if d, err := decimal.NewFromString(nilai); err == nil {
			nilai = dto.FormatUang(d)
		} else {
			nilai = "0.00"
		}
		out = append(out, dto.BarisLaporanStok{
			KodeBarang: r.KodeBarang, NamaItem: r.NamaItem, Brand: r.Brand, Satuan: r.Satuan,
			TotalMasuk: r.TotalMasuk, TotalKeluar: r.TotalKeluar,
			StokTersedia: r.StokTersedia, MinStock: r.MinStock,
			StatusStok: b.StatusStok(), JumlahBatch: r.JumlahBatch,
			BatchTerdekatExp: exp, NilaiStokHPP: nilai,
		})
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, total), ringkas, nil
}

func (s *LaporanService) EksporStok(ctx context.Context, q dto.LaporanStokQuery) ([]byte, string, error) {
	q.Page = 1
	q.PerPage = 100
	// Ambil semua halaman hingga max 5000
	var all []dto.BarisLaporanStok
	for {
		items, meta, _, err := s.Stok(ctx, q)
		if err != nil {
			return nil, "", err
		}
		all = append(all, items...)
		if q.Page >= meta.TotalPages || len(all) >= 5000 {
			break
		}
		q.Page++
	}
	rows := [][]string{{
		"kode_barang", "nama_item", "brand", "satuan",
		"total_masuk", "total_keluar", "stok_tersedia", "min_stock", "status_stok",
		"jumlah_batch", "batch_terdekat_exp", "nilai_stok_hpp",
	}}
	for _, it := range all {
		exp := ""
		if it.BatchTerdekatExp != nil {
			exp = *it.BatchTerdekatExp
		}
		rows = append(rows, []string{
			it.KodeBarang, it.NamaItem, it.Brand, it.Satuan,
			fmt.Sprintf("%d", it.TotalMasuk), fmt.Sprintf("%d", it.TotalKeluar),
			fmt.Sprintf("%d", it.StokTersedia), fmt.Sprintf("%d", it.MinStock), it.StatusStok,
			fmt.Sprintf("%d", it.JumlahBatch), exp, it.NilaiStokHPP,
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "laporan-stok.csv", err
}

func (s *LaporanService) BarangKeluar(
	ctx context.Context,
	q dto.LaporanBarangKeluarQuery,
	role domain.Role,
) ([]dto.BarisBarangKeluar, dto.PageMeta, dto.LaporanBarangKeluarRingkasan, error) {
	db := s.db.WithContext(ctx)
	raw, total, err := s.lap.ListBarangKeluar(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanBarangKeluarRingkasan{}, err
	}
	q.Normalize()

	ids := uniqueTrxIDs(raw)
	detailsByTrx, headers, err := s.lap.DetailsForAlokasi(db, ids)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanBarangKeluarRingkasan{}, err
	}
	alokasiByDetail := map[uint64]domain.BarisFaktur{}
	for trxID, details := range detailsByTrx {
		h := headers[trxID]
		alok := domain.AlokasiProporsionalFaktur(h.Total, h.PPNNominal, details)
		for i, d := range details {
			if i < len(alok) {
				alokasiByDetail[d.ID] = alok[i]
			}
		}
	}

	sensitif := role.Punya(domain.PermLaporanLaba)
	out := make([]dto.BarisBarangKeluar, 0, len(raw))
	sumQty := 0
	sumFinal := decimal.Zero
	sumHPP := decimal.Zero
	sumProvit := decimal.Zero

	for _, r := range raw {
		al := alokasiByDetail[r.DetailID]
		final := al.TotalFinalBaris
		hppSnap, _ := decimal.NewFromString(r.HPPSnapshot)
		hasil := domain.HitungProvit(domain.InputProvit{
			TotalFinalBaris: final,
			HPPSnapshot:     hppSnap,
			TotalQtyKeluar:  r.TotalQtyKeluar,
		})
		sumQty += r.TotalQtyKeluar
		sumFinal = sumFinal.Add(final)
		sumHPP = sumHPP.Add(hasil.HPPTotal)
		sumProvit = sumProvit.Add(hasil.Provit)

		var exp *string
		if r.ExpiryDate != nil {
			s := r.ExpiryDate.UTC().Format("2006-01-02")
			exp = &s
		}
		item := dto.BarisBarangKeluar{
			TransaksiID: r.TransaksiID, NoTransaksi: r.NoTransaksi,
			Tanggal: r.Tanggal.UTC().Format("2006-01-02"),
			KodePelanggan: r.KodePelanggan, NamaPelanggan: r.NamaPelanggan,
			ChannelOutlet: r.ChannelOutlet, Area: r.Area, NamaSales: r.NamaSales,
			KodeItem: r.KodeItem, NamaItem: r.NamaItem, Brand: r.Brand,
			NoBatch: r.BatchNumber, Exp: exp,
			Qty: r.Qty, QtyPromo: r.QtyPromo, TotalQtyKeluar: r.TotalQtyKeluar,
			Harga: fmtDec(r.Harga), Disc1Persen: fmtDec(r.Disc1), Disc2Persen: fmtDec(r.Disc2), Disc3Persen: fmtDec(r.Disc3),
			TotalAfterDisc: fmtDec(r.TotalAfterDisc),
			PPNBaris: dto.FormatUang(al.PPNBaris),
			TotalFinalBaris: dto.FormatUang(final),
		}
		if sensitif {
			alamat := r.Alamat
			item.Alamat = &alamat
			hs := dto.FormatUang(hppSnap)
			ht := dto.FormatUang(hasil.HPPTotal)
			pv := dto.FormatUang(hasil.Provit)
			mg := dto.FormatUang(hasil.MarginPersen)
			item.HPPSnapshot = &hs
			item.HPPTotal = &ht
			item.Provit = &pv
			item.MarginPersen = &mg
		}
		out = append(out, item)
	}

	ringkas := dto.LaporanBarangKeluarRingkasan{
		TotalQtyKeluar: sumQty,
		TotalFinal:     dto.FormatUang(sumFinal),
	}
	if sensitif {
		ringkas.TotalHPP = dto.FormatUang(sumHPP)
		ringkas.TotalProvit = dto.FormatUang(sumProvit)
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, total), ringkas, nil
}

func (s *LaporanService) EksporBarangKeluar(ctx context.Context, q dto.LaporanBarangKeluarQuery, role domain.Role) ([]byte, string, error) {
	q.Page = 1
	q.PerPage = 100
	sensitif := role.Punya(domain.PermLaporanLaba)
	header := []string{
		"no_transaksi", "tanggal", "kode_pelanggan", "nama_pelanggan",
		"channel_outlet", "area", "kode_item", "nama_item", "brand",
		"qty", "qty_promo", "total_qty_keluar", "harga", "total_after_disc",
		"ppn_baris", "total_final_baris",
	}
	if sensitif {
		header = []string{
			"no_transaksi", "tanggal", "kode_pelanggan", "nama_pelanggan", "alamat",
			"channel_outlet", "area", "kode_item", "nama_item", "brand",
			"qty", "qty_promo", "total_qty_keluar", "harga", "total_after_disc",
			"ppn_baris", "total_final_baris", "hpp_snapshot", "hpp_total", "provit", "margin_persen",
		}
	}
	rows := [][]string{header}
	var all []dto.BarisBarangKeluar
	for {
		items, meta, _, err := s.BarangKeluar(ctx, q, role)
		if err != nil {
			return nil, "", err
		}
		all = append(all, items...)
		if q.Page >= meta.TotalPages || len(all) >= 5000 {
			break
		}
		q.Page++
	}
	for _, it := range all {
		no := ""
		if it.NoTransaksi != nil {
			no = *it.NoTransaksi
		}
		row := []string{no, it.Tanggal, it.KodePelanggan, it.NamaPelanggan}
		if sensitif {
			alamat := ""
			if it.Alamat != nil {
				alamat = *it.Alamat
			}
			row = append(row, alamat)
		}
		row = append(row,
			it.ChannelOutlet, it.Area, it.KodeItem, it.NamaItem, it.Brand,
			fmt.Sprintf("%d", it.Qty), fmt.Sprintf("%d", it.QtyPromo), fmt.Sprintf("%d", it.TotalQtyKeluar),
			it.Harga, it.TotalAfterDisc, it.PPNBaris, it.TotalFinalBaris,
		)
		if sensitif {
			row = append(row,
				deref(it.HPPSnapshot), deref(it.HPPTotal), deref(it.Provit), deref(it.MarginPersen),
			)
		}
		rows = append(rows, row)
	}
	b, err := csvx.Bytes(rows)
	return b, "laporan-barang-keluar.csv", err
}

func uniqueTrxIDs(rows []repository.BarangKeluarRow) []uint64 {
	seen := map[uint64]struct{}{}
	var ids []uint64
	for _, r := range rows {
		if _, ok := seen[r.TransaksiID]; ok {
			continue
		}
		seen[r.TransaksiID] = struct{}{}
		ids = append(ids, r.TransaksiID)
	}
	return ids
}

func fmtDec(s string) string {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return "0.00"
	}
	return dto.FormatUang(d)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *LaporanService) ChannelAnalytics(ctx context.Context, q dto.ChannelAnalyticsQuery) (dto.ChannelAnalyticsData, error) {
	q.Normalize()
	db := s.db.WithContext(ctx)
	out := dto.ChannelAnalyticsData{
		PerChannel:     []dto.BarisChannelAnalytics{},
		PerTerritory:   []dto.BarisTerritoryAnalytics{},
		ProdukTerlaris: []dto.BarisProdukTerlaris{},
		TrenHarian:     []dto.BarisTrenHarian{},
	}

	chRows, err := s.lap.AgregatPerChannel(db, q.DateFrom, q.DateTo)
	if err != nil {
		return out, err
	}
	for _, r := range chRows {
		total := fmtDec(r.TotalPenjualan)
		rata := "0.00"
		if r.JumlahTransaksi > 0 {
			if d, e := decimal.NewFromString(r.TotalPenjualan); e == nil {
				rata = dto.FormatUang(d.Div(decimal.NewFromInt(int64(r.JumlahTransaksi))))
			}
		}
		out.PerChannel = append(out.PerChannel, dto.BarisChannelAnalytics{
			ChannelOutlet:   r.ChannelOutlet,
			JumlahTransaksi: r.JumlahTransaksi,
			TotalPenjualan:  total,
			TotalQty:        r.TotalQty,
			RataNilaiOrder:  rata,
			JumlahOutlet:    r.JumlahOutlet,
		})
	}

	terRows, err := s.lap.AgregatPerTerritory(db, q.DateFrom, q.DateTo)
	if err != nil {
		return out, err
	}
	for _, r := range terRows {
		out.PerTerritory = append(out.PerTerritory, dto.BarisTerritoryAnalytics{
			Territory:       r.Territory,
			JumlahTransaksi: r.JumlahTransaksi,
			TotalPenjualan:  fmtDec(r.TotalPenjualan),
			TotalQty:        r.TotalQty,
			JumlahOutlet:    r.JumlahOutlet,
		})
	}

	prodRows, err := s.lap.ProdukTerlaris(db, q.DateFrom, q.DateTo, q.Limit)
	if err != nil {
		return out, err
	}
	for _, r := range prodRows {
		out.ProdukTerlaris = append(out.ProdukTerlaris, dto.BarisProdukTerlaris{
			KodeItem: r.KodeItem, NamaItem: r.NamaItem,
			TotalQty: r.TotalQty, TotalQtyKeluar: r.TotalQtyKeluar,
			JumlahTransaksi: r.JumlahTransaksi,
			TotalAfterDisc:  fmtDec(r.TotalAfterDisc),
		})
	}

	trenRows, err := s.lap.TrenHarian(db, q.DateFrom, q.DateTo)
	if err != nil {
		return out, err
	}
	for _, r := range trenRows {
		out.TrenHarian = append(out.TrenHarian, dto.BarisTrenHarian{
			Tanggal:         r.Tanggal.UTC().Format("2006-01-02"),
			JumlahTransaksi: r.JumlahTransaksi,
			TotalPenjualan:  fmtDec(r.TotalPenjualan),
		})
	}
	return out, nil
}

func (s *LaporanService) EksporChannelAnalytics(ctx context.Context, q dto.ChannelAnalyticsQuery) ([]byte, string, error) {
	data, err := s.ChannelAnalytics(ctx, q)
	if err != nil {
		return nil, "", err
	}
	rows := [][]string{
		{"section", "key", "jumlah_transaksi", "total_penjualan", "total_qty", "rata_nilai_order", "jumlah_outlet", "nama_item", "total_qty_keluar", "total_after_disc"},
	}
	for _, c := range data.PerChannel {
		rows = append(rows, []string{
			"per_channel", c.ChannelOutlet,
			fmt.Sprintf("%d", c.JumlahTransaksi), c.TotalPenjualan, fmt.Sprintf("%d", c.TotalQty),
			c.RataNilaiOrder, fmt.Sprintf("%d", c.JumlahOutlet), "", "", "",
		})
	}
	for _, t := range data.PerTerritory {
		rows = append(rows, []string{
			"per_territory", t.Territory,
			fmt.Sprintf("%d", t.JumlahTransaksi), t.TotalPenjualan, fmt.Sprintf("%d", t.TotalQty),
			"", fmt.Sprintf("%d", t.JumlahOutlet), "", "", "",
		})
	}
	for _, p := range data.ProdukTerlaris {
		rows = append(rows, []string{
			"produk_terlaris", p.KodeItem,
			fmt.Sprintf("%d", p.JumlahTransaksi), "", fmt.Sprintf("%d", p.TotalQty),
			"", "", p.NamaItem, fmt.Sprintf("%d", p.TotalQtyKeluar), p.TotalAfterDisc,
		})
	}
	for _, t := range data.TrenHarian {
		rows = append(rows, []string{
			"tren_harian", t.Tanggal,
			fmt.Sprintf("%d", t.JumlahTransaksi), t.TotalPenjualan, "",
			"", "", "", "", "",
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "channel-analytics.csv", err
}
