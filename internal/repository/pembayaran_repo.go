package repository

import (
	"errors"
	"time"

	"app/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PembayaranRepo struct{}

func NewPembayaranRepo() *PembayaranRepo { return &PembayaranRepo{} }

// Create menulis baris riwayat_pembayaran.
func (r *PembayaranRepo) Create(db *gorm.DB, row *domain.RiwayatPembayaran) error {
	return db.Create(row).Error
}

// ListByTransaksi kronologi ascending.
func (r *PembayaranRepo) ListByTransaksi(db *gorm.DB, transaksiID uint64) ([]domain.RiwayatPembayaran, error) {
	var rows []domain.RiwayatPembayaran
	err := db.Where("transaksi_penjualan_id = ?", transaksiID).
		Order("changed_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

// IdempotencyRecord jejak respons idempotent.
type IdempotencyRecord struct {
	ID           uint64    `gorm:"primaryKey"`
	Scope        string    `gorm:"size:64;not null"`
	IdemKey      string    `gorm:"column:idem_key;size:128;not null"`
	UserID       uint64    `gorm:"column:user_id;not null"`
	ResourceID   *uint64   `gorm:"column:resource_id"`
	RequestHash  string    `gorm:"column:request_hash;size:64;not null"`
	ResponseJSON []byte    `gorm:"column:response_json;type:json;not null"`
	StatusCode   int       `gorm:"column:status_code;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

func (IdempotencyRecord) TableName() string { return "idempotency_keys" }

type IdempotencyRepo struct{}

func NewIdempotencyRepo() *IdempotencyRepo { return &IdempotencyRepo{} }

func (r *IdempotencyRepo) Find(db *gorm.DB, scope string, userID uint64, key string) (*IdempotencyRecord, error) {
	var row IdempotencyRecord
	err := db.Where("scope = ? AND user_id = ? AND idem_key = ?", scope, userID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *IdempotencyRepo) Save(db *gorm.DB, row *IdempotencyRecord) error {
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}
