package service

import (
	"context"
	"fmt"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// DashboardService GET /dashboard per role (SE-05).
type DashboardService struct {
	db    *gorm.DB
	trx   *repository.TransaksiRepo
	clock clock.Clock
}

func NewDashboardService(db *gorm.DB, trx *repository.TransaksiRepo, clk clock.Clock) *DashboardService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &DashboardService{db: db, trx: trx, clock: clk}
}

func (s *DashboardService) Ambil(ctx context.Context, userID uint64, role domain.Role) (*dto.DashboardResponse, error) {
	db := s.db.WithContext(ctx)
	now := s.clock.Now().UTC()
	periode := now.Format("2006-01")

	out := &dto.DashboardResponse{
		Role: string(role),
		AktivitasTerkini: dto.DashboardAktivitas{
			TransaksiPending: []dto.DashboardAktivitasItem{},
			BarangMasuk:      []dto.DashboardAktivitasItem{},
			Pembayaran:       []dto.DashboardAktivitasItem{},
		},
	}

	lihatSemua := role.Punya(domain.PermTransaksiLihatSemua)
	lihatPiutang := role.Punya(domain.PermPembayaranLihat)
	lihatNilaiPenjualan := role == domain.RoleSuperAdmin || role == domain.RoleAdmin || role == domain.RoleSales
	// Afiliasi: tanpa piutang & nilai penjualan; tetap stok + pending volume bila punya lihat semua.
	if role == domain.RoleAfiliasi {
		lihatNilaiPenjualan = false
		lihatPiutang = false
	}

	var salesFilter *uint64
	if role == domain.RoleSales {
		salesFilter = &userID
	}

	// Pending count
	if lihatSemua || role == domain.RoleSales {
		n, err := s.countPending(db, salesFilter)
		if err != nil {
			return nil, err
		}
		out.Kartu.TransaksiPending = &n
		items, err := s.listPending(db, salesFilter, 5)
		if err != nil {
			return nil, err
		}
		out.AktivitasTerkini.TransaksiPending = items
	}

	// Penjualan nilai bulan ini (admin/SA/sales)
	if lihatNilaiPenjualan {
		sum, err := s.sumPenjualanBulan(db, periode, salesFilter)
		if err != nil {
			return nil, err
		}
		d, _ := decimal.NewFromString(sum)
		sSum := dto.FormatUang(d)
		out.Kartu.PenjualanBulanIni = &sSum
	}

	// Jumlah trx bulan (afiliasi / semua)
	if role == domain.RoleAfiliasi || lihatSemua || role == domain.RoleSales {
		n, err := s.countTrxBulan(db, periode, salesFilter)
		if err != nil {
			return nil, err
		}
		out.Kartu.JumlahTransaksiBulanIni = &n
	}

	// Piutang
	if lihatPiutang {
		ringkas, err := s.trx.RingkasanPiutang(db, dto.PiutangListQuery{}, now)
		if err != nil {
			return nil, err
		}
		tp := ringkas.TotalPiutang
		po := ringkas.PiutangOverdue
		out.Kartu.TotalPiutang = &tp
		out.Kartu.PiutangOverdue = &po
		over, dekat, err := s.trx.HitungNotifikasiPiutang(db, now, 7)
		if err != nil {
			return nil, err
		}
		out.NotifikasiPiutang = &dto.NotifikasiPiutangResponse{
			Overdue: over, MendekatiJatuhTempo: dekat,
		}
		pays, err := s.listPembayaran(db, 5)
		if err != nil {
			return nil, err
		}
		out.AktivitasTerkini.Pembayaran = pays
	}

	// Stok (semua role yang punya barang.lihat — semua role punya)
	if role.Punya(domain.PermBarangLihat) {
		rendah, habis, err := s.countStok(db)
		if err != nil {
			return nil, err
		}
		out.Kartu.SKUStokRendah = &rendah
		out.Kartu.SKUStokHabis = &habis
		batchN, err := s.countBatchExp(db, now)
		if err != nil {
			return nil, err
		}
		out.Kartu.BatchMendekatiExp = &batchN
	}

	if role.Punya(domain.PermBarangMasukLihat) {
		bm, err := s.listBarangMasuk(db, 5)
		if err != nil {
			return nil, err
		}
		out.AktivitasTerkini.BarangMasuk = bm
	}

	return out, nil
}

func (s *DashboardService) countPending(db *gorm.DB, salesID *uint64) (int, error) {
	q := db.Model(&domain.TransaksiPenjualan{}).Where("status_approval = ?", domain.ApprovalPending)
	if salesID != nil {
		q = q.Where("sales_id = ?", *salesID)
	}
	var n int64
	err := q.Count(&n).Error
	return int(n), err
}

