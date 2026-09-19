package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app/internal/config"
	"app/internal/domain"
	"app/internal/handler"
	"app/internal/repository"
	"app/internal/service"
)

func setupPelangganRouter(t *testing.T) (*ginEngine, string, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)

	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}

	prefix := "SB06_%"
	cleanup := func() {
		_ = db.Exec(`DELETE FROM transaksi_penjualan WHERE kode_pelanggan LIKE ?`, prefix)
		_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'pelanggan'`)
		_ = db.Exec(`DELETE FROM pelanggan WHERE kode_pelanggan LIKE ?`, prefix)
		cleanupAuth()
	}
	_ = db.Exec(`DELETE FROM transaksi_penjualan WHERE kode_pelanggan LIKE ?`, prefix)
	_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'pelanggan'`)
	_ = db.Exec(`DELETE FROM pelanggan WHERE kode_pelanggan LIKE ?`, prefix)

	plRepo := repository.NewPelangganRepo()
	pelangganSvc := service.NewPelangganService(db, plRepo, repository.NewAuditRepo())
	exportSvc := service.NewExportService(db, repository.NewBarangMasukRepo(), plRepo, repository.NewTransaksiRepo())
	bmSvc := service.NewBarangMasukService(
		db, repository.NewBarangMasukRepo(), repository.NewBarangRepo(), repository.NewStockRepo(),
		repository.NewPriceChangeRepo(), repository.NewAuditRepo(), deps.Config.Bisnis.PPNPersenDefault,
	)
	importSvc := service.NewImportService(db, bmSvc, pelangganSvc)
	deps.PelangganHandler = handler.NewPelangganHandler(pelangganSvc, exportSvc, importSvc)

	emailSA := fmt.Sprintf("sa06_plg_sa_%d@pkb.test", time.Now().UnixNano()%100000)
	emailAdmin := fmt.Sprintf("sa06_plg_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailSA, "rahasia123", domain.RoleSuperAdmin, true)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)

	r := handler.NewRouter(deps)
	tokenSA := loginToken(t, r, emailSA, "rahasia123")
	tokenAdmin := loginToken(t, r, emailAdmin, "rahasia123")
	return &ginEngine{r: r}, tokenSA, tokenAdmin, cleanup
}

func pelangganBody(kode string) map[string]any {
	return map[string]any{
		"kode_pelanggan": kode,
		"nama_pelanggan": "Toko SB06",
		"tgl_registrasi": "2026-01-15",
		"phone":          "08123456789",
		"territory":      "Jakarta",
		"distrik":        "Selatan",
		"alamat_toko":    "Jl. Contoh 1",
		"provinsi":       "DKI Jakarta",
		"kabupaten":      "Jakarta Selatan",
		"kecamatan":      "Kebayoran",
		"kelurahan":      "Senayan",
		"channel_outlet": "General Trade",
	}
}

