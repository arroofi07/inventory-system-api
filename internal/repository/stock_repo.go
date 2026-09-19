package repository

import (
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"gorm.io/gorm"
)

type StockRepo struct{}

func NewStockRepo() *StockRepo { return &StockRepo{} }

// BatchSaldo baris penerimaan + saldo ledger per batch.
type BatchSaldo struct {
	domain.BarangMasuk
	QtyTersedia int
}

// CatatPenerimaan menulis movement PENERIMAAN dan menaikkan stok_tersedia.
// Caller harus sudah mengunci baris barang (FOR UPDATE).
func (r *StockRepo) CatatPenerimaan(
	db *gorm.DB,
	barang *domain.Barang,
	bm *domain.BarangMasuk,
	userID *uint64,
) error {
	saldo := barang.StokTersedia + bm.Qty
	bmID := bm.ID
	refType := "barang_masuk"
	refNo := bm.NoFaktur
	mv := domain.StockMovement{
		BarangID:      barang.ID,
		BarangMasukID: &bmID,
		MovementType:  domain.MovementPenerimaan,
		Qty:           bm.Qty,
		SaldoSetelah:  saldo,
		ReferenceType: &refType,
		ReferenceID:   &bmID,
		ReferenceNo:   &refNo,
		CreatedBy:     userID,
		CreatedAt:     time.Now(),
	}
	if err := db.Create(&mv).Error; err != nil {
		return err
	}
	return db.Model(barang).Update("stok_tersedia", saldo).Error
}

// QtyTersediaBatch = qty penerimaan + SUM movement non-PENERIMAAN untuk batch.
func (r *StockRepo) QtyTersediaBatch(db *gorm.DB, bm *domain.BarangMasuk) (int, error) {
	var delta int64
	err := db.Model(&domain.StockMovement{}).
		Select("COALESCE(SUM(qty), 0)").
		Where("barang_masuk_id = ? AND movement_type <> ?", bm.ID, domain.MovementPenerimaan).
		Scan(&delta).Error
	if err != nil {
		return 0, err
	}
	return bm.Qty + int(delta), nil
}

