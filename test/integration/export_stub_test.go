package integration_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/domain"
	"app/internal/handler"
	"app/internal/pkg/csvx"
	"app/internal/repository"
	"app/internal/service"
)

func setupExportRouter(t *testing.T) (*ginEngine, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)
	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		cleanupAuth()
	}

	bmRepo := repository.NewBarangMasukRepo()
	plRepo := repository.NewPelangganRepo()
	exportSvc := service.NewExportService(db, bmRepo, plRepo, repository.NewTransaksiRepo())
	bmSvc := service.NewBarangMasukService(
		db, bmRepo, repository.NewBarangRepo(), repository.NewStockRepo(),
		repository.NewPriceChangeRepo(), repository.NewAuditRepo(), cfg.Bisnis.PPNPersenDefault,
	)
	plSvc := service.NewPelangganService(db, plRepo, repository.NewAuditRepo())
	importSvc := service.NewImportService(db, bmSvc, plSvc)
	deps.BarangMasukHandler = handler.NewBarangMasukHandler(bmSvc, exportSvc, importSvc)
	deps.PelangganHandler = handler.NewPelangganHandler(plSvc, exportSvc, importSvc)

	email := fmt.Sprintf("sb09_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, email, "rahasia123", domain.RoleAdmin, true)
	r := handler.NewRouter(deps)
	return &ginEngine{r: r}, loginToken(t, r, email, "rahasia123"), cleanup
}

func TestTemplateDanEksporCSVBOM(t *testing.T) {
	eng, token, cleanup := setupExportRouter(t)
	defer cleanup()
	r := eng.r

	// Template barang masuk — BOM + contoh HPP
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/barang-masuk/template-impor", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("template bm: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.Bytes()
	if !bytes.HasPrefix(body, csvx.BOMUTF8) {
		t.Fatalf("template bm tanpa BOM UTF-8")
	}
	if !strings.Contains(string(body), "20000.00") || !strings.Contains(string(body), "23.10") {
		t.Fatalf("template bm harus berisi contoh HPP 04 §1.1")
	}

	// Template pelanggan
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan/template-impor", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("template plg: %d", w2.Code)
	}
	if !bytes.HasPrefix(w2.Body.Bytes(), csvx.BOMUTF8) {
		t.Fatalf("template plg tanpa BOM")
	}

	// Ekspor list
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/barang-masuk/export", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("export bm: %d %s", w3.Code, w3.Body.String())
	}
	if !bytes.HasPrefix(w3.Body.Bytes(), csvx.BOMUTF8) {
		t.Fatalf("export bm tanpa BOM")
	}

	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan/export", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w4, req4)
	if w4.Code != 200 {
		t.Fatalf("export plg: %d %s", w4.Code, w4.Body.String())
	}

	// Impor CSV invalid → 422 (bukan 501)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "x.csv")
	_, _ = part.Write([]byte("a,b\n1,2\n"))
	_ = mw.Close()
	w5 := httptest.NewRecorder()
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk/impor", &buf)
	req5.Header.Set("Authorization", "Bearer "+token)
	req5.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusUnprocessableEntity {
		t.Fatalf("impor invalid want 422 got %d %s", w5.Code, w5.Body.String())
	}
}
