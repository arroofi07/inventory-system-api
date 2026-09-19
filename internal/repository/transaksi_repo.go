package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var KolomSortTransaksi = map[string]string{
	"id":          "id",
	"tanggal":     "tanggal",
	"total_akhir": "total_akhir",
	"created_at":  "created_at",
	"updated_at":  "updated_at",
}

type TransaksiRepo struct{}

func NewTransaksiRepo() *TransaksiRepo { return &TransaksiRepo{} }

type TransaksiListHasil struct {
	Items []domain.TransaksiPenjualan
	Total int64
}

// CreatePending menyimpan header + detail + promo snapshot.
// Caller mengatur transaksi DB bila perlu.
func (r *TransaksiRepo) CreatePending(db *gorm.DB, trx *domain.TransaksiPenjualan) error {
	details := trx.Details
	trx.Details = nil
	if err := db.Omit(clause.Associations).Create(trx).Error; err != nil {
		return err
	}
	for i := range details {
		d := details[i]
		d.ID = 0
		d.TransaksiPenjualanID = trx.ID
		promos := d.Promos
		d.Promos = nil
		if err := db.Omit(clause.Associations).Create(&d).Error; err != nil {
			return err
		}
		for j := range promos {
			p := promos[j]
			p.ID = 0
			p.TransaksiDetailID = d.ID
			if err := db.Create(&p).Error; err != nil {
				return err
			}
		}
		d.Promos = promos
		details[i] = d
	}
	trx.Details = details
	return nil
}

// FindByID memuat transaksi beserta detail dan promo.
func (r *TransaksiRepo) FindByID(db *gorm.DB, id uint64) (*domain.TransaksiPenjualan, error) {
	var trx domain.TransaksiPenjualan
	err := db.Preload("Details", func(db *gorm.DB) *gorm.DB {
		return db.Order("urutan ASC, id ASC")
	}).Preload("Details.Promos").First(&trx, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &trx, err
}

// LockByID mengunci header transaksi FOR UPDATE (SD-02).
func (r *TransaksiRepo) LockByID(db *gorm.DB, id uint64) (*domain.TransaksiPenjualan, error) {
	var trx domain.TransaksiPenjualan
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&trx, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, err
	}
	return &trx, nil
}

// UpdatePembayaran memperbarui field pembayaran header.
func (r *TransaksiRepo) UpdatePembayaran(db *gorm.DB, trx *domain.TransaksiPenjualan) error {
	return db.Model(trx).Select(
		"status_pembayaran", "jumlah_dibayar", "sisa_hutang",
		"tanggal_jatuh_tempo", "tanggal_pembayaran_terakhir", "keterangan_pembayaran", "updated_at",
	).Updates(trx).Error
}

// List header saja (tanpa detail) dengan filter SC-07.
func (r *TransaksiRepo) List(db *gorm.DB, q dto.TransaksiListQuery) (*TransaksiListHasil, error) {
	q.Normalize()
	tx := db.Model(&domain.TransaksiPenjualan{})
	tx = applyTransaksiListFilters(tx, q)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortTransaksi, "tanggal DESC, id DESC")
	var items []domain.TransaksiPenjualan
	if err := tx.Offset(q.Offset()).Limit(q.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &TransaksiListHasil{Items: items, Total: total}, nil
}

// ListForExport daftar tanpa cap paginasi UI (maxRows, default 5000).
func (r *TransaksiRepo) ListForExport(db *gorm.DB, q dto.TransaksiListQuery, maxRows int) (*TransaksiListHasil, error) {
	if maxRows < 1 {
		maxRows = 5000
	}
	if maxRows > 20000 {
		maxRows = 20000
	}
	tx := db.Model(&domain.TransaksiPenjualan{})
	tx = applyTransaksiListFilters(tx, q)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortTransaksi, "tanggal DESC, id DESC")
	var items []domain.TransaksiPenjualan
	if err := tx.Limit(maxRows).Find(&items).Error; err != nil {
		return nil, err
	}
	return &TransaksiListHasil{Items: items, Total: total}, nil
}

