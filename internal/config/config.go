package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"app/internal/domain"
	"github.com/shopspring/decimal"
)

type Config struct {
	App        AppConfig
	DB         DBConfig
	JWT        JWTConfig
	CORS       CORSConfig
	Bisnis     BisnisConfig
	Perusahaan PerusahaanConfig
	Seed       SeedConfig
	Log        LogConfig
	Login      LoginConfig
}

type AppConfig struct {
	Env      string
	Port     string
	BaseURL  string
	Timezone string
}

type DBConfig struct {
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type BisnisConfig struct {
	PPNPersenDefault   decimal.Decimal
	FakturHalfPageMax  int
	FakturFullPageMax  int
	FakturItemsPerPage int
	StokMetodeAlokasi  domain.MetodeAlokasi
	FakturLockAktif    bool
}

type PerusahaanConfig struct {
	Nama    string
	Alamat  string
	Telepon string
	NPWP    string
}

type SeedConfig struct {
	SuperAdminEmail    string
	SuperAdminPassword string
}

type LogConfig struct {
	Level  string
	Format string
}

type LoginConfig struct {
	RateLimitPerMinute int
}

// Load membaca env, menerapkan default, dan gagal cepat bila konfigurasi
// wajib kosong di lingkungan produksi.
func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		App: AppConfig{
			Env:      getEnv("APP_ENV", "development"),
			Port:     getEnv("APP_PORT", "8080"),
			BaseURL:  getEnv("APP_BASE_URL", "http://localhost:8080"),
			Timezone: getEnv("APP_TIMEZONE", "Asia/Jakarta"),
		},
		DB: DBConfig{
			Host:         getEnv("DB_HOST", "127.0.0.1"),
			Port:         getEnv("DB_PORT", "3306"),
			Name:         getEnv("DB_NAME", "pkb"),
			User:         getEnv("DB_USER", "pkb_app"),
			Password:     os.Getenv("DB_PASSWORD"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 10),
		},
		JWT: JWTConfig{
			AccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
			RefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		},
		Bisnis: BisnisConfig{
			FakturHalfPageMax:  getEnvInt("FAKTUR_ITEMS_HALF_PAGE_MAX", 10),
			FakturFullPageMax:  getEnvInt("FAKTUR_ITEMS_FULL_PAGE_MAX", 30),
			FakturItemsPerPage: getEnvInt("FAKTUR_ITEMS_PER_PAGE", 30),
			FakturLockAktif:    getEnvBool("FAKTUR_LOCK_AKTIF", true),
		},
		Perusahaan: PerusahaanConfig{
			Nama:    os.Getenv("PERUSAHAAN_NAMA"),
			Alamat:  os.Getenv("PERUSAHAAN_ALAMAT"),
			Telepon: os.Getenv("PERUSAHAAN_TELEPON"),
			NPWP:    os.Getenv("PERUSAHAAN_NPWP"),
		},
		Seed: SeedConfig{
			SuperAdminEmail:    getEnv("SEED_SUPER_ADMIN_EMAIL", "admin@example.com"),
			SuperAdminPassword: os.Getenv("SEED_SUPER_ADMIN_PASSWORD"),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Login: LoginConfig{
			RateLimitPerMinute: getEnvInt("LOGIN_RATE_LIMIT", 6),
		},
	}

	lifetime, err := time.ParseDuration(getEnv("DB_CONN_MAX_LIFETIME", "30m"))
	if err != nil {
		return nil, fmt.Errorf("DB_CONN_MAX_LIFETIME: %w", err)
	}
	cfg.DB.ConnMaxLifetime = lifetime

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	cfg.JWT.AccessTTL = accessTTL

	refreshTTL, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}
	cfg.JWT.RefreshTTL = refreshTTL

	ppn, err := decimal.NewFromString(getEnv("PPN_PERSEN_DEFAULT", "11"))
	if err != nil {
		return nil, fmt.Errorf("PPN_PERSEN_DEFAULT: %w", err)
	}
	cfg.Bisnis.PPNPersenDefault = ppn

	metode := domain.MetodeAlokasi(strings.ToUpper(getEnv("STOK_METODE_ALOKASI_DEFAULT", "FEFO")))
	if !metode.Valid() {
		return nil, fmt.Errorf("STOK_METODE_ALOKASI_DEFAULT tidak valid: %s", metode)
	}
	cfg.Bisnis.StokMetodeAlokasi = metode

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.App.Env == "production" {
		if c.JWT.AccessSecret == "" {
			return fmt.Errorf("JWT_ACCESS_SECRET wajib di production")
		}
		if c.JWT.RefreshSecret == "" {
			return fmt.Errorf("JWT_REFRESH_SECRET wajib di production")
		}
		if c.DB.Password == "" {
			return fmt.Errorf("DB_PASSWORD wajib di production")
		}
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&charset=utf8mb4",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv mencari .env di cwd dan beberapa level di atas (penting untuk go test
// yang menjalankan binary dengan working directory = paket sumber).
func loadDotEnv() {
	candidates := []string{".env", "../.env", "../../.env", "../../../.env"}
	for _, p := range candidates {
		if err := godotenv.Load(p); err == nil {
			return
		}
	}
}
