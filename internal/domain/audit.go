package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type PriceChangeLog struct {
	ID                 uint64           `gorm:"primaryKey" json:"id"`
	BarangMasukID      uint64           `gorm:"column:barang_masuk_id;not null" json:"barang_masuk_id"`
	BarangID           uint64           `gorm:"column:barang_id;not null" json:"barang_id"`
	BulkOperationID    string           `gorm:"column:bulk_operation_id;size:36;not null" json:"bulk_operation_id"`
	OldHarga           *decimal.Decimal `gorm:"column:old_harga;type:decimal(15,2)" json:"old_harga,omitempty"`
	OldDiscHPP1        *decimal.Decimal `gorm:"column:old_disc_hpp_1;type:decimal(5,2)" json:"old_disc_hpp_1,omitempty"`
	OldDiscHPP2        *decimal.Decimal `gorm:"column:old_disc_hpp_2;type:decimal(5,2)" json:"old_disc_hpp_2,omitempty"`
	OldDiscHPP3        *decimal.Decimal `gorm:"column:old_disc_hpp_3;type:decimal(5,2)" json:"old_disc_hpp_3,omitempty"`
	OldMarkupMTType    *MarkupType      `gorm:"column:old_markup_mt_type" json:"old_markup_mt_type,omitempty"`
	OldMarkupMTAmount  *decimal.Decimal `gorm:"column:old_markup_mt_amount;type:decimal(15,2)" json:"old_markup_mt_amount,omitempty"`
	OldMarkupGTType    *MarkupType      `gorm:"column:old_markup_gt_type" json:"old_markup_gt_type,omitempty"`
	OldMarkupGTAmount  *decimal.Decimal `gorm:"column:old_markup_gt_amount;type:decimal(15,2)" json:"old_markup_gt_amount,omitempty"`
	OldHargaMT         *decimal.Decimal `gorm:"column:old_harga_mt;type:decimal(15,2)" json:"old_harga_mt,omitempty"`
	OldHargaGT         *decimal.Decimal `gorm:"column:old_harga_gt;type:decimal(15,2)" json:"old_harga_gt,omitempty"`
	OldHPP             *decimal.Decimal `gorm:"column:old_hpp;type:decimal(15,2)" json:"old_hpp,omitempty"`
	NewHarga           *decimal.Decimal `gorm:"column:new_harga;type:decimal(15,2)" json:"new_harga,omitempty"`
	NewDiscHPP1        *decimal.Decimal `gorm:"column:new_disc_hpp_1;type:decimal(5,2)" json:"new_disc_hpp_1,omitempty"`
	NewDiscHPP2        *decimal.Decimal `gorm:"column:new_disc_hpp_2;type:decimal(5,2)" json:"new_disc_hpp_2,omitempty"`
	NewDiscHPP3        *decimal.Decimal `gorm:"column:new_disc_hpp_3;type:decimal(5,2)" json:"new_disc_hpp_3,omitempty"`
	NewMarkupMTType    *MarkupType      `gorm:"column:new_markup_mt_type" json:"new_markup_mt_type,omitempty"`
	NewMarkupMTAmount  *decimal.Decimal `gorm:"column:new_markup_mt_amount;type:decimal(15,2)" json:"new_markup_mt_amount,omitempty"`
	NewMarkupGTType    *MarkupType      `gorm:"column:new_markup_gt_type" json:"new_markup_gt_type,omitempty"`
	NewMarkupGTAmount  *decimal.Decimal `gorm:"column:new_markup_gt_amount;type:decimal(15,2)" json:"new_markup_gt_amount,omitempty"`
	NewHargaMT         *decimal.Decimal `gorm:"column:new_harga_mt;type:decimal(15,2)" json:"new_harga_mt,omitempty"`
	NewHargaGT         *decimal.Decimal `gorm:"column:new_harga_gt;type:decimal(15,2)" json:"new_harga_gt,omitempty"`
	NewHPP             *decimal.Decimal `gorm:"column:new_hpp;type:decimal(15,2)" json:"new_hpp,omitempty"`
	Keterangan         *string          `gorm:"type:text" json:"keterangan,omitempty"`
	ChangedBy          *uint64          `gorm:"column:changed_by" json:"changed_by,omitempty"`
	ChangedAt          time.Time        `gorm:"column:changed_at;not null" json:"changed_at"`
	CreatedAt          time.Time        `json:"created_at"`
}

func (PriceChangeLog) TableName() string { return "price_change_logs" }

type StockAlert struct {
	ID             uint64         `gorm:"primaryKey" json:"id"`
	BarangID       uint64         `gorm:"column:barang_id;not null" json:"barang_id"`
	BarangMasukID  *uint64        `gorm:"column:barang_masuk_id" json:"barang_masuk_id,omitempty"`
	AlertType      AlertType      `gorm:"column:alert_type;not null" json:"alert_type"`
	Severity       AlertSeverity  `gorm:"not null;default:SEDANG" json:"severity"`
	Judul          string         `gorm:"size:255;not null" json:"judul"`
	Pesan          string         `gorm:"type:text;not null" json:"pesan"`
	StokSaatItu    *int           `gorm:"column:stok_saat_itu" json:"stok_saat_itu,omitempty"`
	NilaiAmbang    *int           `gorm:"column:nilai_ambang" json:"nilai_ambang,omitempty"`
	TanggalExp     *datatypes.Date `gorm:"column:tanggal_exp" json:"tanggal_exp,omitempty"`
	IsResolved     bool           `gorm:"column:is_resolved;not null;default:false" json:"is_resolved"`
	ResolvedAt     *time.Time     `gorm:"column:resolved_at" json:"resolved_at,omitempty"`
	ResolvedBy     *uint64        `gorm:"column:resolved_by" json:"resolved_by,omitempty"`
	CatatanResolusi *string       `gorm:"column:catatan_resolusi;type:text" json:"catatan_resolusi,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (StockAlert) TableName() string { return "stock_alerts" }

type NoTransaksiSeq struct {
	ID         uint8     `gorm:"primaryKey" json:"id"`
	LastNumber uint64    `gorm:"column:last_number;not null;default:0" json:"last_number"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (NoTransaksiSeq) TableName() string { return "no_transaksi_seq" }

type AuditLog struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	UserID      *uint64   `gorm:"column:user_id" json:"user_id,omitempty"`
	Aksi        string    `gorm:"size:80;not null" json:"aksi"`
	EntityType  string    `gorm:"column:entity_type;size:50;not null" json:"entity_type"`
	EntityID    *uint64   `gorm:"column:entity_id" json:"entity_id,omitempty"`
	Ringkasan   *string   `gorm:"size:255" json:"ringkasan,omitempty"`
	DataSebelum *string   `gorm:"column:data_sebelum;type:json" json:"data_sebelum,omitempty"`
	DataSesudah *string   `gorm:"column:data_sesudah;type:json" json:"data_sesudah,omitempty"`
	IPAddress   *string   `gorm:"column:ip_address;size:45" json:"ip_address,omitempty"`
	RequestID   *string   `gorm:"column:request_id;size:36" json:"request_id,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
