package repository

import (
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"gorm.io/gorm"
)

// LaporanRepo query laporan stok & barang keluar (SE-07/08).
type LaporanRepo struct{}

func NewLaporanRepo() *LaporanRepo { return &LaporanRepo{} }

// LaporanStokRow hasil join agregat stok.
type LaporanStokRow struct {
	ID               uint64
	KodeBarang       string
	NamaItem         string
	Brand            string
	Satuan           string
	StokTersedia     int
	MinStock         int
	TotalMasuk       int
	TotalKeluar      int
	JumlahBatch      int
	BatchTerdekatExp *time.Time
	NilaiStokHPP     string
}

func (r *LaporanRepo) ListStok(db *gorm.DB, q dto.LaporanStokQuery) ([]LaporanStokRow, int64, error) {
	q.Normalize()
	base := `
FROM barang b
LEFT JOIN (
  SELECT barang_id,
         COALESCE(SUM(CASE WHEN qty > 0 THEN qty ELSE 0 END),0) AS qty_batch,
         COUNT(CASE WHEN qty > 0 THEN 1 END) AS jumlah_batch,
         MIN(CASE WHEN qty > 0 THEN exp END) AS batch_terdekat_exp,
         COALESCE(SUM(CASE WHEN qty > 0 THEN qty * hpp ELSE 0 END),0) AS nilai_hpp
  FROM barang_masuk
  GROUP BY barang_id
) bm ON bm.barang_id = b.id
LEFT JOIN (
  SELECT barang_id,
         COALESCE(SUM(CASE WHEN qty > 0 THEN qty ELSE 0 END),0) AS total_masuk,
         COALESCE(SUM(CASE WHEN qty < 0 THEN -qty ELSE 0 END),0) AS total_keluar
  FROM stock_movements
  GROUP BY barang_id
) sm ON sm.barang_id = b.id
WHERE b.is_active = 1`
	args := []any{}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		base += ` AND (b.kode_barang LIKE ? OR b.nama_item LIKE ? OR b.brand LIKE ?)`
		args = append(args, like, like, like)
	}
	if strings.TrimSpace(q.Brand) != "" {
		base += ` AND b.brand = ?`
		args = append(args, strings.TrimSpace(q.Brand))
	}
	switch strings.ToUpper(strings.TrimSpace(q.StatusStok)) {
	case "HABIS":
		base += ` AND b.stok_tersedia <= 0`
	case "RENDAH":
		base += ` AND b.stok_tersedia > 0 AND b.stok_tersedia <= b.min_stock`
	case "NORMAL":
		base += ` AND b.stok_tersedia > b.min_stock`
	}

	var total int64
	if err := db.Raw(`SELECT COUNT(*) `+base, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	selectSQL := `
SELECT b.id, b.kode_barang, b.nama_item, b.brand, b.satuan, b.stok_tersedia, b.min_stock,
       COALESCE(sm.total_masuk,0) AS total_masuk,
       COALESCE(sm.total_keluar,0) AS total_keluar,
       COALESCE(bm.jumlah_batch,0) AS jumlah_batch,
       bm.batch_terdekat_exp AS batch_terdekat_exp,
       CAST(COALESCE(bm.nilai_hpp,0) AS CHAR) AS nilai_stok_hpp
` + base + ` ORDER BY b.kode_barang ASC LIMIT ? OFFSET ?`
	argsPage := append(append([]any{}, args...), q.PerPage, q.Offset())
	var rows []LaporanStokRow
	if err := db.Raw(selectSQL, argsPage...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *LaporanRepo) RingkasanStok(db *gorm.DB, q dto.LaporanStokQuery) (dto.LaporanStokRingkasan, error) {
	where := `WHERE is_active = 1`
	args := []any{}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		where += ` AND (kode_barang LIKE ? OR nama_item LIKE ? OR brand LIKE ?)`
		args = append(args, like, like, like)
	}
	if strings.TrimSpace(q.Brand) != "" {
		where += ` AND brand = ?`
		args = append(args, strings.TrimSpace(q.Brand))
	}
	type cnt struct {
		Normal int
		Rendah int
		Habis  int
	}
	var c cnt
	err := db.Raw(fmt.Sprintf(`
SELECT
  SUM(CASE WHEN stok_tersedia > min_stock THEN 1 ELSE 0 END) AS normal,
  SUM(CASE WHEN stok_tersedia > 0 AND stok_tersedia <= min_stock THEN 1 ELSE 0 END) AS rendah,
  SUM(CASE WHEN stok_tersedia <= 0 THEN 1 ELSE 0 END) AS habis
FROM barang %s`, where), args...).Scan(&c).Error
	return dto.LaporanStokRingkasan{Normal: c.Normal, Rendah: c.Rendah, Habis: c.Habis}, err
}

