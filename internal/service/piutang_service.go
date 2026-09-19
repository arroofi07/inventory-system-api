package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/pkg/csvx"
	"app/internal/repository"
	"gorm.io/gorm"
)

// PiutangService daftar piutang, notifikasi, penerimaan (SD-05/06).
type PiutangService struct {
	db    *gorm.DB
	trx   *repository.TransaksiRepo
	bayar *repository.PembayaranRepo
	clock clock.Clock
}

func NewPiutangService(
	db *gorm.DB,
	trx *repository.TransaksiRepo,
	bayar *repository.PembayaranRepo,
	clk clock.Clock,
) *PiutangService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &PiutangService{db: db, trx: trx, bayar: bayar, clock: clk}
}

func (s *PiutangService) Daftar(
	ctx context.Context,
	q dto.PiutangListQuery,
) ([]dto.PiutangItem, dto.PageMeta, dto.PiutangRingkasan, error) {
	hariIni := s.clock.Now().UTC()
	db := s.db.WithContext(ctx)
	hasil, err := s.trx.ListPiutang(db, q, hariIni)
	if err != nil {
		return nil, dto.PageMeta{}, dto.PiutangRingkasan{}, err
	}
	ringkas, err := s.trx.RingkasanPiutang(db, q, hariIni)
	if err != nil {
		return nil, dto.PageMeta{}, dto.PiutangRingkasan{}, err
	}
	out := make([]dto.PiutangItem, 0, len(hasil.Items))
	for i := range hasil.Items {
		out = append(out, mapPiutangItem(&hasil.Items[i], hariIni))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), ringkas, nil
}

func (s *PiutangService) Overdue(ctx context.Context, q dto.PiutangListQuery) ([]dto.PiutangItem, dto.PageMeta, dto.PiutangRingkasan, error) {
	q.HanyaOverdue = true
	q.KategoriJatuhTempo = "overdue"
	return s.Daftar(ctx, q)
}

func (s *PiutangService) Pelanggan(ctx context.Context, kode string, q dto.PiutangListQuery) ([]dto.PiutangItem, dto.PageMeta, dto.PiutangRingkasan, error) {
	kode = strings.TrimSpace(kode)
	if kode == "" {
		return nil, dto.PageMeta{}, dto.PiutangRingkasan{}, fieldErr("kode_pelanggan", "wajib diisi")
	}
	q.KodePelanggan = kode
	return s.Daftar(ctx, q)
}

