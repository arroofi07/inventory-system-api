package repository

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KolomSortBarang whitelist sort daftar barang.
var KolomSortBarang = map[string]string{
	"kode_barang":   "kode_barang",
	"nama_item":     "nama_item",
	"brand":         "brand",
	"stok_tersedia": "stok_tersedia",
	"created_at":    "created_at",
	"updated_at":    "updated_at",
}

type BarangRepo struct{}

func NewBarangRepo() *BarangRepo { return &BarangRepo{} }

type BarangListHasil struct {
	Items []domain.Barang
	Total int64
}

func (r *BarangRepo) List(db *gorm.DB, q dto.BarangListQuery) (*BarangListHasil, error) {
	q.Normalize()
	tx := db.Model(&domain.Barang{})

	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where("kode_barang LIKE ? OR nama_item LIKE ? OR brand LIKE ?", like, like, like)
	}
	if q.Brand != "" {
		tx = tx.Where("brand = ?", q.Brand)
	}
	// Default: hanya aktif. include_inactive=true → semua (histori tetap terlihat).
	// is_active eksplisit mengalahkan default.
	if q.IsActive != nil {
		tx = tx.Where("is_active = ?", *q.IsActive)
	} else if !q.IncludeInactive {
		tx = tx.Where("is_active = ?", true)
	}
	if q.WithStock {
		tx = tx.Where("stok_tersedia > 0")
	}
	switch q.StatusStok {
	case "HABIS":
		tx = tx.Where("stok_tersedia <= 0")
	case "RENDAH":
		tx = tx.Where("stok_tersedia > 0 AND stok_tersedia <= min_stock")
	case "NORMAL":
		tx = tx.Where("stok_tersedia > min_stock")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	defaultSort := "kode_barang ASC"
	if q.Popular {
		// Popular: sering diterima / stok aktif — cukup untuk autocomplete kosong.
		defaultSort = "stok_tersedia DESC, updated_at DESC"
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortBarang, defaultSort)
	var items []domain.Barang
	if err := tx.Offset(q.Offset()).Limit(q.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &BarangListHasil{Items: items, Total: total}, nil
}

func (r *BarangRepo) FindByID(db *gorm.DB, id uint64) (*domain.Barang, error) {
	var b domain.Barang
	err := db.First(&b, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &b, err
}

func (r *BarangRepo) FindByKode(db *gorm.DB, kode string) (*domain.Barang, error) {
	var b domain.Barang
	err := db.Where("kode_barang = ?", kode).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &b, err
}

// FindByKodes memuat banyak SKU sekaligus.
func (r *BarangRepo) FindByKodes(db *gorm.DB, kodes []string) ([]domain.Barang, error) {
	if len(kodes) == 0 {
		return nil, nil
	}
	var items []domain.Barang
	if err := db.Where("kode_barang IN ?", kodes).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// LockByIDs mengunci baris barang FOR UPDATE diurutkan id ASC (cegah deadlock).
func (r *BarangRepo) LockByIDs(db *gorm.DB, ids []uint64) ([]domain.Barang, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []domain.Barang
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *BarangRepo) Create(db *gorm.DB, b *domain.Barang) error {
	err := db.Create(b).Error
	return mapMySQLDuplicate(err)
}

func (r *BarangRepo) Update(db *gorm.DB, b *domain.Barang) error {
	err := db.Save(b).Error
	return mapMySQLDuplicate(err)
}

func (r *BarangRepo) SetActive(db *gorm.DB, id uint64, active bool) error {
	res := db.Model(&domain.Barang{}).Where("id = ?", id).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

func mapMySQLDuplicate(err error) error {
	if err == nil {
		return nil
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return domain.ErrDuplikat
	}
	return err
}
