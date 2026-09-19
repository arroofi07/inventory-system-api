package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCekStokSoftDariLedger(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SC02_CS_%d", suffix)

	bodyBM, _ := json.Marshal(map[string]any{
		"kode_barang": kode, "buat_barang_baru": true, "nama_item": "Item Cek Stok",
		"brand": "SC05", "no_faktur": fmt.Sprintf("SC05_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 5, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bodyBM))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)
	if wBM.Code != http.StatusCreated {
		t.Fatalf("bm: %d %s", wBM.Code, wBM.Body.String())
	}

	cekOK, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{"kode_item": kode, "qty": 3}},
	})
	wOK := httptest.NewRecorder()
	reqOK := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi/cek-stok", bytes.NewReader(cekOK))
	reqOK.Header.Set("Content-Type", "application/json")
	reqOK.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wOK, reqOK)
	if wOK.Code != http.StatusOK {
		t.Fatalf("cek cukup: %d %s", wOK.Code, wOK.Body.String())
	}
	var respOK struct {
		Data struct {
			SemuaStokCukup bool `json:"semua_stok_cukup"`
			Items          []struct {
				StokTersedia int  `json:"stok_tersedia"`
				StokCukup    bool `json:"stok_cukup"`
			} `json:"items"`
			Catatan string `json:"catatan"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wOK.Body.Bytes(), &respOK)
	if !respOK.Data.SemuaStokCukup || len(respOK.Data.Items) != 1 || respOK.Data.Items[0].StokTersedia != 5 {
		t.Fatalf("cek cukup: %+v", respOK.Data)
	}
	if respOK.Data.Catatan == "" {
		t.Fatal("catatan soft-check wajib ada")
	}

	cekKurang, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{"kode_item": kode, "qty": 10}},
	})
	wKurang := httptest.NewRecorder()
	reqKurang := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi/cek-stok", bytes.NewReader(cekKurang))
	reqKurang.Header.Set("Content-Type", "application/json")
	reqKurang.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wKurang, reqKurang)
	if wKurang.Code != http.StatusOK {
		t.Fatalf("cek kurang: %d %s", wKurang.Code, wKurang.Body.String())
	}
	var respKurang struct {
		Data struct {
			SemuaStokCukup bool `json:"semua_stok_cukup"`
			Items          []struct {
				StokCukup bool `json:"stok_cukup"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wKurang.Body.Bytes(), &respKurang)
	if respKurang.Data.SemuaStokCukup || respKurang.Data.Items[0].StokCukup {
		t.Fatal("harapkan stok tidak cukup tanpa error 409 (soft-check)")
	}
}
