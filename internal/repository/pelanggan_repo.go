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

var KolomSortPelanggan = map[string]string{
	"kode_pelanggan": "kode_pelanggan",
	"nama_pelanggan": "nama_pelanggan",
	"territory":      "territory",
	"distrik":        "distrik",
	"created_at":     "created_at",
	"updated_at":     "updated_at",
}

type PelangganRepo struct{}

func NewPelangganRepo() *PelangganRepo { return &PelangganRepo{} }

type PelangganListHasil struct {
	Items []domain.Pelanggan
	Total int64
}

func (r *PelangganRepo) List(db *gorm.DB, q dto.PelangganListQuery) (*PelangganListHasil, error) {
	q.Normalize()
	return r.list(db, q, q.PerPage, q.Offset())
}

// ListForExport daftar tanpa batas paginasi UI (cap maxRows).
func (r *PelangganRepo) ListForExport(db *gorm.DB, q dto.PelangganListQuery, maxRows int) (*PelangganListHasil, error) {
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

func (r *PelangganRepo) list(db *gorm.DB, q dto.PelangganListQuery, limit, offset int) (*PelangganListHasil, error) {
	tx := db.Model(&domain.Pelanggan{})

	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where("kode_pelanggan LIKE ? OR nama_pelanggan LIKE ? OR phone LIKE ?", like, like, like)
	}
	if q.ChannelOutlet != "" {
		tx = tx.Where("channel_outlet = ?", q.ChannelOutlet)
	}
	if q.Territory != "" {
		tx = tx.Where("territory = ?", q.Territory)
	}
	if q.Distrik != "" {
		tx = tx.Where("distrik = ?", q.Distrik)
	}
	if q.IsActive != nil {
		tx = tx.Where("is_active = ?", *q.IsActive)
	} else if !q.IncludeInactive {
		tx = tx.Where("is_active = ?", true)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	tx = query.TerapkanSort(tx, q.Sort, KolomSortPelanggan, "kode_pelanggan ASC")
	var items []domain.Pelanggan
	if err := tx.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PelangganListHasil{Items: items, Total: total}, nil
}

func (r *PelangganRepo) FindByID(db *gorm.DB, id uint64) (*domain.Pelanggan, error) {
	var p domain.Pelanggan
	err := db.First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &p, err
}

func (r *PelangganRepo) FindByKode(db *gorm.DB, kode string) (*domain.Pelanggan, error) {
	var p domain.Pelanggan
	err := db.Where("kode_pelanggan = ?", kode).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	return &p, err
}

func (r *PelangganRepo) Create(db *gorm.DB, p *domain.Pelanggan) error {
	return mapMySQLDuplicate(db.Create(p).Error)
}

func (r *PelangganRepo) Update(db *gorm.DB, p *domain.Pelanggan) error {
	return mapMySQLDuplicate(db.Save(p).Error)
}

func (r *PelangganRepo) SetActive(db *gorm.DB, id uint64, active bool) error {
	res := db.Model(&domain.Pelanggan{}).Where("id = ?", id).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

func (r *PelangganRepo) PunyaTransaksi(db *gorm.DB, kode string) (bool, error) {
	var n int64
	err := db.Model(&domain.TransaksiPenjualan{}).
		Where("kode_pelanggan = ?", kode).
		Count(&n).Error
	return n > 0, err
}

// NextKodePelanggan menghasilkan PLG+YYYYMM+4 digit dengan lock baris bulan berjalan.
func (r *PelangganRepo) NextKodePelanggan(db *gorm.DB, sekarang time.Time) (string, error) {
	prefix := "PLG" + sekarang.Format("200601")
	var terakhir domain.Pelanggan
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kode_pelanggan LIKE ?", prefix+"%").
		Order("kode_pelanggan DESC").
		Limit(1).
		Find(&terakhir).Error
	if err != nil {
		return "", err
	}
	urut := 1
	if terakhir.ID != 0 && len(terakhir.KodePelanggan) >= 4 {
		suf := terakhir.KodePelanggan[len(terakhir.KodePelanggan)-4:]
		var n int
		if _, err := fmt.Sscanf(suf, "%d", &n); err == nil {
			urut = n + 1
		}
	}
	if urut > 9999 {
		return "", fmt.Errorf("urutan kode pelanggan bulan ini penuh")
	}
	return fmt.Sprintf("%s%04d", prefix, urut), nil
}
