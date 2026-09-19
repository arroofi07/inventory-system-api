package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app/internal/dto"
	"app/internal/pkg/csvx"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)
// ExportService template unduhan + ekspor CSV master (SB-09).
type ExportService struct {
	db          *gorm.DB
	barangMasuk *repository.BarangMasukRepo
	pelanggan   *repository.PelangganRepo
	transaksi   *repository.TransaksiRepo
}

func NewExportService(
	db *gorm.DB,
	bm *repository.BarangMasukRepo,
	pl *repository.PelangganRepo,
	trx *repository.TransaksiRepo,
) *ExportService {
	return &ExportService{db: db, barangMasuk: bm, pelanggan: pl, transaksi: trx}
}

// TemplateBarangMasuk CSV header + contoh HPP terverifikasi (04 §1.1).
func (s *ExportService) TemplateBarangMasuk() ([]byte, string, error) {
	rows := [][]string{
		{
			"kode_barang", "nama_item", "brand", "no_faktur", "no_batch", "exp", "tanggal_masuk",
			"qty", "harga", "disc_hpp_1", "disc_hpp_2", "disc_hpp_3",
			"markup_mt_type", "markup_mt_amount", "markup_gt_type", "markup_gt_amount",
		},
		{
			"SKU-CONTOH", "Item Contoh HPP", "BrandContoh", "FK-CONTOH-001", "BATCH-001",
			"2027-12-31", "2026-09-01", "100", "20000.00", "23.10", "2.00", "5.00",
			"percent", "20", "percent", "15",
		},
		{
			"# Contoh: harga=20000 disc 23.1/2/5 → HPP 14318.78 (server hitung; jangan isi kolom hpp/harga_mt/harga_gt)",
		},
	}
	b, err := csvx.Bytes(rows)
	return b, "template-barang-masuk.csv", err
}

// TemplatePelanggan CSV header + satu baris contoh.
func (s *ExportService) TemplatePelanggan() ([]byte, string, error) {
	rows := [][]string{
		{
			"kode_pelanggan", "nama_pelanggan", "tgl_registrasi", "phone",
			"territory", "distrik", "alamat_toko", "provinsi", "kabupaten", "kecamatan", "kelurahan",
			"channel_outlet", "npwp_nik", "rt_rw", "kode_pos",
			"nominal_pengambilan_pertama", "estimasi_batas_kredit",
		},
		{
			"PLG-CONTOH", "Toko Contoh", "2026-01-15", "08123456789",
			"Jakarta", "Selatan", "Jl. Contoh 1", "DKI Jakarta", "Jakarta Selatan", "Kebayoran", "Senayan",
			"General Trade", "", "", "12190",
			"0.00", "0.00",
		},
	}
	b, err := csvx.Bytes(rows)
	return b, "template-pelanggan.csv", err
}

