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

	"app/internal/domain"
	"github.com/shopspring/decimal"
)

func TestLaporanStokStatusNormalRendahHabis(t *testing.T) {
	r, _, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kNormal := fmt.Sprintf("SC02_N_%d", suffix)
	kRendah := fmt.Sprintf("SC02_R_%d", suffix)
	kHabis := fmt.Sprintf("SC02_H_%d", suffix)

	postBM := func(kode string, qty int) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_barang": kode, "buat_barang_baru": true, "nama_item": "Lap " + kode,
			"brand": "LAPSTOK", "no_faktur": "F_" + kode, "no_batch": "B1",
			"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": qty, "harga": "5000.00",
			"markup_mt_type": "percent", "markup_mt_amount": "0.00",
			"markup_gt_type": "percent", "markup_gt_amount": "0.00",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("bm %s: %d %s", kode, w.Code, w.Body.String())
		}
	}
	postBM(kNormal, 100)
	postBM(kRendah, 30)
	postBM(kHabis, 5)

	if err := db.Exec(`UPDATE barang SET min_stock = 10 WHERE kode_barang = ?`, kNormal).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE barang SET min_stock = 50 WHERE kode_barang = ?`, kRendah).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE barang SET stok_tersedia = 0, min_stock = 5 WHERE kode_barang = ?`, kHabis).Error; err != nil {
		t.Fatal(err)
	}

	getStatus := func(status string) []map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		url := fmt.Sprintf("/api/v1/laporan/stok?status_stok=%s&brand=LAPSTOK&per_page=50", status)
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("stok %s: %d %s", status, w.Code, w.Body.String())
		}
		var env struct {
			Data []map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		return env.Data
	}

	assertHas := func(rows []map[string]any, kode, wantStatus string) {
		t.Helper()
		for _, row := range rows {
			if row["kode_barang"] == kode {
				if row["status_stok"] != wantStatus {
					t.Fatalf("%s status=%v want %s", kode, row["status_stok"], wantStatus)
				}
				return
			}
		}
		t.Fatalf("%s tidak ada di filter %s: %v", kode, wantStatus, rows)
	}

	assertHas(getStatus("NORMAL"), kNormal, "NORMAL")
	assertHas(getStatus("RENDAH"), kRendah, "RENDAH")
	assertHas(getStatus("HABIS"), kHabis, "HABIS")

	wRing := httptest.NewRecorder()
	reqRing := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/stok?brand=LAPSTOK", nil)
	reqRing.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wRing, reqRing)
	var envRing struct {
		Ringkasan map[string]any `json:"ringkasan"`
	}
	_ = json.Unmarshal(wRing.Body.Bytes(), &envRing)
	if int(envRing.Ringkasan["normal"].(float64)) < 1 ||
		int(envRing.Ringkasan["rendah"].(float64)) < 1 ||
		int(envRing.Ringkasan["habis"].(float64)) < 1 {
		t.Fatalf("ringkasan: %v", envRing.Ringkasan)
	}
}