func (s *PiutangService) RiwayatPelanggan(ctx context.Context, kode string) ([]dto.RiwayatPembayaranItem, error) {
	kode = strings.TrimSpace(kode)
	if kode == "" {
		return nil, fieldErr("kode_pelanggan", "wajib diisi")
	}
	rows, err := s.bayar.ListByPelanggan(s.db.WithContext(ctx), kode, 500)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RiwayatPembayaranItem, 0, len(rows))
	for _, row := range rows {
		item := dto.RiwayatPembayaranItem{
			ID:                row.ID,
			NominalPembayaran: dto.FormatUang(row.NominalPembayaran),
			OldJumlahDibayar:  dto.FormatUang(row.OldJumlahDibayar),
			NewJumlahDibayar:  dto.FormatUang(row.NewJumlahDibayar),
			OldSisaHutang:     dto.FormatUang(row.OldSisaHutang),
			NewSisaHutang:     dto.FormatUang(row.NewSisaHutang),
			MetodePembayaran:  row.MetodePembayaran,
			Keterangan:        row.Keterangan,
			ChangedBy:         row.ChangedBy,
			ChangedAt:         row.ChangedAt.UTC().Format(time.RFC3339),
		}
		if row.OldStatus != nil {
			st := string(*row.OldStatus)
			item.OldStatus = &st
		}
		if row.NewStatus != nil {
			st := string(*row.NewStatus)
			item.NewStatus = &st
		}
		if row.TanggalPembayaran != nil {
			s := time.Time(*row.TanggalPembayaran).UTC().Format("2006-01-02")
			item.TanggalPembayaran = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *PiutangService) Notifikasi(ctx context.Context) (*dto.NotifikasiPiutangResponse, error) {
	over, dekat, err := s.trx.HitungNotifikasiPiutang(s.db.WithContext(ctx), s.clock.Now().UTC(), 7)
	if err != nil {
		return nil, err
	}
	return &dto.NotifikasiPiutangResponse{Overdue: over, MendekatiJatuhTempo: dekat}, nil
}

func (s *PiutangService) Penerimaan(ctx context.Context, q dto.PenerimaanLaporanQuery) (*dto.PenerimaanLaporanResponse, error) {
	from := strings.TrimSpace(q.DateFrom)
	to := strings.TrimSpace(q.DateTo)
	if from == "" || to == "" {
		return nil, fieldErr("date_from", "date_from dan date_to wajib")
	}
	total, n, err := s.bayar.SumNominalPenerimaan(s.db.WithContext(ctx), from, to)
	if err != nil {
		return nil, err
	}
	return &dto.PenerimaanLaporanResponse{
		DateFrom: from, DateTo: to, TotalNominal: total, JumlahBaris: n,
	}, nil
}

func (s *PiutangService) Ekspor(ctx context.Context, q dto.PiutangListQuery) ([]byte, string, error) {
	q.Page = 1
	q.PerPage = 5000
	hariIni := s.clock.Now().UTC()
	// Bypass Normalize cap via ListForExport-style: set PerPage high then list uses Normalize to 100 — use raw list with q.PerPage after Normalize override
	hasil, err := s.trx.ListPiutangExport(s.db.WithContext(ctx), q, hariIni, 5000)
	if err != nil {
		return nil, "", err
	}
	rows := [][]string{{
		"transaksi_id", "no_transaksi", "tanggal", "kode_pelanggan", "nama_pelanggan",
		"total_akhir", "jumlah_dibayar", "sisa_hutang", "status_pembayaran",
		"tanggal_jatuh_tempo", "kategori_jatuh_tempo", "hari_terlambat",
	}}
	for i := range hasil.Items {
		it := mapPiutangItem(&hasil.Items[i], hariIni)
		no := ""
		if it.NoTransaksi != nil {
			no = *it.NoTransaksi
		}
		tjt := ""
		if it.TanggalJatuhTempo != nil {
			tjt = *it.TanggalJatuhTempo
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", it.TransaksiID), no, it.Tanggal, it.KodePelanggan, it.NamaPelanggan,
			it.TotalAkhir, it.JumlahDibayar, it.SisaHutang, it.StatusPembayaran,
			tjt, it.KategoriJatuhTempo, fmt.Sprintf("%d", it.HariTerlambat),
		})
	}
	b, err := csvx.Bytes(rows)
	return b, "piutang.csv", err
}

func mapPiutangItem(trx *domain.TransaksiPenjualan, hariIni time.Time) dto.PiutangItem {
	kat, hari := domain.KlasifikasiJatuhTempo(trx.StatusPembayaran, trx.TanggalJatuhTempo, hariIni, 7)
	item := dto.PiutangItem{
		TransaksiID:        trx.ID,
		NoTransaksi:        trx.NoTransaksi,
		Tanggal:            time.Time(trx.Tanggal).UTC().Format("2006-01-02"),
		KodePelanggan:      trx.KodePelanggan,
		NamaPelanggan:      trx.NamaPelanggan,
		TotalAkhir:         dto.FormatUang(trx.TotalAkhir),
		JumlahDibayar:      dto.FormatUang(trx.JumlahDibayar),
		SisaHutang:         dto.FormatUang(trx.SisaHutang),
		StatusPembayaran:   string(trx.StatusPembayaran),
		KategoriJatuhTempo: string(kat),
		HariTerlambat:      hari,
	}
	if trx.TanggalJatuhTempo != nil {
		s := time.Time(*trx.TanggalJatuhTempo).UTC().Format("2006-01-02")
		item.TanggalJatuhTempo = &s
	}
	if trx.TanggalPembayaranTerakhir != nil {
		s := trx.TanggalPembayaranTerakhir.UTC().Format(time.RFC3339)
		item.TanggalPembayaranTerakhir = &s
	}
	return item
}
