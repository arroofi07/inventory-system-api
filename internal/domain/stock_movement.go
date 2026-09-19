package domain

import "time"

type StockMovement struct {
	ID            uint64       `gorm:"primaryKey" json:"id"`
	BarangID      uint64       `gorm:"column:barang_id;not null" json:"barang_id"`
	BarangMasukID *uint64      `gorm:"column:barang_masuk_id" json:"barang_masuk_id,omitempty"`
	MovementType  MovementType `gorm:"column:movement_type;type:enum('PENERIMAAN','PENJUALAN','PEMBATALAN','PENYESUAIAN','OPNAME');not null" json:"movement_type"`
	Qty           int          `gorm:"not null" json:"qty"`
	SaldoSetelah  int          `gorm:"column:saldo_setelah;not null" json:"saldo_setelah"`
	ReferenceType *string      `gorm:"column:reference_type;size:50" json:"reference_type,omitempty"`
	ReferenceID   *uint64      `gorm:"column:reference_id" json:"reference_id,omitempty"`
	ReferenceNo   *string      `gorm:"column:reference_no;size:100" json:"reference_no,omitempty"`
	Keterangan    *string      `gorm:"type:text" json:"keterangan,omitempty"`
	CreatedBy     *uint64      `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt     time.Time    `gorm:"not null" json:"created_at"`
}

func (StockMovement) TableName() string { return "stock_movements" }
