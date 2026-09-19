package repository

import (
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/query"
	"gorm.io/gorm"
)

// PiutangListHasil hasil daftar + agregat.
type PiutangListHasil struct {
	Items []domain.TransaksiPenjualan
	Total int64
}

func (r *TransaksiRepo) applyPiutangFilter(db *gorm.DB, q dto.PiutangListQuery, hariIni time.Time) *gorm.DB {
	tx := db.Model(&domain.TransaksiPenjualan{}).Where("status_approval = ?", domain.ApprovalApproved)
	if q.KodePelanggan != "" {
		tx = tx.Where("kode_pelanggan = ?", q.KodePelanggan)
	}
	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where(
			"kode_pelanggan LIKE ? OR nama_pelanggan LIKE ? OR IFNULL(no_transaksi,'') LIKE ?",
			like, like, like,
		)
	}
	if q.StatusPembayaran != "" {
		tx = tx.Where("status_pembayaran = ?", q.StatusPembayaran)
	}
	if q.Brand != "" {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM transaksi_detail td
			INNER JOIN barang b ON b.kode_barang = td.kode_item
			WHERE td.transaksi_penjualan_id = transaksi_penjualan.id AND b.brand = ?
		)`, q.Brand)
	}

	dateCol := "tanggal"
	switch strings.TrimSpace(q.DateType) {
	case "tanggal_jatuh_tempo":
		dateCol = "tanggal_jatuh_tempo"
	case "tanggal_pembayaran_terakhir":
		dateCol = "DATE(tanggal_pembayaran_terakhir)"
	case "tanggal_transaksi", "":
		dateCol = "tanggal"
	}
	if q.DateFrom != "" {
		tx = tx.Where(fmt.Sprintf("%s >= ?", dateCol), q.DateFrom)
	}
	if q.DateTo != "" {
		tx = tx.Where(fmt.Sprintf("%s <= ?", dateCol), q.DateTo)
	}

	ambang := 7
	hariStr := hariIni.UTC().Format("2006-01-02")
	katExpr := fmt.Sprintf(`CASE
		WHEN status_pembayaran = 'lunas' THEN 'lunas'
		WHEN tanggal_jatuh_tempo IS NULL THEN 'tanpa_jatuh_tempo'
		WHEN tanggal_jatuh_tempo < '%s' THEN 'overdue'
		WHEN tanggal_jatuh_tempo <= DATE_ADD('%s', INTERVAL %d DAY) THEN 'mendekati_jatuh_tempo'
		ELSE 'normal' END`, hariStr, hariStr, ambang)

	if q.HanyaOverdue || q.KategoriJatuhTempo == "overdue" {
		tx = tx.Where(katExpr + " = 'overdue'")
	} else if q.KategoriJatuhTempo != "" {
		tx = tx.Where(katExpr+" = ?", q.KategoriJatuhTempo)
	}
	return tx
}

// ListPiutang daftar transaksi approved untuk monitoring piutang (SD-05).
func (r *TransaksiRepo) ListPiutang(db *gorm.DB, q dto.PiutangListQuery, hariIni time.Time) (*PiutangListHasil, error) {
	q.Normalize()
	tx := r.applyPiutangFilter(db, q, hariIni)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortTransaksi, "tanggal_jatuh_tempo ASC, id ASC")
	var items []domain.TransaksiPenjualan
	if err := tx.Offset(q.Offset()).Limit(q.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PiutangListHasil{Items: items, Total: total}, nil
}

// ListPiutangExport sama filter ListPiutang tanpa cap paginasi UI.
func (r *TransaksiRepo) ListPiutangExport(db *gorm.DB, q dto.PiutangListQuery, hariIni time.Time, maxRows int) (*PiutangListHasil, error) {
	if maxRows < 1 {
		maxRows = 5000
	}
	tx := r.applyPiutangFilter(db, q, hariIni)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortTransaksi, "tanggal_jatuh_tempo ASC, id ASC")
	var items []domain.TransaksiPenjualan
	if err := tx.Limit(maxRows).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PiutangListHasil{Items: items, Total: total}, nil
}

// RingkasanPiutang agregat 04 §5.6 atas filter yang sama (tanpa paginasi).
func (r *TransaksiRepo) RingkasanPiutang(db *gorm.DB, q dto.PiutangListQuery, hariIni time.Time) (dto.PiutangRingkasan, error) {
	tx := r.applyPiutangFilter(db, q, hariIni)
	hariStr := hariIni.UTC().Format("2006-01-02")
	var row struct {
		TotalNilai     string
		TotalDibayar   string
		TotalPiutang   string
		PiutangOverdue string
		Jumlah         int
		JumlahOverdue  int
	}
	err := tx.Select(fmt.Sprintf(`
		COALESCE(SUM(total_akhir),0) AS total_nilai,
		COALESCE(SUM(jumlah_dibayar),0) AS total_dibayar,
		COALESCE(SUM(CASE WHEN status_pembayaran <> 'lunas' THEN sisa_hutang ELSE 0 END),0) AS total_piutang,
		COALESCE(SUM(CASE
			WHEN status_pembayaran <> 'lunas'
			 AND tanggal_jatuh_tempo IS NOT NULL
			 AND tanggal_jatuh_tempo < '%s'
			THEN sisa_hutang ELSE 0 END),0) AS piutang_overdue,
		COUNT(*) AS jumlah,
		COALESCE(SUM(CASE
			WHEN status_pembayaran <> 'lunas'
			 AND tanggal_jatuh_tempo IS NOT NULL
			 AND tanggal_jatuh_tempo < '%s'
			THEN 1 ELSE 0 END),0) AS jumlah_overdue
	`, hariStr, hariStr)).Scan(&row).Error
	if err != nil {
		return dto.PiutangRingkasan{}, err
	}
	return dto.PiutangRingkasan{
		TotalNilai:             formatDecScan(row.TotalNilai),
		TotalDibayar:           formatDecScan(row.TotalDibayar),
		TotalPiutang:           formatDecScan(row.TotalPiutang),
		PiutangOverdue:         formatDecScan(row.PiutangOverdue),
		JumlahTransaksi:        row.Jumlah,
		JumlahTransaksiOverdue: row.JumlahOverdue,
	}, nil
}

func formatDecScan(s string) string {
	if s == "" {
		return "0.00"
	}
	// MySQL DECIMAL scan as string may be "1234.5000"
	d, err := dto.ParseUangOpsional(s, "x")
	if err != nil {
		return s
	}
	return dto.FormatUang(d)
}

// HitungNotifikasiPiutang count overdue + mendekati (SD-06).
func (r *TransaksiRepo) HitungNotifikasiPiutang(db *gorm.DB, hariIni time.Time, ambang int) (overdue, mendekati int, err error) {
	if ambang < 1 {
		ambang = 7
	}
	hariStr := hariIni.UTC().Format("2006-01-02")
	var nOver, nDekat int64
	err = db.Model(&domain.TransaksiPenjualan{}).
		Where("status_approval = ? AND status_pembayaran <> ?", domain.ApprovalApproved, domain.PembayaranLunas).
		Where("tanggal_jatuh_tempo IS NOT NULL AND tanggal_jatuh_tempo < ?", hariStr).
		Count(&nOver).Error
	if err != nil {
		return 0, 0, err
	}
	err = db.Model(&domain.TransaksiPenjualan{}).
		Where("status_approval = ? AND status_pembayaran <> ?", domain.ApprovalApproved, domain.PembayaranLunas).
		Where("tanggal_jatuh_tempo IS NOT NULL AND tanggal_jatuh_tempo >= ? AND tanggal_jatuh_tempo <= DATE_ADD(?, INTERVAL ? DAY)",
			hariStr, hariStr, ambang).
		Count(&nDekat).Error
	return int(nOver), int(nDekat), err
}

// SumNominalPenerimaan SUM(riwayat.nominal_pembayaran) pada rentang tanggal (SD-06).
func (r *PembayaranRepo) SumNominalPenerimaan(db *gorm.DB, from, to string) (total string, jumlah int, err error) {
	var row struct {
		Total  string
		Jumlah int
	}
	err = db.Model(&domain.RiwayatPembayaran{}).
		Select("COALESCE(SUM(nominal_pembayaran),0) AS total, COUNT(*) AS jumlah").
		Where("tanggal_pembayaran >= ? AND tanggal_pembayaran <= ?", from, to).
		Scan(&row).Error
	if err != nil {
		return "0.00", 0, err
	}
	return formatDecScan(row.Total), row.Jumlah, nil
}

// ListByPelanggan semua riwayat pembayaran lintas transaksi pelanggan.
func (r *PembayaranRepo) ListByPelanggan(db *gorm.DB, kodePelanggan string, limit int) ([]domain.RiwayatPembayaran, error) {
	if limit < 1 {
		limit = 200
	}
	var rows []domain.RiwayatPembayaran
	err := db.Table("riwayat_pembayaran rp").
		Joins("INNER JOIN transaksi_penjualan tp ON tp.id = rp.transaksi_penjualan_id").
		Where("tp.kode_pelanggan = ?", kodePelanggan).
		Order("rp.changed_at DESC, rp.id DESC").
		Limit(limit).
		Select("rp.*").
		Scan(&rows).Error
	return rows, err
}
