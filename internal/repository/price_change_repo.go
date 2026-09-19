package repository

import (
	"app/internal/domain"
	"gorm.io/gorm"
)

type PriceChangeRepo struct{}

func NewPriceChangeRepo() *PriceChangeRepo { return &PriceChangeRepo{} }

func (r *PriceChangeRepo) Create(db *gorm.DB, log *domain.PriceChangeLog) error {
	return db.Create(log).Error
}

type PriceChangeListHasil struct {
	Items []domain.PriceChangeLog
	Total int64
}

func (r *PriceChangeRepo) ListByBarangID(db *gorm.DB, barangID uint64, page, perPage int) (*PriceChangeListHasil, error) {
	tx := db.Model(&domain.PriceChangeLog{}).Where("barang_id = ?", barangID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.PriceChangeLog
	err := tx.Order("changed_at DESC, id DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return &PriceChangeListHasil{Items: items, Total: total}, nil
}

// FindBarangMasukByIDsForBarang memuat batch yang milik SKU tertentu.
func (r *BarangMasukRepo) FindByIDsForBarang(db *gorm.DB, barangID uint64, ids []uint64) ([]domain.BarangMasuk, error) {
	var rows []domain.BarangMasuk
	err := db.Where("barang_id = ? AND id IN ?", barangID, ids).Find(&rows).Error
	return rows, err
}
