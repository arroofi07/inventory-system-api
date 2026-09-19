package domain

type Permission string

const (
	PermUserLihat  Permission = "user.lihat"
	PermUserKelola Permission = "user.kelola"

	PermBarangLihat  Permission = "barang.lihat"
	PermBarangKelola Permission = "barang.kelola"
	PermBarangHapus  Permission = "barang.hapus"

	PermBarangMasukLihat Permission = "barang_masuk.lihat"
	PermBarangMasukBuat  Permission = "barang_masuk.buat"
	PermBarangMasukUbah  Permission = "barang_masuk.ubah"
	PermBarangMasukHapus Permission = "barang_masuk.hapus"
	PermHargaKelola      Permission = "harga.kelola"

	PermPelangganLihat Permission = "pelanggan.lihat"
	PermPelangganBuat  Permission = "pelanggan.buat"
	PermPelangganUbah  Permission = "pelanggan.ubah"

	PermTransaksiLihatSemua Permission = "transaksi.lihat_semua"
	PermTransaksiLihatMilik Permission = "transaksi.lihat_milik"
	PermTransaksiBuat       Permission = "transaksi.buat"
	PermTransaksiUbah       Permission = "transaksi.ubah"

	PermApprovalLakukan  Permission = "approval.lakukan"
	PermApprovalBatalkan Permission = "approval.batalkan"

	PermFakturCetak      Permission = "faktur.cetak"
	PermFakturCetakUlang Permission = "faktur.cetak_ulang"

	PermPembayaranLihat Permission = "pembayaran.lihat"
	PermPembayaranCatat Permission = "pembayaran.catat"

	PermPromoLihat  Permission = "promo.lihat"
	PermPromoKelola Permission = "promo.kelola"
	PermPromoHapus  Permission = "promo.hapus"

	PermStokLihat     Permission = "stok.lihat"
	PermStokSesuaikan Permission = "stok.sesuaikan"

	PermLaporanStok      Permission = "laporan.stok"
	PermLaporanPenjualan Permission = "laporan.penjualan"
	PermLaporanLaba      Permission = "laporan.laba"
	PermLaporanAnalytics Permission = "laporan.analytics"
	PermLaporanEkspor    Permission = "laporan.ekspor"

	PermAuditLihat Permission = "audit.lihat"
)

var semuaPermission = []Permission{
	PermUserLihat, PermUserKelola,
	PermBarangLihat, PermBarangKelola, PermBarangHapus,
	PermBarangMasukLihat, PermBarangMasukBuat, PermBarangMasukUbah, PermBarangMasukHapus, PermHargaKelola,
	PermPelangganLihat, PermPelangganBuat, PermPelangganUbah,
	PermTransaksiLihatSemua, PermTransaksiLihatMilik, PermTransaksiBuat, PermTransaksiUbah,
	PermApprovalLakukan, PermApprovalBatalkan,
	PermFakturCetak, PermFakturCetakUlang,
	PermPembayaranLihat, PermPembayaranCatat,
	PermPromoLihat, PermPromoKelola, PermPromoHapus,
	PermStokLihat, PermStokSesuaikan,
	PermLaporanStok, PermLaporanPenjualan, PermLaporanLaba, PermLaporanAnalytics, PermLaporanEkspor,
	PermAuditLihat,
}

func himpunan(ps ...Permission) map[Permission]bool {
	m := make(map[Permission]bool, len(ps))
	for _, p := range ps {
		m[p] = true
	}
	return m
}

func semuaIzin() map[Permission]bool {
	return himpunan(semuaPermission...)
}

// izinPerRole adalah satu-satunya tempat pemetaan role ke izin.
var izinPerRole = map[Role]map[Permission]bool{
	RoleSuperAdmin: semuaIzin(),
	RoleAdmin: himpunan(
		PermUserLihat,
		PermBarangLihat, PermBarangKelola,
		PermBarangMasukLihat, PermBarangMasukBuat,
		PermPelangganLihat, PermPelangganBuat,
		PermTransaksiLihatSemua, PermTransaksiUbah,
		PermApprovalLakukan,
		PermFakturCetak,
		PermPembayaranLihat, PermPembayaranCatat,
		PermPromoLihat, PermPromoKelola,
		PermStokLihat,
		PermLaporanStok, PermLaporanPenjualan, PermLaporanLaba,
		PermLaporanAnalytics, PermLaporanEkspor,
	),
	RoleAfiliasi: himpunan(
		PermBarangLihat,
		PermBarangMasukLihat,
		PermTransaksiLihatSemua,
		PermFakturCetak,
		PermPembayaranLihat,
		PermPromoLihat,
		PermStokLihat,
		PermLaporanStok, PermLaporanPenjualan, PermLaporanEkspor,
	),
	RoleSales: himpunan(
		PermBarangLihat,
		PermPelangganLihat,
		PermPromoLihat,
		PermTransaksiLihatMilik, PermTransaksiBuat,
	),
}

func (r Role) Punya(p Permission) bool {
	return izinPerRole[r][p]
}

func (r Role) PunyaSalahSatu(ps ...Permission) bool {
	for _, p := range ps {
		if r.Punya(p) {
			return true
		}
	}
	return false
}

// IzinMengembalikan salinan himpunan izin role (untuk tes / introspeksi).
func (r Role) Izin() map[Permission]bool {
	src := izinPerRole[r]
	out := make(map[Permission]bool, len(src))
	for p, ok := range src {
		out[p] = ok
	}
	return out
}