func TestLaporanBarangKeluarLabaPakaiTotalQtyKeluar(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_LP_%d", suffix)
	kodeA := fmt.Sprintf("SC02_LA_%d", suffix)
	kodePromo := fmt.Sprintf("SC02_PR_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Laba",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. Laba 1",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item Laba",
		"brand": "LABA", "no_faktur": fmt.Sprintf("LF_%d", suffix), "no_batch": "B1",
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

	promoBody, _ := json.Marshal(map[string]any{
		"kode_promo": kodePromo, "nama_promo": "Beli 10 Gratis 1",
		"tipe_promo": "buy_x_get_y", "buy_qty": 10, "get_qty": 1,
		"kode_barang": kodeA,
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

	trxBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
		"items": []map[string]any{
			{
				"kode_item": kodeA, "qty": 10, "harga": "10000.00",
				"kode_promos": []string{kodePromo},
			},
		},
		"ppn_persen": "11.00", "nominal_dibayar": "0.00",
		"tanggal_jatuh_tempo": "2026-10-04",
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

	var detail struct {
		Qty            int
		QtyPromo       int
		TotalQtyKeluar int
		HPPSnapshot    string
	}
	if err := db.Raw(`
SELECT qty, qty_promo, total_qty_keluar, CAST(hpp_snapshot AS CHAR) AS hpp_snapshot
FROM transaksi_detail WHERE transaksi_penjualan_id = ? LIMIT 1`, trxOut.Data.ID).Scan(&detail).Error; err != nil {
		t.Fatal(err)
	}
	if detail.Qty != 10 || detail.QtyPromo != 1 || detail.TotalQtyKeluar != 11 {
		t.Fatalf("detail qty: %+v", detail)
	}

	wLap := httptest.NewRecorder()
	reqLap := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/barang-keluar?kode_item="+kodeA, nil)
	reqLap.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wLap, reqLap)
	if wLap.Code != http.StatusOK {
		t.Fatalf("barang-keluar: %d %s", wLap.Code, wLap.Body.String())
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(wLap.Body.Bytes(), &env)
	if len(env.Data) == 0 {
		t.Fatal("kosong")
	}
	row := env.Data[0]
	if int(row["total_qty_keluar"].(float64)) != 11 {
		t.Fatalf("total_qty_keluar=%v", row["total_qty_keluar"])
	}
	hppSnap, _ := decimal.NewFromString(row["hpp_snapshot"].(string))
	hppTotal, _ := decimal.NewFromString(row["hpp_total"].(string))
	final, _ := decimal.NewFromString(row["total_final_baris"].(string))
	provit, _ := decimal.NewFromString(row["provit"].(string))

	benar := domain.HitungProvit(domain.InputProvit{
		TotalFinalBaris: final, HPPSnapshot: hppSnap, TotalQtyKeluar: 11,
	})
	salah := domain.HitungProvit(domain.InputProvit{
		TotalFinalBaris: final, HPPSnapshot: hppSnap, TotalQtyKeluar: 10,
	})
	if !hppTotal.Equal(benar.HPPTotal) || !provit.Equal(benar.Provit) {
		t.Fatalf("laba API tidak cocok HitungProvit(total_qty): hpp=%s provit=%s want %s / %s",
			hppTotal, provit, benar.HPPTotal, benar.Provit)
	}
	if benar.Provit.Equal(salah.Provit) {
		t.Fatal("provit qty vs total_qty_keluar sama — test tidak membedakan")
	}

	// Afiliasi: tanpa alamat / HPP / laba
	emailAf := fmt.Sprintf("slap_af_%d@pkb.test", suffix)
	createUser(t, emailAf, "rahasia123", domain.RoleAfiliasi, true)
	tokAf := loginToken(t, r, emailAf, "rahasia123")

	wAf := httptest.NewRecorder()
	reqAf := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/barang-keluar?kode_item="+kodeA, nil)
	reqAf.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wAf, reqAf)
	if wAf.Code != http.StatusOK {
		t.Fatalf("afiliasi list: %d %s", wAf.Code, wAf.Body.String())
	}
	var envAf struct {
		Data      []map[string]any `json:"data"`
		Ringkasan map[string]any   `json:"ringkasan"`
	}
	_ = json.Unmarshal(wAf.Body.Bytes(), &envAf)
	if len(envAf.Data) == 0 {
		t.Fatal("afiliasi kosong")
	}
	afRow := envAf.Data[0]
	for _, k := range []string{"alamat", "hpp_snapshot", "hpp_total", "provit", "margin_persen"} {
		if _, ok := afRow[k]; ok {
			t.Fatalf("afiliasi tidak boleh punya %s: %v", k, afRow)
		}
	}
	if _, ok := envAf.Ringkasan["total_hpp"]; ok {
		t.Fatalf("afiliasi ringkasan sensitif: %v", envAf.Ringkasan)
	}

	wExp := httptest.NewRecorder()
	reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/barang-keluar/export?kode_item="+kodeA, nil)
	reqExp.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK {
		t.Fatalf("export afiliasi: %d %s", wExp.Code, wExp.Body.String())
	}
	csv := wExp.Body.String()
	for _, forbidden := range []string{"hpp_snapshot", "hpp_total", "provit", "margin_persen", "alamat"} {
		if strings.Contains(csv, forbidden) {
			t.Fatalf("export afiliasi mengandung %s", forbidden)
		}
	}

	wLaba := httptest.NewRecorder()
	reqLaba := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/laba?kode_item="+kodeA, nil)
	reqLaba.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wLaba, reqLaba)
	if wLaba.Code != http.StatusForbidden {
		t.Fatalf("afiliasi /laba harus 403, got %d %s", wLaba.Code, wLaba.Body.String())
	}
}

func TestChannelAnalyticsPerChannelTerritoryProduk(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, kodeA, _ := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 8)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/channel-analytics?date_from=2026-01-01&date_to=2026-12-31", nil)
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("analytics: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			PerChannel     []map[string]any `json:"per_channel"`
			PerTerritory   []map[string]any `json:"per_territory"`
			ProdukTerlaris []map[string]any `json:"produk_terlaris"`
			TrenHarian     []map[string]any `json:"tren_harian"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if len(env.Data.PerChannel) == 0 {
		t.Fatal("per_channel kosong")
	}
	foundCh := false
	for _, c := range env.Data.PerChannel {
		if c["channel_outlet"] == "General Trade" && int(c["jumlah_transaksi"].(float64)) >= 1 {
			foundCh = true
			break
		}
	}
	if !foundCh {
		t.Fatalf("channel GT: %v", env.Data.PerChannel)
	}
	if len(env.Data.PerTerritory) == 0 {
		t.Fatal("per_territory kosong")
	}
	foundSKU := false
	for _, p := range env.Data.ProdukTerlaris {
		if p["kode_item"] == kodeA && int(p["total_qty"].(float64)) >= 8 {
			foundSKU = true
			break
		}
	}
	if !foundSKU {
		t.Fatalf("produk terlaris: %v", env.Data.ProdukTerlaris)
	}
	if len(env.Data.TrenHarian) == 0 {
		t.Fatal("tren_harian kosong")
	}

	emailAf := fmt.Sprintf("sca_af_%d@pkb.test", suffix)
	createUser(t, emailAf, "rahasia123", domain.RoleAfiliasi, true)
	tokAf := loginToken(t, r, emailAf, "rahasia123")
	wAf := httptest.NewRecorder()
	reqAf := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/channel-analytics", nil)
	reqAf.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wAf, reqAf)
	if wAf.Code != http.StatusForbidden {
		t.Fatalf("afiliasi analytics harus 403, got %d", wAf.Code)
	}

	wExp := httptest.NewRecorder()
	reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/channel-analytics/export", nil)
	reqExp.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK {
		t.Fatalf("export: %d %s", wExp.Code, wExp.Body.String())
	}
	if !strings.Contains(wExp.Body.String(), "per_channel") {
		t.Fatalf("csv: %s", wExp.Body.String()[:min(200, len(wExp.Body.String()))])
	}
}

func TestLaporanPenjualanFilterDanKonsistenEkspor(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 6)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penjualan?channel_outlet=General+Trade&date_from=2026-01-01&date_to=2026-12-31", nil)
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("penjualan: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data []struct {
			ID             uint64 `json:"id"`
			StatusApproval string `json:"status_approval"`
			TotalAkhir     string `json:"total_akhir"`
			ChannelOutlet  string `json:"channel_outlet"`
		} `json:"data"`
		Ringkasan struct {
			JumlahTransaksi int    `json:"jumlah_transaksi"`
			TotalPenjualan  string `json:"total_penjualan"`
		} `json:"ringkasan"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	found := false
	for _, row := range env.Data {
		if row.ID == trxID {
			found = true
			if row.StatusApproval != "approved" {
				t.Fatalf("status=%s", row.StatusApproval)
			}
			if row.ChannelOutlet != "General Trade" {
				t.Fatalf("channel=%s", row.ChannelOutlet)
			}
		}
	}
	if !found {
		t.Fatalf("trx %d tidak di laporan: %v", trxID, env.Data)
	}
	if env.Ringkasan.JumlahTransaksi < 1 {
		t.Fatalf("ringkasan: %+v", env.Ringkasan)
	}

	wExp := httptest.NewRecorder()
	reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penjualan/export?channel_outlet=General+Trade&date_from=2026-01-01&date_to=2026-12-31", nil)
	reqExp.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK {
		t.Fatalf("export: %d %s", wExp.Code, wExp.Body.String())
	}
	csv := wExp.Body.String()
	ok := false
	for _, line := range strings.Split(csv, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("%d,", trxID)) {
			ok = true
			if strings.Contains(line, ",pending,") || strings.Contains(line, ",rejected,") {
				t.Fatalf("non-approved di export: %s", line)
			}
			break
		}
	}
	if !ok {
		t.Fatalf("trx %d tidak di CSV export", trxID)
	}
}
