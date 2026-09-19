package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app/internal/config"
	"app/internal/domain"
	"app/internal/handler"
	"app/internal/pkg/clock"
	"app/internal/pkg/password"
	"app/internal/repository"
	"app/internal/service"
)

func setupAuthRouter(t *testing.T) (*handler.Dependencies, func()) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config: %v", err)
	}
	if cfg.JWT.AccessSecret == "" {
		cfg.JWT.AccessSecret = "test-access-secret-min-32-chars!!"
	}
	if cfg.JWT.RefreshSecret == "" {
		cfg.JWT.RefreshSecret = "test-refresh-secret-min-32-chars!"
	}
	cfg.JWT.AccessTTL = 15 * time.Minute
	cfg.JWT.RefreshTTL = 7 * 24 * time.Hour
	cfg.Login.RateLimitPerMinute = 6

	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Skipf("db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("TEST_DB tidak tersedia: %v", err)
	}

	cleanup := func() {
		_ = db.Exec(`DELETE FROM audit_logs WHERE aksi LIKE 'auth.%'`)
		_ = db.Exec(`DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'sa06_%')`)
		_ = db.Exec(`DELETE FROM users WHERE email LIKE 'sa06_%'`)
	}
	cleanup()

	authSvc := service.NewAuthService(
		db, cfg.JWT, clock.Real{},
		repository.NewUserRepo(),
		repository.NewTokenRepo(),
		repository.NewAuditRepo(),
	)
	deps := &handler.Dependencies{
		Config:      cfg,
		AuthService: authSvc,
		AuthHandler: handler.NewAuthHandler(authSvc, false, cfg.JWT.RefreshTTL),
	}
	return deps, cleanup
}

func createUser(t *testing.T, email, plain string, role domain.Role, active bool) {
	t.Helper()
	cfg, _ := config.Load()
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := password.Hash(plain)
	if err != nil {
		t.Fatal(err)
	}
	activeInt := 0
	if active {
		activeInt = 1
	}
	// Raw insert agar is_active=0 tidak hilang karena zero-value GORM.
	if err := db.Exec(`
		INSERT INTO users (name, email, password, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`, "SA06 Test", email, hash, string(role), activeInt).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAuthLoginRefreshLogout(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()

	email := "sa06_login@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)

	body, _ := json.Marshal(map[string]string{"email": email, "password": "rahasia123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: status=%d body=%s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}
	if loginResp.Data.AccessToken == "" || loginResp.Data.ExpiresIn != 900 {
		t.Fatalf("token/expires_in tidak valid: %+v", loginResp.Data)
	}

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == handler.RefreshCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil || refreshCookie.Value == "" {
		t.Fatal("refresh cookie wajib ada")
	}
	if refreshCookie.HttpOnly != true {
		t.Fatal("cookie harus HttpOnly")
	}

	// /me dengan bearer
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req2.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("me: %d %s", w2.Code, w2.Body.String())
	}

	// refresh
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req3.AddCookie(refreshCookie)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", w3.Code, w3.Body.String())
	}

	// reuse old refresh → invalid + revoke chain
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req4.AddCookie(refreshCookie)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("reuse harus 401, got %d", w4.Code)
	}
}

func TestAuthInactiveRejected(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()

	email := "sa06_inactive@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, false)

	r := handler.NewRouter(deps)
	body, _ := json.Marshal(map[string]string{"email": email, "password": "rahasia123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", w.Code, w.Body.String())
	}
}

func TestAuthWrongPassword(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()

	email := "sa06_wrong@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)
	body, _ := json.Marshal(map[string]string{"email": email, "password": "salah"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequirePermissionForbidden(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()

	email := "sa06_sales@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)
	body, _ := json.Marshal(map[string]string{"email": email, "password": "rahasia123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req2.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("sales ke /audit harus 403, got %d", w2.Code)
	}
}
