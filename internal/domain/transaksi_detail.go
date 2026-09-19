package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type TransaksiDetail struct {
	ID                   uint64          `gorm:"primaryKey" json:"id"`
	TransaksiPenjualanID uint64          `gorm:"column:transaksi_penjualan_id;not null;index" json:"transaksi_penjualan_id"`
	Urutan               uint16          `gorm:"not null;default:1" json:"urutan"`
	KodeItem             string          `gorm:"column:kode_item;size:64;not null;index" json:"kode_item"`
	NamaItem             string          `gorm:"column:nama_item;size:255;not null" json:"nama_item"`
	Satuan               string          `gorm:"size:20;not null" json:"satuan"`
	BarangMasukID        *uint64         `gorm:"column:barang_masuk_id" json:"barang_masuk_id,omitempty"`
	BatchNumber          *string         `gorm:"column:batch_number;size:100" json:"batch_number,omitempty"`
	ExpiryDate           *datatypes.Date `gorm:"column:expiry_date" json:"expiry_date,omitempty"`
	Qty                  int             `gorm:"not null" json:"qty"`
	QtyPromo             int             `gorm:"column:qty_promo;not null;default:0" json:"qty_promo"`
	TotalQtyKeluar       int             `gorm:"column:total_qty_keluar;not null;default:0" json:"total_qty_keluar"`
	Harga                decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"harga"`
	Subtotal             decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"subtotal"`
	Disc1Persen          decimal.Decimal `gorm:"column:disc1_persen;type:decimal(5,2);default:0" json:"disc1_persen"`
	Disc2Persen          decimal.Decimal `gorm:"column:disc2_persen;type:decimal(5,2);default:0" json:"disc2_persen"`
	Disc3Persen          decimal.Decimal `gorm:"column:disc3_persen;type:decimal(5,2);default:0" json:"disc3_persen"`
	TotalAfterDisc       decimal.Decimal `gorm:"column:total_after_disc;type:decimal(15,2);not null" json:"total_after_disc"`
	HPPSnapshot          decimal.Decimal `gorm:"column:hpp_snapshot;type:decimal(15,2);default:0" json:"hpp_snapshot"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`

	Promos []TransaksiDetailPromo `gorm:"foreignKey:TransaksiDetailID" json:"promos,omitempty"`
}

func (TransaksiDetail) TableName() string { return "transaksi_detail" }

type TransaksiDetailPromo struct {
	ID                uint64          `gorm:"primaryKey" json:"id"`
	TransaksiDetailID uint64          `gorm:"column:transaksi_detail_id;not null;index" json:"transaksi_detail_id"`
	PromoID           uint64          `gorm:"column:promo_id;not null;index" json:"promo_id"`
	KodePromo         string          `gorm:"column:kode_promo;size:64;not null" json:"kode_promo"`
	NamaPromo         string          `gorm:"column:nama_promo;size:255;not null" json:"nama_promo"`
	TipePromo         TipePromo       `gorm:"column:tipe_promo;not null" json:"tipe_promo"`
	QtyBonus          int             `gorm:"column:qty_bonus;not null;default:0" json:"qty_bonus"`
	NilaiDiskon       decimal.Decimal `gorm:"column:nilai_diskon;type:decimal(15,2);default:0" json:"nilai_diskon"`
	CreatedAt         time.Time       `json:"created_at"`
}

func (TransaksiDetailPromo) TableName() string { return "transaksi_detail_promo" }
