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

func setupBarangMasukRouter(t *testing.T) (*ginEngine, string, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)

	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = db.Exec(`DELETE FROM stock_movements WHERE reference_type = 'barang_masuk' AND reference_id IN (
			SELECT id FROM barang_masuk WHERE no_faktur LIKE 'SB03_%')`)
		_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type IN ('barang_masuk','barang') AND ringkasan LIKE '%SB03%'`)
		_ = db.Exec(`DELETE FROM barang_masuk WHERE no_faktur LIKE 'SB03_%'`)
		_ = db.Exec(`DELETE FROM barang WHERE kode_barang LIKE 'SB03_%'`)
		cleanupAuth()
	}
	_ = db.Exec(`DELETE FROM stock_movements WHERE barang_id IN (SELECT id FROM barang WHERE kode_barang LIKE 'SB03_%')`)
	_ = db.Exec(`DELETE FROM barang_masuk WHERE no_faktur LIKE 'SB03_%' OR barang_id IN (SELECT id FROM barang WHERE kode_barang LIKE 'SB03_%')`)
	_ = db.Exec(`DELETE FROM barang WHERE kode_barang LIKE 'SB03_%'`)

	barangRepo := repository.NewBarangRepo()
	bmRepo := repository.NewBarangMasukRepo()
	stockRepo := repository.NewStockRepo()
	priceRepo := repository.NewPriceChangeRepo()
	auditRepo := repository.NewAuditRepo()
	barangSvc := service.NewBarangService(db, barangRepo, bmRepo, stockRepo, priceRepo, auditRepo, nil, cfg.Bisnis.PPNPersenDefault)
	bmSvc := service.NewBarangMasukService(db, bmRepo, barangRepo, stockRepo, priceRepo, auditRepo, cfg.Bisnis.PPNPersenDefault)
	exportSvc := service.NewExportService(db, bmRepo, repository.NewPelangganRepo(), repository.NewTransaksiRepo())
	plSvc := service.NewPelangganService(db, repository.NewPelangganRepo(), auditRepo)
	importSvc := service.NewImportService(db, bmSvc, plSvc)
	deps.BarangHandler = handler.NewBarangHandler(barangSvc)
	deps.BarangMasukHandler = handler.NewBarangMasukHandler(bmSvc, exportSvc, importSvc)

	emailAdmin := fmt.Sprintf("sb03_admin_%d@pkb.test", time.Now().UnixNano()%100000)
	emailSA := fmt.Sprintf("sb03_sa_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)
	createUser(t, emailSA, "rahasia123", domain.RoleSuperAdmin, true)

	r := handler.NewRouter(deps)
	tokAdmin := loginToken(t, r, emailAdmin, "rahasia123")
	tokSA := loginToken(t, r, emailSA, "rahasia123")
	return &ginEngine{r: r}, tokAdmin, tokSA, cleanup
}

func TestBarangMasukCreateHPPDanUnique(t *testing.T) {
	eng, tokAdmin, tokSA, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SB03_%d", suffix)
	faktur := fmt.Sprintf("SB03_F_%d", suffix)

	body, _ := json.Marshal(map[string]any{
		"kode_barang":      kode,
		"buat_barang_baru": true,
		"nama_item":        "Item SB03",
		"brand":            "BrandSB03",
		"no_faktur":        faktur,
		"no_batch":         "B-001",
		"exp":              "2027-06-30",
		"tanggal_masuk":    "2026-09-01",
		"qty":              10,
		"harga":            "20000.00",
		"disc_hpp_1":       "23.10",
		"disc_hpp_2":       "2.00",
		"disc_hpp_3":       "5.00",
		"markup_mt_type":   "percent",
		"markup_mt_amount": "15.00",
		"markup_gt_type":   "percent",
		"markup_gt_amount": "12.50",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	var created struct {
		Data struct {
			ID           uint64 `json:"id"`
			HPP          string `json:"hpp"`
			HPPDenganPPN string `json:"hpp_dengan_ppn"`
			HargaMT      string `json:"harga_mt"`
			HargaGT      string `json:"harga_gt"`
			QtyTersedia  int    `json:"qty_tersedia"`
			KodeBarang   string `json:"kode_barang"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.ID == 0 || created.Data.KodeBarang != kode {
		t.Fatalf("create body: %+v", created)
	}
	if created.Data.HPP != "14318.78" {
		t.Fatalf("hpp want 14318.78 got %s", created.Data.HPP)
	}
	if created.Data.HPPDenganPPN != "15893.85" {
		t.Fatalf("hpp+ppn want 15893.85 got %s", created.Data.HPPDenganPPN)
	}
	if created.Data.HargaMT != "23000.00" || created.Data.HargaGT != "22500.00" {
		t.Fatalf("harga mt/gt %s / %s", created.Data.HargaMT, created.Data.HargaGT)
	}
	if created.Data.QtyTersedia != 10 {
		t.Fatalf("qty_tersedia %d", created.Data.QtyTersedia)
	}

	// Client tidak boleh override harga jual — field diabaikan / tidak di body create.
	// Duplikat unique (barang_id, no_batch, no_faktur)
	wDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	reqDup.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("duplikat want 409 got %d %s", wDup.Code, wDup.Body.String())
	}

	// List
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/barang-masuk?kode_barang="+kode, nil)
	reqList.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wList, reqList)
	if wList.Code != 200 {
		t.Fatalf("list: %d", wList.Code)
	}

	// Admin tidak boleh hapus
	wDelAdmin := httptest.NewRecorder()
	reqDelAdmin := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/barang-masuk/%d", created.Data.ID), nil)
	reqDelAdmin.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wDelAdmin, reqDelAdmin)
	if wDelAdmin.Code != http.StatusForbidden {
		t.Fatalf("admin hapus want 403 got %d", wDelAdmin.Code)
	}

	// SA boleh ubah markup; harga_mt dihitung ulang server
	upd, _ := json.Marshal(map[string]any{
		"markup_mt_amount": "20.00",
	})
	wUpd := httptest.NewRecorder()
	reqUpd := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/barang-masuk/%d", created.Data.ID), bytes.NewReader(upd))
	reqUpd.Header.Set("Content-Type", "application/json")
	reqUpd.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != 200 {
		t.Fatalf("ubah: %d %s", wUpd.Code, wUpd.Body.String())
	}
	var updated struct {
		Data struct {
			HargaMT string `json:"harga_mt"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wUpd.Body.Bytes(), &updated)
	if updated.Data.HargaMT != "24000.00" {
		t.Fatalf("harga_mt setelah markup 20%% want 24000.00 got %s", updated.Data.HargaMT)
	}

	// Stok master naik
	cfg, _ := config.Load()
	db, _ := repository.NewDB(cfg.DB)
	var stok int
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kode).Scan(&stok)
	if stok != 10 {
		t.Fatalf("stok_tersedia want 10 got %d", stok)
	}

	// SA hapus → stok turun
	wDel := httptest.NewRecorder()
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/barang-masuk/%d", created.Data.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != 200 {
		t.Fatalf("hapus: %d %s", wDel.Code, wDel.Body.String())
	}
	_ = db.Raw(`SELECT stok_tersedia FROM barang WHERE kode_barang = ?`, kode).Scan(&stok)
	if stok != 0 {
		t.Fatalf("stok setelah hapus want 0 got %d", stok)
	}
}

// TestBarangMasukAbaikanHargaClientMTGT: payload client yang menyetel harga_mt/harga_gt diabaikan (12 F3).
func TestBarangMasukAbaikanHargaClientMTGT(t *testing.T) {
	eng, tokAdmin, _, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode := fmt.Sprintf("SB10_%d", suffix)
	body, _ := json.Marshal(map[string]any{
		"kode_barang":      kode,
		"buat_barang_baru": true,
		"nama_item":        "Item SB10 MTGT",
		"brand":            "BrandSB10",
		"no_faktur":        fmt.Sprintf("SB10_F_%d", suffix),
		"no_batch":         "B-MTGT",
		"exp":              "2027-06-30",
		"tanggal_masuk":    "2026-09-01",
		"qty":              5,
		"harga":            "20000.00",
		"disc_hpp_1":       "0",
		"disc_hpp_2":       "0",
		"disc_hpp_3":       "0",
		"markup_mt_type":   "percent",
		"markup_mt_amount": "15.00",
		"markup_gt_type":   "percent",
		"markup_gt_amount": "12.50",
		// Client mencoba override — harus diabaikan.
		"harga_mt": "1.00",
		"harga_gt": "2.00",
		"hpp":      "3.00",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			HPP     string `json:"hpp"`
			HargaMT string `json:"harga_mt"`
			HargaGT string `json:"harga_gt"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.HPP == "3.00" || created.Data.HargaMT == "1.00" || created.Data.HargaGT == "2.00" {
		t.Fatalf("client override diterima: %+v", created.Data)
	}
	// harga 20000, markup MT 15% → 23000; GT 12.5% → 22500; hpp=harga tanpa disc
	if created.Data.HargaMT != "23000.00" || created.Data.HargaGT != "22500.00" {
		t.Fatalf("server harga mt/gt %s / %s", created.Data.HargaMT, created.Data.HargaGT)
	}
	if created.Data.HPP != "20000.00" {
		t.Fatalf("hpp server want 20000.00 got %s", created.Data.HPP)
	}
}
