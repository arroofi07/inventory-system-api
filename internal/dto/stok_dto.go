package dto

// PergerakanStokListQuery GET /stok/pergerakan.
type PergerakanStokListQuery struct {
	ListQuery
	KodeBarang   string `form:"kode_barang" validate:"required"`
	DateFrom     string `form:"date_from"`
	DateTo       string `form:"date_to"`
	MovementType string `form:"movement_type"`
}

// PergerakanStokItemResponse baris kartu stok.
type PergerakanStokItemResponse struct {
	ID            uint64  `json:"id"`
	CreatedAt     string  `json:"created_at"`
	MovementType  string  `json:"movement_type"`
	NoBatch       *string `json:"no_batch"`
	Qty           int     `json:"qty"`
	SaldoSetelah  int     `json:"saldo_setelah"`
	ReferenceType *string `json:"reference_type"`
	ReferenceNo   *string `json:"reference_no"`
	Keterangan    *string `json:"keterangan"`
	Oleh          *string `json:"oleh"`
}

// PergerakanStokRingkasan agregat kartu stok.
type PergerakanStokRingkasan struct {
	SaldoAwal   int `json:"saldo_awal"`
	TotalMasuk  int `json:"total_masuk"`
	TotalKeluar int `json:"total_keluar"`
	SaldoAkhir  int `json:"saldo_akhir"`
}

// PenyesuaianStokRequest POST /stok/penyesuaian (super_admin).
type PenyesuaianStokRequest struct {
	KodeBarang    string  `json:"kode_barang" validate:"required"`
	Qty           int     `json:"qty" validate:"required,ne=0"`
	Alasan        string  `json:"alasan" validate:"required,min=3,max=2000"`
	BarangMasukID *uint64 `json:"barang_masuk_id"`
}

// PenyesuaianStokResponse hasil penyesuaian.
type PenyesuaianStokResponse struct {
	ID           uint64 `json:"id"`
	KodeBarang   string `json:"kode_barang"`
	Qty          int    `json:"qty"`
	SaldoSetelah int    `json:"saldo_setelah"`
	MovementType string `json:"movement_type"`
	CreatedAt    string `json:"created_at"`
}

// RekonsiliasiBaris penyimpangan ledger vs stok_tersedia.
type RekonsiliasiBaris struct {
	BarangID      uint64 `json:"barang_id"`
	KodeBarang    string `json:"kode_barang"`
	NamaItem      string `json:"nama_item"`
	StokTersedia  int    `json:"stok_tersedia"`
	SaldoLedger   int    `json:"saldo_ledger"`
	Selisih       int    `json:"selisih"`
}

// RekonsiliasiHasil hasil job/API rekonsiliasi.
type RekonsiliasiHasil struct {
	Diperiksa   int                 `json:"diperiksa"`
	Menyimpang  int                 `json:"menyimpang"`
	Penyimpangan []RekonsiliasiBaris `json:"penyimpangan"`
}
