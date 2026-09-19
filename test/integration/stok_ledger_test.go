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

func TestLedgerKonsistenDanPergerakan(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko SC13",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. SC13",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item SC13",
		"brand": "SC13", "no_faktur": fmt.Sprintf("SC13_F_%d", suffix), "no_batch": "B1",
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
		t.Fatalf("bm: %d %s", wBM.Code, wBM.Body.String())
	}

	trxBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
		"item": map[string]any{"kode_item": kodeA, "qty": 5, "harga": "10000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00",
		"tanggal_jatuh_tempo": "2026-10-04",
	})
	wTrx := httptest.NewRecorder()
	reqTrx := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(trxBody))
	reqTrx.Header.Set("Content-Type", "application/json")
	reqTrx.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wTrx, reqTrx)
	var trxOut struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wTrx.Body.Bytes(), &trxOut)

	wAp := httptest.NewRecorder()
	reqAp := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/approve", trxOut.Data.ID), bytes.NewReader([]byte(`{}`)))
	reqAp.Header.Set("Content-Type", "application/json")
	reqAp.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wAp, reqAp)
	if wAp.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", wAp.Code, wAp.Body.String())
	}

	var stok, ledger int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok)
	_ = db.Raw(`SELECT COALESCE(SUM(qty),0) FROM stock_movements WHERE barang_id = (SELECT id FROM barang WHERE kode_barang = ?)`, kodeA).Scan(&ledger)
	if stok != ledger || stok != 15 {
		t.Fatalf("stok=%d ledger=%d want 15", stok, ledger)
	}

	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/stok/pergerakan?kode_barang="+kodeA+"&per_page=50", nil)
	reqList.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("pergerakan: %d %s", wList.Code, wList.Body.String())
	}
	var listOut struct {
		Data []struct {
			MovementType string `json:"movement_type"`
			Qty          int    `json:"qty"`
		} `json:"data"`
		Ringkasan struct {
			TotalMasuk  int `json:"total_masuk"`
			TotalKeluar int `json:"total_keluar"`
			SaldoAkhir  int `json:"saldo_akhir"`
		} `json:"ringkasan"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &listOut)
	if len(listOut.Data) < 2 {
		t.Fatalf("pergerakan rows=%d", len(listOut.Data))
	}
	if listOut.Ringkasan.TotalMasuk != 20 || listOut.Ringkasan.TotalKeluar != 5 || listOut.Ringkasan.SaldoAkhir != 15 {
		t.Fatalf("ringkasan=%+v", listOut.Ringkasan)
	}

	// Penyesuaian -2
	penBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "qty": -2, "alasan": "Koreksi opname gudang",
	})
	wPen := httptest.NewRecorder()
	reqPen := httptest.NewRequest(http.MethodPost, "/api/v1/stok/penyesuaian", bytes.NewReader(penBody))
	reqPen.Header.Set("Content-Type", "application/json")
	reqPen.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPen, reqPen)
	if wPen.Code != http.StatusOK {
		t.Fatalf("penyesuaian: %d %s", wPen.Code, wPen.Body.String())
	}
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kodeA).Scan(&stok)
	_ = db.Raw(`SELECT COALESCE(SUM(qty),0) FROM stock_movements WHERE barang_id = (SELECT id FROM barang WHERE kode_barang = ?)`, kodeA).Scan(&ledger)
	if stok != 13 || ledger != 13 {
		t.Fatalf("setelah penyesuaian stok=%d ledger=%d", stok, ledger)
	}

	wRek := httptest.NewRecorder()
	reqRek := httptest.NewRequest(http.MethodGet, "/api/v1/stok/rekonsiliasi", nil)
	reqRek.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wRek, reqRek)
	if wRek.Code != http.StatusOK {
		t.Fatalf("rekonsiliasi: %d %s", wRek.Code, wRek.Body.String())
	}
	var rekOut struct {
		Data struct {
			Menyimpang int `json:"menyimpang"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wRek.Body.Bytes(), &rekOut)
	if rekOut.Data.Menyimpang != 0 {
		t.Fatalf("menyimpang=%d", rekOut.Data.Menyimpang)
	}

	// Sales tidak boleh penyesuaian
	wDenied := httptest.NewRecorder()
	reqDenied := httptest.NewRequest(http.MethodPost, "/api/v1/stok/penyesuaian", bytes.NewReader(penBody))
	reqDenied.Header.Set("Content-Type", "application/json")
	reqDenied.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wDenied, reqDenied)
	if wDenied.Code == http.StatusOK {
		t.Fatal("sales tidak boleh penyesuaian")
	}
}