// BarangKeluarRow baris mentah detail+header sebelum alokasi.
type BarangKeluarRow struct {
	DetailID             uint64
	TransaksiID          uint64
	NoTransaksi          *string
	Tanggal              time.Time
	KodePelanggan        string
	NamaPelanggan        string
	Alamat               string
	ChannelOutlet        string
	Area                 string
	SalesID              *uint64
	NamaSales            *string
	Urutan               uint16
	KodeItem             string
	NamaItem             string
	Brand                string
	BatchNumber          *string
	ExpiryDate           *time.Time
	Qty                  int
	QtyPromo             int
	TotalQtyKeluar       int
	Harga                string
	Disc1                string
	Disc2                string
	Disc3                string
	TotalAfterDisc       string
	HPPSnapshot          string
	HeaderTotal          string
	HeaderPPN            string
}

func (r *LaporanRepo) ListBarangKeluar(db *gorm.DB, q dto.LaporanBarangKeluarQuery) ([]BarangKeluarRow, int64, error) {
	q.Normalize()
	base := `
FROM transaksi_detail d
INNER JOIN transaksi_penjualan t ON t.id = d.transaksi_penjualan_id
LEFT JOIN users u ON u.id = t.sales_id
LEFT JOIN barang b ON b.kode_barang = d.kode_item
WHERE t.status_approval = 'approved'`
	args := []any{}
	if q.DateFrom != "" {
		base += ` AND t.tanggal >= ?`
		args = append(args, q.DateFrom)
	}
	if q.DateTo != "" {
		base += ` AND t.tanggal <= ?`
		args = append(args, q.DateTo)
	}
	if strings.TrimSpace(q.KodePelanggan) != "" {
		base += ` AND t.kode_pelanggan = ?`
		args = append(args, strings.TrimSpace(q.KodePelanggan))
	}
	if strings.TrimSpace(q.KodeItem) != "" {
		base += ` AND d.kode_item = ?`
		args = append(args, strings.TrimSpace(q.KodeItem))
	}
	if q.SalesID != nil {
		base += ` AND t.sales_id = ?`
		args = append(args, *q.SalesID)
	}
	if strings.TrimSpace(q.Brand) != "" {
		base += ` AND b.brand = ?`
		args = append(args, strings.TrimSpace(q.Brand))
	}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		base += ` AND (t.no_transaksi LIKE ? OR t.nama_pelanggan LIKE ? OR d.kode_item LIKE ? OR d.nama_item LIKE ?)`
		args = append(args, like, like, like, like)
	}

	var total int64
	if err := db.Raw(`SELECT COUNT(*) `+base, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	selectSQL := `
SELECT d.id AS detail_id, t.id AS transaksi_id, t.no_transaksi, t.tanggal,
       t.kode_pelanggan, t.nama_pelanggan, t.alamat, t.channel_outlet, t.area,
       t.sales_id, u.name AS nama_sales,
       d.urutan, d.kode_item, d.nama_item, COALESCE(b.brand,'') AS brand,
       d.batch_number, d.expiry_date,
       d.qty, d.qty_promo, d.total_qty_keluar,
       CAST(d.harga AS CHAR) AS harga,
       CAST(d.disc1_persen AS CHAR) AS disc1,
       CAST(d.disc2_persen AS CHAR) AS disc2,
       CAST(d.disc3_persen AS CHAR) AS disc3,
       CAST(d.total_after_disc AS CHAR) AS total_after_disc,
       CAST(d.hpp_snapshot AS CHAR) AS hpp_snapshot,
       CAST(t.total AS CHAR) AS header_total,
       CAST(t.ppn_nominal AS CHAR) AS header_ppn
` + base + ` ORDER BY t.tanggal DESC, t.id DESC, d.urutan ASC LIMIT ? OFFSET ?`
	argsPage := append(append([]any{}, args...), q.PerPage, q.Offset())
	var rows []BarangKeluarRow
	if err := db.Raw(selectSQL, argsPage...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DetailsForAlokasi memuat detail satu transaksi untuk AlokasiProporsionalFaktur.
func (r *LaporanRepo) DetailsForAlokasi(db *gorm.DB, transaksiIDs []uint64) (map[uint64][]domain.TransaksiDetail, map[uint64]domain.TransaksiPenjualan, error) {
	out := map[uint64][]domain.TransaksiDetail{}
	headers := map[uint64]domain.TransaksiPenjualan{}
	if len(transaksiIDs) == 0 {
		return out, headers, nil
	}
	var hdrs []domain.TransaksiPenjualan
	if err := db.Where("id IN ?", transaksiIDs).Find(&hdrs).Error; err != nil {
		return nil, nil, err
	}
	for _, h := range hdrs {
		headers[h.ID] = h
	}
	var details []domain.TransaksiDetail
	if err := db.Where("transaksi_penjualan_id IN ?", transaksiIDs).
		Order("transaksi_penjualan_id ASC, urutan ASC, id ASC").
		Find(&details).Error; err != nil {
		return nil, nil, err
	}
	for _, d := range details {
		out[d.TransaksiPenjualanID] = append(out[d.TransaksiPenjualanID], d)
	}
	return out, headers, nil
}

func (r *LaporanRepo) filterApproved(dateFrom, dateTo string) (string, []any) {
	where := `t.status_approval = 'approved'`
	args := []any{}
	if dateFrom != "" {
		where += ` AND t.tanggal >= ?`
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		where += ` AND t.tanggal <= ?`
		args = append(args, dateTo)
	}
	return where, args
}

type channelAggRow struct {
	ChannelOutlet   string
	JumlahTransaksi int
	TotalPenjualan  string
	TotalQty        int
	JumlahOutlet    int
}

func (r *LaporanRepo) AgregatPerChannel(db *gorm.DB, dateFrom, dateTo string) ([]channelAggRow, error) {
	where, args := r.filterApproved(dateFrom, dateTo)
	sql := `
SELECT t.channel_outlet,
       COUNT(DISTINCT t.id) AS jumlah_transaksi,
       CAST(COALESCE(SUM(t.total_akhir),0) AS CHAR) AS total_penjualan,
       COALESCE(SUM(q.qty),0) AS total_qty,
       COUNT(DISTINCT t.kode_pelanggan) AS jumlah_outlet
FROM transaksi_penjualan t
LEFT JOIN (
  SELECT transaksi_penjualan_id, SUM(total_qty_keluar) AS qty
  FROM transaksi_detail GROUP BY transaksi_penjualan_id
) q ON q.transaksi_penjualan_id = t.id
WHERE ` + where + `
GROUP BY t.channel_outlet
ORDER BY SUM(t.total_akhir) DESC`
	var rows []channelAggRow
	err := db.Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

type territoryAggRow struct {
	Territory       string
	JumlahTransaksi int
	TotalPenjualan  string
	TotalQty        int
	JumlahOutlet    int
}

func (r *LaporanRepo) AgregatPerTerritory(db *gorm.DB, dateFrom, dateTo string) ([]territoryAggRow, error) {
	where, args := r.filterApproved(dateFrom, dateTo)
	sql := `
SELECT COALESCE(NULLIF(p.territory,''), NULLIF(t.area,''), '(tanpa territory)') AS territory,
       COUNT(DISTINCT t.id) AS jumlah_transaksi,
       CAST(COALESCE(SUM(t.total_akhir),0) AS CHAR) AS total_penjualan,
       COALESCE(SUM(q.qty),0) AS total_qty,
       COUNT(DISTINCT t.kode_pelanggan) AS jumlah_outlet
FROM transaksi_penjualan t
LEFT JOIN pelanggan p ON p.kode_pelanggan = t.kode_pelanggan
LEFT JOIN (
  SELECT transaksi_penjualan_id, SUM(total_qty_keluar) AS qty
  FROM transaksi_detail GROUP BY transaksi_penjualan_id
) q ON q.transaksi_penjualan_id = t.id
WHERE ` + where + `
GROUP BY COALESCE(NULLIF(p.territory,''), NULLIF(t.area,''), '(tanpa territory)')
ORDER BY SUM(t.total_akhir) DESC`
	var rows []territoryAggRow
	err := db.Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

type produkTerlarisRow struct {
	KodeItem        string
	NamaItem        string
	TotalQty        int
	TotalQtyKeluar  int
	JumlahTransaksi int
	TotalAfterDisc  string
}

func (r *LaporanRepo) ProdukTerlaris(db *gorm.DB, dateFrom, dateTo string, limit int) ([]produkTerlarisRow, error) {
	where, args := r.filterApproved(dateFrom, dateTo)
	sql := `
SELECT d.kode_item, d.nama_item,
       COALESCE(SUM(d.qty),0) AS total_qty,
       COALESCE(SUM(d.total_qty_keluar),0) AS total_qty_keluar,
       COUNT(DISTINCT d.transaksi_penjualan_id) AS jumlah_transaksi,
       CAST(COALESCE(SUM(d.total_after_disc),0) AS CHAR) AS total_after_disc
FROM transaksi_detail d
INNER JOIN transaksi_penjualan t ON t.id = d.transaksi_penjualan_id
WHERE ` + where + `
GROUP BY d.kode_item, d.nama_item
ORDER BY SUM(d.qty) DESC
LIMIT ?`
	args = append(args, limit)
	var rows []produkTerlarisRow
	err := db.Raw(sql, args...).Scan(&rows).Error
	return rows, err
}

type trenHarianRow struct {
	Tanggal         time.Time
	JumlahTransaksi int
	TotalPenjualan  string
}

func (r *LaporanRepo) TrenHarian(db *gorm.DB, dateFrom, dateTo string) ([]trenHarianRow, error) {
	where, args := r.filterApproved(dateFrom, dateTo)
	sql := `
SELECT t.tanggal,
       COUNT(DISTINCT t.id) AS jumlah_transaksi,
       CAST(COALESCE(SUM(t.total_akhir),0) AS CHAR) AS total_penjualan
FROM transaksi_penjualan t
WHERE ` + where + `
GROUP BY t.tanggal
ORDER BY t.tanggal ASC`
	var rows []trenHarianRow
	err := db.Raw(sql, args...).Scan(&rows).Error
	return rows, err
}
