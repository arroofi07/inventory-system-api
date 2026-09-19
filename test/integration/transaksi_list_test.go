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

func TestDaftarTransaksiIsolasiSalesDanFilter(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC07",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC07 1",
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

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item SC07",
		"brand": "SC07", "no_faktur": fmt.Sprintf("SC07_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 50, "harga": "10000.00",
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

	buat := func(tok string) uint64 {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
			"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
			"ppn_persen": "11.00", "nominal_dibayar": "0.00",
			"tanggal_jatuh_tempo": "2026-10-04",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
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

	id1 := buat(tokSales)

	emailOther := fmt.Sprintf("sa06_sc07_sl2_%d@pkb.test", suffix)
	createUser(t, emailOther, "rahasia123", domain.RoleSales, true)
	tokOther := loginToken(t, r, emailOther, "rahasia123")
	id2 := buat(tokOther)

	// Sales 1 list — hanya miliknya
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/transaksi?q="+kodePlg, nil)
	reqList.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("list sales: %d %s", wList.Code, wList.Body.String())
	}
	var list struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &list)
	seen := map[uint64]bool{}
	for _, row := range list.Data {
		seen[row.ID] = true
	}
	if !seen[id1] || seen[id2] {
		t.Fatalf("isolasi list: seen=%v id1=%d id2=%d", seen, id1, id2)
	}

	// Filter status
	wF := httptest.NewRecorder()
	reqF := httptest.NewRequest(http.MethodGet, "/api/v1/transaksi?status_approval=pending&q="+kodePlg, nil)
	reqF.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wF, reqF)
	if wF.Code != http.StatusOK {
		t.Fatalf("filter: %d", wF.Code)
	}

	// Detail sales lain → 404
	wD := httptest.NewRecorder()
	reqD := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d", id2), nil)
	reqD.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wD, reqD)
	if wD.Code != http.StatusNotFound {
		t.Fatalf("detail sales lain: %d", wD.Code)
	}

	// Detail milik → jumlah vs total_qty_keluar
	wOk := httptest.NewRecorder()
	reqOk := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d", id1), nil)
	reqOk.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wOk, reqOk)
	if wOk.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", wOk.Code, wOk.Body.String())
	}
	var det struct {
		Data struct {
			Items []struct {
				Jumlah         int `json:"jumlah"`
				TotalQtyKeluar int `json:"total_qty_keluar"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wOk.Body.Bytes(), &det)
	if len(det.Data.Items) != 1 || det.Data.Items[0].Jumlah < 1 {
		t.Fatalf("detail items: %+v", det.Data.Items)
	}
	if det.Data.Items[0].TotalQtyKeluar < det.Data.Items[0].Jumlah {
		t.Fatalf("total_qty_keluar harus >= jumlah")
	}

	// SA list melihat keduanya
	wSA := httptest.NewRecorder()
	reqSA := httptest.NewRequest(http.MethodGet, "/api/v1/transaksi?q="+kodePlg, nil)
	reqSA.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wSA, reqSA)
	if wSA.Code != http.StatusOK {
		t.Fatalf("list sa: %d", wSA.Code)
	}
	var listSA struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wSA.Body.Bytes(), &listSA)
	seenSA := map[uint64]bool{}
	for _, row := range listSA.Data {
		seenSA[row.ID] = true
	}
	if !seenSA[id1] || !seenSA[id2] {
		t.Fatalf("sa harus lihat semua: %v", seenSA)
	}
}

func TestSalesTidakBolehMutasiTransaksi(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC08",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC08 1",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)
	if wPl.Code != http.StatusCreated {
		t.Fatalf("pelanggan: %d", wPl.Code)
	}

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item SC08",
		"brand": "SC08", "no_faktur": fmt.Sprintf("SC08_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 20, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)
	if wBM.Code != http.StatusCreated {
		t.Fatalf("bm: %d", wBM.Code)
	}

	buatBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
		"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
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
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wBuat.Body.Bytes(), &created)

	patchBody := []byte(`{"area":"diubah"}`)
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, fmt.Sprintf("/api/v1/transaksi/%d", created.Data.ID), bytes.NewReader(patchBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSales)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s mutasi: got %d want 404/405", method, w.Code)
		}
	}
}
