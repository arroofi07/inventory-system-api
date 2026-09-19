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

func setupBarangRouter(t *testing.T) (*ginEngine, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)

	cfg := deps.Config
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'barang'`)
		_ = db.Exec(`DELETE FROM barang WHERE kode_barang LIKE 'SB01_%'`)
		cleanupAuth()
	}
	_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'barang'`)
	_ = db.Exec(`DELETE FROM barang WHERE kode_barang LIKE 'SB01_%'`)

	barangSvc := service.NewBarangService(
		db, repository.NewBarangRepo(), repository.NewBarangMasukRepo(),
		repository.NewStockRepo(), repository.NewPriceChangeRepo(), repository.NewAuditRepo(),
		nil, deps.Config.Bisnis.PPNPersenDefault,
	)
	deps.BarangHandler = handler.NewBarangHandler(barangSvc)

	// Pastikan user admin untuk kelola
	email := fmt.Sprintf("sa06_barang_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, email, "rahasia123", domain.RoleAdmin, true)

	r := handler.NewRouter(deps)
	token := loginToken(t, r, email, "rahasia123")
	return &ginEngine{r: r}, token, cleanup
}

type ginEngine struct {
	r http.Handler
}

func loginToken(t *testing.T, r http.Handler, email, pass string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": pass})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Data.AccessToken
}

func TestBarangCRUDPola(t *testing.T) {
	eng, token, cleanup := setupBarangRouter(t)
	defer cleanup()
	r := eng.r

	kode := fmt.Sprintf("SB01_%d", time.Now().UnixNano()%100000)

	// create
	createBody, _ := json.Marshal(map[string]any{
		"kode_barang": kode,
		"nama_item":   "Item SB01",
		"brand":       "TestBrand",
		"min_stock":   5,
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/barang", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID         uint64 `json:"id"`
			KodeBarang string `json:"kode_barang"`
			StatusStok string `json:"status_stok"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.ID == 0 || created.Data.KodeBarang != kode {
		t.Fatalf("create body: %+v", created)
	}
	if created.Data.StatusStok != "HABIS" {
		t.Fatalf("stok 0 → HABIS, got %s", created.Data.StatusStok)
	}

	// duplikat
	wDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/barang", bytes.NewReader(createBody))
	reqDup.Header.Set("Content-Type", "application/json")
	reqDup.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("duplikat want 409 got %d", wDup.Code)
	}

	// list search
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/barang?q="+kode+"&page=1&per_page=10", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wList, reqList)
	if wList.Code != 200 {
		t.Fatalf("list: %d %s", wList.Code, wList.Body.String())
	}
	var list struct {
		Data []any `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &list)
	if list.Meta.Total < 1 || len(list.Data) < 1 {
		t.Fatalf("list kosong: %+v", list)
	}

	// detail
	wDet := httptest.NewRecorder()
	reqDet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/barang/%d", created.Data.ID), nil)
	reqDet.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDet, reqDet)
	if wDet.Code != 200 {
		t.Fatalf("detail: %d", wDet.Code)
	}

	// update
	upd, _ := json.Marshal(map[string]any{"nama_item": "Item SB01 Ubah"})
	wUpd := httptest.NewRecorder()
	reqUpd := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/barang/%d", created.Data.ID), bytes.NewReader(upd))
	reqUpd.Header.Set("Content-Type", "application/json")
	reqUpd.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != 200 {
		t.Fatalf("update: %d %s", wUpd.Code, wUpd.Body.String())
	}

	// soft nonaktif
	st, _ := json.Marshal(map[string]any{"is_active": false})
	wSt := httptest.NewRecorder()
	reqSt := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/barang/%d/status", created.Data.ID), bytes.NewReader(st))
	reqSt.Header.Set("Content-Type", "application/json")
	reqSt.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wSt, reqSt)
	if wSt.Code != 200 {
		t.Fatalf("status: %d %s", wSt.Code, wSt.Body.String())
	}

	// default list menyembunyikan nonaktif
	wHide := httptest.NewRecorder()
	reqHide := httptest.NewRequest(http.MethodGet, "/api/v1/barang?q="+kode, nil)
	reqHide.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wHide, reqHide)
	var hide struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(wHide.Body.Bytes(), &hide)
	if hide.Meta.Total != 0 {
		t.Fatalf("nonaktif harus tersembunyi, total=%d", hide.Meta.Total)
	}

	// include_inactive=true → histori tetap terlihat
	wHist := httptest.NewRecorder()
	reqHist := httptest.NewRequest(http.MethodGet, "/api/v1/barang?q="+kode+"&include_inactive=true", nil)
	reqHist.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wHist, reqHist)
	if wHist.Code != 200 {
		t.Fatalf("include_inactive: %d %s", wHist.Code, wHist.Body.String())
	}
	var hist struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(wHist.Body.Bytes(), &hist)
	if hist.Meta.Total < 1 {
		t.Fatalf("histori nonaktif harus muncul dengan include_inactive, total=%d", hist.Meta.Total)
	}

	// detail by id tetap OK meski nonaktif
	wDet2 := httptest.NewRecorder()
	reqDet2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/barang/%d", created.Data.ID), nil)
	reqDet2.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(wDet2, reqDet2)
	if wDet2.Code != 200 {
		t.Fatalf("detail nonaktif: %d", wDet2.Code)
	}

	// audit tertulis
	cfg, _ := config.Load()
	db, _ := repository.NewDB(cfg.DB)
	var auditCount int64
	_ = db.Raw(`SELECT COUNT(*) FROM audit_logs WHERE entity_type='barang' AND entity_id=?`, created.Data.ID).Scan(&auditCount)
	if auditCount < 3 {
		t.Fatalf("audit_logs kurang: %d (buat+ubah+status)", auditCount)
	}
}

func TestBarangValidasiCreate(t *testing.T) {
	eng, token, cleanup := setupBarangRouter(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]any{"kode_barang": "", "nama_item": "", "brand": ""})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/barang", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	eng.r.ServeHTTP(w, req)
	if w.Code != 422 {
		t.Fatalf("want 422 got %d %s", w.Code, w.Body.String())
	}
}
