package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/domain"
)

func TestFakturJSONLayoutLockDanCetakUlang(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	emailAdmin := fmt.Sprintf("sd09_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)
	tokAdmin := loginToken(t, r, emailAdmin, "rahasia123")

	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, time.Now().UnixNano()%100000, 5)

	getFaktur := func(tok string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/faktur", trxID), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		return w
	}

	wSales := getFaktur(tokSales)
	if wSales.Code != http.StatusForbidden {
		t.Fatalf("sales: %d %s", wSales.Code, wSales.Body.String())
	}

	w1 := getFaktur(tokAdmin)
	if w1.Code != http.StatusOK {
		t.Fatalf("cetak1: %d %s", w1.Code, w1.Body.String())
	}
	var out1 struct {
		Data struct {
			Layout       string `json:"layout"`
			TotalHalaman int    `json:"total_halaman"`
			CetakUlang   bool   `json:"cetak_ulang"`
			Ringkasan    struct {
				Terbilang string `json:"terbilang"`
			} `json:"ringkasan"`
			FakturDicetakAt *string `json:"faktur_dicetak_at"`
			Pelanggan       struct {
				NamaPelanggan string `json:"nama_pelanggan"`
			} `json:"pelanggan"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &out1)
	if out1.Data.Layout != "half" || out1.Data.TotalHalaman != 1 {
		t.Fatalf("layout=%s halaman=%d", out1.Data.Layout, out1.Data.TotalHalaman)
	}
	if out1.Data.CetakUlang {
		t.Fatal("cetak pertama tidak boleh cetak_ulang")
	}
	if out1.Data.FakturDicetakAt == nil || *out1.Data.FakturDicetakAt == "" {
		t.Fatal("faktur_dicetak_at kosong")
	}
	if !strings.Contains(out1.Data.Ringkasan.Terbilang, "rupiah") {
		t.Fatalf("terbilang=%q", out1.Data.Ringkasan.Terbilang)
	}
	if out1.Data.Pelanggan.NamaPelanggan == "" {
		t.Fatal("snapshot pelanggan kosong")
	}

	var auditN int64
	if err := db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE aksi = 'faktur.cetak' AND entity_id = ?`, trxID).Scan(&auditN).Error; err != nil {
		t.Fatal(err)
	}
	if auditN < 1 {
		t.Fatalf("audit cetak pertama: %d", auditN)
	}

	w2 := getFaktur(tokAdmin)
	if w2.Code != http.StatusConflict {
		t.Fatalf("cetak2 admin: %d %s", w2.Code, w2.Body.String())
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &errBody)
	if errBody.Error.Code != "FAKTUR_TERKUNCI" {
		t.Fatalf("code=%s body=%s", errBody.Error.Code, w2.Body.String())
	}

	w3 := getFaktur(tokSA)
	if w3.Code != http.StatusOK {
		t.Fatalf("cetak ulang SA: %d %s", w3.Code, w3.Body.String())
	}
	var out3 struct {
		Data struct {
			CetakUlang      bool    `json:"cetak_ulang"`
			FakturDicetakAt *string `json:"faktur_dicetak_at"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &out3)
	if !out3.Data.CetakUlang {
		t.Fatal("harus cetak_ulang")
	}
	if out3.Data.FakturDicetakAt == nil || *out3.Data.FakturDicetakAt != *out1.Data.FakturDicetakAt {
		t.Fatalf("timestamp berubah: %v vs %v", out3.Data.FakturDicetakAt, out1.Data.FakturDicetakAt)
	}

	if err := db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE aksi = 'faktur.cetak' AND entity_id = ?`, trxID).Scan(&auditN).Error; err != nil {
		t.Fatal(err)
	}
	if auditN < 2 {
		t.Fatalf("audit cetak ulang: %d", auditN)
	}
}

func TestFakturPDFUnduhLockDanSalesDilarang(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	emailAdmin := fmt.Sprintf("sd11_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)
	tokAdmin := loginToken(t, r, emailAdmin, "rahasia123")

	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, time.Now().UnixNano()%100000, 5)

	getJSON := func(tok string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/faktur", trxID), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		return w
	}
	getPDF := func(tok string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/faktur/pdf", trxID), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		return w
	}

	wSales := getPDF(tokSales)
	if wSales.Code != http.StatusForbidden {
		t.Fatalf("sales pdf: %d %s", wSales.Code, wSales.Body.String())
	}

	wJSON := getJSON(tokSA)
	if wJSON.Code != http.StatusOK {
		t.Fatalf("json: %d %s", wJSON.Code, wJSON.Body.String())
	}
	var env struct {
		Data struct {
			NoTransaksi *string `json:"no_transaksi"`
			Ringkasan   struct {
				TotalAkhir string `json:"total_akhir"`
				Terbilang  string `json:"terbilang"`
			} `json:"ringkasan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wJSON.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}

	wAdmin := getPDF(tokAdmin)
	if wAdmin.Code != http.StatusConflict {
		t.Fatalf("admin setelah kunci: %d %s", wAdmin.Code, wAdmin.Body.String())
	}

	wPDF := getPDF(tokSA)
	if wPDF.Code != http.StatusOK {
		t.Fatalf("pdf SA: %d %s", wPDF.Code, wPDF.Body.String())
	}
	ct := wPDF.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/pdf") {
		t.Fatalf("content-type %s", ct)
	}
	cd := wPDF.Header().Get("Content-Disposition")
	if env.Data.NoTransaksi != nil && *env.Data.NoTransaksi != "" {
		if !strings.Contains(cd, *env.Data.NoTransaksi) {
			t.Fatalf("filename tanpa no_transaksi: %s", cd)
		}
	}
	if !strings.HasPrefix(wPDF.Body.String(), "%PDF") {
		t.Fatal("bukan PDF")
	}
	body := wPDF.Body.String()
	if !strings.Contains(body, env.Data.Ringkasan.TotalAkhir) {
		t.Fatalf("PDF tanpa total_akhir %s", env.Data.Ringkasan.TotalAkhir)
	}
	if !strings.Contains(body, env.Data.Ringkasan.Terbilang) {
		t.Fatalf("PDF tanpa terbilang %q", env.Data.Ringkasan.Terbilang)
	}
}

func TestFakturPendingDitolak(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, time.Now().UnixNano()%100000, 3)
	if err := db.Exec(`UPDATE transaksi_penjualan SET status_approval = 'pending', no_transaksi = NULL, faktur_dicetak_at = NULL WHERE id = ?`, trxID).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/faktur", trxID), nil)
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("pending: %d %s", w.Code, w.Body.String())
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &errBody)
	if errBody.Error.Code != "STATUS_TIDAK_VALID" {
		t.Fatalf("code=%s body=%s", errBody.Error.Code, w.Body.String())
	}
}
