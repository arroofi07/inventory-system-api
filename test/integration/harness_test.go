package integration_test

import (
	"testing"

	"app/internal/config"
	"app/internal/repository"
	"gorm.io/gorm"
)

func closeGorm(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}

func capTestPool(cfg config.DBConfig) config.DBConfig {
	if cfg.MaxOpenConns > 8 {
		cfg.MaxOpenConns = 8
	}
	if cfg.MaxIdleConns > 2 {
		cfg.MaxIdleConns = 2
	}
	return cfg
}

func openGormCfg(t *testing.T, cfg config.DBConfig) *gorm.DB {
	t.Helper()
	db, err := repository.NewDB(capTestPool(cfg))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { closeGorm(db) })
	return db
}

func ptrStr(v *string) string {
	if v == nil {
		return "<nil>"
	}
	return *v
}
