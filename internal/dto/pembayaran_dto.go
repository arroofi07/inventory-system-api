package dto

// PembayaranCreateRequest POST /transaksi/{id}/pembayaran (SD-02).
// Nominal adalah tambahan yang diterima saat itu, bukan total kumulatif.
type PembayaranCreateRequest struct {
	NominalPembayaran string `json:"nominal_pembayaran" example:"150000.00" validate:"required"`
	TanggalPembayaran string `json:"tanggal_pembayaran,omitempty" example:"2026-09-10"`
	MetodePembayaran  string `json:"metode_pembayaran,omitempty" example:"Transfer BCA" validate:"omitempty,max=50"`
	Keterangan        string `json:"keterangan,omitempty" validate:"omitempty,max=2000"`
}

// PembayaranCreateResponse hasil catat pembayaran.
type PembayaranCreateResponse struct {
	TransaksiID               uint64  `json:"transaksi_id"`
	NoTransaksi               *string `json:"no_transaksi"`
	TotalAkhir                string  `json:"total_akhir" example:"347985.00"`
	JumlahDibayar             string  `json:"jumlah_dibayar" example:"150000.00"`
	SisaHutang                string  `json:"sisa_hutang" example:"197985.00"`
	StatusPembayaran          string  `json:"status_pembayaran" example:"sebagian"`
	TanggalPembayaranTerakhir *string `json:"tanggal_pembayaran_terakhir,omitempty"`
	RiwayatID                 uint64  `json:"riwayat_id"`
}

// PembayaranCreateOKResponse envelope 200.
type PembayaranCreateOKResponse struct {
	Data PembayaranCreateResponse `json:"data"`
}

// RiwayatPembayaranItem GET /transaksi/{id}/pembayaran.
type RiwayatPembayaranItem struct {
	ID                uint64  `json:"id"`
	NominalPembayaran string  `json:"nominal_pembayaran"`
	OldJumlahDibayar  string  `json:"old_jumlah_dibayar"`
	NewJumlahDibayar  string  `json:"new_jumlah_dibayar"`
	OldSisaHutang     string  `json:"old_sisa_hutang"`
	NewSisaHutang     string  `json:"new_sisa_hutang"`
	OldStatus         *string `json:"old_status,omitempty"`
	NewStatus         *string `json:"new_status,omitempty"`
	MetodePembayaran  *string `json:"metode_pembayaran,omitempty"`
	TanggalPembayaran *string `json:"tanggal_pembayaran,omitempty"`
	Keterangan        *string `json:"keterangan,omitempty"`
	ChangedBy         *uint64 `json:"changed_by,omitempty"`
	ChangedByNama     *string `json:"changed_by_nama,omitempty"`
	ChangedAt         string  `json:"changed_at"`
}

// RiwayatPembayaranListOKResponse envelope daftar riwayat.
type RiwayatPembayaranListOKResponse struct {
	Data []RiwayatPembayaranItem `json:"data"`
}
