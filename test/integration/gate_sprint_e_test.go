package integration_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/domain"
)

// TestGateSprintEF6 — checklist ringkas Sprint E (SE-12).
func TestGateSprintEF6(t *testing.T) {
	r, tokSales, tokSA, _, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, _, _ = seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 4)

	// Channel analytics tertaut (admin)
	wCA := httptest.NewRecorder()
	reqCA := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/channel-analytics", nil)
	reqCA.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wCA, reqCA)
	if wCA.Code != http.StatusOK {
		t.Fatalf("channel-analytics: %d %s", wCA.Code, wCA.Body.String())
	}

	// Afiliasi dilarang analytics
	emailAf := "se12_af_" + time.Now().Format("150405") + "@pkb.test"
	createUser(t, emailAf, "rahasia123", domain.RoleAfiliasi, true)
	tokAf := loginToken(t, r, emailAf, "rahasia123")
	wAf := httptest.NewRecorder()
	reqAf := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/channel-analytics", nil)
	reqAf.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wAf, reqAf)
	if wAf.Code != http.StatusForbidden {
		t.Fatalf("afiliasi analytics: %d", wAf.Code)
	}

	// Ekspor afiliasi barang keluar tersaring
	wExp := httptest.NewRecorder()
	reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/barang-keluar/export", nil)
	reqExp.Header.Set("Authorization", "Bearer "+tokAf)
	r.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK {
		t.Fatalf("export afiliasi: %d %s", wExp.Code, wExp.Body.String())
	}
	csv := wExp.Body.String()
	for _, forbidden := range []string{"hpp_snapshot", "provit", "alamat"} {
		if strings.Contains(csv, forbidden) {
			t.Fatalf("afiliasi export mengandung %s", forbidden)
		}
	}

	// Dashboard SA hidup
	wDash := httptest.NewRecorder()
	reqDash := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	reqDash.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wDash, reqDash)
	if wDash.Code != http.StatusOK {
		t.Fatalf("dashboard: %d", wDash.Code)
	}

	// Laporan stok + penjualan
	for _, path := range []string{"/api/v1/laporan/stok", "/api/v1/laporan/penjualan"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tokSA)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
