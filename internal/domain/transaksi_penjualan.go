package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type TransaksiPenjualan struct {
	ID            uint64         `gorm:"primaryKey" json:"id"`
	NoTransaksi   *string        `gorm:"column:no_transaksi;size:32;uniqueIndex" json:"no_transaksi"`
	Tanggal       datatypes.Date `gorm:"not null" json:"tanggal"`
	Periode       string         `gorm:"size:7;not null;index" json:"periode"`
	KodePelanggan string         `gorm:"column:kode_pelanggan;size:32;not null;index" json:"kode_pelanggan"`
	NamaPelanggan string         `gorm:"column:nama_pelanggan;size:255;not null" json:"nama_pelanggan"`
	Alamat        string         `gorm:"type:text;not null" json:"alamat"`
	ChannelOutlet ChannelOutlet  `gorm:"column:channel_outlet;size:50;not null" json:"channel_outlet"`
	Area          string         `gorm:"size:100;not null" json:"area"`
	IsMultiItem   bool           `gorm:"column:is_multi_item;not null;default:true" json:"is_multi_item"`

	Disc1Persen decimal.Decimal `gorm:"column:disc1_persen;type:decimal(5,2);default:0" json:"disc1_persen"`
	Disc2Persen decimal.Decimal `gorm:"column:disc2_persen;type:decimal(5,2);default:0" json:"disc2_persen"`
	Disc3Persen decimal.Decimal `gorm:"column:disc3_persen;type:decimal(5,2);default:0" json:"disc3_persen"`
	PPNPersen   decimal.Decimal `gorm:"column:ppn_persen;type:decimal(5,2);default:11" json:"ppn_persen"`
	Total       decimal.Decimal `gorm:"type:decimal(15,2);default:0" json:"total"`
	PPNNominal  decimal.Decimal `gorm:"column:ppn_nominal;type:decimal(15,2);default:0" json:"ppn_nominal"`
	TotalAkhir  decimal.Decimal `gorm:"column:total_akhir;type:decimal(15,2);default:0" json:"total_akhir"`

	JumlahItem      int `gorm:"column:jumlah_item;default:0" json:"jumlah_item"`
	TotalQtyDitagih int `gorm:"column:total_qty_ditagih;default:0" json:"total_qty_ditagih"`
	TotalQtyKeluar  int `gorm:"column:total_qty_keluar;default:0" json:"total_qty_keluar"`

	StatusApproval StatusApproval `gorm:"column:status_approval;type:enum('pending','approved','rejected');default:pending" json:"status_approval"`
	ApprovedAt     *time.Time     `gorm:"column:approved_at" json:"approved_at,omitempty"`
	ApprovedBy     *uint64        `gorm:"column:approved_by" json:"approved_by,omitempty"`
	ApprovalNotes  *string        `gorm:"column:approval_notes;type:text" json:"approval_notes,omitempty"`

	FulfillmentStatus FulfillmentStatus `gorm:"column:fulfillment_status;default:awaiting_approval" json:"fulfillment_status"`
	QtyFulfilled      int               `gorm:"column:qty_fulfilled;default:0" json:"qty_fulfilled"`
	QtyBackorder      int               `gorm:"column:qty_backorder;default:0" json:"qty_backorder"`
	StockNotes        *string           `gorm:"column:stock_notes;type:text" json:"stock_notes,omitempty"`

	StatusPembayaran          StatusPembayaran `gorm:"column:status_pembayaran;default:hutang" json:"status_pembayaran"`
	JumlahDibayar             decimal.Decimal  `gorm:"column:jumlah_dibayar;type:decimal(15,2);default:0" json:"jumlah_dibayar"`
	SisaHutang                decimal.Decimal  `gorm:"column:sisa_hutang;type:decimal(15,2);default:0" json:"sisa_hutang"`
	TanggalJatuhTempo         *datatypes.Date  `gorm:"column:tanggal_jatuh_tempo" json:"tanggal_jatuh_tempo,omitempty"`
	TanggalPembayaranTerakhir *time.Time       `gorm:"column:tanggal_pembayaran_terakhir" json:"tanggal_pembayaran_terakhir,omitempty"`
	KeteranganPembayaran      *string          `gorm:"column:keterangan_pembayaran;type:text" json:"keterangan_pembayaran,omitempty"`

	FakturDicetakAt *time.Time `gorm:"column:faktur_dicetak_at" json:"faktur_dicetak_at,omitempty"`
	FakturDicetakBy *uint64    `gorm:"column:faktur_dicetak_by" json:"faktur_dicetak_by,omitempty"`

	SalesID   *uint64   `gorm:"column:sales_id;index" json:"sales_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Details []TransaksiDetail `gorm:"foreignKey:TransaksiPenjualanID" json:"details,omitempty"`
}

func (TransaksiPenjualan) TableName() string { return "transaksi_penjualan" }
