package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestApprovalGateStokDanNomor(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC10",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC10 1",
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

	// Stok hanya 3 unit
	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item SC10",
		"brand": "SC10", "no_faktur": fmt.Sprintf("SC10_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 3, "harga": "10000.00",
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

	buat := func() uint64 {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
			"item": map[string]any{"kode_item": kodeA, "qty": 3, "harga": "10000.00"},
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

	id1 := buat()
	id2 := buat()
	id3 := buat()

	var stokPending int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stokPending)
	if stokPending != 3 {
		t.Fatalf("pending tidak boleh potong stok: %d", stokPending)
	}

	// Kecukupan
	wKet := httptest.NewRecorder()
	reqKet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/ketersediaan-stok", id1), nil)
	reqKet.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wKet, reqKet)
	if wKet.Code != http.StatusOK {
		t.Fatalf("ketersediaan: %d %s", wKet.Code, wKet.Body.String())
	}

	approve := func(id uint64) (int, string) {
		t.Helper()
		body := []byte(`{"approval_notes":"ok"}`)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", id), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	code1, body1 := approve(id1)
	if code1 != http.StatusOK {
		t.Fatalf("approve1: %d %s", code1, body1)
	}
	var appr struct {
		Data struct {
			NoTransaksi string `json:"no_transaksi"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body1), &appr)
	if len(appr.Data.NoTransaksi) < 6 {
		t.Fatalf("no_transaksi: %q", appr.Data.NoTransaksi)
	}

	var stokAfter int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stokAfter)
	if stokAfter != 0 {
		t.Fatalf("stok setelah approve1: %d", stokAfter)
	}

	code2, body2 := approve(id2)
	if code2 != http.StatusConflict {
		t.Fatalf("approve2 harus 409: %d %s", code2, body2)
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(body2), &errBody)
	if errBody.Error.Code != "STOK_TIDAK_CUKUP" {
		t.Fatalf("code: %s", errBody.Error.Code)
	}
	// tetap pending
	var status2 string
	_ = db.Raw(`SELECT status_approval FROM transaksi_penjualan WHERE id = ?`, id2).Scan(&status2)
	if status2 != "pending" {
		t.Fatalf("status2=%s", status2)
	}

	code3, _ := approve(id3)
	if code3 != http.StatusConflict {
		t.Fatalf("approve3 harus 409: %d", code3)
	}

	// Reject id2
	rejBody, _ := json.Marshal(map[string]any{"approval_notes": "Stok kosong, batalkan"})
	wRej := httptest.NewRecorder()
	reqRej := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/reject", id2), bytes.NewReader(rejBody))
	reqRej.Header.Set("Content-Type", "application/json")
	reqRej.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wRej, reqRej)
	if wRej.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", wRej.Code, wRej.Body.String())
	}
	var stokRej int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stokRej)
	if stokRej != 0 {
		t.Fatalf("reject tidak boleh ubah stok: %d", stokRej)
	}
	var no2 *string
	_ = db.Raw(`SELECT no_transaksi FROM transaksi_penjualan WHERE id = ?`, id2).Scan(&no2)
	if no2 != nil {
		t.Fatalf("rejected punya no_transaksi")
	}

	// Reject tanpa catatan
	wBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/reject", id3), bytes.NewReader([]byte(`{"approval_notes":""}`)))
	reqBad.Header.Set("Content-Type", "application/json")
	reqBad.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code == http.StatusOK {
		t.Fatal("reject kosong harus gagal")
	}
}

func TestApprovalKonkurensiTidakOversell(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC10C",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC10C",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item SC10C",
		"brand": "SC10", "no_faktur": fmt.Sprintf("SC10C_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 10, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)

	ids := make([]uint64, 0, 20)
	for i := 0; i < 20; i++ {
		body, _ := json.Marshal(map[string]any{
			"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
			"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
			"ppn_persen": "11.00", "nominal_dibayar": "0.00",
			"tanggal_jatuh_tempo": "2026-10-04",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSales)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("buat[%d]: %d %s", i, w.Code, w.Body.String())
		}
		var out struct {
			Data struct {
				ID uint64 `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		ids = append(ids, out.Data.ID)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	nos := map[string]bool{}
	for _, id := range ids {
		wg.Add(1)
		go func(trxID uint64) {
			defer wg.Done()
			body := []byte(`{}`)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", trxID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokSA)
			r.ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				var appr struct {
					Data struct {
						NoTransaksi string `json:"no_transaksi"`
					} `json:"data"`
				}
				_ = json.Unmarshal(w.Body.Bytes(), &appr)
				mu.Lock()
				okCount++
				if appr.Data.NoTransaksi != "" {
					if nos[appr.Data.NoTransaksi] {
						t.Errorf("duplikat no_transaksi %s", appr.Data.NoTransaksi)
					}
					nos[appr.Data.NoTransaksi] = true
				}
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()

	if okCount != 10 {
		t.Fatalf("lolos=%d want 10", okCount)
	}
	var stok int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok)
	if stok != 0 {
		t.Fatalf("stok akhir=%d", stok)
	}
}

func TestApprovalMultiSKUNoDeadlock(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)
	kodeB := fmt.Sprintf("SC02_B_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Multi",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. Multi",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	for _, pair := range []struct {
		kode string
		qty  int
	}{{kodeA, 50}, {kodeB, 50}} {
		bmBody, _ := json.Marshal(map[string]any{
			"kode_barang": pair.kode, "buat_barang_baru": true, "nama_item": "Item " + pair.kode,
			"brand": "SC10", "no_faktur": fmt.Sprintf("MSKU_%s_%d", pair.kode, suffix), "no_batch": "B1",
			"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": pair.qty, "harga": "10000.00",
			"markup_mt_type": "percent", "markup_mt_amount": "0.00",
			"markup_gt_type": "percent", "markup_gt_amount": "0.00",
		})
		wBM := httptest.NewRecorder()
		reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
		reqBM.Header.Set("Content-Type", "application/json")
		reqBM.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(wBM, reqBM)
		if wBM.Code != http.StatusCreated {
			t.Fatalf("bm %s: %d %s", pair.kode, wBM.Code, wBM.Body.String())
		}
	}

	buat := func(items []map[string]any) uint64 {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
			"items": items,
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

	// Urutan SKU berbeda — lock by barang.id ASC harus aman.
	idAB := buat([]map[string]any{
		{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
		{"kode_item": kodeB, "qty": 1, "harga": "10000.00"},
	})
	idBA := buat([]map[string]any{
		{"kode_item": kodeB, "qty": 1, "harga": "10000.00"},
		{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
	})

	var wg sync.WaitGroup
	errs := make(chan int, 2)
	for _, id := range []uint64{idAB, idBA} {
		wg.Add(1)
		go func(trxID uint64) {
			defer wg.Done()
			body := []byte(`{}`)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", trxID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokSA)
			r.ServeHTTP(w, req)
			errs <- w.Code
		}(id)
	}
	wg.Wait()
	close(errs)
	okN := 0
	for code := range errs {
		if code == http.StatusOK {
			okN++
		} else if code != http.StatusConflict {
			t.Fatalf("unexpected status %d", code)
		}
	}
	if okN != 2 {
		t.Fatalf("want 2 approved, got %d", okN)
	}
	var stokA, stokB int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stokA)
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeB).Scan(&stokB)
	if stokA != 48 || stokB != 48 {
		t.Fatalf("stok A=%d B=%d", stokA, stokB)
	}
}
