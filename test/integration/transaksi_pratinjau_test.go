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
	"app/internal/handler"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"app/internal/service"
	"gorm.io/gorm"
)

func setupTransaksiPratinjauRouter(t *testing.T) (http.Handler, string, string, *gorm.DB, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)
	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}

	prefix := "SC02_%"
	cleanup := func() {
		_ = db.Exec(`DELETE FROM transaksi_penjualan WHERE kode_pelanggan LIKE ?`, prefix)
		_ = db.Exec(`DELETE FROM stock_movements WHERE barang_id IN (SELECT id FROM barang WHERE kode_barang LIKE ?)`, prefix)
		_ = db.Exec(`DELETE FROM barang_masuk WHERE barang_id IN (SELECT id FROM barang WHERE kode_barang LIKE ?)`, prefix)
		_ = db.Exec(`DELETE FROM barang WHERE kode_barang LIKE ?`, prefix)
		_ = db.Exec(`DELETE FROM promos WHERE kode_promo LIKE ?`, prefix)
		_ = db.Exec(`DELETE FROM pelanggan WHERE kode_pelanggan LIKE ?`, prefix)
		cleanupAuth()
	}
	cleanup()

	barangRepo := repository.NewBarangRepo()
	bmRepo := repository.NewBarangMasukRepo()
	stockRepo := repository.NewStockRepo()
	priceRepo := repository.NewPriceChangeRepo()
	auditRepo := repository.NewAuditRepo()
	plRepo := repository.NewPelangganRepo()
	promoRepo := repository.NewPromoRepo()

	barangSvc := service.NewBarangService(db, barangRepo, bmRepo, stockRepo, priceRepo, auditRepo, nil, cfg.Bisnis.PPNPersenDefault)
	bmSvc := service.NewBarangMasukService(db, bmRepo, barangRepo, stockRepo, priceRepo, auditRepo, cfg.Bisnis.PPNPersenDefault)
	exportSvc := service.NewExportService(db, bmRepo, plRepo, repository.NewTransaksiRepo())
	plSvc := service.NewPelangganService(db, plRepo, auditRepo)
	importSvc := service.NewImportService(db, bmSvc, plSvc)
	promoSvc := service.NewPromoService(db, promoRepo, barangRepo, auditRepo, clock.Real{})
	trxSvc := service.NewTransaksiService(db, plRepo, barangRepo, stockRepo, promoRepo,
		repository.NewTransaksiRepo(), repository.NewPembayaranRepo(), auditRepo, clock.Real{}, cfg.Bisnis.PPNPersenDefault)

	deps.BarangHandler = handler.NewBarangHandler(barangSvc)
	deps.BarangMasukHandler = handler.NewBarangMasukHandler(bmSvc, exportSvc, importSvc)
	deps.PelangganHandler = handler.NewPelangganHandler(plSvc, exportSvc, importSvc)
	deps.PromoHandler = handler.NewPromoHandler(promoSvc)
	deps.TransaksiHandler = handler.NewTransaksiHandler(trxSvc)
	bayarSvc := service.NewPembayaranService(
		db, repository.NewTransaksiRepo(), repository.NewPembayaranRepo(),
		repository.NewIdempotencyRepo(), auditRepo, clock.Real{},
	)
	deps.PembayaranHandler = handler.NewPembayaranHandler(bayarSvc)
	piutangSvc := service.NewPiutangService(db, repository.NewTransaksiRepo(), repository.NewPembayaranRepo(), clock.Real{})
	deps.PiutangHandler = handler.NewPiutangHandler(piutangSvc)
	fakturSvc := service.NewFakturService(
		db, repository.NewTransaksiRepo(), repository.NewUserRepo(), auditRepo, clock.Real{},
		cfg.Bisnis, cfg.Perusahaan,
	)
	deps.FakturHandler = handler.NewFakturHandler(fakturSvc)
	deps.DashboardHandler = handler.NewDashboardHandler(
		service.NewDashboardService(db, repository.NewTransaksiRepo(), clock.Real{}),
	)
	stockSvc := service.NewStockService(db, barangRepo, stockRepo, auditRepo, clock.Real{})
	deps.StokHandler = handler.NewStokHandler(stockSvc)
	deps.LaporanHandler = handler.NewLaporanHandler(exportSvc, service.NewLaporanService(db, repository.NewLaporanRepo()), service.NewExportJobStore())

	// Pastikan tabel idempotency ada (migrasi 000002).
	_ = db.Exec(`
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    scope         VARCHAR(64) NOT NULL,
    idem_key      VARCHAR(128) NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    resource_id   BIGINT UNSIGNED NULL,
    request_hash  CHAR(64) NOT NULL,
    response_json JSON NOT NULL,
    status_code   INT NOT NULL DEFAULT 200,
    created_at    DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_idempotency_scope_user_key (scope, user_id, idem_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)

	emailSales := fmt.Sprintf("sa06_sc02_sl_%d@pkb.test", time.Now().UnixNano()%100000)
	emailSA := fmt.Sprintf("sa06_sc02_sa_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailSales, "rahasia123", domain.RoleSales, true)
	createUser(t, emailSA, "rahasia123", domain.RoleSuperAdmin, true)

	r := handler.NewRouter(deps)
	return r, loginToken(t, r, emailSales, "rahasia123"), loginToken(t, r, emailSA, "rahasia123"), db, cleanup
}

func TestPratinjauTotalDanHargaServerTanpaTulisDB(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)
	kodeB := fmt.Sprintf("SC02_B_%d", suffix)
	kodePromo := fmt.Sprintf("SC02_P_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg,
		"nama_pelanggan": "Toko SC02",
		"tgl_registrasi": "2026-01-15",
		"phone":          "08123456789",
		"territory":      "Bandung",
		"distrik":        "Timur",
		"alamat_toko":    "Jl. SC02",
		"provinsi":       "Jawa Barat",
		"kabupaten":      "Bandung",
		"kecamatan":      "Cibiru",
		"kelurahan":      "Cipadung",
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

	postBM := func(kode, faktur, batch string, qty int, harga string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"kode_barang": kode, "buat_barang_baru": true, "nama_item": "Item " + kode,
			"brand": "SC02", "no_faktur": faktur, "no_batch": batch,
			"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": qty, "harga": harga,
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
	postBM(kodeA, fmt.Sprintf("SC02_FA_%d", suffix), "B-A", 100, "20000.00")
	postBM(kodeB, fmt.Sprintf("SC02_FB_%d", suffix), "B-B", 100, "30000.00")

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

	var nBefore int64
	if err := db.Table("transaksi_penjualan").Count(&nBefore).Error; err != nil {
		t.Fatal(err)
	}
	var stockBefore int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockBefore)

	body, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg,
		"tanggal":        "2026-09-04",
		"items": []map[string]any{
			{
				"kode_item": kodeA, "qty": 10, "harga": "99999.00",
				"disc1_persen": "10.00", "kode_promos": []string{kodePromo},
			},
			{"kode_item": kodeB, "qty": 5, "harga": "1.00"},
		},
		"disc1_persen": "5.00",
		"ppn_persen":   "11.00",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi/pratinjau", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pratinjau: %d %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Items []struct {
				Harga          string `json:"harga"`
				QtyPromo       int    `json:"qty_promo"`
				TotalQtyKeluar int    `json:"total_qty_keluar"`
			} `json:"items"`
			Ringkasan struct {
				GrandTotal string `json:"grand_total"`
				Total      string `json:"total"`
				PPNNominal string `json:"ppn_nominal"`
				TotalAkhir string `json:"total_akhir"`
			} `json:"ringkasan"`
			SemuaStokCukup bool     `json:"semua_stok_cukup"`
			Peringatan     []string `json:"peringatan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Data.SemuaStokCukup || len(resp.Data.Items) != 2 {
		t.Fatalf("resp: %+v / body=%s", resp.Data, w.Body.String())
	}
	if resp.Data.Items[0].Harga != "20000.00" || resp.Data.Items[1].Harga != "30000.00" {
		t.Fatalf("harga server diganti? got %s / %s", resp.Data.Items[0].Harga, resp.Data.Items[1].Harga)
	}
	if resp.Data.Items[0].QtyPromo != 1 || resp.Data.Items[0].TotalQtyKeluar != 11 {
		t.Fatalf("promo qty: %+v", resp.Data.Items[0])
	}
	if resp.Data.Ringkasan.GrandTotal != "330000.00" ||
		resp.Data.Ringkasan.Total != "313500.00" ||
		resp.Data.Ringkasan.PPNNominal != "34485.00" ||
		resp.Data.Ringkasan.TotalAkhir != "347985.00" {
		t.Fatalf("ringkasan: %+v", resp.Data.Ringkasan)
	}
	if len(resp.Data.Peringatan) == 0 {
		t.Fatal("harapkan peringatan harga client diganti")
	}

	var nAfter int64
	_ = db.Table("transaksi_penjualan").Count(&nAfter)
	if nAfter != nBefore {
		t.Fatalf("pratinjau menulis transaksi: before=%d after=%d", nBefore, nAfter)
	}
	var stockAfter int64
	_ = db.Raw(`SELECT COALESCE(SUM(stok_tersedia),0) FROM barang WHERE kode_barang IN (?,?)`, kodeA, kodeB).Scan(&stockAfter)
	if stockAfter != stockBefore {
		t.Fatalf("stok berubah: %d → %d", stockBefore, stockAfter)
	}

	bodyKurang, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg,
		"tanggal":        "2026-09-04",
		"item": map[string]any{
			"kode_item": kodeA, "qty": 5000, "harga": "20000.00",
		},
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi/pratinjau", bytes.NewReader(bodyKurang))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("stok kurang: %d %s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Data struct {
			SemuaStokCukup bool `json:"semua_stok_cukup"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Data.SemuaStokCukup {
		t.Fatal("harapkan semua_stok_cukup=false")
	}
}
