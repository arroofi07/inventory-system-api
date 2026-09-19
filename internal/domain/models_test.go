package domain_test

import (
	"testing"

	"app/internal/domain"
)

func TestTableNames(t *testing.T) {
	kasus := []struct {
		got, want string
	}{
		{domain.User{}.TableName(), "users"},
		{domain.RefreshToken{}.TableName(), "refresh_tokens"},
		{domain.Barang{}.TableName(), "barang"},
		{domain.BarangMasuk{}.TableName(), "barang_masuk"},
		{domain.Pelanggan{}.TableName(), "pelanggan"},
		{domain.Promo{}.TableName(), "promos"},
		{domain.StockMovement{}.TableName(), "stock_movements"},
		{domain.TransaksiPenjualan{}.TableName(), "transaksi_penjualan"},
		{domain.TransaksiDetail{}.TableName(), "transaksi_detail"},
		{domain.TransaksiDetailPromo{}.TableName(), "transaksi_detail_promo"},
		{domain.RiwayatPembayaran{}.TableName(), "riwayat_pembayaran"},
		{domain.PriceChangeLog{}.TableName(), "price_change_logs"},
		{domain.StockAlert{}.TableName(), "stock_alerts"},
		{domain.NoTransaksiSeq{}.TableName(), "no_transaksi_seq"},
		{domain.AuditLog{}.TableName(), "audit_logs"},
	}
	for _, k := range kasus {
		if k.got != k.want {
			t.Fatalf("TableName: got %q want %q", k.got, k.want)
		}
	}
}

func TestStatusStok(t *testing.T) {
	kasus := []struct {
		stok, min int
		want      string
	}{
		{0, 5, "HABIS"},
		{-1, 5, "HABIS"},
		{3, 5, "RENDAH"},
		{5, 5, "RENDAH"},
		{6, 5, "NORMAL"},
	}
	for _, k := range kasus {
		got := domain.Barang{StokTersedia: k.stok, MinStock: k.min}.StatusStok()
		if got != k.want {
			t.Fatalf("stok=%d min=%d: got %s want %s", k.stok, k.min, got, k.want)
		}
	}
}

func TestMetodeAlokasiValid(t *testing.T) {
	if !domain.AlokasiFEFO.Valid() || !domain.AlokasiFIFO.Valid() {
		t.Fatal("FEFO/FIFO harus valid")
	}
	if domain.MetodeAlokasi("LIFO").Valid() {
		t.Fatal("LIFO tidak valid")
	}
}

func TestUserPublic(t *testing.T) {
	u := domain.User{ID: 7, Name: "Ada", Email: "a@b.c", Role: domain.RoleAdmin, IsActive: true}
	p := u.Public()
	if p.ID != 7 || p.Name != "Ada" || p.Email != "a@b.c" || p.Role != domain.RoleAdmin || !p.IsActive {
		t.Fatalf("Public mismatch: %+v", p)
	}
}

func TestPunyaSalahSatu(t *testing.T) {
	if !domain.RoleSales.PunyaSalahSatu(domain.PermLaporanLaba, domain.PermTransaksiBuat) {
		t.Fatal("sales punya transaksi.buat")
	}
	if domain.RoleSales.PunyaSalahSatu(domain.PermLaporanLaba) {
		t.Fatal("sales tidak punya laporan.laba")
	}
	if domain.RoleSales.PunyaSalahSatu() {
		t.Fatal("daftar kosong → false")
	}
}
