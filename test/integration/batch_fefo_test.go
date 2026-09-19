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

func TestBatchTersediaFEFODanTerbaru(t *testing.T) {
	eng, tokAdmin, _, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SB04_%d", suffix)

	postBM := func(body map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokAdmin)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
	}

	// B-EARLY: exp lebih jauh, masuk lebih dulu
	postBM(map[string]any{
		"kode_barang":      kode,
		"buat_barang_baru": true,
		"nama_item":        "Item SB04",
		"brand":            "BrandSB04",
		"no_faktur":        fmt.Sprintf("SB04_F1_%d", suffix),
		"no_batch":         "B-EARLY",
		"exp":              "2027-06-30",
		"tanggal_masuk":    "2026-01-10",
		"qty":              15,
		"harga":            "20000.00",
		"markup_mt_type":   "percent",
		"markup_mt_amount": "15.00",
		"markup_gt_type":   "percent",
		"markup_gt_amount": "10.00",
	})
	// B-LATE: exp lebih dekat (masih valid) → FEFO ambil dulu; tanggal_masuk lebih baru
	postBM(map[string]any{
		"kode_barang":      kode,
		"no_faktur":        fmt.Sprintf("SB04_F2_%d", suffix),
		"no_batch":         "B-LATE",
		"exp":              "2026-11-30",
		"tanggal_masuk":    "2026-02-01",
		"qty":              20,
		"harga":            "20000.00",
		"markup_mt_type":   "percent",
		"markup_mt_amount": "15.00",
		"markup_gt_type":   "percent",
		"markup_gt_amount": "10.00",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/barang/"+kode+"/batch-tersedia?channel=Modern+Trade&qty=25",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("batch-tersedia: %d %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			MetodeAlokasi string `json:"metode_alokasi"`
			StokTersedia  int    `json:"stok_tersedia"`
			Batch         []struct {
				NoBatch     string `json:"no_batch"`
				QtyTersedia int    `json:"qty_tersedia"`
				SisaHari    int    `json:"sisa_hari"`
				HargaJual   string `json:"harga_jual"`
			} `json:"batch"`
			RencanaAlokasi []struct {
				NoBatch string `json:"no_batch"`
				Qty     int    `json:"qty"`
			} `json:"rencana_alokasi"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.MetodeAlokasi != "FEFO" {
		t.Fatalf("metode %s", resp.Data.MetodeAlokasi)
	}
	if resp.Data.StokTersedia != 35 {
		t.Fatalf("stok %d", resp.Data.StokTersedia)
	}
	if len(resp.Data.Batch) < 2 {
		t.Fatalf("batch count %d", len(resp.Data.Batch))
	}
	if resp.Data.Batch[0].NoBatch != "B-LATE" || resp.Data.Batch[1].NoBatch != "B-EARLY" {
		t.Fatalf("urut FEFO got %s lalu %s", resp.Data.Batch[0].NoBatch, resp.Data.Batch[1].NoBatch)
	}
	if resp.Data.Batch[0].SisaHari <= 0 {
		t.Fatalf("sisa_hari harus positif, got %d", resp.Data.Batch[0].SisaHari)
	}
	if resp.Data.Batch[0].HargaJual != "23000.00" {
		t.Fatalf("harga MT %s", resp.Data.Batch[0].HargaJual)
	}
	if len(resp.Data.RencanaAlokasi) != 2 {
		t.Fatalf("rencana len %d %+v", len(resp.Data.RencanaAlokasi), resp.Data.RencanaAlokasi)
	}
	if resp.Data.RencanaAlokasi[0].NoBatch != "B-LATE" || resp.Data.RencanaAlokasi[0].Qty != 20 {
		t.Fatalf("rencana[0] %+v", resp.Data.RencanaAlokasi[0])
	}
	if resp.Data.RencanaAlokasi[1].NoBatch != "B-EARLY" || resp.Data.RencanaAlokasi[1].Qty != 5 {
		t.Fatalf("rencana[1] %+v", resp.Data.RencanaAlokasi[1])
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/barang/"+kode+"/batch?limit=1", nil)
	req2.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("batch: %d %s", w2.Code, w2.Body.String())
	}
	var latest struct {
		Data []struct {
			NoBatch string `json:"no_batch"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &latest)
	if len(latest.Data) != 1 || latest.Data[0].NoBatch != "B-LATE" {
		t.Fatalf("batch terbaru %+v", latest.Data)
	}

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/barang/"+kode+"/batch-tersedia?qty=999", nil)
	req3.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusConflict {
		t.Fatalf("stok kurang want 409 got %d", w3.Code)
	}
}

func TestQuickAddAdalahPenerimaanBaru(t *testing.T) {
	// Per docs/05 + 11: quick-add ke qty batch dihapus; penambahan = dokumen penerimaan baru.
	eng, tokAdmin, _, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SB04Q_%d", suffix)

	post := func(body map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokAdmin)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
	}

	post(map[string]any{
		"kode_barang": kode, "buat_barang_baru": true, "nama_item": "Quick SB04", "brand": "Q",
		"no_faktur": fmt.Sprintf("SB04_QF_%d", suffix), "no_batch": "B-Q1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-09-05", "qty": 5, "harga": "10000.00",
	})
	post(map[string]any{
		"kode_barang": kode, "no_faktur": fmt.Sprintf("SB04_QF2_%d", suffix), "no_batch": "B-Q2",
		"exp": "2027-12-31", "tanggal_masuk": "2026-09-05", "qty": 3, "harga": "10000.00",
	})

	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/barang/"+kode+"/batch", nil)
	reqList.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wList, reqList)
	var list struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &list)
	if len(list.Data) != 2 {
		t.Fatalf("harus 2 baris penerimaan, got %d", len(list.Data))
	}
}
