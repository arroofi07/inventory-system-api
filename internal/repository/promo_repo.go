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

var KolomSortPromo = map[string]string{
	"kode_promo":       "kode_promo",
	"nama_promo":       "nama_promo",
	"tipe_promo":       "tipe_promo",
	"tanggal_mulai":    "tanggal_mulai",
	"tanggal_berakhir": "tanggal_berakhir",
	"created_at":       "created_at",
	"updated_at":       "updated_at",
}

type PromoRepo struct{}

func NewPromoRepo() *PromoRepo { return &PromoRepo{} }

type PromoListHasil struct {
	Items []domain.Promo
	Total int64
}

func (r *PromoRepo) List(db *gorm.DB, q dto.PromoListQuery, hariIni time.Time) (*PromoListHasil, error) {
	q.Normalize()
	tx := db.Model(&domain.Promo{})

	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where("kode_promo LIKE ? OR nama_promo LIKE ?", like, like)
	}
	if q.TipePromo != "" {
		tx = tx.Where("tipe_promo = ?", q.TipePromo)
	}
	if q.KodeBarang != "" {
		// Promo global (kode_barang NULL) atau cocok SKU.
		tx = tx.Where("kode_barang IS NULL OR kode_barang = ?", q.KodeBarang)
	}
	if q.Aktif != nil && *q.Aktif {
		hari := hariIni.Format("2006-01-02")
		tx = tx.Where("is_active = ? AND tanggal_mulai <= ? AND tanggal_berakhir >= ?", true, hari, hari)
	} else if q.IsActive != nil {
		tx = tx.Where("is_active = ?", *q.IsActive)
	} else if !q.IncludeInactive {
		tx = tx.Where("is_active = ?", true)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortPromo, "tanggal_mulai DESC, kode_promo ASC")
	var items []domain.Promo
	if err := tx.Offset(q.Offset()).Limit(q.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PromoListHasil{Items: items, Total: total}, nil
}

func (r *PromoRepo) FindByID(db *gorm.DB, id uint64) (*domain.Promo, error) {
	var p domain.Promo
	err := db.First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &p, err
}

func (r *PromoRepo) FindByKode(db *gorm.DB, kode string) (*domain.Promo, error) {
	var p domain.Promo
	err := db.Where("kode_promo = ?", kode).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &p, err
}

func (r *PromoRepo) Create(db *gorm.DB, p *domain.Promo) error {
	return mapMySQLDuplicate(db.Create(p).Error)
}

func (r *PromoRepo) Update(db *gorm.DB, p *domain.Promo) error {
	return mapMySQLDuplicate(db.Save(p).Error)
}

func (r *PromoRepo) SetActive(db *gorm.DB, id uint64, active bool) error {
	res := db.Model(&domain.Promo{}).Where("id = ?", id).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

func (r *PromoRepo) Delete(db *gorm.DB, id uint64) error {
	res := db.Delete(&domain.Promo{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

// NextKodePromo menghasilkan PROMO+YYYYMM+4 digit dengan lock baris bulan berjalan.
func (r *PromoRepo) NextKodePromo(db *gorm.DB, sekarang time.Time) (string, error) {
	prefix := "PROMO" + sekarang.Format("200601")
	var terakhir domain.Promo
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kode_promo LIKE ?", prefix+"%").
		Order("kode_promo DESC").
		Limit(1).
		Find(&terakhir).Error
	if err != nil {
		return "", err
	}
	urut := 1
	if terakhir.ID != 0 && len(terakhir.KodePromo) >= 4 {
		suf := terakhir.KodePromo[len(terakhir.KodePromo)-4:]
		var n int
		if _, err := fmt.Sscanf(suf, "%d", &n); err == nil {
			urut = n + 1
		}
	}
	if urut > 9999 {
		return "", fmt.Errorf("urutan kode promo bulan ini penuh")
	}
	return fmt.Sprintf("%s%04d", prefix, urut), nil
}
