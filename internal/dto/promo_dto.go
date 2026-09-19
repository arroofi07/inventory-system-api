package dto

// PromoResponse respons master promo.
type PromoResponse struct {
	ID                 uint64  `json:"id"`
	KodePromo          string  `json:"kode_promo"`
	NamaPromo          string  `json:"nama_promo"`
	Deskripsi          *string `json:"deskripsi,omitempty"`
	TipePromo          string  `json:"tipe_promo"`
	BuyQty             *int    `json:"buy_qty,omitempty"`
	GetQty             *int    `json:"get_qty,omitempty"`
	BonusQty           int     `json:"bonus_qty"`
	DiscountPercentage string  `json:"discount_percentage"`
	DiscountAmount     string  `json:"discount_amount"`
	MinQty             int     `json:"min_qty"`
	MinAmount          string  `json:"min_amount"`
	MaxApplications    *int    `json:"max_applications,omitempty"`
	KodeBarang         *string `json:"kode_barang,omitempty"`
	TanggalMulai       string  `json:"tanggal_mulai"`
	TanggalBerakhir    string  `json:"tanggal_berakhir"`
	IsActive           bool    `json:"is_active"`
	SyaratKetentuan    *string `json:"syarat_ketentuan,omitempty"`
}

// PromoCreateRequest body POST /promo.
type PromoCreateRequest struct {
	KodePromo          *string `json:"kode_promo" validate:"omitempty,max=64"`
	NamaPromo          string  `json:"nama_promo" validate:"required,max=255"`
	Deskripsi          *string `json:"deskripsi"`
	TipePromo          string  `json:"tipe_promo" validate:"required,oneof=buy_x_get_y bonus_qty percentage_discount fixed_discount"`
	BuyQty             *int    `json:"buy_qty" validate:"omitempty,min=1"`
	GetQty             *int    `json:"get_qty" validate:"omitempty,min=1"`
	BonusQty           *int    `json:"bonus_qty" validate:"omitempty,min=0"`
	DiscountPercentage *string `json:"discount_percentage"`
	DiscountAmount     *string `json:"discount_amount"`
	MinQty             *int    `json:"min_qty" validate:"omitempty,min=1"`
	MinAmount          *string `json:"min_amount"`
	MaxApplications    *int    `json:"max_applications" validate:"omitempty,min=1"`
	KodeBarang         *string `json:"kode_barang" validate:"omitempty,max=64"`
	TanggalMulai       string  `json:"tanggal_mulai" validate:"required"`
	TanggalBerakhir    string  `json:"tanggal_berakhir" validate:"required"`
	SyaratKetentuan    *string `json:"syarat_ketentuan"`
	IsActive           *bool   `json:"is_active"`
}

// PromoUpdateRequest body PATCH /promo/{id}.
type PromoUpdateRequest struct {
	NamaPromo          *string `json:"nama_promo" validate:"omitempty,max=255"`
	Deskripsi          *string `json:"deskripsi"`
	TipePromo          *string `json:"tipe_promo" validate:"omitempty,oneof=buy_x_get_y bonus_qty percentage_discount fixed_discount"`
	BuyQty             *int    `json:"buy_qty" validate:"omitempty,min=1"`
	GetQty             *int    `json:"get_qty" validate:"omitempty,min=1"`
	BonusQty           *int    `json:"bonus_qty" validate:"omitempty,min=0"`
	DiscountPercentage *string `json:"discount_percentage"`
	DiscountAmount     *string `json:"discount_amount"`
	MinQty             *int    `json:"min_qty" validate:"omitempty,min=1"`
	MinAmount          *string `json:"min_amount"`
	MaxApplications    *int    `json:"max_applications" validate:"omitempty,min=1"`
	KodeBarang         *string `json:"kode_barang" validate:"omitempty,max=64"`
	TanggalMulai       *string `json:"tanggal_mulai"`
	TanggalBerakhir    *string `json:"tanggal_berakhir"`
	SyaratKetentuan    *string `json:"syarat_ketentuan"`
}

// PromoStatusRequest toggle aktif.
type PromoStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// PromoListQuery filter daftar / aktif untuk form sales.
type PromoListQuery struct {
	ListQuery
	TipePromo       string `form:"tipe_promo"`
	KodeBarang      string `form:"kode_barang"`
	Aktif           *bool  `form:"aktif"`
	IsActive        *bool  `form:"is_active"`
	IncludeInactive bool   `form:"include_inactive"`
}
