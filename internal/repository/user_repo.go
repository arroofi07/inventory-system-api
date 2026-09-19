package repository

import (
	"strings"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/query"
	"gorm.io/gorm"
)

var KolomSortUser = map[string]string{
	"name":       "name",
	"email":      "email",
	"role":       "role",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

type UserListHasil struct {
	Items []domain.User
	Total int64
}

func (r *UserRepo) List(db *gorm.DB, q dto.UserListQuery) (*UserListHasil, error) {
	q.Normalize()
	tx := db.Model(&domain.User{})

	if q.Q != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where("name LIKE ? OR email LIKE ? OR no_hp LIKE ?", like, like, like)
	}
	if q.Role != "" {
		tx = tx.Where("role = ?", q.Role)
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
	tx = query.TerapkanSort(tx, q.Sort, KolomSortUser, "name ASC")
	var items []domain.User
	if err := tx.Offset(q.Offset()).Limit(q.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &UserListHasil{Items: items, Total: total}, nil
}

func (r *UserRepo) Update(db *gorm.DB, u *domain.User) error {
	return mapMySQLDuplicate(db.Save(u).Error)
}

func (r *UserRepo) Delete(db *gorm.DB, id uint64) error {
	res := db.Delete(&domain.User{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

func (r *UserRepo) SetActive(db *gorm.DB, id uint64, active bool) error {
	res := db.Model(&domain.User{}).Where("id = ?", id).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTidakDitemukan
	}
	return nil
}

func (r *UserRepo) CountActiveByRole(db *gorm.DB, role domain.Role) (int64, error) {
	var n int64
	err := db.Model(&domain.User{}).
		Where("role = ? AND is_active = ?", role, true).
		Count(&n).Error
	return n, err
}

// CreateMapped membungkus Create dengan deteksi duplikat email/ktp.
func (r *UserRepo) CreateMapped(db *gorm.DB, u *domain.User) error {
	return mapMySQLDuplicate(r.Create(db, u))
}
