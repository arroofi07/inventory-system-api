package config_test

import (
	"testing"

	"app/internal/config"
	"app/internal/domain"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("JWT_REFRESH_SECRET", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("PPN_PERSEN_DEFAULT", "11")
	t.Setenv("STOK_METODE_ALOKASI_DEFAULT", "FEFO")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != "8080" {
		t.Fatalf("port = %s", cfg.App.Port)
	}
	if cfg.Bisnis.StokMetodeAlokasi != domain.AlokasiFEFO {
		t.Fatalf("metode = %s", cfg.Bisnis.StokMetodeAlokasi)
	}
	if cfg.Bisnis.PPNPersenDefault.String() != "11" {
		t.Fatalf("ppn = %s", cfg.Bisnis.PPNPersenDefault)
	}
}

func TestLoadRejectsInvalidMetode(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STOK_METODE_ALOKASI_DEFAULT", "XYZ")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProductionRequiresSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("JWT_REFRESH_SECRET", "x")
	t.Setenv("DB_PASSWORD", "x")
	t.Setenv("STOK_METODE_ALOKASI_DEFAULT", "FEFO")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected JWT_ACCESS_SECRET error")
	}
}
