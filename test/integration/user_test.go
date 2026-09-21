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
	"app/internal/pkg/clock"
	"app/internal/repository"
	"app/internal/service"
)

func setupUserRouter(t *testing.T) (*ginEngine, string, string, func()) {
	t.Helper()
	deps, cleanupAuth := setupAuthRouter(t)

	cfg := deps.Config
	db := openGormCfg(t, cfg.DB)

	cleanup := func() {
		_ = db.Exec(`DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'sb07_%')`)
		_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'user'`)
		_ = db.Exec(`DELETE FROM users WHERE email LIKE 'sb07_%'`)
		cleanupAuth()
	}
	_ = db.Exec(`DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'sb07_%')`)
	_ = db.Exec(`DELETE FROM audit_logs WHERE entity_type = 'user'`)
	_ = db.Exec(`DELETE FROM users WHERE email LIKE 'sb07_%'`)

	userRepo := repository.NewUserRepo()
	tokenRepo := repository.NewTokenRepo()
	userSvc := service.NewUserService(db, userRepo, tokenRepo, repository.NewAuditRepo(), clock.Real{})
	deps.UserHandler = handler.NewUserHandler(userSvc)

	emailSA := fmt.Sprintf("sb07_sa_%d@pkb.test", time.Now().UnixNano()%100000)
	emailAdmin := fmt.Sprintf("sb07_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailSA, "rahasia123", domain.RoleSuperAdmin, true)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)

	r := handler.NewRouter(deps)
	tokenSA := loginToken(t, r, emailSA, "rahasia123")
	tokenAdmin := loginToken(t, r, emailAdmin, "rahasia123")
	return &ginEngine{r: r}, tokenSA, tokenAdmin, cleanup
}

func TestUserCRUDRoleRevokeAdminProtected(t *testing.T) {
	eng, tokenSA, tokenAdmin, cleanup := setupUserRouter(t)
	defer cleanup()
	r := eng.r

	suf := fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	emailSales := fmt.Sprintf("sb07_sales_%s@pkb.test", suf)

	// Admin boleh list
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/users?role=sales", nil)
	reqList.Header.Set("Authorization", "Bearer "+tokenAdmin)
	r.ServeHTTP(wList, reqList)
	if wList.Code != 200 {
		t.Fatalf("admin list: %d %s", wList.Code, wList.Body.String())
	}

	// Admin tidak boleh buat
	createBody, _ := json.Marshal(map[string]any{
		"name":     "Sales SB07",
		"email":    emailSales,
		"password": "rahasia123",
		"role":     "sales",
	})
	wForbid := httptest.NewRecorder()
	reqForbid := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBody))
	reqForbid.Header.Set("Content-Type", "application/json")
	reqForbid.Header.Set("Authorization", "Bearer "+tokenAdmin)
	r.ServeHTTP(wForbid, reqForbid)
	if wForbid.Code != http.StatusForbidden {
		t.Fatalf("admin buat want 403 got %d", wForbid.Code)
	}

	// Tolak role admin
	badRole, _ := json.Marshal(map[string]any{
		"name": "Bad", "email": fmt.Sprintf("sb07_bad_%s@pkb.test", suf),
		"password": "rahasia123", "role": "admin",
	})
	wBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(badRole))
	reqBad.Header.Set("Content-Type", "application/json")
	reqBad.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("role admin want 422 got %d %s", wBad.Code, wBad.Body.String())
	}

	// SA buat sales
	wCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data struct {
			ID   uint64 `json:"id"`
			Role string `json:"role"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	if created.Data.ID == 0 || created.Data.Role != "sales" {
		t.Fatalf("create body: %+v", created)
	}

	// Login sales → ada refresh token
	_ = loginToken(t, r, emailSales, "rahasia123")
	cfg, _ := config.Load()
	db := openGormCfg(t, cfg.DB)
	var nToken int64
	_ = db.Raw(`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ? AND revoked_at IS NULL`, created.Data.ID).Scan(&nToken)
	if nToken < 1 {
		t.Fatalf("expected refresh token, got %d", nToken)
	}

	// Ubah role → cabut token
	patch, _ := json.Marshal(map[string]string{"role": "afiliasi"})
	wPatch := httptest.NewRecorder()
	reqPatch := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", created.Data.ID), bytes.NewReader(patch))
	reqPatch.Header.Set("Content-Type", "application/json")
	reqPatch.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wPatch, reqPatch)
	if wPatch.Code != 200 {
		t.Fatalf("ubah role: %d %s", wPatch.Code, wPatch.Body.String())
	}
	_ = db.Raw(`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ? AND revoked_at IS NULL`, created.Data.ID).Scan(&nToken)
	if nToken != 0 {
		t.Fatalf("token harus dicabut setelah ubah role, masih %d", nToken)
	}

	// List sales untuk filter laporan (sekarang afiliasi — kosong untuk role=sales id ini)
	wSales := httptest.NewRecorder()
	reqSales := httptest.NewRequest(http.MethodGet, "/api/v1/users?role=sales&q=sb07_sales_"+suf, nil)
	reqSales.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wSales, reqSales)
	if wSales.Code != 200 {
		t.Fatalf("list sales: %d", wSales.Code)
	}

	// Buat sales lagi lalu hapus OK
	email2 := fmt.Sprintf("sb07_sales2_%s@pkb.test", suf)
	body2, _ := json.Marshal(map[string]any{
		"name": "Sales2", "email": email2, "password": "rahasia123", "role": "sales",
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(w2, req2)
	var c2 struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &c2)

	wDel := httptest.NewRecorder()
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", c2.Data.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("hapus sales: %d %s", wDel.Code, wDel.Body.String())
	}

	// Admin tidak bisa dihapus
	var adminID uint64
	_ = db.Raw(`SELECT id FROM users WHERE email LIKE 'sb07_adm_%' ORDER BY id DESC LIMIT 1`).Scan(&adminID)
	if adminID == 0 {
		t.Fatal("admin id kosong")
	}
	wDelAdm := httptest.NewRecorder()
	reqDelAdm := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", adminID), nil)
	reqDelAdm.Header.Set("Authorization", "Bearer "+tokenSA)
	r.ServeHTTP(wDelAdm, reqDelAdm)
	if wDelAdm.Code != http.StatusUnprocessableEntity {
		t.Fatalf("hapus admin want 422 got %d %s", wDelAdm.Code, wDelAdm.Body.String())
	}
}
