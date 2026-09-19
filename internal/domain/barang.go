package domain

import "time"

type Barang struct {
	ID              uint64        `gorm:"primaryKey" json:"id"`
	KodeBarang      string        `gorm:"column:kode_barang;size:64;not null;uniqueIndex" json:"kode_barang"`
	NamaItem        string        `gorm:"column:nama_item;size:255;not null" json:"nama_item"`
	Brand           string        `gorm:"size:100;not null;index" json:"brand"`
	Satuan          string        `gorm:"size:20;not null;default:PCS" json:"satuan"`
	StokTersedia    int           `gorm:"column:stok_tersedia;not null;default:0" json:"stok_tersedia"`
	MinStock        int           `gorm:"column:min_stock;not null;default:0" json:"min_stock"`
	ReorderPoint    int           `gorm:"column:reorder_point;not null;default:0" json:"reorder_point"`
	MetodeAlokasi   MetodeAlokasi `gorm:"column:metode_alokasi;type:enum('FEFO','FIFO');default:FEFO" json:"metode_alokasi"`
	ExpiryAlertDays int           `gorm:"column:expiry_alert_days;not null;default:30" json:"expiry_alert_days"`
	IsActive        bool          `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`

	BarangMasuk []BarangMasuk `gorm:"foreignKey:BarangID" json:"barang_masuk,omitempty"`
}

func (Barang) TableName() string { return "barang" }

func (b Barang) StatusStok() string {
	switch {
	case b.StokTersedia <= 0:
		return "HABIS"
	case b.StokTersedia <= b.MinStock:
		return "RENDAH"
	default:
		return "NORMAL"
	}
}
