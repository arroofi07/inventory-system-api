package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Promo struct {
	ID                  uint64          `gorm:"primaryKey" json:"id"`
	KodePromo           string          `gorm:"column:kode_promo;size:64;not null;uniqueIndex" json:"kode_promo"`
	NamaPromo           string          `gorm:"column:nama_promo;size:255;not null" json:"nama_promo"`
	Deskripsi           *string         `gorm:"type:text" json:"deskripsi,omitempty"`
	TipePromo           TipePromo       `gorm:"column:tipe_promo;not null" json:"tipe_promo"`
	BuyQty              *int            `gorm:"column:buy_qty" json:"buy_qty,omitempty"`
	GetQty              *int            `gorm:"column:get_qty" json:"get_qty,omitempty"`
	BonusQty            int             `gorm:"column:bonus_qty;not null;default:0" json:"bonus_qty"`
	DiscountPercentage  decimal.Decimal `gorm:"column:discount_percentage;type:decimal(5,2);default:0" json:"discount_percentage"`
	DiscountAmount      decimal.Decimal `gorm:"column:discount_amount;type:decimal(15,2);default:0" json:"discount_amount"`
	MinQty              int             `gorm:"column:min_qty;not null;default:1" json:"min_qty"`
	MinAmount           decimal.Decimal `gorm:"column:min_amount;type:decimal(15,2);default:0" json:"min_amount"`
	MaxApplications     *int            `gorm:"column:max_applications" json:"max_applications,omitempty"`
	KodeBarang          *string         `gorm:"column:kode_barang;size:64" json:"kode_barang,omitempty"`
	TanggalMulai        datatypes.Date  `gorm:"column:tanggal_mulai;not null" json:"tanggal_mulai"`
	TanggalBerakhir     datatypes.Date  `gorm:"column:tanggal_berakhir;not null" json:"tanggal_berakhir"`
	IsActive            bool            `gorm:"column:is_active;not null;default:true" json:"is_active"`
	SyaratKetentuan     *string         `gorm:"column:syarat_ketentuan;type:text" json:"syarat_ketentuan,omitempty"`
	CreatedBy           *uint64         `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

func (Promo) TableName() string { return "promos" }
