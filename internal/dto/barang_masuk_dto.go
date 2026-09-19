package dto

// BarangMasukResponse respons penerimaan (harga jual dihitung server).
type BarangMasukResponse struct {
	ID            uint64 `json:"id"`
	BarangID      uint64 `json:"barang_id"`
	KodeBarang    string `json:"kode_barang"`
	NamaItem      string `json:"nama_item"`
	Brand         string `json:"brand"`
	NoFaktur      string `json:"no_faktur"`
	NoBatch       string `json:"no_batch"`
	Exp           string `json:"exp"`
	TanggalMasuk  string `json:"tanggal_masuk"`
	Qty           int    `json:"qty"`
	QtyTersedia   int    `json:"qty_tersedia"`
	Harga         string `json:"harga"`
	DiscHPP1      string `json:"disc_hpp_1"`
	DiscHPP2      string `json:"disc_hpp_2"`
	DiscHPP3      string `json:"disc_hpp_3"`
	HPP           string `json:"hpp"`
	HPPDenganPPN  string `json:"hpp_dengan_ppn"`
	MarkupMTType  string `json:"markup_mt_type"`
	MarkupMTAmount string `json:"markup_mt_amount"`
	MarkupGTType  string `json:"markup_gt_type"`
	MarkupGTAmount string `json:"markup_gt_amount"`
	HargaMT       string `json:"harga_mt"`
	HargaGT       string `json:"harga_gt"`
	AgingMonth    int    `json:"aging_month"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// BarangMasukCreateRequest body POST /barang-masuk.
// Client mengirim harga list + disc + markup; tidak boleh mengirim harga_mt/harga_gt/hpp.
type BarangMasukCreateRequest struct {
	KodeBarang     string `json:"kode_barang" validate:"required,max=64"`
	BuatBarangBaru bool   `json:"buat_barang_baru"`
	NamaItem       string `json:"nama_item" validate:"omitempty,max=255"`
	Brand          string `json:"brand" validate:"omitempty,max=100"`
	Satuan         string `json:"satuan" validate:"omitempty,max=20"`
	NoFaktur       string `json:"no_faktur" validate:"required,max=100"`
	NoBatch        string `json:"no_batch" validate:"required,max=100"`
	Exp            string `json:"exp" validate:"required"`
	TanggalMasuk   string `json:"tanggal_masuk" validate:"required"`
	Qty            int    `json:"qty" validate:"required,min=1"`
	Harga          string `json:"harga" validate:"required"`
	DiscHPP1       string `json:"disc_hpp_1"`
	DiscHPP2       string `json:"disc_hpp_2"`
	DiscHPP3       string `json:"disc_hpp_3"`
	MarkupMTType   string `json:"markup_mt_type" validate:"omitempty,oneof=percent value"`
	MarkupMTAmount string `json:"markup_mt_amount"`
	MarkupGTType   string `json:"markup_gt_type" validate:"omitempty,oneof=percent value"`
	MarkupGTAmount string `json:"markup_gt_amount"`
	AgingMonth     *int   `json:"aging_month" validate:"omitempty,min=0"`
}

// BarangMasukUpdateRequest body PATCH /barang-masuk/{id} (partial pricing/meta).
// Harga jual (harga_mt/harga_gt) dan HPP dihitung ulang di server.
type BarangMasukUpdateRequest struct {
	NoFaktur       *string `json:"no_faktur" validate:"omitempty,max=100"`
	NoBatch        *string `json:"no_batch" validate:"omitempty,max=100"`
	Exp            *string `json:"exp"`
	TanggalMasuk   *string `json:"tanggal_masuk"`
	Qty            *int    `json:"qty" validate:"omitempty,min=1"`
	Harga          *string `json:"harga"`
	DiscHPP1       *string `json:"disc_hpp_1"`
	DiscHPP2       *string `json:"disc_hpp_2"`
	DiscHPP3       *string `json:"disc_hpp_3"`
	MarkupMTType   *string `json:"markup_mt_type" validate:"omitempty,oneof=percent value"`
	MarkupMTAmount *string `json:"markup_mt_amount"`
	MarkupGTType   *string `json:"markup_gt_type" validate:"omitempty,oneof=percent value"`
	MarkupGTAmount *string `json:"markup_gt_amount"`
	AgingMonth     *int    `json:"aging_month" validate:"omitempty,min=0"`
}

// BarangMasukListQuery filter daftar penerimaan.
type BarangMasukListQuery struct {
	ListQuery
	Brand      string `form:"brand"`
	KodeBarang string `form:"kode_barang"`
	DateFrom   string `form:"date_from"`
	DateTo     string `form:"date_to"`
}