func applyTransaksiListFilters(tx *gorm.DB, q dto.TransaksiListQuery) *gorm.DB {
	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where(
			"kode_pelanggan LIKE ? OR nama_pelanggan LIKE ? OR CAST(id AS CHAR) LIKE ? OR IFNULL(no_transaksi,'') LIKE ?",
			like, like, like, like,
		)
	}
	if q.StatusApproval != "" {
		tx = tx.Where("status_approval = ?", q.StatusApproval)
	}
	if q.StatusPembayaran != "" {
		tx = tx.Where("status_pembayaran = ?", q.StatusPembayaran)
	}
	if q.SalesID != nil {
		tx = tx.Where("sales_id = ?", *q.SalesID)
	}
	if strings.TrimSpace(q.ChannelOutlet) != "" {
		tx = tx.Where("channel_outlet = ?", strings.TrimSpace(q.ChannelOutlet))
	}
	if q.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(q.DateFrom), time.UTC); err == nil {
			tx = tx.Where("tanggal >= ?", t.Format("2006-01-02"))
		}
	}
	if q.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(q.DateTo), time.UTC); err == nil {
			tx = tx.Where("tanggal <= ?", t.Format("2006-01-02"))
		}
	}
	switch strings.TrimSpace(strings.ToLower(q.KecukupanStok)) {
	case "cukup":
		tx = tx.Where(`status_approval = 'pending' AND NOT EXISTS (
			SELECT 1
			FROM (
				SELECT td.kode_item AS kode_item, SUM(td.total_qty_keluar) AS need
				FROM transaksi_detail td
				WHERE td.transaksi_penjualan_id = transaksi_penjualan.id
				GROUP BY td.kode_item
			) need
			LEFT JOIN barang b ON b.kode_barang = need.kode_item
			WHERE COALESCE(b.stok_tersedia, 0) < need.need
		)`)
	case "kurang":
		tx = tx.Where(`status_approval = 'pending' AND EXISTS (
			SELECT 1
			FROM (
				SELECT td.kode_item AS kode_item, SUM(td.total_qty_keluar) AS need
				FROM transaksi_detail td
				WHERE td.transaksi_penjualan_id = transaksi_penjualan.id
				GROUP BY td.kode_item
			) need
			LEFT JOIN barang b ON b.kode_barang = need.kode_item
			WHERE COALESCE(b.stok_tersedia, 0) < need.need
		)`)
	}
	return tx
}

// RingkasanList agregat total_akhir untuk filter yang sama dengan List.
func (r *TransaksiRepo) RingkasanList(db *gorm.DB, q dto.TransaksiListQuery) (jumlah int64, totalAkhir string, err error) {
	tx := applyTransaksiListFilters(db.Model(&domain.TransaksiPenjualan{}), q)
	type agg struct {
		Jumlah int64
		Total  string
	}
	var a agg
	err = tx.Select("COUNT(*) AS jumlah, CAST(COALESCE(SUM(total_akhir),0) AS CHAR) AS total").Scan(&a).Error
	if err != nil {
		return 0, "0.00", err
	}
	return a.Jumlah, a.Total, nil
}

// NextNoTransaksi mengambil nomor berikutnya dari counter atomik (FOR UPDATE).
func (r *TransaksiRepo) NextNoTransaksi(db *gorm.DB) (string, error) {
	var last int64
	if err := db.Raw(`SELECT last_number FROM no_transaksi_seq WHERE id = 1 FOR UPDATE`).Scan(&last).Error; err != nil {
		return "", err
	}
	next := last + 1
	if err := db.Exec(`UPDATE no_transaksi_seq SET last_number = ? WHERE id = 1`, next).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", next), nil
}

