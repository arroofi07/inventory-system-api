package repository

import (
	"encoding/json"
	"errors"
	"time"

	"app/internal/domain"
	"gorm.io/gorm"
)

type UserRepo struct{}

func NewUserRepo() *UserRepo { return &UserRepo{} }

func (r *UserRepo) FindByEmail(db *gorm.DB, email string) (*domain.User, error) {
	var u domain.User
	err := db.Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) FindByID(db *gorm.DB, id uint64) (*domain.User, error) {
	var u domain.User
	err := db.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTidakDitemukan
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Create(db *gorm.DB, u *domain.User) error {
	// Select agar field bool zero-value (is_active=false) ikut tersimpan.
	return db.Select("Name", "Email", "Password", "Role", "NoHP", "NoKTP", "Alamat", "JenisKelamin", "IsActive", "EmailVerifiedAt").
		Create(u).Error
}

type TokenRepo struct{}

func NewTokenRepo() *TokenRepo { return &TokenRepo{} }

func (r *TokenRepo) Create(db *gorm.DB, rt *domain.RefreshToken) error {
	return db.Create(rt).Error
}

func (r *TokenRepo) FindByHash(db *gorm.DB, hash string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := db.Where("token_hash = ?", hash).First(&rt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRefreshTokenTidakValid
	}
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *TokenRepo) Revoke(db *gorm.DB, id uint64, at time.Time) error {
	return db.Model(&domain.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

func (r *TokenRepo) RevokeAllForUser(db *gorm.DB, userID uint64, at time.Time) error {
	return db.Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
}

type AuditRepo struct{}

func NewAuditRepo() *AuditRepo { return &AuditRepo{} }

func (r *AuditRepo) Catat(db *gorm.DB, userID *uint64, aksi, entityType string, entityID *uint64, ringkasan *string) error {
	return r.CatatLengkap(db, AuditTulis{
		UserID: userID, Aksi: aksi, EntityType: entityType, EntityID: entityID, Ringkasan: ringkasan,
	})
}

// AuditTulis input audit untuk operasi tulis CRUD.
type AuditTulis struct {
	UserID      *uint64
	Aksi        string
	EntityType  string
	EntityID    *uint64
	Ringkasan   *string
	DataSebelum any
	DataSesudah any
	IPAddress   *string
	RequestID   *string
}

func (r *AuditRepo) CatatLengkap(db *gorm.DB, in AuditTulis) error {
	log := domain.AuditLog{
		UserID:     in.UserID,
		Aksi:       in.Aksi,
		EntityType: in.EntityType,
		EntityID:   in.EntityID,
		Ringkasan:  in.Ringkasan,
		IPAddress:  in.IPAddress,
		RequestID:  in.RequestID,
		CreatedAt:  time.Now(),
	}
	if in.DataSebelum != nil {
		b, err := json.Marshal(in.DataSebelum)
		if err != nil {
			return err
		}
		s := string(b)
		log.DataSebelum = &s
	}
	if in.DataSesudah != nil {
		b, err := json.Marshal(in.DataSesudah)
		if err != nil {
			return err
		}
		s := string(b)
		log.DataSesudah = &s
	}
	return db.Create(&log).Error
}
