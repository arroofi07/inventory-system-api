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
	"golang.org/x/crypto/bcrypt"
)

func setupAuthRouterTTL(t *testing.T, accessTTL time.Duration) (*handler.Dependencies, func()) {
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
	cfg.JWT.AccessTTL = accessTTL
	cfg.JWT.RefreshTTL = 7 * 24 * time.Hour
	cfg.Login.RateLimitPerMinute = 6
	cfg.DB = capTestPool(cfg.DB)

	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		t.Skipf("db: %v", err)
	}
	t.Cleanup(func() { closeGorm(db) })
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

func createUserWithHash(t *testing.T, email, hash string, role domain.Role, active bool) {
	t.Helper()
	cfg, _ := config.Load()
	db, err := repository.NewDB(capTestPool(cfg.DB))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGorm(db)
	activeInt := 0
	if active {
		activeInt = 1
	}
	if err := db.Exec(`
		INSERT INTO users (name, email, password, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`, "SA13 Laravel", email, hash, string(role), activeInt).Error; err != nil {
		t.Fatal(err)
	}
}

// SA-13: password hash Laravel $2y$ harus bisa login end-to-end.
func TestAuthLoginLaravelHash(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()

	email := "sa06_laravel@pkb.test"
	plain := "rahasia123"
	hash2a, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		t.Fatal(err)
	}
	laravel := "$2y$" + string(hash2a)[4:]
	if !password.Verifikasi(laravel, plain) {
		t.Fatal("fixture $2y$ tidak valid")
	}
	createUserWithHash(t, email, laravel, domain.RoleAdmin, true)

	r := handler.NewRouter(deps)
	body, _ := json.Marshal(map[string]string{"email": email, "password": plain})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login dengan hash Laravel: %d %s", w.Code, w.Body.String())
	}
}

// SA-13: access kedaluwarsa → refresh → akses pulih (TTL pendek).
func TestAuthExpiredAccessThenRefresh(t *testing.T) {
	deps, cleanup := setupAuthRouterTTL(t, 1*time.Second)
	defer cleanup()

	email := "sa06_ttl@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)
	body, _ := json.Marshal(map[string]string{"email": email, "password": "rahasia123"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	if loginResp.Data.ExpiresIn != 1 {
		t.Fatalf("expires_in=%d want 1", loginResp.Data.ExpiresIn)
	}

	var refreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == handler.RefreshCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("refresh cookie hilang")
	}

	time.Sleep(1100 * time.Millisecond)

	wMe := httptest.NewRecorder()
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	r.ServeHTTP(wMe, reqMe)
	if wMe.Code != http.StatusUnauthorized {
		t.Fatalf("access kedaluwarsa harus 401, got %d", wMe.Code)
	}

	wRef := httptest.NewRecorder()
	reqRef := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reqRef.AddCookie(refreshCookie)
	r.ServeHTTP(wRef, reqRef)
	if wRef.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", wRef.Code, wRef.Body.String())
	}
	var refResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wRef.Body.Bytes(), &refResp)
	if refResp.Data.AccessToken == "" || refResp.Data.AccessToken == loginResp.Data.AccessToken {
		t.Fatal("harus dapat access token baru")
	}

	wOk := httptest.NewRecorder()
	reqOk := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqOk.Header.Set("Authorization", "Bearer "+refResp.Data.AccessToken)
	r.ServeHTTP(wOk, reqOk)
	if wOk.Code != http.StatusOK {
		t.Fatalf("me setelah refresh: %d %s", wOk.Code, wOk.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	deps, cleanup := setupAuthRouter(t)
	defer cleanup()
	deps.Config.Login.RateLimitPerMinute = 3

	email := "sa06_ratelimit@pkb.test"
	createUser(t, email, "rahasia123", domain.RoleSales, true)

	r := handler.NewRouter(deps)
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]string{"email": email, "password": "salah"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.66.66.66:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("percobaan %d: want 401 got %d %s", i+1, w.Code, w.Body.String())
		}
	}

	body, _ := json.Marshal(map[string]string{"email": email, "password": "salah"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.66.66.66:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit: want 429 got %d %s", w.Code, w.Body.String())
	}
}
