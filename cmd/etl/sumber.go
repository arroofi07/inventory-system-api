package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

// DBConfig adalah parameter koneksi MySQL untuk sumber atau target.
type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func (c DBConfig) Label() string {
	return fmt.Sprintf("%s @ %s:%s", c.Name, c.Host, c.Port)
}

func (c DBConfig) dsn() string {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.DBName = c.Name
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Params = map[string]string{
		"charset": "utf8mb4",
	}
	return cfg.FormatDSN()
}

// Sumber adalah koneksi ke database Laravel lama.
// Default read-only (ETL). Mode tulis hanya lewat BukaSumberTulis (SF-02).
type Sumber struct {
	db         *sql.DB
	cfg        DBConfig
	bolehTulis bool
	mu         sync.Mutex
	kolom      map[string]map[string]bool
	tabel      map[string]bool
}

func bukaKoneksiMySQL(cfg DBConfig, maxOpen, maxIdle int) (*sql.DB, error) {
	if err := cfg.valid(); err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", cfg.dsn())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// BukaSumber membuka koneksi MySQL sumber dan mengunci sesi ke READ ONLY.
func BukaSumber(cfg DBConfig) (*Sumber, error) {
	db, err := bukaKoneksiMySQL(cfg, 10, 5)
	if err != nil {
		return nil, fmt.Errorf("sumber: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 09 §2 prinsip 1: ETL tidak pernah menulis ke sumber produksi.
	if _, err := db.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sumber set read only: %w", err)
	}
	var ro int
	if err := db.QueryRowContext(ctx, "SELECT @@session.transaction_read_only").Scan(&ro); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sumber cek read_only: %w", err)
	}
	if ro != 1 {
		_ = db.Close()
		return nil, errors.New("sumber: transaction_read_only tidak aktif")
	}

	return &Sumber{db: db, cfg: cfg, bolehTulis: false}, nil
}

// bolehTulisSumber true hanya bila env gerbang SF-02 disetel eksplisit.
func bolehTulisSumber() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ETL_SUMBER_BOLEH_TULIS")), "true")
}

// BukaSumberTulis membuka sumber tanpa READ ONLY. Hanya untuk salinan
// (bukan produksi) setelah ETL_SUMBER_BOLEH_TULIS=true.
func BukaSumberTulis(cfg DBConfig) (*Sumber, error) {
	if !bolehTulisSumber() {
		return nil, errors.New("menulis sumber ditolak: setel ETL_SUMBER_BOLEH_TULIS=true pada salinan, bukan produksi")
	}
	db, err := bukaKoneksiMySQL(cfg, 10, 5)
	if err != nil {
		return nil, fmt.Errorf("sumber tulis: %w", err)
	}
	return &Sumber{db: db, cfg: cfg, bolehTulis: true}, nil
}

func (s *Sumber) Label() string { return s.cfg.Label() }

func (s *Sumber) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Sumber) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Sumber) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *Sumber) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s == nil || !s.bolehTulis {
		return nil, errors.New("sumber: tulis ditolak (butuh --bersihkan-sumber + ETL_SUMBER_BOLEH_TULIS=true)")
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Sumber) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if s == nil || !s.bolehTulis {
		return nil, errors.New("sumber: tulis ditolak (butuh --bersihkan-sumber + ETL_SUMBER_BOLEH_TULIS=true)")
	}
	return s.db.BeginTx(ctx, opts)
}

// PunyaTabel true bila tabel ada di schema sumber.
func (s *Sumber) PunyaTabel(ctx context.Context, tabel string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tabel == nil {
		s.tabel = map[string]bool{}
		rows, err := s.db.QueryContext(ctx, `
			SELECT TABLE_NAME FROM information_schema.TABLES
			WHERE TABLE_SCHEMA = DATABASE()`)
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return false
			}
			s.tabel[strings.ToLower(n)] = true
		}
	}
	return s.tabel[strings.ToLower(tabel)]
}

