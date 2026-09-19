package dto

// DashboardResponse GET /dashboard (SE-05).
type DashboardResponse struct {
	Role             string                      `json:"role"`
	Kartu            DashboardKartu              `json:"kartu"`
	AktivitasTerkini DashboardAktivitas          `json:"aktivitas_terkini"`
	NotifikasiPiutang *NotifikasiPiutangResponse `json:"notifikasi_piutang,omitempty"`
}

// DashboardKartu metrik kartu; field kosong dihilangkan per role via omitempty + pointer/zero.
type DashboardKartu struct {
	TransaksiPending   *int    `json:"transaksi_pending,omitempty"`
	PenjualanBulanIni  *string `json:"penjualan_bulan_ini,omitempty"`
	TotalPiutang       *string `json:"total_piutang,omitempty"`
	PiutangOverdue     *string `json:"piutang_overdue,omitempty"`
	SKUStokRendah      *int    `json:"sku_stok_rendah,omitempty"`
	SKUStokHabis       *int    `json:"sku_stok_habis,omitempty"`
	BatchMendekatiExp  *int    `json:"batch_mendekati_exp,omitempty"`
	// Sales: volume transaksi bulan ini (jumlah, bukan nilai) — opsional; afiliasi tanpa penjualan nilai.
	JumlahTransaksiBulanIni *int `json:"jumlah_transaksi_bulan_ini,omitempty"`
}

// DashboardAktivitas cuplikan aktivitas terkini.
type DashboardAktivitas struct {
	TransaksiPending []DashboardAktivitasItem `json:"transaksi_pending"`
	BarangMasuk      []DashboardAktivitasItem `json:"barang_masuk"`
	Pembayaran       []DashboardAktivitasItem `json:"pembayaran"`
}

// DashboardAktivitasItem baris ringkas.
type DashboardAktivitasItem struct {
	ID        uint64  `json:"id"`
	Judul     string  `json:"judul"`
	Subjudul  string  `json:"subjudul,omitempty"`
	Nominal   *string `json:"nominal,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// DashboardOKResponse envelope.
type DashboardOKResponse struct {
	Data DashboardResponse `json:"data"`
}
