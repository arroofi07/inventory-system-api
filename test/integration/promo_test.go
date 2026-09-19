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
	"app/internal/handler"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"app/internal/service"
)

func setupPromoRouter(t *testing.T) (*ginEngine, string, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)
	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'promo'`)
		_ = db.Exec(`DELETE FROM promos WHERE kode_promo LIKE 'PROMO%' OR kode_promo LIKE 'SB08_%'`)
		cleanupAuth()
	}
	_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'promo'`)
	_ = db.Exec(`DELETE FROM promos WHERE kode_promo LIKE 'SB08_%'`)

	promoSvc := service.NewPromoService(
		db, repository.NewPromoRepo(), repository.NewBarangRepo(), repository.NewAuditRepo(), clock.Real{},
	)
	deps.PromoHandler = handler.NewPromoHandler(promoSvc)

	emailSA := fmt.Sprintf("sa06_promo_sa_%d@pkb.test", time.Now().UnixNano()%100000)
	emailSales := fmt.Sprintf("sa06_promo_sl_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailSA, "rahasia123", domain.RoleSuperAdmin, true)
	createUser(t, emailSales, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)
	return &ginEngine{r: r}, loginToken(t, r, emailSA, "rahasia123"), loginToken(t, r, emailSales, "rahasia123"), cleanup
}

func TestPromoCRUDValidasiAktifKode(t *testing.T) {
	eng, tokenSA, tokenSales, cleanup := setupPromoRouter(t)
	defer cleanup()
	r := eng.r

	// Percentage tanpa nilai → 422 (validasi server)
	badPct, _ := json.Marshal(map[string]any{
		"nama_promo": "Bad pct", "tipe_promo": "percentage_discount",
		"tanggal_mulai": "2026-01-01", "tanggal_berakhir": "2026-12-31",
		"discount_percentage": "0",
	})
	wBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/promo", bytes.NewReader(badPct))
	reqBad.Header.Set("Content-Type", "application/json")
	reqBad.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("pct 0 want 422 got %d %s", wBad.Code, wBad.Body.String())
	}

	// Fixed tanpa amount → 422
	badFix, _ := json.Marshal(map[string]any{
		"nama_promo": "Bad fix", "tipe_promo": "fixed_discount",
		"tanggal_mulai": "2026-01-01", "tanggal_berakhir": "2026-12-31",
	})
	wFix := httptest.NewRecorder()
	reqFix := httptest.NewRequest(http.MethodPost, "/api/v1/promo", bytes.NewReader(badFix))
	reqFix.Header.Set("Content-Type", "application/json")
	reqFix.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wFix, reqFix)
	if wFix.Code != http.StatusUnprocessableEntity {
		t.Fatalf("fixed kosong want 422 got %d", wFix.Code)
	}

	// Sales tidak boleh buat
	okBody, _ := json.Marshal(map[string]any{
		"nama_promo": "Promo SB08", "tipe_promo": "percentage_discount",
		"discount_percentage": "10",
		"tanggal_mulai":       "2026-01-01", "tanggal_berakhir": "2026-12-31",
		"min_qty": 1,
	})
	wSales := httptest.NewRecorder()
	reqSales := httptest.NewRequest(http.MethodPost, "/api/v1/promo", bytes.NewReader(okBody))
	reqSales.Header.Set("Content-Type", "application/json")
	reqSales.Header.Set("Authorization", "Bearer "+tokenSales)
	r.ServeHTTP(wSales, reqSales)
	if wSales.Code != http.StatusForbidden {
		t.Fatalf("sales buat want 403 got %d", wSales.Code)
	}

	// SA buat → kode otomatis PROMOYYYYMM####
	wCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/promo", bytes.NewReader(okBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data struct {
			ID        uint64 `json:"id"`
			KodePromo string `json:"kode_promo"`
			IsActive  bool   `json:"is_active"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	prefix := "PROMO" + time.Now().Format("200601")
	if !strings.HasPrefix(created.Data.KodePromo, prefix) || len(created.Data.KodePromo) != len(prefix)+4 {
		t.Fatalf("kode otomatis: %s want prefix %s+4digit", created.Data.KodePromo, prefix)
	}

	// Sales boleh list aktif
	wAktif := httptest.NewRecorder()
	reqAktif := httptest.NewRequest(http.MethodGet, "/api/v1/promo?aktif=true", nil)
	reqAktif.Header.Set("Authorization", "Bearer "+tokenSales)
	r.ServeHTTP(wAktif, reqAktif)
	if wAktif.Code != 200 {
		t.Fatalf("sales list aktif: %d %s", wAktif.Code, wAktif.Body.String())
	}
	var list struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wAktif.Body.Bytes(), &list)
	found := false
	for _, it := range list.Data {
		if it.ID == created.Data.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("promo aktif tidak muncul di list sales")
	}

	// Toggle nonaktif → hilang dari aktif=true
	st, _ := json.Marshal(map[string]bool{"is_active": false})
	wSt := httptest.NewRecorder()
	reqSt := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/promo/%d/status", created.Data.ID), bytes.NewReader(st))
	reqSt.Header.Set("Content-Type", "application/json")
	reqSt.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wSt, reqSt)
	if wSt.Code != 200 {
		t.Fatalf("status: %d %s", wSt.Code, wSt.Body.String())
	}

	wAktif2 := httptest.NewRecorder()
	reqAktif2 := httptest.NewRequest(http.MethodGet, "/api/v1/promo?aktif=true&q="+created.Data.KodePromo, nil)
	reqAktif2.Header.Set("Authorization", "Bearer "+tokenSales)
	r.ServeHTTP(wAktif2, reqAktif2)
	_ = json.Unmarshal(wAktif2.Body.Bytes(), &list)
	for _, it := range list.Data {
		if it.ID == created.Data.ID {
			t.Fatalf("nonaktif masih di aktif=true")
		}
	}

	// Hapus (SA)
	wDel := httptest.NewRecorder()
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/promo/%d", created.Data.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("hapus: %d %s", wDel.Code, wDel.Body.String())
	}
}