// PunyaKolom true bila kolom ada di tabel sumber.
func (s *Sumber) PunyaKolom(ctx context.Context, tabel, kolom string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kolom == nil {
		s.kolom = map[string]map[string]bool{}
	}
	key := strings.ToLower(tabel)
	if _, ok := s.kolom[key]; !ok {
		s.kolom[key] = map[string]bool{}
		rows, err := s.db.QueryContext(ctx, `
			SELECT COLUMN_NAME FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, tabel)
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return false
			}
			s.kolom[key][strings.ToLower(n)] = true
		}
	}
	return s.kolom[key][strings.ToLower(kolom)]
}

// ExprKolom mengembalikan `col AS alias` atau fallback SQL bila kolom tidak ada.
func (s *Sumber) ExprKolom(ctx context.Context, tabel, kolom, alias, fallback string) string {
	if s.PunyaKolom(ctx, tabel, kolom) {
		return fmt.Sprintf("`%s` AS `%s`", kolom, alias)
	}
	return fmt.Sprintf("%s AS `%s`", fallback, alias)
}

// KolomPertama nama kolom pertama yang ada, atau "".
func (s *Sumber) KolomPertama(ctx context.Context, tabel string, kandidat ...string) string {
	for _, k := range kandidat {
		if s.PunyaKolom(ctx, tabel, k) {
			return k
		}
	}
	return ""
}

// Target adalah koneksi tulis ke database skema baru.
type Target struct {
	db  *sql.DB
	cfg DBConfig
}

// BukaTarget membuka koneksi MySQL target (read-write).
func BukaTarget(cfg DBConfig) (*Target, error) {
	if err := cfg.valid(); err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	db, err := sql.Open("mysql", cfg.dsn())
	if err != nil {
		return nil, fmt.Errorf("target open: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("target ping: %w", err)
	}
	return &Target{db: db, cfg: cfg}, nil
}

func (t *Target) Label() string { return t.cfg.Label() }

func (t *Target) Close() error {
	if t == nil || t.db == nil {
		return nil
	}
	return t.db.Close()
}

func (t *Target) DB() *sql.DB { return t.db }

func (t *Target) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.db.ExecContext(ctx, query, args...)
}

func (t *Target) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.db.QueryContext(ctx, query, args...)
}

func (t *Target) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.db.QueryRowContext(ctx, query, args...)
}

func (t *Target) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return t.db.BeginTx(ctx, opts)
}

// tabelETL dikosongkan untuk idempotensi (anak → induk). Tidak menyentuh
// schema_migrations / refresh_tokens / idempotency_keys.
var tabelETL = []string{
	"audit_logs",
	"stock_alerts",
	"price_change_logs",
	"riwayat_pembayaran",
	"transaksi_detail_promo",
	"transaksi_detail",
	"transaksi_penjualan",
	"promos",
	"stock_movements",
	"barang_masuk",
	"pelanggan",
	"barang",
	"users",
}

// TruncateETLTables mengosongkan tabel target agar ETL idempoten (09 §2 prinsip 2).
func (t *Target) TruncateETLTables(ctx context.Context) error {
	if _, err := t.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return fmt.Errorf("disable fk: %w", err)
	}
	defer func() {
		_, _ = t.db.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
	}()

	for _, tabel := range tabelETL {
		if _, err := t.db.ExecContext(ctx, "TRUNCATE TABLE `"+tabel+"`"); err != nil {
			return fmt.Errorf("truncate %s: %w", tabel, err)
		}
	}
	_, _ = t.db.ExecContext(ctx, "UPDATE no_transaksi_seq SET last_number = 0 WHERE id = 1")
	return nil
}

func (c DBConfig) valid() error {
	if c.Host == "" {
		return errors.New("host kosong")
	}
	if c.Port == "" {
		return errors.New("port kosong")
	}
	if c.Name == "" {
		return errors.New("nama database kosong")
	}
	if c.User == "" {
		return errors.New("user kosong")
	}
	return nil
}

// ETLConfig menampung sumber, target, dan direktori laporan.
type ETLConfig struct {
	Sumber    DBConfig
	Target    DBConfig
	ReportDir string
}

// LoadETLConfig membaca env sumber/target terpisah.
// Target default ke DB_* (database aplikasi baru).
// allowMissingSumber: true untuk dry-run / framework-only.
func LoadETLConfig(allowMissingSumber bool) (ETLConfig, error) {
	cfg := ETLConfig{
		Sumber: DBConfig{
			Host:     envOr("ETL_SUMBER_HOST", ""),
			Port:     envOr("ETL_SUMBER_PORT", "3306"),
			Name:     envOr("ETL_SUMBER_NAME", ""),
			User:     envOr("ETL_SUMBER_USER", ""),
			Password: os.Getenv("ETL_SUMBER_PASSWORD"),
		},
		Target: DBConfig{
			Host:     envOr("ETL_TARGET_HOST", envOr("DB_HOST", "127.0.0.1")),
			Port:     envOr("ETL_TARGET_PORT", envOr("DB_PORT", "3306")),
			Name:     envOr("ETL_TARGET_NAME", envOr("DB_NAME", "pkb")),
			User:     envOr("ETL_TARGET_USER", envOr("DB_USER", "pkb_app")),
			Password: firstNonEmpty(os.Getenv("ETL_TARGET_PASSWORD"), os.Getenv("DB_PASSWORD")),
		},
		ReportDir: envOr("ETL_REPORT_DIR", "artifacts/etl"),
	}

	sumberLengkap := cfg.Sumber.Host != "" && cfg.Sumber.Name != "" && cfg.Sumber.User != ""
	if !allowMissingSumber && !sumberLengkap {
		return cfg, errors.New(
			"ETL_SUMBER_HOST, ETL_SUMBER_NAME, ETL_SUMBER_USER wajib diisi (sumber Laravel read-only)",
		)
	}
	if sumberLengkap {
		if cfg.Sumber.Host == cfg.Target.Host &&
			cfg.Sumber.Port == cfg.Target.Port &&
			cfg.Sumber.Name == cfg.Target.Name {
			return cfg, errors.New("sumber dan target tidak boleh database yang sama")
		}
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
