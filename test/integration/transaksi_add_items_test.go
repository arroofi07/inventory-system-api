package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app/internal/domain"
)

// TestTambahItemsHitungUlangDariNol memverifikasi perbaikan bug 11 §2.1:
// DPP 200k + item 100k + PPN 11% → total_akhir = 333.000 (bukan 357.420).
func TestTambahItemsHitungUlangDariNol(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)
	kodeB := fmt.Sprintf("SC02_B_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC04",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC04 1",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)
	if wPl.Code != http.StatusCreated {
		t.Fatalf("pelanggan: %d %s", wPl.Code, wPl.Body.String())
	}

	postBM := func(kode, faktur, harga string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_barang": kode, "buat_barang_baru": true, "nama_item": "Item " + kode,
			"brand": "SC04", "no_faktur": faktur, "no_batch": "B1",
			"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 100, "harga": harga,
			"markup_mt_type": "percent", "markup_mt_amount": "0.00",
			"markup_gt_type": "percent", "markup_gt_amount": "0.00",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("bm: %d %s", w.Code, w.Body.String())
		}
	}
	postBM(kodeA, fmt.Sprintf("SC04_FA_%d", suffix), "200000.00")
	postBM(kodeB, fmt.Sprintf("SC04_FB_%d", suffix), "100000.00")

	buatBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung Timur",
		"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "200000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00",
		"tanggal_jatuh_tempo": "2026-10-04",
	})
	wBuat := httptest.NewRecorder()
	reqBuat := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(buatBody))
	reqBuat.Header.Set("Content-Type", "application/json")
	reqBuat.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wBuat, reqBuat)
	if wBuat.Code != http.StatusCreated {
		t.Fatalf("buat: %d %s", wBuat.Code, wBuat.Body.String())
	}
	var created struct {
		Data struct {
			ID         uint64 `json:"id"`
			TotalAkhir string `json:"total_akhir"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wBuat.Body.Bytes(), &created)
	if created.Data.TotalAkhir != "222000.00" {
		t.Fatalf("awal total_akhir: got %s want 222000.00", created.Data.TotalAkhir)
	}

	var stockBefore int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockBefore)
	var mvBefore int64
	_ = db.Table("stock_movements").Where("barang_id IN (SELECT id FROM barang WHERE kode_barang IN (?,?))", kodeA, kodeB).Count(&mvBefore)

	addBody, _ := json.Marshal(map[string]any{
		"item": map[string]any{"kode_item": kodeB, "qty": 1, "harga": "100000.00"},
	})
	wAdd := httptest.NewRecorder()
	reqAdd := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/transaksi/%d/items", created.Data.ID), bytes.NewReader(addBody))
	reqAdd.Header.Set("Content-Type", "application/json")
	reqAdd.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wAdd, reqAdd)
	if wAdd.Code != http.StatusOK {
		t.Fatalf("tambah items: %d %s", wAdd.Code, wAdd.Body.String())
	}

	var after struct {
		Data struct {
			IsMultiItem bool   `json:"is_multi_item"`
			JumlahItem  int    `json:"jumlah_item"`
			Total       string `json:"total"`
			PPNNominal  string `json:"ppn_nominal"`
			TotalAkhir  string `json:"total_akhir"`
			Items       []any  `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wAdd.Body.Bytes(), &after)

	if !after.Data.IsMultiItem || after.Data.JumlahItem != 2 || len(after.Data.Items) != 2 {
		t.Fatalf("multi-item: is_multi=%v jumlah=%d items=%d",
			after.Data.IsMultiItem, after.Data.JumlahItem, len(after.Data.Items))
	}
	if after.Data.Total != "300000.00" {
		t.Fatalf("total (DPP): got %s want 300000.00", after.Data.Total)
	}
	if after.Data.PPNNominal != "33000.00" {
		t.Fatalf("ppn: got %s want 33000.00", after.Data.PPNNominal)
	}
	if after.Data.TotalAkhir != "333000.00" {
		t.Fatalf("total_akhir: got %s want 333000.00 (bukan 357420.00)", after.Data.TotalAkhir)
	}

	var stockAfter int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockAfter)
	var mvAfter int64
	_ = db.Table("stock_movements").Where("barang_id IN (SELECT id FROM barang WHERE kode_barang IN (?,?))", kodeA, kodeB).Count(&mvAfter)
	if stockAfter != stockBefore {
		t.Fatalf("stok berubah: before=%d after=%d", stockBefore, stockAfter)
	}
	if mvAfter != mvBefore {
		t.Fatalf("stock_movements berubah: before=%d after=%d", mvBefore, mvAfter)
	}

	emailOther := fmt.Sprintf("sa06_sc04_sl2_%d@pkb.test", suffix)
	createUser(t, emailOther, "rahasia123", domain.RoleSales, true)
	tokOther := loginToken(t, r, emailOther, "rahasia123")
	wOther := httptest.NewRecorder()
	reqOther := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/transaksi/%d/items", created.Data.ID), bytes.NewReader(addBody))
	reqOther.Header.Set("Content-Type", "application/json")
	reqOther.Header.Set("Authorization", "Bearer "+tokOther)
	r.ServeHTTP(wOther, reqOther)
	if wOther.Code != http.StatusNotFound {
		t.Fatalf("sales lain: got %d want 404", wOther.Code)
	}
}
