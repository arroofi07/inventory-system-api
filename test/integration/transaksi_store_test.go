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

func TestStorePendingSamaDenganPratinjau(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)
	kodeB := fmt.Sprintf("SC02_B_%d", suffix)
	kodePromo := fmt.Sprintf("SC02_P_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC03",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC03 1",
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
			"brand": "SC03", "no_faktur": faktur, "no_batch": "B1",
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
	postBM(kodeA, fmt.Sprintf("SC02_FA_%d", suffix), "20000.00")
	postBM(kodeB, fmt.Sprintf("SC02_FB_%d", suffix), "30000.00")

	promoBody, _ := json.Marshal(map[string]any{
		"kode_promo": kodePromo, "nama_promo": "Beli 10 Gratis 1",
		"tipe_promo": "buy_x_get_y", "buy_qty": 10, "get_qty": 1, "kode_barang": kodeA,
		"tanggal_mulai": "2026-01-01", "tanggal_berakhir": "2026-12-31",
	})
	wPromo := httptest.NewRecorder()
	reqPromo := httptest.NewRequest(http.MethodPost, "/api/v1/promo", bytes.NewReader(promoBody))
	reqPromo.Header.Set("Content-Type", "application/json")
	reqPromo.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPromo, reqPromo)
	if wPromo.Code != http.StatusCreated {
		t.Fatalf("promo: %d %s", wPromo.Code, wPromo.Body.String())
	}

	payload := map[string]any{
		"kode_pelanggan": kodePlg,
		"tanggal":        "2026-09-04",
		"area":           "Bandung Timur",
		"items": []map[string]any{
			{"kode_item": kodeA, "qty": 10, "harga": "20000.00", "disc1_persen": "10.00", "kode_promos": []string{kodePromo}},
			{"kode_item": kodeB, "qty": 5, "harga": "30000.00"},
		},
		"disc1_persen":           "5.00",
		"ppn_persen":             "11.00",
		"nominal_dibayar":        "0.00",
		"tanggal_jatuh_tempo":    "2026-10-04",
		"keterangan_pembayaran":  "Tempo 30 hari",
	}

	// Pratinjau
	pratinjauBody := map[string]any{
		"kode_pelanggan": payload["kode_pelanggan"],
		"tanggal":        payload["tanggal"],
		"items":          payload["items"],
		"disc1_persen":   payload["disc1_persen"],
		"ppn_persen":     payload["ppn_persen"],
	}
	bPratinjau, _ := json.Marshal(pratinjauBody)
	wP := httptest.NewRecorder()
	reqP := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi/pratinjau", bytes.NewReader(bPratinjau))
	reqP.Header.Set("Content-Type", "application/json")
	reqP.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wP, reqP)
	if wP.Code != http.StatusOK {
		t.Fatalf("pratinjau: %d %s", wP.Code, wP.Body.String())
	}
	var prat struct {
		Data struct {
			Ringkasan struct {
				TotalAkhir string `json:"total_akhir"`
			} `json:"ringkasan"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wP.Body.Bytes(), &prat)

	var stockBefore int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockBefore)
	var mvBefore int64
	_ = db.Table("stock_movements").Where("barang_id IN (SELECT id FROM barang WHERE kode_barang IN (?,?))", kodeA, kodeB).Count(&mvBefore)

	bStore, _ := json.Marshal(payload)
	wS := httptest.NewRecorder()
	reqS := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(bStore))
	reqS.Header.Set("Content-Type", "application/json")
	reqS.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wS, reqS)
	if wS.Code != http.StatusCreated {
		t.Fatalf("store: %d %s", wS.Code, wS.Body.String())
	}

	var stored struct {
		Data struct {
			ID               uint64  `json:"id"`
			NoTransaksi      *string `json:"no_transaksi"`
			StatusApproval   string  `json:"status_approval"`
			NamaPelanggan    string  `json:"nama_pelanggan"`
			Alamat           string  `json:"alamat"`
			ChannelOutlet    string  `json:"channel_outlet"`
			TotalAkhir       string  `json:"total_akhir"`
			StatusPembayaran string  `json:"status_pembayaran"`
			SisaHutang       string  `json:"sisa_hutang"`
			Items            []struct {
				HPPSnapshot     string `json:"hpp_snapshot"`
				PromoDiterapkan []struct {
					KodePromo string `json:"kode_promo"`
				} `json:"promo_diterapkan"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wS.Body.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Data.StatusApproval != "pending" {
		t.Fatalf("status=%s", stored.Data.StatusApproval)
	}
	if stored.Data.NoTransaksi != nil {
		t.Fatalf("no_transaksi harus null, got %v", *stored.Data.NoTransaksi)
	}
	if stored.Data.TotalAkhir != prat.Data.Ringkasan.TotalAkhir || stored.Data.TotalAkhir != "347985.00" {
		t.Fatalf("total store=%s pratinjau=%s", stored.Data.TotalAkhir, prat.Data.Ringkasan.TotalAkhir)
	}
	if stored.Data.NamaPelanggan != "Toko SC03" || stored.Data.Alamat == "" || stored.Data.ChannelOutlet != "General Trade" {
		t.Fatalf("snapshot pelanggan: %+v", stored.Data)
	}
	if stored.Data.StatusPembayaran != "hutang" || stored.Data.SisaHutang != "347985.00" {
		t.Fatalf("pembayaran: %+v", stored.Data)
	}
	if len(stored.Data.Items) != 2 || stored.Data.Items[0].HPPSnapshot == "" {
		t.Fatalf("items/hpp: %+v", stored.Data.Items)
	}
	if len(stored.Data.Items[0].PromoDiterapkan) != 1 || stored.Data.Items[0].PromoDiterapkan[0].KodePromo != kodePromo {
		t.Fatalf("promo snapshot: %+v", stored.Data.Items[0].PromoDiterapkan)
	}

	var stockAfter int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockAfter)
	if stockAfter != stockBefore {
		t.Fatalf("stok berubah %d→%d", stockBefore, stockAfter)
	}
	var mvAfter int64
	_ = db.Table("stock_movements").Where("barang_id IN (SELECT id FROM barang WHERE kode_barang IN (?,?))", kodeA, kodeB).Count(&mvAfter)
	if mvAfter != mvBefore {
		t.Fatalf("stock_movements berubah %d→%d", mvBefore, mvAfter)
	}

	// Detail milik sales OK
	wD := httptest.NewRecorder()
	reqD := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d", stored.Data.ID), nil)
	reqD.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wD, reqD)
	if wD.Code != http.StatusOK {
		t.Fatalf("detail milik: %d %s", wD.Code, wD.Body.String())
	}

	// Sales lain → 404
	emailOther := fmt.Sprintf("sa06_sc03_other_%d@pkb.test", suffix)
	createUser(t, emailOther, "rahasia123", domain.RoleSales, true)
	tokOther := loginToken(t, r, emailOther, "rahasia123")
	wO := httptest.NewRecorder()
	reqO := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d", stored.Data.ID), nil)
	reqO.Header.Set("Authorization", "Bearer "+tokOther)
	r.ServeHTTP(wO, reqO)
	if wO.Code != http.StatusNotFound {
		t.Fatalf("sales lain want 404 got %d %s", wO.Code, wO.Body.String())
	}

	// Harga salah → 409
	badHarga := map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "X",
		"items": []map[string]any{
			{"kode_item": kodeA, "qty": 1, "harga": "1.00"},
		},
		"nominal_dibayar": "0.00", "tanggal_jatuh_tempo": "2026-10-04",
	}
	bBad, _ := json.Marshal(badHarga)
	wH := httptest.NewRecorder()
	reqH := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(bBad))
	reqH.Header.Set("Content-Type", "application/json")
	reqH.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wH, reqH)
	if wH.Code != http.StatusConflict {
		t.Fatalf("harga want 409 got %d %s", wH.Code, wH.Body.String())
	}
}
