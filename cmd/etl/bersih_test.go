package main

import (
	"context"
	"strings"
	"testing"
)

func TestBolehTulisSumber(t *testing.T) {
	t.Setenv("ETL_SUMBER_BOLEH_TULIS", "")
	if bolehTulisSumber() {
		t.Fatal("kosong harus ditolak")
	}
	t.Setenv("ETL_SUMBER_BOLEH_TULIS", "yes")
	if bolehTulisSumber() {
		t.Fatal("yes harus ditolak")
	}
	t.Setenv("ETL_SUMBER_BOLEH_TULIS", "true")
	if !bolehTulisSumber() {
		t.Fatal("true harus diizinkan")
	}
	t.Setenv("ETL_SUMBER_BOLEH_TULIS", "TRUE")
	if !bolehTulisSumber() {
		t.Fatal("TRUE harus diizinkan")
	}
}

func TestSumberExecDitolakTanpaGerbang(t *testing.T) {
	s := &Sumber{}
	if _, err := s.ExecContext(context.Background(), "UPDATE users SET id = id"); err == nil {
		t.Fatal("exec tanpa bolehTulis harus error")
	}
}

func TestRencanaDupKTP_PertahankanIDTerkecil(t *testing.T) {
	got := rencanaDupKTP(map[string][]uint64{
		"123": {10, 3, 7},
		"999": {1},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 grup duplikat, got %d", len(got))
	}
	if got[0].PertahankanID != 3 {
		t.Fatalf("pertahankan %d want 3", got[0].PertahankanID)
	}
	if len(got[0].NullkanIDs) != 2 || got[0].NullkanIDs[0] != 7 || got[0].NullkanIDs[1] != 10 {
		t.Fatalf("nullkan %v", got[0].NullkanIDs)
	}
}

func TestRencanaDupBatch_SuffixDanTabrak(t *testing.T) {
	baris := []barisBatch{
		{ID: 1, BarangID: 9, NoBatch: "B1", NoFaktur: "FK-001"},
		{ID: 2, BarangID: 9, NoBatch: "B1", NoFaktur: "FK-001"},
		{ID: 3, BarangID: 9, NoBatch: "B1", NoFaktur: "FK-001-2"},
	}
	existing := map[string]uint64{}
	for _, b := range baris {
		existing[kunciBatch(b.BarangID, b.NoBatch, b.NoFaktur)] = b.ID
	}
	got := rencanaDupBatch(baris, existing)
	if len(got) != 1 || len(got[0].Ubah) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Ubah[0].ID != 2 {
		t.Fatalf("ubah id %d", got[0].Ubah[0].ID)
	}
	if got[0].Ubah[0].NoFakturBaru != "FK-001-3" {
		t.Fatalf("suffix tabrak FK-001-2 harus -3, got %s", got[0].Ubah[0].NoFakturBaru)
	}
}

func TestIsiNomorI7(t *testing.T) {
	got := isiNomorI7([]TemuanI7{
		{ID: 20, PerluNomor: true},
		{ID: 5, PerluNomor: true, PerluApprovedAt: true},
		{ID: 9, PerluApprovedAt: true},
	}, 41)
	if got[0].ID != 5 || got[0].NomorBaru != "000042" {
		t.Fatalf("id terkecil dulu: %+v", got[0])
	}
	if got[1].ID != 9 || got[1].NomorBaru != "" {
		t.Fatalf("tanpa nomor: %+v", got[1])
	}
	if got[2].NomorBaru != "000043" {
		t.Fatalf("nomor kedua %s", got[2].NomorBaru)
	}
}

func TestParseNomorMurni(t *testing.T) {
	n, ok := parseNomorMurni("000010")
	if !ok || n != 10 {
		t.Fatalf("%d %v", n, ok)
	}
	if _, ok := parseNomorMurni("INV-1"); ok {
		t.Fatal("bukan angka murni")
	}
}

func TestLaporanAuditFormat(t *testing.T) {
	lap := &LaporanAudit{
		Sumber:       "pkb_lama_copy @ 127.0.0.1:3306",
		Mode:         "AUDIT",
		DupKTP:       []TemuanDupKTP{{NoKTP: "111", PertahankanID: 1, NullkanIDs: []uint64{2}}},
		ChannelGagal: []TemuanChannelUnmapped{{ID: 8, Nilai: "ritel"}},
	}
	var b strings.Builder
	if err := lap.Format(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"=== Audit sumber Laravel (SF-02) ===",
		"--- Duplikat no_ktp ---",
		"--- Channel tidak terpetakan (koreksi manual) ---",
		"[id=8] \"ritel\"",
		"1 perubahan otomatis, 1 channel belum terpetakan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("tidak mengandung %q\n%s", want, out)
		}
	}
}
