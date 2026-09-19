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

func seedApprovedTransaksi(t *testing.T, r http.Handler, tokSales, tokSA string, suffix int64, qty int) (kodePlg, kodeA string, trxID uint64) {
	return seedApprovedTransaksiPada(t, r, tokSales, tokSA, suffix, qty, "2026-09-04", "2026-10-04")
}

func seedApprovedTransaksiPada(t *testing.T, r http.Handler, tokSales, tokSA string, suffix int64, qty int, tanggal, jatuhTempo string) (kodePlg, kodeA string, trxID uint64) {
	t.Helper()
	kodePlg = fmt.Sprintf("SC02_%d", suffix)
	kodeA = fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Bayar",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. Bayar",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item Bayar",
		"brand": "PAY", "no_faktur": fmt.Sprintf("PAY_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": qty, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)

	trxBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": tanggal, "area": "Bandung",
		"item":       map[string]any{"kode_item": kodeA, "qty": qty, "harga": "10000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00",
		"tanggal_jatuh_tempo": jatuhTempo,
	})
	wTrx := httptest.NewRecorder()
	reqTrx := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(trxBody))
	reqTrx.Header.Set("Content-Type", "application/json")
	reqTrx.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wTrx, reqTrx)
	if wTrx.Code != http.StatusCreated {
		t.Fatalf("buat: %d %s", wTrx.Code, wTrx.Body.String())
	}
	var trxOut struct {
		Data struct {
			ID         uint64 `json:"id"`
			TotalAkhir string `json:"total_akhir"`
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
	return kodePlg, kodeA, trxOut.Data.ID
}

func TestCatatPembayaranNominalDanInvariant(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 10)

	var totalAkhir string
	_ = db.Raw(`SELECT total_akhir FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&totalAkhir)

	// Pending ditolak
	pendingBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": fmt.Sprintf("SC02_%d", suffix), "tanggal": "2026-09-04", "area": "Bandung",
		"item":       map[string]any{"kode_item": fmt.Sprintf("SC02_A_%d", suffix), "qty": 1, "harga": "10000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00", "tanggal_jatuh_tempo": "2026-10-04",
	})
	// stok sudah 0 — buat pending di SKU baru
	_ = pendingBody

	bayar := func(nominal, idem string) (int, string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"nominal_pembayaran": nominal,
			"metode_pembayaran":  "Transfer",
			"keterangan":         "cicilan",
			"tanggal_pembayaran": "2026-09-10",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSA)
		if idem != "" {
			req.Header.Set("Idempotency-Key", idem)
		}
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	code, body := bayar("50000.00", "pay-key-1")
	if code != http.StatusOK {
		t.Fatalf("bayar1: %d %s", code, body)
	}
	var out1 struct {
		Data struct {
			JumlahDibayar    string `json:"jumlah_dibayar"`
			SisaHutang       string `json:"sisa_hutang"`
			StatusPembayaran string `json:"status_pembayaran"`
			RiwayatID        uint64 `json:"riwayat_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &out1)
	if out1.Data.StatusPembayaran != "sebagian" || out1.Data.JumlahDibayar != "50000.00" {
		t.Fatalf("out1=%+v", out1.Data)
	}

	// Idempotent replay
	code2, body2 := bayar("50000.00", "pay-key-1")
	if code2 != http.StatusOK {
		t.Fatalf("replay: %d %s", code2, body2)
	}
	var out2 struct {
		Data struct {
			RiwayatID uint64 `json:"riwayat_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body2), &out2)
	if out2.Data.RiwayatID != out1.Data.RiwayatID {
		t.Fatalf("idempotency mengubah riwayat %d→%d", out1.Data.RiwayatID, out2.Data.RiwayatID)
	}
	var nRiwayat int64
	_ = db.Raw(`SELECT COUNT(*) FROM riwayat_pembayaran WHERE transaksi_penjualan_id = ?`, trxID).Scan(&nRiwayat)
	if nRiwayat != 1 {
		t.Fatalf("riwayat count=%d", nRiwayat)
	}

	// Kelebihan bayar
	code3, body3 := bayar("999999.00", "")
	if code3 != http.StatusConflict || !bytes.Contains([]byte(body3), []byte("KELEBIHAN_BAYAR")) {
		t.Fatalf("overpay: %d %s", code3, body3)
	}

	// Nominal 0
	code0, _ := bayar("0.00", "")
	if code0 != http.StatusUnprocessableEntity {
		t.Fatalf("nominal0: %d", code0)
	}

	// Pelunasan
	code4, body4 := bayar("61000.00", "pay-lunas") // total 10*10000*1.11 = 111000
	if code4 != http.StatusOK {
		t.Fatalf("lunas: %d %s", code4, body4)
	}
	var out4 struct {
		Data struct {
			StatusPembayaran string `json:"status_pembayaran"`
			SisaHutang       string `json:"sisa_hutang"`
			JumlahDibayar    string `json:"jumlah_dibayar"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body4), &out4)
	if out4.Data.StatusPembayaran != "lunas" || out4.Data.SisaHutang != "0.00" {
		t.Fatalf("lunas out=%+v total=%s", out4.Data, totalAkhir)
	}
	var tjt *string
	_ = db.Raw(`SELECT tanggal_jatuh_tempo FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&tjt)
	if tjt != nil {
		t.Fatal("lunas harus null-kan jatuh tempo")
	}

	// Riwayat GET
	wR := httptest.NewRecorder()
	reqR := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), nil)
	reqR.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wR, reqR)
	if wR.Code != http.StatusOK {
		t.Fatalf("riwayat: %d %s", wR.Code, wR.Body.String())
	}
	var hist struct {
		Data []struct {
			NominalPembayaran string `json:"nominal_pembayaran"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wR.Body.Bytes(), &hist)
	if len(hist.Data) != 2 {
		t.Fatalf("riwayat len=%d", len(hist.Data))
	}
}

func TestPembayaranPendingDitolak(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()
	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Pending",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. P",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item P",
		"brand": "PAY", "no_faktur": fmt.Sprintf("PEND_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 5, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)

	trxBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
		"item":       map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00", "tanggal_jatuh_tempo": "2026-10-04",
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

	body, _ := json.Marshal(map[string]any{"nominal_pembayaran": "1000.00"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxOut.Data.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("pending harus 409: %d %s", w.Code, w.Body.String())
	}
}

func TestPembayaranKonkurensiAkumulasi(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()
	suffix := time.Now().UnixNano() % 100000
	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 20)

	var wg sync.WaitGroup
	okN := 0
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"nominal_pembayaran": "10000.00"})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokSA)
			r.ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				mu.Lock()
				okN++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if okN != 5 {
		t.Fatalf("ok=%d want 5", okN)
	}
	var dibayar string
	_ = db.Raw(`SELECT jumlah_dibayar FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&dibayar)
	if dibayar != "50000.00" {
		t.Fatalf("jumlah_dibayar=%s", dibayar)
	}
}
