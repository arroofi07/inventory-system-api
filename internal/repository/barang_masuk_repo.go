package repository

import (
	"errors"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KolomSortBarangMasuk whitelist sort daftar penerimaan.
var KolomSortBarangMasuk = map[string]string{
	"tanggal_masuk": "barang_masuk.tanggal_masuk",
	"exp":           "barang_masuk.exp",
	"created_at":    "barang_masuk.created_at",
	"no_faktur":     "barang_masuk.no_faktur",
	"no_batch":      "barang_masuk.no_batch",
	"qty":           "barang_masuk.qty",
}

type BarangMasukRepo struct{}

func NewBarangMasukRepo() *BarangMasukRepo { return &BarangMasukRepo{} }

type BarangMasukListHasil struct {
	Items []domain.BarangMasuk
	Total int64
}

func (r *BarangMasukRepo) List(db *gorm.DB, q dto.BarangMasukListQuery) (*BarangMasukListHasil, error) {
	q.Normalize()
	return r.list(db, q, q.PerPage, q.Offset())
}

// ListForExport daftar tanpa batas paginasi UI (cap maxRows).
func (r *BarangMasukRepo) ListForExport(db *gorm.DB, q dto.BarangMasukListQuery, maxRows int) (*BarangMasukListHasil, error) {
	if maxRows < 1 {
		maxRows = 5000
	}
	if maxRows > 20000 {
		maxRows = 20000
	}
	q.Page = 1
	q.PerPage = maxRows
	return r.list(db, q, maxRows, 0)
}

func (r *BarangMasukRepo) list(db *gorm.DB, q dto.BarangMasukListQuery, limit, offset int) (*BarangMasukListHasil, error) {
	tx := db.Model(&domain.BarangMasuk{}).
		Joins("LEFT JOIN barang ON barang.id = barang_masuk.barang_id").
		Preload("Barang")

	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where(
			"barang_masuk.no_faktur LIKE ? OR barang_masuk.no_batch LIKE ? OR barang.kode_barang LIKE ? OR barang.nama_item LIKE ?",
			like, like, like, like,
		)
	}
	if q.Brand != "" {
		tx = tx.Where("barang.brand = ?", q.Brand)
	}
	if q.KodeBarang != "" {
		tx = tx.Where("barang.kode_barang = ?", q.KodeBarang)
	}
	if q.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", q.DateFrom); err == nil {
			tx = tx.Where("barang_masuk.tanggal_masuk >= ?", t.Format("2006-01-02"))
		}
	}
	if q.DateTo != "" {
		if t, err := time.Parse("2006-01-02", q.DateTo); err == nil {
			tx = tx.Where("barang_masuk.tanggal_masuk <= ?", t.Format("2006-01-02"))
		}
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	tx = query.TerapkanSort(tx, q.Sort, KolomSortBarangMasuk, "barang_masuk.tanggal_masuk DESC, barang_masuk.id DESC")
	var items []domain.BarangMasuk
	if err := tx.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return &BarangMasukListHasil{Items: items, Total: total}, nil
}

func (r *BarangMasukRepo) FindByID(db *gorm.DB, id uint64) (*domain.BarangMasuk, error) {
	var bm domain.BarangMasuk
	err := db.Preload("Barang").First(&bm, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &bm, err
}

func (r *BarangMasukRepo) Create(db *gorm.DB, bm *domain.BarangMasuk) error {
	err := db.Create(bm).Error
	return mapMySQLDuplicate(err)
}

func (r *BarangMasukRepo) Update(db *gorm.DB, bm *domain.BarangMasuk) error {
	err := db.Save(bm).Error
	return mapMySQLDuplicate(err)
}

func (r *BarangMasukRepo) Delete(db *gorm.DB, id uint64) error {
	res := db.Delete(&domain.BarangMasuk{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

// LockBarangForUpdate mengunci baris barang untuk update stok.
func (r *BarangMasukRepo) LockBarangForUpdate(db *gorm.DB, barangID uint64) (*domain.Barang, error) {
	var b domain.Barang
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, barangID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &b, err
}
