package integration_test

import (
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"app/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		cfg, err := config.Load()
		if err != nil {
			t.Skipf("config: %v", err)
		}
		dsn = cfg.DSN()
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetConnMaxLifetime(time.Minute)

	if err := db.Ping(); err != nil {
		t.Skipf("TEST_DB tidak tersedia: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedPelangganBarang(t *testing.T, db *sql.DB) (kodePelanggan, kodeBarang string, barangID, bmID int64) {
	t.Helper()
	kodePelanggan = "T0874"
	kodeBarang = "TSHS001"

	_, _ = pgExec(db, `DELETE FROM transaksi_detail WHERE kode_item = ?`, kodeBarang)
	_, _ = pgExec(db, `DELETE FROM transaksi_penjualan WHERE kode_pelanggan = ?`, kodePelanggan)
	_, _ = pgExec(db, `DELETE FROM stock_movements WHERE barang_id IN (SELECT id FROM barang WHERE kode_barang = ?)`, kodeBarang)
	_, _ = pgExec(db, `DELETE FROM barang_masuk WHERE barang_id IN (SELECT id FROM barang WHERE kode_barang = ?)`, kodeBarang)
	_, _ = pgExec(db, `DELETE FROM barang WHERE kode_barang = ?`, kodeBarang)
	_, _ = pgExec(db, `DELETE FROM pelanggan WHERE kode_pelanggan = ?`, kodePelanggan)

	_, err := pgExec(db, `
		INSERT INTO pelanggan
			(kode_pelanggan, nama_pelanggan, tgl_registrasi, phone, territory, distrik,
			 alamat_toko, provinsi, kabupaten, kecamatan, kelurahan, channel_outlet, is_active, created_at, updated_at)
		VALUES (?, 'Toko Melati', '2026-01-01', '08123456789', 'Jabar', 'Bandung',
		        'Jl. Melati', 'Jawa Barat', 'Bandung', 'Coblong', 'Dago', 'General Trade', TRUE, NOW(), NOW())
	`, kodePelanggan)
	if err != nil {
		t.Fatalf("seed pelanggan: %v", err)
	}

	barangID, err = insertID(db, `
		INSERT INTO barang (kode_barang, nama_item, brand, satuan, stok_tersedia, min_stock, is_active, created_at, updated_at)
		VALUES (?, 'Shisena 100ml', 'Nalpamara', 'PCS', 10, 2, TRUE, NOW(), NOW())
	`, kodeBarang)
	if err != nil {
		t.Fatalf("seed barang: %v", err)
	}

	bmID, err = insertID(db, `
		INSERT INTO barang_masuk
			(barang_id, no_faktur, no_batch, exp, tanggal_masuk, qty, harga, hpp, harga_mt, harga_gt, created_at, updated_at)
		VALUES (?, 'FK-1', 'B-001', '2027-01-01', '2026-01-01', 10, 20000, 18000, 25000, 24000, NOW(), NOW())
	`, barangID)
	if err != nil {
		t.Fatalf("seed barang_masuk: %v", err)
	}

	_, err = pgExec(db, `
		INSERT INTO stock_movements
			(barang_id, barang_masuk_id, movement_type, qty, saldo_setelah, reference_type, reference_id, created_at)
		VALUES (?, ?, 'PENERIMAAN', 10, 10, 'barang_masuk', ?, NOW())
	`, barangID, bmID, bmID)
	if err != nil {
		t.Fatalf("seed stock_movements: %v", err)
	}

	return kodePelanggan, kodeBarang, barangID, bmID
}

func TestConstraintDatabase(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	t.Run("CHECK konsistensi pembayaran menolak nilai tidak konsisten", func(t *testing.T) {
		kodePelanggan, _, _, _ := seedPelangganBarang(t, db)

		_, err := pgExec(db, `
			INSERT INTO transaksi_penjualan
				(tanggal, periode, kode_pelanggan, nama_pelanggan, alamat,
				 channel_outlet, area, total, ppn_nominal, total_akhir,
				 status_pembayaran, jumlah_dibayar, sisa_hutang, created_at, updated_at)
			VALUES ('2026-09-04', '2026-09', ?, 'Toko Melati', 'Jl. Melati',
			        'General Trade', 'Bandung', 100000, 11000, 111000,
			        'sebagian', 50000, 30000, NOW(), NOW())
		`, kodePelanggan)

		if err == nil {
			t.Fatal("database wajib menolak jumlah_dibayar + sisa_hutang <> total_akhir")
		}
	})

	t.Run("CHECK qty keluar menolak nilai tidak sesuai", func(t *testing.T) {
		kodePelanggan, kodeBarang, _, bmID := seedPelangganBarang(t, db)

		trxID, err := insertID(db, `
			INSERT INTO transaksi_penjualan
				(tanggal, periode, kode_pelanggan, nama_pelanggan, alamat,
				 channel_outlet, area, total, ppn_nominal, total_akhir,
				 status_pembayaran, jumlah_dibayar, sisa_hutang, created_at, updated_at)
			VALUES ('2026-09-04', '2026-09', ?, 'Toko Melati', 'Jl. Melati',
			        'General Trade', 'Bandung', 100000, 11000, 111000,
			        'hutang', 0, 111000, NOW(), NOW())
		`, kodePelanggan)
		if err != nil {
			t.Fatalf("insert transaksi: %v", err)
		}

		_, err = pgExec(db, `
			INSERT INTO transaksi_detail
				(transaksi_penjualan_id, urutan, kode_item, nama_item, satuan, barang_masuk_id,
				 qty, qty_promo, total_qty_keluar, harga, subtotal, total_after_disc, created_at, updated_at)
			VALUES (?, 1, ?, 'Shisena 100ml', 'PCS', ?, 10, 1, 11, 10000, 100000, 100000, NOW(), NOW())
		`, trxID, kodeBarang, bmID)
		if err != nil {
			t.Fatalf("insert detail valid: %v", err)
		}

		_, err = pgExec(db, `
			UPDATE transaksi_detail SET qty = 10, qty_promo = 1, total_qty_keluar = 15
			WHERE transaksi_penjualan_id = ?
		`, trxID)
		if err == nil {
			t.Fatal("CHECK total_qty_keluar wajib menolak nilai tidak sesuai")
		}
	})

	t.Run("barang_masuk yang punya pergerakan tidak bisa dihapus", func(t *testing.T) {
		_, _, _, bmID := seedPelangganBarang(t, db)

		_, err := pgExec(db, `DELETE FROM barang_masuk WHERE id = ?`, bmID)
		if err == nil {
			t.Fatal("ON DELETE RESTRICT wajib melindungi jejak stok")
		}
	})

	t.Run("CHECK qty barang_masuk menolak nol", func(t *testing.T) {
		_, _, barangID, _ := seedPelangganBarang(t, db)

		_, err := pgExec(db, `
			INSERT INTO barang_masuk
				(barang_id, no_faktur, no_batch, exp, tanggal_masuk, qty, harga, created_at, updated_at)
			VALUES (?, 'FK-0', 'B-0', '2027-01-01', '2026-01-01', 0, 1000, NOW(), NOW())
		`, barangID)
		if err == nil {
			t.Fatal("CHECK qty > 0 wajib menolak")
		}
	})

	t.Run("CHECK approved tanpa no_transaksi ditolak", func(t *testing.T) {
		kodePelanggan, _, _, _ := seedPelangganBarang(t, db)

		_, err := pgExec(db, `
			INSERT INTO transaksi_penjualan
				(tanggal, periode, kode_pelanggan, nama_pelanggan, alamat,
				 channel_outlet, area, total, ppn_nominal, total_akhir,
				 status_approval, no_transaksi, approved_at,
				 status_pembayaran, jumlah_dibayar, sisa_hutang, created_at, updated_at)
			VALUES ('2026-09-04', '2026-09', ?, 'Toko Melati', 'Jl. Melati',
			        'General Trade', 'Bandung', 100000, 11000, 111000,
			        'approved', NULL, NOW(),
			        'hutang', 0, 111000, NOW(), NOW())
		`, kodePelanggan)
		if err == nil {
			t.Fatal("CHECK approved wajib menuntut no_transaksi")
		}
	})
}

func toPG(query string) string {
	var b strings.Builder
	n := 1
	for _, r := range query {
		if r == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			n++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func pgExec(db *sql.DB, query string, args ...any) (sql.Result, error) {
	return db.Exec(toPG(query), args...)
}

func insertID(db *sql.DB, query string, args ...any) (int64, error) {
	var id int64
	err := db.QueryRow(toPG(query)+" RETURNING id", args...).Scan(&id)
	return id, err
}
