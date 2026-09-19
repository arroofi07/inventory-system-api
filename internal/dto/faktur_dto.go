package dto

// FakturResponse GET /transaksi/{id}/faktur (SD-09/10).
type FakturResponse struct {
	NoTransaksi     *string             `json:"no_transaksi"`
	Tanggal         string              `json:"tanggal"`
	Layout          string              `json:"layout" example:"half"`
	TotalHalaman    int                 `json:"total_halaman" example:"1"`
	Perusahaan      FakturPerusahaan    `json:"perusahaan"`
	Pelanggan       FakturPelanggan     `json:"pelanggan"`
	Items           []FakturItem        `json:"items"`
	Ringkasan       FakturRingkasan     `json:"ringkasan"`
	Sales           *UserRingkasResponse `json:"sales,omitempty"`
	Approver        *UserRingkasResponse `json:"approver,omitempty"`
	FakturDicetakAt *string             `json:"faktur_dicetak_at,omitempty"`
	CetakUlang      bool                `json:"cetak_ulang"`
}

// FakturPerusahaan kop dokumen dari env.
type FakturPerusahaan struct {
	Nama    string `json:"nama"`
	Alamat  string `json:"alamat"`
	Telepon string `json:"telepon"`
	NPWP    string `json:"npwp"`
}

// FakturPelanggan snapshot header transaksi (bukan live join).
type FakturPelanggan struct {
	KodePelanggan string `json:"kode_pelanggan"`
	NamaPelanggan string `json:"nama_pelanggan"`
	Alamat        string `json:"alamat"`
	ChannelOutlet string `json:"channel_outlet"`
}

// FakturItem baris siap cetak (termasuk bonus).
type FakturItem struct {
	Urutan            uint16  `json:"urutan"`
	KodeItem          string  `json:"kode_item"`
	NamaItem          string  `json:"nama_item"`
	NoBatch           *string `json:"no_batch,omitempty"`
	Exp               *string `json:"exp,omitempty"`
	Qty               int     `json:"qty"`
	Satuan            string  `json:"satuan"`
	Harga             string  `json:"harga"`
	Disc1Persen       string  `json:"disc1_persen,omitempty"`
	Disc2Persen       string  `json:"disc2_persen,omitempty"`
	Disc3Persen       string  `json:"disc3_persen,omitempty"`
	NilaiSetelahGlobal string `json:"nilai_setelah_global,omitempty"`
	PPNBaris          string  `json:"ppn_baris,omitempty"`
	TotalFinalBaris   string  `json:"total_final_baris"`
	IsBonus           bool    `json:"is_bonus"`
}

// FakturRingkasan footer nilai + terbilang.
type FakturRingkasan struct {
	Total       string `json:"total"`
	Disc1Persen string `json:"disc1_persen"`
	Disc2Persen string `json:"disc2_persen,omitempty"`
	Disc3Persen string `json:"disc3_persen,omitempty"`
	PPNPersen   string `json:"ppn_persen"`
	PPNNominal  string `json:"ppn_nominal"`
	TotalAkhir  string `json:"total_akhir"`
	Terbilang   string `json:"terbilang"`
}

// FakturOKResponse envelope 200.
type FakturOKResponse struct {
	Data FakturResponse `json:"data"`
}
