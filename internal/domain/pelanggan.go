package domain

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Pelanggan struct {
	ID                        uint64          `gorm:"primaryKey" json:"id"`
	KodePelanggan             string          `gorm:"column:kode_pelanggan;size:32;not null;uniqueIndex" json:"kode_pelanggan"`
	NamaPelanggan             string          `gorm:"column:nama_pelanggan;size:255;not null" json:"nama_pelanggan"`
	TglRegistrasi             datatypes.Date  `gorm:"column:tgl_registrasi;not null" json:"tgl_registrasi"`
	Phone                     string          `gorm:"size:30;not null" json:"phone"`
	NPWPNIK                   *string         `gorm:"column:npwp_nik;size:32" json:"npwp_nik,omitempty"`
	NamaPemilikNPWPNIK        *string         `gorm:"column:nama_pemilik_npwp_nik;size:255" json:"nama_pemilik_npwp_nik,omitempty"`
	AlamatNPWPNIK             *string         `gorm:"column:alamat_npwp_nik;type:text" json:"alamat_npwp_nik,omitempty"`
	Territory                 string          `gorm:"size:100;not null" json:"territory"`
	Distrik                   string          `gorm:"size:100;not null" json:"distrik"`
	AlamatToko                string          `gorm:"column:alamat_toko;type:text;not null" json:"alamat_toko"`
	RTRW                      *string         `gorm:"column:rt_rw;size:20" json:"rt_rw,omitempty"`
	Provinsi                  string          `gorm:"size:100;not null" json:"provinsi"`
	Kabupaten                 string          `gorm:"size:100;not null" json:"kabupaten"`
	Kecamatan                 string          `gorm:"size:100;not null" json:"kecamatan"`
	Kelurahan                 string          `gorm:"size:100;not null" json:"kelurahan"`
	KodePos                   *string         `gorm:"column:kode_pos;size:10" json:"kode_pos,omitempty"`
	ChannelOutlet             ChannelOutlet   `gorm:"column:channel_outlet;not null" json:"channel_outlet"`
	AlamatPengantaranBarang   *string         `gorm:"column:alamat_pengantaran_barang;type:text" json:"alamat_pengantaran_barang,omitempty"`
	JenisBangunan             *string         `gorm:"column:jenis_bangunan;size:50" json:"jenis_bangunan,omitempty"`
	StatusBangunan            *string         `gorm:"column:status_bangunan;size:50" json:"status_bangunan,omitempty"`
	NominalPengambilanPertama decimal.Decimal `gorm:"column:nominal_pengambilan_pertama;type:decimal(15,2);default:0" json:"nominal_pengambilan_pertama"`
	EstimasiBatasKredit       decimal.Decimal `gorm:"column:estimasi_batas_kredit;type:decimal(15,2);default:0" json:"estimasi_batas_kredit"`
	IsActive                  bool            `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
}

func (Pelanggan) TableName() string { return "pelanggan" }
