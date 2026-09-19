package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app/internal/domain"
	"github.com/shopspring/decimal"
)

func TestDashboardPerRole(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	_, _, _ = seedApprovedTransaksi(t, r, tokSales, tokSA, time.Now().UnixNano()%100000, 4)

	getDash := func(tok string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("dashboard: %d %s", w.Code, w.Body.String())
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		return env.Data
	}

	sa := getDash(tokSA)
	if sa["role"] != "super_admin" {
		t.Fatalf("role sa=%v", sa["role"])
	}
	kartuSA, _ := sa["kartu"].(map[string]any)
	if kartuSA["penjualan_bulan_ini"] == nil || kartuSA["total_piutang"] == nil {
		t.Fatalf("sa kartu=%v", kartuSA)
	}
	if sa["notifikasi_piutang"] == nil {
		t.Fatal("sa tanpa notifikasi_piutang")
	}

	sales := getDash(tokSales)
	if sales["role"] != "sales" {
		t.Fatalf("role sales=%v", sales["role"])
	}
	kartuSales, _ := sales["kartu"].(map[string]any)
	if kartuSales["penjualan_bulan_ini"] == nil {
		t.Fatal("sales harus punya penjualan (filter milik)")
	}
	if _, ok := kartuSales["total_piutang"]; ok {
		t.Fatal("sales tidak boleh piutang")
	}
	if sales["notifikasi_piutang"] != nil {
		t.Fatal("sales tidak boleh notifikasi piutang")
	}

	emailAfil := "sdash_af_" + time.Now().Format("150405") + "@pkb.test"
	createUser(t, emailAfil, "rahasia123", domain.RoleAfiliasi, true)
	tokAf := loginToken(t, r, emailAfil, "rahasia123")
	af := getDash(tokAf)
	kartuAf, _ := af["kartu"].(map[string]any)
	if _, ok := kartuAf["penjualan_bulan_ini"]; ok {
		t.Fatal("afiliasi tanpa penjualan nilai")
	}
	if _, ok := kartuAf["total_piutang"]; ok {
		t.Fatal("afiliasi tanpa piutang")
	}
	if kartuAf["sku_stok_rendah"] == nil {
		t.Fatalf("afiliasi harus stok: %v", kartuAf)
	}
}

