package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type BarangMasuk struct {
	ID             uint64          `gorm:"primaryKey" json:"id"`
	BarangID       uint64          `gorm:"column:barang_id;not null;index" json:"barang_id"`
	NoFaktur       string          `gorm:"column:no_faktur;size:100;not null" json:"no_faktur"`
	NoBatch        string          `gorm:"column:no_batch;size:100;not null" json:"no_batch"`
	Exp            datatypes.Date  `gorm:"column:exp;not null" json:"exp"`
	TanggalMasuk   datatypes.Date  `gorm:"column:tanggal_masuk;not null" json:"tanggal_masuk"`
	Qty            int             `gorm:"not null" json:"qty"`
	Harga          decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"harga"`
	DiscHPP1       decimal.Decimal `gorm:"column:disc_hpp_1;type:decimal(5,2);not null;default:0" json:"disc_hpp_1"`
	DiscHPP2       decimal.Decimal `gorm:"column:disc_hpp_2;type:decimal(5,2);not null;default:0" json:"disc_hpp_2"`
	DiscHPP3       decimal.Decimal `gorm:"column:disc_hpp_3;type:decimal(5,2);not null;default:0" json:"disc_hpp_3"`
	HPP            decimal.Decimal `gorm:"column:hpp;type:decimal(15,2);not null;default:0" json:"hpp"`
	HPPDenganPPN   decimal.Decimal `gorm:"column:hpp_dengan_ppn;type:decimal(15,2);not null;default:0" json:"hpp_dengan_ppn"`
	MarkupMTType   MarkupType      `gorm:"column:markup_mt_type;type:enum('percent','value');default:percent" json:"markup_mt_type"`
	MarkupMTAmount decimal.Decimal `gorm:"column:markup_mt_amount;type:decimal(15,2);not null;default:0" json:"markup_mt_amount"`
	MarkupGTType   MarkupType      `gorm:"column:markup_gt_type;type:enum('percent','value');default:percent" json:"markup_gt_type"`
	MarkupGTAmount decimal.Decimal `gorm:"column:markup_gt_amount;type:decimal(15,2);not null;default:0" json:"markup_gt_amount"`
	HargaMT        decimal.Decimal `gorm:"column:harga_mt;type:decimal(15,2);not null;default:0" json:"harga_mt"`
	HargaGT        decimal.Decimal `gorm:"column:harga_gt;type:decimal(15,2);not null;default:0" json:"harga_gt"`
	AgingMonth     int             `gorm:"column:aging_month;not null;default:0" json:"aging_month"`
	CreatedBy      *uint64         `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`

	Barang *Barang `gorm:"foreignKey:BarangID" json:"barang,omitempty"`
}

func (BarangMasuk) TableName() string { return "barang_masuk" }
