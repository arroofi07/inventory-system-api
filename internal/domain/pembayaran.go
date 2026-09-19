package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type RiwayatPembayaran struct {
	ID                   uint64            `gorm:"primaryKey" json:"id"`
	TransaksiPenjualanID uint64            `gorm:"column:transaksi_penjualan_id;not null;index" json:"transaksi_penjualan_id"`
	OldJumlahDibayar     decimal.Decimal   `gorm:"column:old_jumlah_dibayar;type:decimal(15,2);default:0" json:"old_jumlah_dibayar"`
	NewJumlahDibayar     decimal.Decimal   `gorm:"column:new_jumlah_dibayar;type:decimal(15,2);default:0" json:"new_jumlah_dibayar"`
	OldSisaHutang        decimal.Decimal   `gorm:"column:old_sisa_hutang;type:decimal(15,2);default:0" json:"old_sisa_hutang"`
	NewSisaHutang        decimal.Decimal   `gorm:"column:new_sisa_hutang;type:decimal(15,2);default:0" json:"new_sisa_hutang"`
	OldStatus            *StatusPembayaran `gorm:"column:old_status" json:"old_status,omitempty"`
	NewStatus            *StatusPembayaran `gorm:"column:new_status" json:"new_status,omitempty"`
	NominalPembayaran    decimal.Decimal   `gorm:"column:nominal_pembayaran;type:decimal(15,2);default:0" json:"nominal_pembayaran"`
	MetodePembayaran     *string           `gorm:"column:metode_pembayaran;size:50" json:"metode_pembayaran,omitempty"`
	TanggalPembayaran    *datatypes.Date   `gorm:"column:tanggal_pembayaran" json:"tanggal_pembayaran,omitempty"`
	Keterangan           *string           `gorm:"type:text" json:"keterangan,omitempty"`
	ChangedBy            *uint64           `gorm:"column:changed_by" json:"changed_by,omitempty"`
	ChangedAt            time.Time         `gorm:"column:changed_at;not null" json:"changed_at"`
	CreatedAt            time.Time         `json:"created_at"`
}

func (RiwayatPembayaran) TableName() string { return "riwayat_pembayaran" }
