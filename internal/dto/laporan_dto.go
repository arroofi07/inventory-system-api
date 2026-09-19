package dto

// LaporanStokQuery GET /laporan/stok.
type LaporanStokQuery struct {
	ListQuery
	Brand      string `form:"brand"`
	StatusStok string `form:"status_stok"` // NORMAL|RENDAH|HABIS
}

// BarisLaporanStok satu SKU.
type BarisLaporanStok struct {
	KodeBarang       string  `json:"kode_barang"`
	NamaItem         string  `json:"nama_item"`
	Brand            string  `json:"brand"`
	Satuan           string  `json:"satuan"`
	TotalMasuk       int     `json:"total_masuk"`
	TotalKeluar      int     `json:"total_keluar"`
	StokTersedia     int     `json:"stok_tersedia"`
	MinStock         int     `json:"min_stock"`
	StatusStok       string  `json:"status_stok"`
	JumlahBatch      int     `json:"jumlah_batch"`
	BatchTerdekatExp *string `json:"batch_terdekat_exp,omitempty"`
	NilaiStokHPP     string  `json:"nilai_stok_hpp"`
}

// LaporanStokRingkasan agregat status.
type LaporanStokRingkasan struct {
	Normal int `json:"normal"`
	Rendah int `json:"rendah"`
	Habis  int `json:"habis"`
}

// LaporanBarangKeluarQuery GET /laporan/barang-keluar|laba.
type LaporanBarangKeluarQuery struct {
	ListQuery
	DateFrom      string `form:"date_from"`
	DateTo        string `form:"date_to"`
	KodePelanggan string `form:"kode_pelanggan"`
	KodeItem      string `form:"kode_item"`
	SalesID       *uint64 `form:"sales_id"`
	Brand         string `form:"brand"`
}

// BarisBarangKeluar baris detail penjualan approved + laba.
type BarisBarangKeluar struct {
	TransaksiID     uint64  `json:"transaksi_id"`
	NoTransaksi     *string `json:"no_transaksi"`
	Tanggal         string  `json:"tanggal"`
	KodePelanggan   string  `json:"kode_pelanggan"`
	NamaPelanggan   string  `json:"nama_pelanggan"`
	Alamat          *string `json:"alamat,omitempty"`
	ChannelOutlet   string  `json:"channel_outlet"`
	Area            string  `json:"area"`
	NamaSales       *string `json:"nama_sales,omitempty"`
	KodeItem        string  `json:"kode_item"`
	NamaItem        string  `json:"nama_item"`
	Brand           string  `json:"brand"`
	NoBatch         *string `json:"no_batch,omitempty"`
	Exp             *string `json:"exp,omitempty"`
	Qty             int     `json:"qty"`
	QtyPromo        int     `json:"qty_promo"`
	TotalQtyKeluar  int     `json:"total_qty_keluar"`
	Harga           string  `json:"harga"`
	Disc1Persen     string  `json:"disc1_persen"`
	Disc2Persen     string  `json:"disc2_persen"`
	Disc3Persen     string  `json:"disc3_persen"`
	TotalAfterDisc  string  `json:"total_after_disc"`
	PPNBaris        string  `json:"ppn_baris"`
	TotalFinalBaris string  `json:"total_final_baris"`
	HPPSnapshot     *string `json:"hpp_snapshot,omitempty"`
	HPPTotal        *string `json:"hpp_total,omitempty"`
	Provit          *string `json:"provit,omitempty"`
	MarginPersen    *string `json:"margin_persen,omitempty"`
}

// LaporanBarangKeluarRingkasan agregat halaman/filter.
type LaporanBarangKeluarRingkasan struct {
	TotalQtyKeluar int    `json:"total_qty_keluar"`
	TotalFinal     string `json:"total_final"`
	TotalHPP       string `json:"total_hpp,omitempty"`
	TotalProvit    string `json:"total_provit,omitempty"`
}

// LaporanPenjualanRingkasan agregat GET /laporan/penjualan.
type LaporanPenjualanRingkasan struct {
	JumlahTransaksi int    `json:"jumlah_transaksi"`
	TotalPenjualan  string `json:"total_penjualan"`
}

// ChannelAnalyticsQuery GET /laporan/channel-analytics.
type ChannelAnalyticsQuery struct {
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Limit    int    `form:"limit"` // produk terlaris, default 20
}

func (q *ChannelAnalyticsQuery) Normalize() {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
}

// BarisChannelAnalytics agregat per channel_outlet (04 §7.3).
type BarisChannelAnalytics struct {
	ChannelOutlet    string `json:"channel_outlet"`
	JumlahTransaksi  int    `json:"jumlah_transaksi"`
	TotalPenjualan   string `json:"total_penjualan"`
	TotalQty         int    `json:"total_qty"`
	RataNilaiOrder   string `json:"rata_nilai_order"`
	JumlahOutlet     int    `json:"jumlah_outlet"`
}

// BarisTerritoryAnalytics agregat per territory pelanggan.
type BarisTerritoryAnalytics struct {
	Territory       string `json:"territory"`
	JumlahTransaksi int    `json:"jumlah_transaksi"`
	TotalPenjualan  string `json:"total_penjualan"`
	TotalQty        int    `json:"total_qty"`
	JumlahOutlet    int    `json:"jumlah_outlet"`
}

// BarisProdukTerlaris agregat per SKU.
type BarisProdukTerlaris struct {
	KodeItem        string `json:"kode_item"`
	NamaItem        string `json:"nama_item"`
	TotalQty        int    `json:"total_qty"`
	TotalQtyKeluar  int    `json:"total_qty_keluar"`
	JumlahTransaksi int    `json:"jumlah_transaksi"`
	TotalAfterDisc  string `json:"total_after_disc"`
}

// BarisTrenHarian penjualan per tanggal transaksi.
type BarisTrenHarian struct {
	Tanggal         string `json:"tanggal"`
	JumlahTransaksi int    `json:"jumlah_transaksi"`
	TotalPenjualan  string `json:"total_penjualan"`
}

// ChannelAnalyticsData payload GET /laporan/channel-analytics.
type ChannelAnalyticsData struct {
	PerChannel     []BarisChannelAnalytics   `json:"per_channel"`
	PerTerritory   []BarisTerritoryAnalytics `json:"per_territory"`
	ProdukTerlaris []BarisProdukTerlaris     `json:"produk_terlaris"`
	TrenHarian     []BarisTrenHarian         `json:"tren_harian"`
}
