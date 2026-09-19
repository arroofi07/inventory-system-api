package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLaporanFormat_Struktur09Pasal7(t *testing.T) {
	lap := NewLaporan("pkb_lama @ 10.0.0.5", "pkb @ 10.0.0.9")
	lap.Dijalankan = time.Date(2026, 10, 19, 2, 14, 33, 0, time.FixedZone("WIB", 7*3600))
	lap.Berhasil("users")
	lap.Berhasil("users")
	lap.Gagal("barang_masuk", 302, errors.New("duplikat unique key"))
	lap.Peringatan("pelanggan", 0, "14 baris placeholder npwp")
	lap.SetChecksum("abc123")
	lap.SetVerifikasi([]HasilVerifikasi{
		{Kode: "V1", Nama: "Jumlah baris per tabel", Status: "SKIP", Detail: "SF-06"},
		{Kode: "V9", Nama: "Laba total", Status: "TINJAU", Detail: "selisih", WajibLulus: false},
	}, "uji")
	lap.Selesai = lap.Dijalankan.Add(4*time.Minute + 12*time.Second)

	var buf bytes.Buffer
	if err := lap.Format(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"=== Laporan ETL PKB Web ===",
		"Sumber       : pkb_lama @ 10.0.0.5",
		"Target       : pkb @ 10.0.0.9",
		"Checksum     : abc123",
		"--- Ringkasan per tahap ---",
		"users",
		"barang_masuk",
		"--- Baris gagal (perlu tindakan) ---",
		"[barang_masuk id=302]",
		"--- Peringatan ---",
		"--- Transaksi total_akhir tidak konsisten (keuangan) ---",
		"--- Verifikasi paritas ---",
		"V1",
		"KESIMPULAN: uji",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("laporan tidak mengandung %q\n%s", want, out)
		}
	}
}

func TestBandingkanChecksum(t *testing.T) {
	if err := BandingkanChecksum("aaa", "aaa"); err != nil {
		t.Fatalf("sama harus lulus: %v", err)
	}
	if err := BandingkanChecksum("aaa", "bbb"); err == nil {
		t.Fatal("berbeda harus error")
	}
	if err := BandingkanChecksum("", "aaa"); err == nil {
		t.Fatal("kosong harus error")
	}
}

func TestDaftarTahap_Urutan09(t *testing.T) {
	want := []string{
		"users",
		"barang",
		"barang_masuk",
		"pelanggan",
		"promos",
		"transaksi_penjualan",
		"transaksi_detail",
		"transaksi_detail_promo",
		"riwayat_pembayaran",
		"price_change_logs",
		"stock_movements",
		"barang.stok_tersedia",
		"no_transaksi_seq",
	}
	got := daftarTahap()
	if len(got) != len(want) {
		t.Fatalf("jumlah tahap: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Nama() != want[i] {
			t.Errorf("tahap[%d]=%s want %s", i, got[i].Nama(), want[i])
		}
	}
}

func TestDaftarPemeriksaan_V1sampaiV9(t *testing.T) {
	if len(daftarPemeriksaan) != 9 {
		t.Fatalf("want 9 pemeriksaan, got %d", len(daftarPemeriksaan))
	}
	if daftarPemeriksaan[8].Kode != "V9" || daftarPemeriksaan[8].WajibLulus {
		t.Fatal("V9 harus tidak wajib lulus otomatis")
	}
	for i := 0; i < 8; i++ {
		if !daftarPemeriksaan[i].WajibLulus {
			t.Errorf("%s harus wajib lulus", daftarPemeriksaan[i].Kode)
		}
	}
}

func TestLoadETLConfig_SumberTargetBerbeda(t *testing.T) {
	t.Setenv("ETL_SUMBER_HOST", "127.0.0.1")
	t.Setenv("ETL_SUMBER_PORT", "3306")
	t.Setenv("ETL_SUMBER_NAME", "pkb_lama")
	t.Setenv("ETL_SUMBER_USER", "ro")
	t.Setenv("ETL_TARGET_HOST", "127.0.0.1")
	t.Setenv("ETL_TARGET_PORT", "3307")
	t.Setenv("ETL_TARGET_NAME", "pkb")
	t.Setenv("ETL_TARGET_USER", "pkb_app")
	t.Setenv("DB_PASSWORD", "secret")

	cfg, err := LoadETLConfig(false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sumber.Name == cfg.Target.Name && cfg.Sumber.Port == cfg.Target.Port {
		t.Fatal("sumber/target tidak terpisah")
	}
}

func TestLoadETLConfig_TolakDatabaseSama(t *testing.T) {
	t.Setenv("ETL_SUMBER_HOST", "127.0.0.1")
	t.Setenv("ETL_SUMBER_PORT", "3307")
	t.Setenv("ETL_SUMBER_NAME", "pkb")
	t.Setenv("ETL_SUMBER_USER", "ro")
	t.Setenv("ETL_TARGET_HOST", "127.0.0.1")
	t.Setenv("ETL_TARGET_PORT", "3307")
	t.Setenv("ETL_TARGET_NAME", "pkb")
	t.Setenv("ETL_TARGET_USER", "pkb_app")

	_, err := LoadETLConfig(false)
	if err == nil {
		t.Fatal("harus menolak sumber==target")
	}
}

func TestRingkasKesimpulan_Skip(t *testing.T) {
	hasil := []HasilVerifikasi{
		{Kode: "V1", Status: "SKIP", WajibLulus: true},
		{Kode: "V9", Status: "SKIP", WajibLulus: false},
	}
	s := ringkasKesimpulan(hasil)
	if !strings.Contains(s, "SKIP") {
		t.Fatalf("want SKIP di kesimpulan, got %q", s)
	}
}
