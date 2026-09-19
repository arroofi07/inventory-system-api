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

func TestHargaMassalDanRiwayat(t *testing.T) {
	eng, tokAdmin, tokSA, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SB05_%d", suffix)

	postBM := func(batch, faktur string) uint64 {
		t.Helper()
		body := map[string]any{
			"kode_barang": kode, "no_faktur": faktur, "no_batch": batch,
			"exp": "2027-12-31", "tanggal_masuk": "2026-09-01", "qty": 5, "harga": "20000.00",
			"markup_mt_type": "percent", "markup_mt_amount": "10.00",
			"markup_gt_type": "percent", "markup_gt_amount": "5.00",
		}
		if batch == "B1" {
			body["buat_barang_baru"] = true
			body["nama_item"] = "Item SB05"
			body["brand"] = "SB05"
		}
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokAdmin)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				ID       uint64 `json:"id"`
				BarangID uint64 `json:"barang_id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.Data.ID
	}

	id1 := postBM("B1", fmt.Sprintf("SB05_F1_%d", suffix))
	id2 := postBM("B2", fmt.Sprintf("SB05_F2_%d", suffix))

	// Ambil barang_id
	wDet := httptest.NewRecorder()
	reqDet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/barang-masuk/%d", id1), nil)
	reqDet.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wDet, reqDet)
	var det struct {
		Data struct {
			BarangID uint64 `json:"barang_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wDet.Body.Bytes(), &det)
	barangID := det.Data.BarangID

	// Admin tidak boleh harga massal
	body, _ := json.Marshal(map[string]any{
		"batch_ids":        []uint64{id1, id2},
		"markup_mt_amount": "20.00",
		"keterangan":       "naik markup MT",
	})
	wAdmin := httptest.NewRecorder()
	reqAdmin := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/barang/%d/harga-massal", barangID), bytes.NewReader(body))
	reqAdmin.Header.Set("Content-Type", "application/json")
	reqAdmin.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusForbidden {
		t.Fatalf("admin want 403 got %d", wAdmin.Code)
	}

	// SA boleh
	wSA := httptest.NewRecorder()
	reqSA := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/barang/%d/harga-massal", barangID), bytes.NewReader(body))
	reqSA.Header.Set("Content-Type", "application/json")
	reqSA.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wSA, reqSA)
	if wSA.Code != 200 {
		t.Fatalf("harga-massal: %d %s", wSA.Code, wSA.Body.String())
	}
	var hasil struct {
		Data struct {
			BulkOperationID       string `json:"bulk_operation_id"`
			JumlahBatchDiperbarui int    `json:"jumlah_batch_diperbarui"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wSA.Body.Bytes(), &hasil)
	if hasil.Data.BulkOperationID == "" || hasil.Data.JumlahBatchDiperbarui != 2 {
		t.Fatalf("hasil %+v", hasil.Data)
	}

	// Harga MT harus 24000 (20000 * 1.2)
	wBatch := httptest.NewRecorder()
	reqBatch := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/barang-masuk/%d", id1), nil)
	reqBatch.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBatch, reqBatch)
	var bm struct {
		Data struct {
			HargaMT string `json:"harga_mt"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wBatch.Body.Bytes(), &bm)
	if bm.Data.HargaMT != "24000.00" {
		t.Fatalf("harga_mt %s", bm.Data.HargaMT)
	}

	// Riwayat
	wHist := httptest.NewRecorder()
	reqHist := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/barang/%d/riwayat-harga", barangID), nil)
	reqHist.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wHist, reqHist)
	if wHist.Code != 200 {
		t.Fatalf("riwayat: %d %s", wHist.Code, wHist.Body.String())
	}
	var hist struct {
		Data []struct {
			BulkOperationID string `json:"bulk_operation_id"`
			NewHargaMT      string `json:"new_harga_mt"`
			NoBatch         string `json:"no_batch"`
		} `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(wHist.Body.Bytes(), &hist)
	if hist.Meta.Total < 2 {
		t.Fatalf("riwayat total %d", hist.Meta.Total)
	}
	for _, row := range hist.Data {
		if row.BulkOperationID != hasil.Data.BulkOperationID {
			t.Fatalf("bulk id mismatch %s vs %s", row.BulkOperationID, hasil.Data.BulkOperationID)
		}
		if row.NewHargaMT != "24000.00" {
			t.Fatalf("new mt %s", row.NewHargaMT)
		}
	}

	// Idempotent: nilai sama → dilewati, tanpa log baru
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/barang/%d/harga-massal", barangID), bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w2, req2)
	var hasil2 struct {
		Data struct {
			JumlahBatchDiperbarui int `json:"jumlah_batch_diperbarui"`
			JumlahDilewati        int `json:"jumlah_dilewati"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &hasil2)
	if hasil2.Data.JumlahBatchDiperbarui != 0 || hasil2.Data.JumlahDilewati != 2 {
		t.Fatalf("skip %+v", hasil2.Data)
	}
}
