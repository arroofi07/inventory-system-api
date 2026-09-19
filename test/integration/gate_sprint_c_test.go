package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGateSprintCF4 mengikat kriteria selesai F4 (SC-15) dalam satu alur E2E.
func TestGateSprintCF4(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Gate F4",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. Gate",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item Gate",
		"brand": "GATE", "no_faktur": fmt.Sprintf("GATE_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 10, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)
	if wBM.Code != http.StatusCreated {
		t.Fatalf("bm: %d %s", wBM.Code, wBM.Body.String())
	}

	buat := func(qty int) uint64 {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
			"item": map[string]any{"kode_item": kodeA, "qty": qty, "harga": "10000.00"},
			"ppn_persen": "11.00", "nominal_dibayar": "0.00",
			"tanggal_jatuh_tempo": "2026-10-04",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSales)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("buat: %d %s", w.Code, w.Body.String())
		}
		var out struct {
			Data struct {
				ID uint64 `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.Data.ID
	}

	idOK := buat(6)
	idOverlap := buat(6)
	idReject := buat(1)

	var stokPending int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stokPending)
	if stokPending != 10 {
		t.Fatalf("pending potong stok: %d", stokPending)
	}

	// Approve pertama → stok turun
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", idOK), bytes.NewReader([]byte(`{}`)))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("approve1: %d %s", w1.Code, w1.Body.String())
	}
	var stok1 int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok1)
	if stok1 != 4 {
		t.Fatalf("stok setelah approve=%d", stok1)
	}

	// Overlap ditolak stok, tetap pending
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", idOverlap), bytes.NewReader([]byte(`{}`)))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("overlap harus 409: %d", w2.Code)
	}
	var status2 string
	_ = db.Raw(`SELECT status_approval FROM transaksi_penjualan WHERE id = ?`, idOverlap).Scan(&status2)
	if status2 != "pending" {
		t.Fatalf("status overlap=%s", status2)
	}
	var stok2 int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok2)
	if stok2 != 4 {
		t.Fatalf("rollback gagal stok=%d", stok2)
	}

	// Reject → stok sama
	rej, _ := json.Marshal(map[string]any{"approval_notes": "Stok tidak cukup, batalkan"})
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/reject", idReject), bytes.NewReader(rej))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w3.Code, w3.Body.String())
	}
	var stok3 int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok3)
	if stok3 != 4 {
		t.Fatalf("reject ubah stok=%d", stok3)
	}

	// Ekspor penjualan default approved (tanpa status_approval)
	wExp := httptest.NewRecorder()
	reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penjualan/export", nil)
	reqExp.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK {
		t.Fatalf("export: %d %s", wExp.Code, wExp.Body.String())
	}
	csv := wExp.Body.String()
	if !strings.Contains(csv, "no_transaksi") {
		t.Fatal("header CSV hilang")
	}
	// pending/rejected tidak boleh muncul bila default approved
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if strings.Contains(line, ",pending,") || strings.Contains(line, ",rejected,") {
			t.Fatalf("baris non-approved di ekspor default: %s", line)
		}
	}

	var ledger int
	_ = db.Raw(`SELECT COALESCE(SUM(qty),0) FROM stock_movements WHERE barang_id=(SELECT id FROM barang WHERE kode_barang=?)`, kodeA).Scan(&ledger)
	if ledger != stok3 {
		t.Fatalf("ledger=%d stok=%d", ledger, stok3)
	}
}

func TestEksporPenjualanDefaultApproved(t *testing.T) {
	r, _, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penjualan/export?status_approval=pending", nil)
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export pending filter: %d", w.Code)
	}
}