func TestPelangganCRUDSelect2LockKode(t *testing.T) {
	eng, tokenSA, tokenAdmin, cleanup := setupPelangganRouter(t)
	defer cleanup()
	r := eng.r

	prefix := fmt.Sprintf("SB06_%d", time.Now().UnixNano()%100000)
	kode := prefix + "_A"

	createBody, _ := json.Marshal(pelangganBody(kode))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenAdmin)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create admin: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID            uint64 `json:"id"`
			KodePelanggan string `json:"kode_pelanggan"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.ID == 0 || created.Data.KodePelanggan != kode {
		t.Fatalf("create body: %+v", created)
	}

	patchBody, _ := json.Marshal(map[string]string{"nama_pelanggan": "Ganti Nama"})
	wForbid := httptest.NewRecorder()
	reqForbid := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/pelanggan/%d", created.Data.ID), bytes.NewReader(patchBody))
	reqForbid.Header.Set("Content-Type", "application/json")
	reqForbid.Header.Set("Authorization", "Bearer "+tokenAdmin)
	r.ServeHTTP(wForbid, reqForbid)
	if wForbid.Code != http.StatusForbidden {
		t.Fatalf("admin ubah want 403 got %d %s", wForbid.Code, wForbid.Body.String())
	}

	wUbah := httptest.NewRecorder()
	reqUbah := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/pelanggan/%d", created.Data.ID), bytes.NewReader(patchBody))
	reqUbah.Header.Set("Content-Type", "application/json")
	reqUbah.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wUbah, reqUbah)
	if wUbah.Code != 200 {
		t.Fatalf("sa ubah: %d %s", wUbah.Code, wUbah.Body.String())
	}

	wCari := httptest.NewRecorder()
	reqCari := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan/cari?q="+prefix+"&page=1", nil)
	reqCari.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wCari, reqCari)
	if wCari.Code != 200 {
		t.Fatalf("cari: %d %s", wCari.Code, wCari.Body.String())
	}
	var cari struct {
		Results []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"results"`
		Pagination struct {
			More bool `json:"more"`
		} `json:"pagination"`
	}
	_ = json.Unmarshal(wCari.Body.Bytes(), &cari)
	if len(cari.Results) < 1 || cari.Results[0].ID != kode {
		t.Fatalf("cari results: %+v", cari)
	}

	statusBody, _ := json.Marshal(map[string]bool{"is_active": false})
	wStatus := httptest.NewRecorder()
	reqStatus := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/pelanggan/%d/status", created.Data.ID), bytes.NewReader(statusBody))
	reqStatus.Header.Set("Content-Type", "application/json")
	reqStatus.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != 200 {
		t.Fatalf("status: %d %s", wStatus.Code, wStatus.Body.String())
	}

	wCari2 := httptest.NewRecorder()
	reqCari2 := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan/cari?q="+prefix, nil)
	reqCari2.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wCari2, reqCari2)
	_ = json.Unmarshal(wCari2.Body.Bytes(), &cari)
	if len(cari.Results) != 0 {
		t.Fatalf("nonaktif masih di cari: %+v", cari)
	}

	statusOn, _ := json.Marshal(map[string]bool{"is_active": true})
	wOn := httptest.NewRecorder()
	reqOn := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/pelanggan/%d/status", created.Data.ID), bytes.NewReader(statusOn))
	reqOn.Header.Set("Content-Type", "application/json")
	reqOn.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wOn, reqOn)
	if wOn.Code != 200 {
		t.Fatalf("aktifkan: %d", wOn.Code)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO transaksi_penjualan (
			tanggal, periode, kode_pelanggan, nama_pelanggan, alamat, channel_outlet, area,
			jumlah_dibayar, sisa_hutang, total_akhir, created_at, updated_at
		) VALUES (
			'2026-03-01', '2026-03', ?, 'Toko SB06', 'Jl. Contoh', 'General Trade', 'Jakarta',
			0, 0, 0, NOW(), NOW()
		)
	`, kode).Error; err != nil {
		t.Fatalf("insert trx: %v", err)
	}

	kodeBaru := prefix + "_B"
	lockBody, _ := json.Marshal(map[string]string{"kode_pelanggan": kodeBaru})
	wLock := httptest.NewRecorder()
	reqLock := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/pelanggan/%d", created.Data.ID), bytes.NewReader(lockBody))
	reqLock.Header.Set("Content-Type", "application/json")
	reqLock.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wLock, reqLock)
	if wLock.Code != http.StatusUnprocessableEntity {
		t.Fatalf("lock kode want 422 got %d %s", wLock.Code, wLock.Body.String())
	}

	wLegacy := httptest.NewRecorder()
	reqLegacy := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan/terlambat-bayar", nil)
	reqLegacy.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wLegacy, reqLegacy)
	if wLegacy.Code == 200 {
		t.Fatalf("terlambat-bayar tidak boleh ada")
	}
}
