package dto

// PelangganResponse respons master pelanggan.
type PelangganResponse struct {
	ID                        uint64 `json:"id"`
	KodePelanggan             string `json:"kode_pelanggan"`
	NamaPelanggan             string `json:"nama_pelanggan"`
	TglRegistrasi             string `json:"tgl_registrasi"`
	Phone                     string `json:"phone"`
	NPWPNIK                   *string `json:"npwp_nik,omitempty"`
	NamaPemilikNPWPNIK        *string `json:"nama_pemilik_npwp_nik,omitempty"`
	AlamatNPWPNIK             *string `json:"alamat_npwp_nik,omitempty"`
	Territory                 string `json:"territory"`
	Distrik                   string `json:"distrik"`
	AlamatToko                string `json:"alamat_toko"`
	RTRW                      *string `json:"rt_rw,omitempty"`
	Provinsi                  string `json:"provinsi"`
	Kabupaten                 string `json:"kabupaten"`
	Kecamatan                 string `json:"kecamatan"`
	Kelurahan                 string `json:"kelurahan"`
	KodePos                   *string `json:"kode_pos,omitempty"`
	ChannelOutlet             string `json:"channel_outlet"`
	AlamatPengantaranBarang   *string `json:"alamat_pengantaran_barang,omitempty"`
	JenisBangunan             *string `json:"jenis_bangunan,omitempty"`
	StatusBangunan            *string `json:"status_bangunan,omitempty"`
	NominalPengambilanPertama string `json:"nominal_pengambilan_pertama"`
	EstimasiBatasKredit       string `json:"estimasi_batas_kredit"`
	IsActive                  bool   `json:"is_active"`
	PunyaTransaksi            bool   `json:"punya_transaksi,omitempty"`
}

// RiwayatTransaksiItem ringkas transaksi outlet pelanggan (SC-06).
type RiwayatTransaksiItem struct {
	ID             uint64  `json:"id"`
	Tanggal        string  `json:"tanggal"`
	TotalAkhir     string  `json:"total_akhir"`
	StatusApproval string  `json:"status_approval"`
	NoTransaksi    *string `json:"no_transaksi"`
}

// PelangganCreateRequest body POST /pelanggan.
type PelangganCreateRequest struct {
	KodePelanggan             *string `json:"kode_pelanggan" validate:"omitempty,max=32"`
	NamaPelanggan             string  `json:"nama_pelanggan" validate:"required,max=255"`
	TglRegistrasi             string  `json:"tgl_registrasi" validate:"required"`
	Phone                     string  `json:"phone" validate:"required,max=30"`
	NPWPNIK                   *string `json:"npwp_nik" validate:"omitempty,max=32"`
	NamaPemilikNPWPNIK        *string `json:"nama_pemilik_npwp_nik" validate:"omitempty,max=255"`
	AlamatNPWPNIK             *string `json:"alamat_npwp_nik"`
	Territory                 string  `json:"territory" validate:"required,max=100"`
	Distrik                   string  `json:"distrik" validate:"required,max=100"`
	AlamatToko                string  `json:"alamat_toko" validate:"required"`
	RTRW                      *string `json:"rt_rw" validate:"omitempty,max=20"`
	Provinsi                  string  `json:"provinsi" validate:"required,max=100"`
	Kabupaten                 string  `json:"kabupaten" validate:"required,max=100"`
	Kecamatan                 string  `json:"kecamatan" validate:"required,max=100"`
	Kelurahan                 string  `json:"kelurahan" validate:"required,max=100"`
	KodePos                   *string `json:"kode_pos" validate:"omitempty,max=10"`
	ChannelOutlet             string  `json:"channel_outlet" validate:"required"`
	AlamatPengantaranBarang   *string `json:"alamat_pengantaran_barang"`
	JenisBangunan             *string `json:"jenis_bangunan" validate:"omitempty,max=50"`
	StatusBangunan            *string `json:"status_bangunan" validate:"omitempty,max=50"`
	NominalPengambilanPertama *string `json:"nominal_pengambilan_pertama"`
	EstimasiBatasKredit       *string `json:"estimasi_batas_kredit"`
}

// PelangganUpdateRequest body PATCH /pelanggan/{id} (partial).
type PelangganUpdateRequest struct {
	KodePelanggan             *string `json:"kode_pelanggan" validate:"omitempty,max=32"`
	NamaPelanggan             *string `json:"nama_pelanggan" validate:"omitempty,max=255"`
	TglRegistrasi             *string `json:"tgl_registrasi"`
	Phone                     *string `json:"phone" validate:"omitempty,max=30"`
	NPWPNIK                   *string `json:"npwp_nik" validate:"omitempty,max=32"`
	NamaPemilikNPWPNIK        *string `json:"nama_pemilik_npwp_nik" validate:"omitempty,max=255"`
	AlamatNPWPNIK             *string `json:"alamat_npwp_nik"`
	Territory                 *string `json:"territory" validate:"omitempty,max=100"`
	Distrik                   *string `json:"distrik" validate:"omitempty,max=100"`
	AlamatToko                *string `json:"alamat_toko"`
	RTRW                      *string `json:"rt_rw" validate:"omitempty,max=20"`
	Provinsi                  *string `json:"provinsi" validate:"omitempty,max=100"`
	Kabupaten                 *string `json:"kabupaten" validate:"omitempty,max=100"`
	Kecamatan                 *string `json:"kecamatan" validate:"omitempty,max=100"`
	Kelurahan                 *string `json:"kelurahan" validate:"omitempty,max=100"`
	KodePos                   *string `json:"kode_pos" validate:"omitempty,max=10"`
	ChannelOutlet             *string `json:"channel_outlet" validate:"omitempty"`
	AlamatPengantaranBarang   *string `json:"alamat_pengantaran_barang"`
	JenisBangunan             *string `json:"jenis_bangunan" validate:"omitempty,max=50"`
	StatusBangunan            *string `json:"status_bangunan" validate:"omitempty,max=50"`
	NominalPengambilanPertama *string `json:"nominal_pengambilan_pertama"`
	EstimasiBatasKredit       *string `json:"estimasi_batas_kredit"`
}

// PelangganStatusRequest soft-nonaktif / aktifkan.
type PelangganStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// PelangganListQuery filter daftar.
type PelangganListQuery struct {
	ListQuery
	ChannelOutlet   string `form:"channel_outlet"`
	Territory       string `form:"territory"`
	Distrik         string `form:"distrik"`
	IsActive        *bool  `form:"is_active"`
	IncludeInactive bool   `form:"include_inactive"`
}

// Select2Result format autocomplete Select2.
type Select2Result struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Select2Pagination pagination.more untuk Select2.
type Select2Pagination struct {
	More bool `json:"more"`
}

// Select2Response envelope pencarian Select2.
type Select2Response struct {
	Results    []Select2Result   `json:"results"`
	Pagination Select2Pagination `json:"pagination"`
}

// PelangganCariQuery query GET /pelanggan/cari.
type PelangganCariQuery struct {
	Q       string `form:"q"`
	Page    int    `form:"page" validate:"omitempty,min=1"`
	PerPage int    `form:"per_page" validate:"omitempty,min=1,max=50"`
}