// ListBatchSaldo semua batch SKU dengan qty_tersedia (boleh 0 / kedaluwarsa).
func (r *StockRepo) ListBatchSaldo(db *gorm.DB, barangID uint64) ([]BatchSaldo, error) {
	var rows []domain.BarangMasuk
	if err := db.Where("barang_id = ?", barangID).
		Order("tanggal_masuk DESC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]BatchSaldo, 0, len(rows))
	for i := range rows {
		qty, err := r.QtyTersediaBatch(db, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, BatchSaldo{BarangMasuk: rows[i], QtyTersedia: qty})
	}
	return out, nil
}

// AdaMovementSelainPenerimaan true bila batch sudah dipakai penjualan/dll.
func (r *StockRepo) AdaMovementSelainPenerimaan(db *gorm.DB, barangMasukID uint64) (bool, error) {
	var n int64
	err := db.Model(&domain.StockMovement{}).
		Where("barang_masuk_id = ? AND movement_type <> ?", barangMasukID, domain.MovementPenerimaan).
		Limit(1).
		Count(&n).Error
	return n > 0, err
}

// HapusPenerimaan menghapus movement PENERIMAAN batch dan menurunkan stok.
func (r *StockRepo) HapusPenerimaan(db *gorm.DB, barang *domain.Barang, bm *domain.BarangMasuk) error {
	ada, err := r.AdaMovementSelainPenerimaan(db, bm.ID)
	if err != nil {
		return err
	}
	if ada {
		return domain.ErrStatusTidakValid
	}
	if err := db.Where("barang_masuk_id = ? AND movement_type = ?", bm.ID, domain.MovementPenerimaan).
		Delete(&domain.StockMovement{}).Error; err != nil {
		return err
	}
	saldo := barang.StokTersedia - bm.Qty
	if saldo < 0 {
		return domain.ErrStokTidakCukup
	}
	return db.Model(barang).Update("stok_tersedia", saldo).Error
}

// SesuaikanQtyPenerimaan mengubah qty movement PENERIMAAN dan stok.
func (r *StockRepo) SesuaikanQtyPenerimaan(
	db *gorm.DB,
	barang *domain.Barang,
	bm *domain.BarangMasuk,
	qtyLama, qtyBaru int,
) error {
	ada, err := r.AdaMovementSelainPenerimaan(db, bm.ID)
	if err != nil {
		return err
	}
	if ada {
		return domain.ErrStatusTidakValid
	}
	delta := qtyBaru - qtyLama
	saldo := barang.StokTersedia + delta
	if saldo < 0 {
		return domain.ErrStokTidakCukup
	}
	if err := db.Model(&domain.StockMovement{}).
		Where("barang_masuk_id = ? AND movement_type = ?", bm.ID, domain.MovementPenerimaan).
		Updates(map[string]any{"qty": qtyBaru, "saldo_setelah": saldo}).Error; err != nil {
		return err
	}
	return db.Model(barang).Update("stok_tersedia", saldo).Error
}

// CatatPenjualan menulis movement PENJUALAN (qty negatif) dan menurunkan stok_tersedia.
func (r *StockRepo) CatatPenjualan(
	db *gorm.DB,
	barang *domain.Barang,
	barangMasukID uint64,
	qtyKeluar int,
	refID uint64,
	refNo string,
	userID *uint64,
) (saldoSetelah int, err error) {
	if qtyKeluar <= 0 {
		return barang.StokTersedia, nil
	}
	saldo := barang.StokTersedia - qtyKeluar
	if saldo < 0 {
		return 0, domain.ErrStokTidakCukup
	}
	bmID := barangMasukID
	refType := "transaksi_penjualan"
	mv := domain.StockMovement{
		BarangID:      barang.ID,
		BarangMasukID: &bmID,
		MovementType:  domain.MovementPenjualan,
		Qty:           -qtyKeluar,
		SaldoSetelah:  saldo,
		ReferenceType: &refType,
		ReferenceID:   &refID,
		ReferenceNo:   &refNo,
		CreatedBy:     userID,
		CreatedAt:     time.Now(),
	}
	if err := db.Create(&mv).Error; err != nil {
		return 0, err
	}
	if err := db.Model(barang).Update("stok_tersedia", saldo).Error; err != nil {
		return 0, err
	}
	barang.StokTersedia = saldo
	return saldo, nil
}

// CatatPenyesuaian menulis PENYESUAIAN (qty boleh +/−) dan update stok_tersedia.
func (r *StockRepo) CatatPenyesuaian(
	db *gorm.DB,
	barang *domain.Barang,
	qty int,
	barangMasukID *uint64,
	alasan string,
	userID *uint64,
) (*domain.StockMovement, error) {
	saldo := barang.StokTersedia + qty
	if saldo < 0 {
		return nil, domain.ErrStokTidakCukup
	}
	refType := "penyesuaian_manual"
	ket := alasan
	mv := domain.StockMovement{
		BarangID:      barang.ID,
		BarangMasukID: barangMasukID,
		MovementType:  domain.MovementPenyesuaian,
		Qty:           qty,
		SaldoSetelah:  saldo,
		ReferenceType: &refType,
		Keterangan:    &ket,
		CreatedBy:     userID,
		CreatedAt:     time.Now(),
	}
	if err := db.Create(&mv).Error; err != nil {
		return nil, err
	}
	if err := db.Model(barang).Update("stok_tersedia", saldo).Error; err != nil {
		return nil, err
	}
	barang.StokTersedia = saldo
	return &mv, nil
}

// PergerakanListHasil hasil list + ringkasan kartu stok.
type PergerakanListHasil struct {
	Items     []PergerakanBaris
	Total     int64
	Ringkasan dto.PergerakanStokRingkasan
}

// PergerakanBaris baris join untuk list.
type PergerakanBaris struct {
	ID            uint64
	CreatedAt     time.Time
	MovementType  string
	NoBatch       *string
	Qty           int
	SaldoSetelah  int
	ReferenceType *string
	ReferenceNo   *string
	Keterangan    *string
	Oleh          *string
}

// ListPergerakan daftar ledger per SKU (SC-13).
func (r *StockRepo) ListPergerakan(db *gorm.DB, barangID uint64, q dto.PergerakanStokListQuery) (*PergerakanListHasil, error) {
	q.Normalize()
	if q.PerPage > 100 {
		q.PerPage = 100
	}

	base := db.Table("stock_movements sm").
		Joins("LEFT JOIN barang_masuk bm ON bm.id = sm.barang_masuk_id").
		Joins("LEFT JOIN users u ON u.id = sm.created_by").
		Where("sm.barang_id = ?", barangID)

	if q.MovementType != "" {
		base = base.Where("sm.movement_type = ?", q.MovementType)
	}
	if q.DateFrom != "" {
		base = base.Where("sm.created_at >= ?", q.DateFrom+" 00:00:00")
	}
	if q.DateTo != "" {
		base = base.Where("sm.created_at <= ?", q.DateTo+" 23:59:59.999")
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []PergerakanBaris
	err := base.Select(`
		sm.id AS id,
		sm.created_at AS created_at,
		sm.movement_type AS movement_type,
		bm.no_batch AS no_batch,
		sm.qty AS qty,
		sm.saldo_setelah AS saldo_setelah,
		sm.reference_type AS reference_type,
		sm.reference_no AS reference_no,
		sm.keterangan AS keterangan,
		u.name AS oleh
	`).
		Order("sm.created_at DESC, sm.id DESC").
		Offset(q.Offset()).Limit(q.PerPage).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}

	ringkas, err := r.ringkasanPergerakan(db, barangID, q)
	if err != nil {
		return nil, err
	}
	return &PergerakanListHasil{Items: items, Total: total, Ringkasan: ringkas}, nil
}

func (r *StockRepo) ringkasanPergerakan(db *gorm.DB, barangID uint64, q dto.PergerakanStokListQuery) (dto.PergerakanStokRingkasan, error) {
	var out dto.PergerakanStokRingkasan
	tx := db.Table("stock_movements").Where("barang_id = ?", barangID)
	if q.MovementType != "" {
		tx = tx.Where("movement_type = ?", q.MovementType)
	}
	if q.DateFrom != "" {
		tx = tx.Where("created_at >= ?", q.DateFrom+" 00:00:00")
	}
	if q.DateTo != "" {
		tx = tx.Where("created_at <= ?", q.DateTo+" 23:59:59.999")
	}

	type agg struct {
		Masuk  int
		Keluar int
	}
	var a agg
	if err := tx.Select(`
		COALESCE(SUM(CASE WHEN qty > 0 THEN qty ELSE 0 END), 0) AS masuk,
		COALESCE(SUM(CASE WHEN qty < 0 THEN -qty ELSE 0 END), 0) AS keluar
	`).Scan(&a).Error; err != nil {
		return out, err
	}
	out.TotalMasuk = a.Masuk
	out.TotalKeluar = a.Keluar

	// Saldo awal = saldo_setelah baris terakhir sebelum date_from (bila ada filter).
	if q.DateFrom != "" {
		var saldoAwal *int
		err := db.Raw(`
			SELECT saldo_setelah FROM stock_movements
			WHERE barang_id = ? AND created_at < ?
			ORDER BY created_at DESC, id DESC LIMIT 1
		`, barangID, q.DateFrom+" 00:00:00").Scan(&saldoAwal).Error
		if err != nil {
			return out, err
		}
		if saldoAwal != nil {
			out.SaldoAwal = *saldoAwal
		}
	}
	out.SaldoAkhir = out.SaldoAwal + out.TotalMasuk - out.TotalKeluar
	if q.DateFrom == "" && q.DateTo == "" && q.MovementType == "" {
		var stok int
		if err := db.Model(&domain.Barang{}).Select("stok_tersedia").Where("id = ?", barangID).Scan(&stok).Error; err != nil {
			return out, err
		}
		out.SaldoAkhir = stok
		out.SaldoAwal = stok - out.TotalMasuk + out.TotalKeluar
	}
	return out, nil
}

// SumQtyPerBarang SUM(qty) ledger per barang_id.
func (r *StockRepo) SumQtyPerBarang(db *gorm.DB, barangID uint64) (int, error) {
	var sum int
	err := db.Model(&domain.StockMovement{}).
		Select("COALESCE(SUM(qty), 0)").
		Where("barang_id = ?", barangID).
		Scan(&sum).Error
	return sum, err
}

// RekonsiliasiSelisih mendeteksi SKU dengan SUM(ledger) ≠ stok_tersedia.
func (r *StockRepo) RekonsiliasiSelisih(db *gorm.DB) ([]dto.RekonsiliasiBaris, int, error) {
	type row struct {
		BarangID     uint64
		KodeBarang   string
		NamaItem     string
		StokTersedia int
		SaldoLedger  int
	}
	var rows []row
	err := db.Raw(`
		SELECT b.id AS barang_id, b.kode_barang, b.nama_item, b.stok_tersedia,
		       COALESCE(SUM(sm.qty), 0) AS saldo_ledger
		FROM barang b
		LEFT JOIN stock_movements sm ON sm.barang_id = b.id
		GROUP BY b.id, b.kode_barang, b.nama_item, b.stok_tersedia
	`).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]dto.RekonsiliasiBaris, 0)
	for _, rrow := range rows {
		if rrow.StokTersedia != rrow.SaldoLedger {
			out = append(out, dto.RekonsiliasiBaris{
				BarangID: rrow.BarangID, KodeBarang: rrow.KodeBarang, NamaItem: rrow.NamaItem,
				StokTersedia: rrow.StokTersedia, SaldoLedger: rrow.SaldoLedger,
				Selisih: rrow.StokTersedia - rrow.SaldoLedger,
			})
		}
	}
	return out, len(rows), nil
}
