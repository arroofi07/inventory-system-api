package dto

// BatchTersediaItem satu batch bersaldo untuk form sales.
type BatchTersediaItem struct {
	BarangMasukID uint64 `json:"barang_masuk_id"`
	NoBatch       string `json:"no_batch"`
	NoFaktur      string `json:"no_faktur"`
	Exp           string `json:"exp"`
	TanggalMasuk  string `json:"tanggal_masuk"`
	SisaHari      int    `json:"sisa_hari"`
	QtyMasuk      int    `json:"qty_masuk"`
	QtyTersedia   int    `json:"qty_tersedia"`
	HargaJual     string `json:"harga_jual"`
	HargaMT       string `json:"harga_mt"`
	HargaGT       string `json:"harga_gt"`
	HPPDenganPPN  string `json:"hpp_dengan_ppn"`
	MendekatiExp  bool   `json:"mendekati_exp"`
}

// RencanaAlokasiItem potongan alokasi FEFO/FIFO.
type RencanaAlokasiItem struct {
	BarangMasukID uint64 `json:"barang_masuk_id"`
	NoBatch       string `json:"no_batch"`
	Exp           string `json:"exp"`
	Qty           int    `json:"qty"`
}

// BatchTersediaResponse respons GET .../batch-tersedia.
type BatchTersediaResponse struct {
	KodeBarang      string               `json:"kode_barang"`
	NamaItem        string               `json:"nama_item"`
	Satuan          string               `json:"satuan"`
	MetodeAlokasi   string               `json:"metode_alokasi"`
	StokTersedia    int                  `json:"stok_tersedia"`
	Batch           []BatchTersediaItem  `json:"batch"`
	RencanaAlokasi  []RencanaAlokasiItem `json:"rencana_alokasi,omitempty"`
}

// BatchListItem ringkas untuk GET .../batch (semua / terbaru).
type BatchListItem struct {
	BarangMasukID uint64 `json:"barang_masuk_id"`
	NoBatch       string `json:"no_batch"`
	NoFaktur      string `json:"no_faktur"`
	Exp           string `json:"exp"`
	TanggalMasuk  string `json:"tanggal_masuk"`
	QtyMasuk      int    `json:"qty_masuk"`
	QtyTersedia   int    `json:"qty_tersedia"`
	Harga          string `json:"harga"`
	DiscHPP1       string `json:"disc_hpp_1"`
	DiscHPP2       string `json:"disc_hpp_2"`
	DiscHPP3       string `json:"disc_hpp_3"`
	MarkupMTType   string `json:"markup_mt_type"`
	MarkupMTAmount string `json:"markup_mt_amount"`
	MarkupGTType   string `json:"markup_gt_type"`
	MarkupGTAmount string `json:"markup_gt_amount"`
	HargaMT        string `json:"harga_mt"`
	HargaGT        string `json:"harga_gt"`
	HPP            string `json:"hpp"`
	HPPDenganPPN   string `json:"hpp_dengan_ppn"`
}

// BatchListQuery query GET /barang/{kode}/batch.
type BatchListQuery struct {
	Limit int `form:"limit" validate:"omitempty,min=1,max=100"`
}

// BatchTersediaQuery query GET /barang/{kode}/batch-tersedia.
type BatchTersediaQuery struct {
	Channel string `form:"channel"`
	Qty     *int   `form:"qty" validate:"omitempty,min=1"`
}