func (s *DashboardService) countTrxBulan(db *gorm.DB, periode string, salesID *uint64) (int, error) {
	q := db.Model(&domain.TransaksiPenjualan{}).
		Where("periode = ? AND status_approval = ?", periode, domain.ApprovalApproved)
	if salesID != nil {
		q = q.Where("sales_id = ?", *salesID)
	}
	var n int64
	err := q.Count(&n).Error
	return int(n), err
}

func (s *DashboardService) sumPenjualanBulan(db *gorm.DB, periode string, salesID *uint64) (string, error) {
	q := db.Model(&domain.TransaksiPenjualan{}).
		Select("COALESCE(SUM(total_akhir),0)").
		Where("periode = ? AND status_approval = ?", periode, domain.ApprovalApproved)
	if salesID != nil {
		q = q.Where("sales_id = ?", *salesID)
	}
	var sum string
	err := q.Scan(&sum).Error
	if sum == "" {
		sum = "0"
	}
	return sum, err
}

func (s *DashboardService) countStok(db *gorm.DB) (rendah, habis int, err error) {
	var h, r int64
	if err = db.Model(&domain.Barang{}).Where("is_active = 1 AND stok_tersedia <= 0").Count(&h).Error; err != nil {
		return
	}
	if err = db.Model(&domain.Barang{}).
		Where("is_active = 1 AND stok_tersedia > 0 AND stok_tersedia <= min_stock").
		Count(&r).Error; err != nil {
		return
	}
	return int(r), int(h), nil
}

func (s *DashboardService) countBatchExp(db *gorm.DB, now time.Time) (int, error) {
	// Batch dengan qty>0 dan exp dalam expiry_alert_days barang
	hari := now.Format("2006-01-02")
	var n int64
	err := db.Raw(`
		SELECT COUNT(*) FROM barang_masuk bm
		INNER JOIN barang b ON b.id = bm.barang_id AND b.is_active = 1
		WHERE bm.qty > 0
		  AND bm.exp >= ?
		  AND bm.exp <= DATE_ADD(?, INTERVAL b.expiry_alert_days DAY)
	`, hari, hari).Scan(&n).Error
	return int(n), err
}

func (s *DashboardService) listPending(db *gorm.DB, salesID *uint64, limit int) ([]dto.DashboardAktivitasItem, error) {
	q := db.Model(&domain.TransaksiPenjualan{}).
		Where("status_approval = ?", domain.ApprovalPending).
		Order("created_at DESC").Limit(limit)
	if salesID != nil {
		q = q.Where("sales_id = ?", *salesID)
	}
	var rows []domain.TransaksiPenjualan
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]dto.DashboardAktivitasItem, 0, len(rows))
	for _, t := range rows {
		nom := dto.FormatUang(t.TotalAkhir)
		out = append(out, dto.DashboardAktivitasItem{
			ID: t.ID,
			Judul: fmt.Sprintf("#%d · %s", t.ID, t.NamaPelanggan),
			Subjudul: t.KodePelanggan,
			Nominal: &nom,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *DashboardService) listBarangMasuk(db *gorm.DB, limit int) ([]dto.DashboardAktivitasItem, error) {
	var rows []domain.BarangMasuk
	if err := db.Preload("Barang").Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]dto.DashboardAktivitasItem, 0, len(rows))
	for _, bm := range rows {
		judul := bm.NoFaktur
		sub := bm.NoBatch
		if bm.Barang != nil {
			judul = bm.Barang.NamaItem
			sub = fmt.Sprintf("%s · %s", bm.Barang.KodeBarang, bm.NoBatch)
		}
		out = append(out, dto.DashboardAktivitasItem{
			ID: bm.ID, Judul: judul, Subjudul: sub,
			CreatedAt: bm.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *DashboardService) listPembayaran(db *gorm.DB, limit int) ([]dto.DashboardAktivitasItem, error) {
	type row struct {
		ID                uint64
		NominalPembayaran string
		TransaksiID       uint64
		ChangedAt         time.Time
		NamaPelanggan     string
	}
	var rows []row
	err := db.Raw(`
		SELECT r.id, CAST(r.nominal_pembayaran AS CHAR) AS nominal_pembayaran,
		       r.transaksi_penjualan_id AS transaksi_id, r.changed_at, t.nama_pelanggan
		FROM riwayat_pembayaran r
		INNER JOIN transaksi_penjualan t ON t.id = r.transaksi_penjualan_id
		ORDER BY r.changed_at DESC
		LIMIT ?
	`, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.DashboardAktivitasItem, 0, len(rows))
	for _, r := range rows {
		nom := r.NominalPembayaran
		out = append(out, dto.DashboardAktivitasItem{
			ID: r.ID,
			Judul: fmt.Sprintf("Bayar trx #%d · %s", r.TransaksiID, r.NamaPelanggan),
			Nominal: &nom,
			CreatedAt: r.ChangedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}
