package dto

// PiutangListQuery filter GET /piutang (SD-05).
type PiutangListQuery struct {
	ListQuery
	StatusPembayaran   string `form:"status_pembayaran"`
	KategoriJatuhTempo string `form:"kategori_jatuh_tempo"`
	Brand              string `form:"brand"`
	DateFrom           string `form:"date_from"`
	DateTo             string `form:"date_to"`
	DateType           string `form:"date_type"` // tanggal_transaksi | tanggal_jatuh_tempo | tanggal_pembayaran_terakhir
	KodePelanggan      string `form:"-"`        // diisi dari path
	HanyaOverdue       bool   `form:"-"`
}

// PiutangItem baris daftar piutang.
type PiutangItem struct {
	TransaksiID               uint64  `json:"transaksi_id"`
	NoTransaksi               *string `json:"no_transaksi"`
	Tanggal                   string  `json:"tanggal"`
	KodePelanggan             string  `json:"kode_pelanggan"`
	NamaPelanggan             string  `json:"nama_pelanggan"`
	TotalAkhir                string  `json:"total_akhir"`
	JumlahDibayar             string  `json:"jumlah_dibayar"`
	SisaHutang                string  `json:"sisa_hutang"`
	StatusPembayaran          string  `json:"status_pembayaran"`
	TanggalJatuhTempo         *string `json:"tanggal_jatuh_tempo,omitempty"`
	KategoriJatuhTempo        string  `json:"kategori_jatuh_tempo"`
	HariTerlambat             int     `json:"hari_terlambat"`
	TanggalPembayaranTerakhir *string `json:"tanggal_pembayaran_terakhir,omitempty"`
}

// PiutangRingkasan agregat kartu statistik.
type PiutangRingkasan struct {
	TotalNilai              string `json:"total_nilai"`
	TotalDibayar            string `json:"total_dibayar"`
	TotalPiutang            string `json:"total_piutang"`
	PiutangOverdue          string `json:"piutang_overdue"`
	JumlahTransaksi         int    `json:"jumlah_transaksi"`
	JumlahTransaksiOverdue  int    `json:"jumlah_transaksi_overdue"`
}

// NotifikasiPiutangResponse GET /notifikasi/piutang.
type NotifikasiPiutangResponse struct {
	Overdue               int `json:"overdue"`
	MendekatiJatuhTempo   int `json:"mendekati_jatuh_tempo"`
}

// PenerimaanLaporanQuery GET /laporan/penerimaan.
type PenerimaanLaporanQuery struct {
	DateFrom string `form:"date_from" validate:"required"`
	DateTo   string `form:"date_to" validate:"required"`
}

// PenerimaanLaporanResponse SUM(nominal_pembayaran) periode.
type PenerimaanLaporanResponse struct {
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`
	TotalNominal string `json:"total_nominal"`
	JumlahBaris  int    `json:"jumlah_baris"`
}