// ListPendingBersaing mengembalikan pending lain yang memakai salah satu SKU.
func (r *TransaksiRepo) ListPendingBersaing(db *gorm.DB, kodeItems []string, excludeID uint64, limit int) ([]domain.TransaksiPenjualan, error) {
	if len(kodeItems) == 0 {
		return nil, nil
	}
	if limit < 1 {
		limit = 20
	}
	var ids []uint64
	err := db.Raw(`
		SELECT DISTINCT td.transaksi_penjualan_id
		FROM transaksi_detail td
		INNER JOIN transaksi_penjualan tp ON tp.id = td.transaksi_penjualan_id
		WHERE tp.status_approval = 'pending'
		  AND tp.id <> ?
		  AND td.kode_item IN ?
		ORDER BY tp.id ASC
		LIMIT ?
	`, excludeID, kodeItems, limit).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var items []domain.TransaksiPenjualan
	if err := db.Preload("Details").Where("id IN ?", ids).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// MarkApproved mengupdate header setelah gate sukses.
func (r *TransaksiRepo) MarkApproved(db *gorm.DB, trx *domain.TransaksiPenjualan) error {
	return db.Model(trx).Select(
		"no_transaksi", "status_approval", "approved_at", "approved_by", "approval_notes",
		"fulfillment_status", "updated_at",
	).Updates(trx).Error
}

// MarkRejected menandai rejected terminal.
func (r *TransaksiRepo) MarkRejected(db *gorm.DB, trx *domain.TransaksiPenjualan) error {
	return db.Model(trx).Select(
		"status_approval", "approved_at", "approved_by", "approval_notes", "updated_at",
	).Updates(trx).Error
}

// HitungKecukupanStok mengembalikan map id→cukup untuk pending (SC-09 indikator daftar).
func (r *TransaksiRepo) HitungKecukupanStok(db *gorm.DB, ids []uint64) (map[uint64]bool, error) {
	out := make(map[uint64]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	type baris struct {
		TransaksiID uint64 `gorm:"column:transaksi_id"`
		Kurang      int    `gorm:"column:kurang"`
	}
	var rows []baris
	err := db.Raw(`
		SELECT need.transaksi_id AS transaksi_id,
		       SUM(CASE WHEN COALESCE(b.stok_tersedia, 0) < need.need THEN 1 ELSE 0 END) AS kurang
		FROM (
			SELECT td.transaksi_penjualan_id AS transaksi_id,
			       td.kode_item AS kode_item,
			       SUM(td.total_qty_keluar) AS need
			FROM transaksi_detail td
			WHERE td.transaksi_penjualan_id IN ?
			GROUP BY td.transaksi_penjualan_id, td.kode_item
		) need
		LEFT JOIN barang b ON b.kode_barang = need.kode_item
		GROUP BY need.transaksi_id
	`, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	for _, row := range rows {
		out[row.TransaksiID] = row.Kurang == 0
	}
	return out, nil
}

// ReplaceDetailsAndHeader mengganti seluruh baris detail lalu memperbarui header
// (hitung ulang dari nol — SC-04). Cascade hapus promo via FK.
func (r *TransaksiRepo) ReplaceDetailsAndHeader(db *gorm.DB, trx *domain.TransaksiPenjualan, details []domain.TransaksiDetail) error {
	if err := db.Where("transaksi_penjualan_id = ?", trx.ID).Delete(&domain.TransaksiDetail{}).Error; err != nil {
		return err
	}
	for i := range details {
		d := details[i]
		d.ID = 0
		d.TransaksiPenjualanID = trx.ID
		promos := d.Promos
		d.Promos = nil
		if err := db.Omit(clause.Associations).Create(&d).Error; err != nil {
			return err
		}
		for j := range promos {
			p := promos[j]
			p.ID = 0
			p.TransaksiDetailID = d.ID
			if err := db.Create(&p).Error; err != nil {
				return err
			}
		}
		d.Promos = promos
		details[i] = d
	}
	trx.Details = details
	return db.Model(trx).Select(
		"is_multi_item", "total", "ppn_nominal", "total_akhir",
		"jumlah_item", "total_qty_ditagih", "total_qty_keluar",
		"status_pembayaran", "jumlah_dibayar", "sisa_hutang", "tanggal_jatuh_tempo",
		"updated_at",
	).Updates(trx).Error
}