func TestDashboardAngkaSamaLaporanPeriode(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	now := time.Now().UTC()
	tanggal := now.Format("2006-01-02")
	jatuh := now.AddDate(0, 0, 30).Format("2006-01-02")
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	to := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	_, _, _ = seedApprovedTransaksiPada(t, r, tokSales, tokSA, time.Now().UnixNano()%100000, 4, tanggal, jatuh)

	wDash := httptest.NewRecorder()
	reqDash := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	reqDash.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wDash, reqDash)
	if wDash.Code != http.StatusOK {
		t.Fatalf("dashboard: %d %s", wDash.Code, wDash.Body.String())
	}
	var dash struct {
		Data struct {
			Kartu struct {
				PenjualanBulanIni       *string `json:"penjualan_bulan_ini"`
				JumlahTransaksiBulanIni *int    `json:"jumlah_transaksi_bulan_ini"`
				TotalPiutang            *string `json:"total_piutang"`
				PiutangOverdue          *string `json:"piutang_overdue"`
				SKUStokRendah           *int    `json:"sku_stok_rendah"`
				SKUStokHabis            *int    `json:"sku_stok_habis"`
			} `json:"kartu"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wDash.Body.Bytes(), &dash); err != nil {
		t.Fatal(err)
	}
	if dash.Data.Kartu.PenjualanBulanIni == nil || dash.Data.Kartu.JumlahTransaksiBulanIni == nil {
		t.Fatal("dashboard tanpa penjualan bulan ini")
	}

	lapURL := fmt.Sprintf("/api/v1/laporan/penjualan?status_approval=approved&date_from=%s&date_to=%s&per_page=1", from, to)
	wLap := httptest.NewRecorder()
	reqLap := httptest.NewRequest(http.MethodGet, lapURL, nil)
	reqLap.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wLap, reqLap)
	if wLap.Code != http.StatusOK {
		t.Fatalf("laporan penjualan: %d %s", wLap.Code, wLap.Body.String())
	}
	var lap struct {
		Ringkasan struct {
			JumlahTransaksi int    `json:"jumlah_transaksi"`
			TotalPenjualan  string `json:"total_penjualan"`
		} `json:"ringkasan"`
	}
	if err := json.Unmarshal(wLap.Body.Bytes(), &lap); err != nil {
		t.Fatal(err)
	}

	assertUangSama(t, "penjualan_bulan_ini", *dash.Data.Kartu.PenjualanBulanIni, lap.Ringkasan.TotalPenjualan)
	if *dash.Data.Kartu.JumlahTransaksiBulanIni != lap.Ringkasan.JumlahTransaksi {
		t.Fatalf("jumlah trx dashboard=%d laporan=%d", *dash.Data.Kartu.JumlahTransaksiBulanIni, lap.Ringkasan.JumlahTransaksi)
	}
	if lap.Ringkasan.JumlahTransaksi < 1 {
		t.Fatal("seeder bulan ini tidak masuk laporan")
	}

	wPiu := httptest.NewRecorder()
	reqPiu := httptest.NewRequest(http.MethodGet, "/api/v1/piutang", nil)
	reqPiu.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPiu, reqPiu)
	if wPiu.Code != http.StatusOK {
		t.Fatalf("piutang: %d %s", wPiu.Code, wPiu.Body.String())
	}
	var piu struct {
		Ringkasan struct {
			TotalPiutang   string `json:"total_piutang"`
			PiutangOverdue string `json:"piutang_overdue"`
		} `json:"ringkasan"`
	}
	if err := json.Unmarshal(wPiu.Body.Bytes(), &piu); err != nil {
		t.Fatal(err)
	}
	if dash.Data.Kartu.TotalPiutang == nil || dash.Data.Kartu.PiutangOverdue == nil {
		t.Fatal("dashboard tanpa piutang")
	}
	assertUangSama(t, "total_piutang", *dash.Data.Kartu.TotalPiutang, piu.Ringkasan.TotalPiutang)
	assertUangSama(t, "piutang_overdue", *dash.Data.Kartu.PiutangOverdue, piu.Ringkasan.PiutangOverdue)

	wStok := httptest.NewRecorder()
	reqStok := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/stok?per_page=1", nil)
	reqStok.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wStok, reqStok)
	if wStok.Code != http.StatusOK {
		t.Fatalf("laporan stok: %d %s", wStok.Code, wStok.Body.String())
	}
	var stok struct {
		Ringkasan struct {
			Rendah int `json:"rendah"`
			Habis  int `json:"habis"`
		} `json:"ringkasan"`
	}
	if err := json.Unmarshal(wStok.Body.Bytes(), &stok); err != nil {
		t.Fatal(err)
	}
	if dash.Data.Kartu.SKUStokRendah == nil || dash.Data.Kartu.SKUStokHabis == nil {
		t.Fatal("dashboard tanpa kartu stok")
	}
	if *dash.Data.Kartu.SKUStokRendah != stok.Ringkasan.Rendah {
		t.Fatalf("sku_stok_rendah dashboard=%d laporan=%d", *dash.Data.Kartu.SKUStokRendah, stok.Ringkasan.Rendah)
	}
	if *dash.Data.Kartu.SKUStokHabis != stok.Ringkasan.Habis {
		t.Fatalf("sku_stok_habis dashboard=%d laporan=%d", *dash.Data.Kartu.SKUStokHabis, stok.Ringkasan.Habis)
	}
}

func assertUangSama(t *testing.T, label, a, b string) {
	t.Helper()
	da, errA := decimal.NewFromString(a)
	db, errB := decimal.NewFromString(b)
	if errA != nil || errB != nil {
		t.Fatalf("%s parse %q / %q", label, a, b)
	}
	if !da.Equal(db) {
		t.Fatalf("%s dashboard=%s laporan=%s", label, a, b)
	}
}
