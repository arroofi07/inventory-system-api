package dto

// BarangResponse respons master barang (termasuk status_stok turunan).
type BarangResponse struct {
	ID                  uint64 `json:"id"`
	KodeBarang          string `json:"kode_barang"`
	NamaItem            string `json:"nama_item"`
	Brand               string `json:"brand"`
	Satuan              string `json:"satuan"`
	StokTersedia        int    `json:"stok_tersedia"`
	MinStock            int    `json:"min_stock"`
	ReorderPoint        int    `json:"reorder_point"`
	StatusStok          string `json:"status_stok"`
	MetodeAlokasi       string `json:"metode_alokasi"`
	ExpiryAlertDays     int    `json:"expiry_alert_days"`
	JumlahBatchTersedia int    `json:"jumlah_batch_tersedia,omitempty"`
	IsActive            bool   `json:"is_active"`
}

// BarangCreateRequest body POST /barang.
type BarangCreateRequest struct {
	KodeBarang      string `json:"kode_barang" validate:"required,max=64"`
	NamaItem        string `json:"nama_item" validate:"required,max=255"`
	Brand           string `json:"brand" validate:"required,max=100"`
	Satuan          string `json:"satuan" validate:"omitempty,max=20"`
	MinStock        *int   `json:"min_stock" validate:"omitempty,min=0"`
	ReorderPoint    *int   `json:"reorder_point" validate:"omitempty,min=0"`
	MetodeAlokasi   string `json:"metode_alokasi" validate:"omitempty,oneof=FEFO FIFO"`
	ExpiryAlertDays *int   `json:"expiry_alert_days" validate:"omitempty,min=0"`
}

// BarangUpdateRequest body PATCH /barang/{id} (partial).
type BarangUpdateRequest struct {
	NamaItem        *string `json:"nama_item" validate:"omitempty,max=255"`
	Brand           *string `json:"brand" validate:"omitempty,max=100"`
	Satuan          *string `json:"satuan" validate:"omitempty,max=20"`
	MinStock        *int    `json:"min_stock" validate:"omitempty,min=0"`
	ReorderPoint    *int    `json:"reorder_point" validate:"omitempty,min=0"`
	MetodeAlokasi   *string `json:"metode_alokasi" validate:"omitempty,oneof=FEFO FIFO"`
	ExpiryAlertDays *int    `json:"expiry_alert_days" validate:"omitempty,min=0"`
}

// BarangStatusRequest soft-nonaktif / aktifkan kembali.
type BarangStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// BarangListQuery filter daftar barang.
type BarangListQuery struct {
	ListQuery
	Brand           string `form:"brand"`
	IsActive        *bool  `form:"is_active"`
	IncludeInactive bool   `form:"include_inactive"`
	StatusStok      string `form:"status_stok" validate:"omitempty,oneof=NORMAL RENDAH HABIS"`
	WithStock       bool   `form:"with_stock"`
	Popular         bool   `form:"popular"`
}