// EksporBarangMasuk mengekspor daftar penerimaan (filter sama list, max 5000).
func (s *ExportService) EksporBarangMasuk(ctx context.Context, q dto.BarangMasukListQuery) ([]byte, string, error) {
	hasil, err := s.barangMasuk.ListForExport(s.db.WithContext(ctx), q, 5000)
	if err != nil {
		return nil, "", err
	}
	rows := [][]string{{
		"id", "kode_barang", "nama_item", "brand", "no_faktur", "no_batch", "exp", "tanggal_masuk",
		"qty", "harga", "disc_hpp_1", "disc_hpp_2", "disc_hpp_3",
		"hpp", "hpp_dengan_ppn", "markup_mt_type", "markup_mt_amount", "markup_gt_type", "markup_gt_amount",
		"harga_mt", "harga_gt",
	}}
	for i := range hasil.Items {
		bm := &hasil.Items[i]
		kode, nama, brand := "", "", ""
		if bm.Barang != nil {
			kode = bm.Barang.KodeBarang
			nama = bm.Barang.NamaItem
			brand = bm.Barang.Brand
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", bm.ID),
			kode, nama, brand,
			bm.NoFaktur, bm.NoBatch,
			formatDate(bm.Exp), formatDate(bm.TanggalMasuk),
			fmt.Sprintf("%d", bm.Qty),
			bm.Harga.StringFixed(2),
			bm.DiscHPP1.StringFixed(2), bm.DiscHPP2.StringFixed(2), bm.DiscHPP3.StringFixed(2),
			bm.HPP.StringFixed(2), bm.HPPDenganPPN.StringFixed(2),
			string(bm.MarkupMTType), bm.MarkupMTAmount.StringFixed(2),
			string(bm.MarkupGTType), bm.MarkupGTAmount.StringFixed(2),
			bm.HargaMT.StringFixed(2), bm.HargaGT.StringFixed(2),
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "barang-masuk.csv", err
}

// EksporPelanggan mengekspor daftar pelanggan (filter sama list, max 5000).
func (s *ExportService) EksporPelanggan(ctx context.Context, q dto.PelangganListQuery) ([]byte, string, error) {
	hasil, err := s.pelanggan.ListForExport(s.db.WithContext(ctx), q, 5000)
	if err != nil {
		return nil, "", err
	}
	rows := [][]string{{
		"id", "kode_pelanggan", "nama_pelanggan", "tgl_registrasi", "phone",
		"territory", "distrik", "alamat_toko", "provinsi", "kabupaten", "kecamatan", "kelurahan",
		"channel_outlet", "is_active", "nominal_pengambilan_pertama", "estimasi_batas_kredit",
	}}
	for i := range hasil.Items {
		p := &hasil.Items[i]
		aktif := "0"
		if p.IsActive {
			aktif = "1"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", p.ID),
			p.KodePelanggan, p.NamaPelanggan,
			formatDate(p.TglRegistrasi), p.Phone,
			p.Territory, p.Distrik, p.AlamatToko,
			p.Provinsi, p.Kabupaten, p.Kecamatan, p.Kelurahan,
			string(p.ChannelOutlet), aktif,
			dto.FormatUang(p.NominalPengambilanPertama),
			dto.FormatUang(p.EstimasiBatasKredit),
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "pelanggan.csv", err
}

// EksporPenjualan mengekspor transaksi. Default status_approval=approved (SC-09).
func (s *ExportService) EksporPenjualan(ctx context.Context, q dto.TransaksiListQuery) ([]byte, string, error) {
	if strings.TrimSpace(q.StatusApproval) == "" {
		q.StatusApproval = "approved"
	}
	q.Page = 1
	q.PerPage = 5000
	hasil, err := s.transaksi.ListForExport(s.db.WithContext(ctx), q, 5000)
	if err != nil {
		return nil, "", err
	}
	rows := [][]string{{
		"id", "no_transaksi", "tanggal", "kode_pelanggan", "nama_pelanggan",
		"channel_outlet", "area", "jumlah_item", "total_qty_ditagih", "total_qty_keluar",
		"total", "ppn_nominal", "total_akhir", "status_approval", "status_pembayaran", "sales_id",
	}}
	for i := range hasil.Items {
		t := &hasil.Items[i]
		no := ""
		if t.NoTransaksi != nil {
			no = *t.NoTransaksi
		}
		sales := ""
		if t.SalesID != nil {
			sales = fmt.Sprintf("%d", *t.SalesID)
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", t.ID),
			no,
			time.Time(t.Tanggal).UTC().Format("2006-01-02"),
			t.KodePelanggan, t.NamaPelanggan,
			string(t.ChannelOutlet), t.Area,
			fmt.Sprintf("%d", t.JumlahItem),
			fmt.Sprintf("%d", t.TotalQtyDitagih),
			fmt.Sprintf("%d", t.TotalQtyKeluar),
			dto.FormatUang(t.Total),
			dto.FormatUang(t.PPNNominal),
			dto.FormatUang(t.TotalAkhir),
			string(t.StatusApproval),
			string(t.StatusPembayaran),
			sales,
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "laporan-penjualan.csv", err
}

// HitungPenjualan menghitung jumlah baris untuk ambang ekspor async.
func (s *ExportService) HitungPenjualan(ctx context.Context, q dto.TransaksiListQuery) (int64, int64, error) {
	if strings.TrimSpace(q.StatusApproval) == "" {
		q.StatusApproval = "approved"
	}
	jumlah, _, err := s.transaksi.RingkasanList(s.db.WithContext(ctx), q)
	return jumlah, jumlah, err
}

// LaporanPenjualan daftar penjualan; default status_approval=approved (SE-10).
func (s *ExportService) LaporanPenjualan(ctx context.Context, q dto.TransaksiListQuery) ([]dto.TransaksiListItem, dto.PageMeta, dto.LaporanPenjualanRingkasan, error) {
	if strings.TrimSpace(q.StatusApproval) == "" {
		q.StatusApproval = "approved"
	}
	q.Normalize()
	db := s.db.WithContext(ctx)
	hasil, err := s.transaksi.List(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanPenjualanRingkasan{}, err
	}
	jumlah, totalStr, err := s.transaksi.RingkasanList(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.LaporanPenjualanRingkasan{}, err
	}
	totalDec, _ := decimal.NewFromString(totalStr)
	out := make([]dto.TransaksiListItem, 0, len(hasil.Items))
	for i := range hasil.Items {
		t := &hasil.Items[i]
		out = append(out, dto.TransaksiListItem{
			ID: t.ID, NoTransaksi: t.NoTransaksi,
			Tanggal: time.Time(t.Tanggal).UTC().Format("2006-01-02"),
			KodePelanggan: t.KodePelanggan, NamaPelanggan: t.NamaPelanggan,
			ChannelOutlet: string(t.ChannelOutlet), Area: t.Area,
			IsMultiItem: t.IsMultiItem, JumlahItem: t.JumlahItem,
			TotalQtyDitagih: t.TotalQtyDitagih, TotalQtyKeluar: t.TotalQtyKeluar,
			Total: dto.FormatUang(t.Total), PPNNominal: dto.FormatUang(t.PPNNominal),
			TotalAkhir: dto.FormatUang(t.TotalAkhir),
			StatusApproval: string(t.StatusApproval), StatusPembayaran: string(t.StatusPembayaran),
			SalesID: t.SalesID,
		})
	}
	ringkas := dto.LaporanPenjualanRingkasan{
		JumlahTransaksi: int(jumlah),
		TotalPenjualan:  dto.FormatUang(totalDec),
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), ringkas, nil
}
