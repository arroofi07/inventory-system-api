package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/pkg/password"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	// DemoPasswordDefault dipakai bila SEED_DEMO_PASSWORD kosong.
	DemoPasswordDefault = "rahasia123"
	demoEmailAdmin      = "demo.admin@pkb.test"
	demoEmailAfiliasi   = "demo.afiliasi@pkb.test"
	demoEmailSales      = "demo.sales@pkb.test"
	demoBarangSentinel  = "DEMO-SHS001"
)

// ErrDemoProduction menandai percobaan seed demo di lingkungan produksi.
var ErrDemoProduction = fmt.Errorf("seed --demo dilarang bila APP_ENV=production")

// SeedDemoHasil ringkasan isi demo.
type SeedDemoHasil struct {
	Users      int
	Barang     int
	Pelanggan  int
	Promo      int
	Transaksi  int
	Pembayaran int
	Alert      int
}

// BolehSeedDemo menolak seed demo di production (11 §5.5).
func BolehSeedDemo(appEnv string) error {
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return ErrDemoProduction
	}
	return nil
}

// DemoSudahAda true bila sentinel barang demo sudah ada.
func DemoSudahAda(db *gorm.DB) (bool, error) {
	var n int64
	err := db.Model(&domain.Barang{}).Where("kode_barang = ?", demoBarangSentinel).Count(&n).Error
	return n > 0, err
}

// HapusDataDemo menghapus baris bertanda DEMO- / demo.* (urutan FK).
func HapusDataDemo(db *gorm.DB) error {
	stmts := []string{
		`DELETE tdp FROM transaksi_detail_promo tdp
			INNER JOIN transaksi_detail td ON td.id = tdp.transaksi_detail_id
			INNER JOIN transaksi_penjualan tp ON tp.id = td.transaksi_penjualan_id
			WHERE tp.kode_pelanggan LIKE 'DEMO-%'`,
		`DELETE td FROM transaksi_detail td
			INNER JOIN transaksi_penjualan tp ON tp.id = td.transaksi_penjualan_id
			WHERE tp.kode_pelanggan LIKE 'DEMO-%'`,
		`DELETE rp FROM riwayat_pembayaran rp
			INNER JOIN transaksi_penjualan tp ON tp.id = rp.transaksi_penjualan_id
			WHERE tp.kode_pelanggan LIKE 'DEMO-%'`,
		`DELETE FROM transaksi_penjualan WHERE kode_pelanggan LIKE 'DEMO-%'`,
		`DELETE sm FROM stock_movements sm
			INNER JOIN barang b ON b.id = sm.barang_id
			WHERE b.kode_barang LIKE 'DEMO-%'`,
		`DELETE pcl FROM price_change_logs pcl
			INNER JOIN barang b ON b.id = pcl.barang_id
			WHERE b.kode_barang LIKE 'DEMO-%'`,
		`DELETE sa FROM stock_alerts sa
			INNER JOIN barang b ON b.id = sa.barang_id
			WHERE b.kode_barang LIKE 'DEMO-%'`,
		`DELETE bm FROM barang_masuk bm
			INNER JOIN barang b ON b.id = bm.barang_id
			WHERE b.kode_barang LIKE 'DEMO-%'`,
		`DELETE FROM promos WHERE kode_promo LIKE 'DEMO-%'`,
		`DELETE FROM pelanggan WHERE kode_pelanggan LIKE 'DEMO-%'`,
		`DELETE FROM barang WHERE kode_barang LIKE 'DEMO-%'`,
		`DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'demo.%@pkb.test')`,
		`DELETE FROM idempotency_keys WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'demo.%@pkb.test')`,
		`DELETE FROM users WHERE email LIKE 'demo.%@pkb.test'`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("hapus demo: %w", err)
		}
	}
	return nil
}

type hargaSKU struct {
	MT, GT string
}

type demoSeeder struct {
	db      *gorm.DB
	cfg     *config.Config
	pass    string
	now     time.Time
	meta    AuditMeta
	admin   *domain.User
	sales   *domain.User
	harga   map[string]hargaSKU
	bmID    map[string]uint64
	barang  map[string]uint64
	hasil   SeedDemoHasil
	userR   *repository.UserRepo
	bmSvc   *BarangMasukService
	brgSvc  *BarangService
	plSvc   *PelangganService
	promoS  *PromoService
	trxSvc  *TransaksiService
	bayarS  *PembayaranService
	fakturS *FakturService
}

// SeedDemo mengisi data contoh untuk seluruh modul. Idempoten hanya lewat HapusDataDemo dulu.
func SeedDemo(ctx context.Context, db *gorm.DB, cfg *config.Config, demoPassword string) (*SeedDemoHasil, error) {
	if err := BolehSeedDemo(cfg.App.Env); err != nil {
		return nil, err
	}
	if strings.TrimSpace(demoPassword) == "" {
		demoPassword = DemoPasswordDefault
	}

	clk := clock.Real{}
	audit := repository.NewAuditRepo()
	barangR := repository.NewBarangRepo()
	bmR := repository.NewBarangMasukRepo()
	stockR := repository.NewStockRepo()
	hargaR := repository.NewPriceChangeRepo()
	plR := repository.NewPelangganRepo()
	promoR := repository.NewPromoRepo()
	trxR := repository.NewTransaksiRepo()
	bayarR := repository.NewPembayaranRepo()
	userR := repository.NewUserRepo()

	s := &demoSeeder{
		db:      db,
		cfg:     cfg,
		pass:    demoPassword,
		now:     clk.Now(),
		harga:   map[string]hargaSKU{},
		bmID:    map[string]uint64{},
		barang:  map[string]uint64{},
		userR:   userR,
		bmSvc:   NewBarangMasukService(db, bmR, barangR, stockR, hargaR, audit, cfg.Bisnis.PPNPersenDefault),
		brgSvc:  NewBarangService(db, barangR, bmR, stockR, hargaR, audit, clk, cfg.Bisnis.PPNPersenDefault),
		plSvc:   NewPelangganService(db, plR, audit),
		promoS:  NewPromoService(db, promoR, barangR, audit, clk),
		trxSvc:  NewTransaksiService(db, plR, barangR, stockR, promoR, trxR, bayarR, audit, clk, cfg.Bisnis.PPNPersenDefault),
		bayarS:  NewPembayaranService(db, trxR, bayarR, repository.NewIdempotencyRepo(), audit, clk),
		fakturS: NewFakturService(db, trxR, userR, audit, clk, cfg.Bisnis, cfg.Perusahaan),
	}

	if err := s.users(ctx); err != nil {
		return nil, err
	}
	s.meta = AuditMeta{UserID: &s.admin.ID}
	if err := s.penerimaan(ctx); err != nil {
		return nil, err
	}
	if err := s.pelanggan(ctx); err != nil {
		return nil, err
	}
	if err := s.promo(ctx); err != nil {
		return nil, err
	}
	if err := s.transaksi(ctx); err != nil {
		return nil, err
	}
	if err := s.alertStok(); err != nil {
		return nil, err
	}
	return &s.hasil, nil
}

func (s *demoSeeder) ymd(d time.Time) string { return d.Format("2006-01-02") }

func (s *demoSeeder) users(ctx context.Context) error {
	type row struct {
		name, email, role, hp, ktp string
	}
	akun := []row{
		{"Admin Demo", demoEmailAdmin, string(domain.RoleAdmin), "081211110001", "3273010101010001"},
		{"Sales Demo", demoEmailSales, string(domain.RoleSales), "081211110002", "3273010101010002"},
		{"Afiliasi Demo", demoEmailAfiliasi, string(domain.RoleAfiliasi), "081211110003", "3273010101010003"},
	}
	hash, err := password.Hash(s.pass)
	if err != nil {
		return err
	}
	for _, a := range akun {
		hp, ktp := a.hp, a.ktp
		u := &domain.User{
			Name:     a.name,
			Email:    a.email,
			Password: hash,
			Role:     domain.Role(a.role),
			NoHP:     &hp,
			NoKTP:    &ktp,
			IsActive: true,
		}
		if err := s.userR.CreateMapped(s.db.WithContext(ctx), u); err != nil {
			return fmt.Errorf("user %s: %w", a.email, err)
		}
		s.hasil.Users++
		switch a.email {
		case demoEmailAdmin:
			s.admin = u
		case demoEmailSales:
			s.sales = u
		}
	}
	return nil
}

func (s *demoSeeder) penerimaan(ctx context.Context) error {
	tglMasuk := s.ymd(s.now.AddDate(0, -2, 0))
	expFar := s.ymd(s.now.AddDate(1, 6, 0))
	expMid := s.ymd(s.now.AddDate(0, 10, 0))
	expSoon := s.ymd(s.now.AddDate(0, 0, 18))

	type batch struct {
		baru                           bool
		kode, nama, brand, faktur, lot string
		qty, minStok, expAlert         int
		exp, harga, d1, d2, d3         string
		mt, gt                         string
	}
	batches := []batch{
		{true, "DEMO-SHS001", "Mustika Ratu Shampoo 200ml", "Mustika Ratu", "DEMO-F-001", "LOT-SHS-A", 80, 10, 30, expMid, "20000.00", "23.10", "2.00", "5.00", "25.00", "15.00"},
		{false, "DEMO-SHS001", "", "", "DEMO-F-002", "LOT-SHS-B", 40, 0, 0, expFar, "20000.00", "23.10", "2.00", "5.00", "25.00", "15.00"},
		{true, "DEMO-NAL001", "Nalpamara Cream 50g", "Nalpamara", "DEMO-F-003", "LOT-NAL-A", 60, 15, 30, expFar, "35000.00", "10.00", "0.00", "0.00", "20.00", "12.00"},
		{true, "DEMO-PUT001", "Puteri Body Lotion 100ml", "Puteri", "DEMO-F-004", "LOT-PUT-A", 45, 12, 30, expFar, "18000.00", "5.00", "0.00", "0.00", "22.00", "14.00"},
		{true, "DEMO-LOW01", "Serum Stok Rendah 30ml", "Nalpamara", "DEMO-F-005", "LOT-LOW-A", 8, 50, 30, expFar, "120000.00", "0.00", "0.00", "0.00", "30.00", "20.00"},
		{true, "DEMO-OUT01", "Toner Stok Habis 100ml", "Puteri", "DEMO-F-006", "LOT-OUT-A", 6, 5, 30, expFar, "25000.00", "0.00", "0.00", "0.00", "18.00", "10.00"},
		{true, "DEMO-EXP01", "Masker Batch Dekat Exp", "Mustika Ratu", "DEMO-F-007", "LOT-EXP-A", 25, 8, 30, expSoon, "15000.00", "0.00", "0.00", "0.00", "20.00", "12.00"},
		{true, "DEMO-OFF01", "SKU Nonaktif (arsip)", "Mustika Ratu", "DEMO-F-008", "LOT-OFF-A", 10, 0, 30, expFar, "9000.00", "0.00", "0.00", "0.00", "10.00", "8.00"},
	}

	seen := map[string]bool{}
	for _, b := range batches {
		req := dto.BarangMasukCreateRequest{
			KodeBarang:      b.kode,
			BuatBarangBaru:  b.baru,
			NamaItem:        b.nama,
			Brand:           b.brand,
			Satuan:          "PCS",
			MinStock:        b.minStok,
			ExpiryAlertDays: b.expAlert,
			MetodeAlokasi:   "FEFO",
			NoFaktur:        b.faktur,
			NoBatch:         b.lot,
			Exp:             b.exp,
			TanggalMasuk:    tglMasuk,
			Qty:             b.qty,
			Harga:           b.harga,
			DiscHPP1:        b.d1,
			DiscHPP2:        b.d2,
			DiscHPP3:        b.d3,
			MarkupMTType:    "percent",
			MarkupMTAmount:  b.mt,
			MarkupGTType:    "percent",
			MarkupGTAmount:  b.gt,
		}
		out, err := s.bmSvc.Buat(ctx, req, s.meta)
		if err != nil {
			return fmt.Errorf("barang masuk %s/%s: %w", b.kode, b.lot, err)
		}
		s.bmID[b.lot] = out.ID
		s.barang[b.kode] = out.BarangID
		if !seen[b.kode] {
			s.harga[b.kode] = hargaSKU{MT: out.HargaMT, GT: out.HargaGT}
			s.hasil.Barang++
			seen[b.kode] = true
		}
	}

	offID := s.barang["DEMO-OFF01"]
	if _, err := s.brgSvc.SetStatus(ctx, offID, false, s.meta); err != nil {
		return fmt.Errorf("nonaktif DEMO-OFF01: %w", err)
	}
	return nil
}

func (s *demoSeeder) pelanggan(ctx context.Context) error {
	tgl := s.ymd(s.now.AddDate(0, -6, 0))
	type plg struct {
		kode, nama, channel, territory, distrik, alamat, kab, kec, kel string
		kredit                                                         string
		nonaktif                                                       bool
	}
	list := []plg{
		{"DEMO-GT01", "Toko Maju Jaya", string(domain.ChannelGeneralTrade), "Bandung", "Timur", "Jl. Gatot Subroto 12", "Bandung", "Cibiru", "Cipadung", "5000000.00", false},
		{"DEMO-MT01", "Superindo Cabang Dago", string(domain.ChannelModernTrade), "Jakarta", "Selatan", "Jl. Sudirman 88", "Jakarta Selatan", "Setiabudi", "Kuningan", "25000000.00", false},
		{"DEMO-SA01", "Sub Agen Melati", string(domain.ChannelSubAgen), "Bekasi", "Barat", "Jl. Ahmad Yani 5", "Bekasi", "Bekasi Barat", "Bintara", "8000000.00", false},
		{"DEMO-GK01", "Grosir Kosmetik Sari", string(domain.ChannelGeneralTradeKosmetik), "Bandung", "Utara", "Jl. Cihampelas 3", "Bandung", "Cidadap", "Ledeng", "3000000.00", false},
		{"DEMO-MI01", "Mini Market Independent", string(domain.ChannelModernTradeIndependent), "Bandung", "Barat", "Jl. Pasteur 9", "Bandung", "Sukajadi", "Pasteur", "4000000.00", false},
		{"DEMO-OFFP", "Toko Tutup (nonaktif)", string(domain.ChannelGeneralTrade), "Bandung", "Selatan", "Jl. Buah Batu 1", "Bandung", "Lengkong", "Turangga", "0.00", true},
	}
	for i, p := range list {
		kode := p.kode
		kredit := p.kredit
		req := dto.PelangganCreateRequest{
			KodePelanggan:       &kode,
			NamaPelanggan:       p.nama,
			TglRegistrasi:       tgl,
			Phone:               fmt.Sprintf("0813222200%02d", i+1),
			Territory:           p.territory,
			Distrik:             p.distrik,
			AlamatToko:          p.alamat,
			Provinsi:            "Jawa Barat",
			Kabupaten:           p.kab,
			Kecamatan:           p.kec,
			Kelurahan:           p.kel,
			ChannelOutlet:       p.channel,
			EstimasiBatasKredit: &kredit,
		}
		out, err := s.plSvc.Buat(ctx, req, s.meta)
		if err != nil {
			return fmt.Errorf("pelanggan %s: %w", p.kode, err)
		}
		s.hasil.Pelanggan++
		if p.nonaktif {
			if _, err := s.plSvc.SetStatus(ctx, out.ID, false, s.meta); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *demoSeeder) promo(ctx context.Context) error {
	mulai := s.ymd(s.now.AddDate(0, -1, 0))
	akhir := s.ymd(s.now.AddDate(0, 6, 0))
	laluMulai := s.ymd(s.now.AddDate(0, -8, 0))
	laluAkhir := s.ymd(s.now.AddDate(0, -6, 0))
	shs := "DEMO-SHS001"
	buy, get := 12, 1
	bonus := 1
	minQty := 10
	pct := "5.00"
	fix := "5000.00"

	type prow struct {
		kode, nama, tipe string
		req              dto.PromoCreateRequest
	}
	list := []dto.PromoCreateRequest{
		{
			KodePromo: strPtr("DEMO-BXGY"), NamaPromo: "Beli 12 gratis 1 Shampoo",
			TipePromo: "buy_x_get_y", BuyQty: &buy, GetQty: &get, KodeBarang: &shs,
			TanggalMulai: mulai, TanggalBerakhir: akhir,
		},
		{
			KodePromo: strPtr("DEMO-BONUS"), NamaPromo: "Bonus 1 pcs min 10",
			TipePromo: "bonus_qty", BonusQty: &bonus, MinQty: &minQty, KodeBarang: &shs,
			TanggalMulai: mulai, TanggalBerakhir: akhir,
		},
		{
			KodePromo: strPtr("DEMO-PCT"), NamaPromo: "Diskon 5% semua SKU",
			TipePromo: "percentage_discount", DiscountPercentage: &pct,
			TanggalMulai: mulai, TanggalBerakhir: akhir,
		},
		{
			KodePromo: strPtr("DEMO-FIX"), NamaPromo: "Potongan Rp 5.000",
			TipePromo: "fixed_discount", DiscountAmount: &fix,
			TanggalMulai: mulai, TanggalBerakhir: akhir,
		},
		{
			KodePromo: strPtr("DEMO-OLD"), NamaPromo: "Promo kedaluwarsa (contoh)",
			TipePromo: "percentage_discount", DiscountPercentage: &pct,
			TanggalMulai: laluMulai, TanggalBerakhir: laluAkhir,
		},
	}
	for _, req := range list {
		if _, err := s.promoS.Buat(ctx, req, s.meta); err != nil {
			kode := ""
			if req.KodePromo != nil {
				kode = *req.KodePromo
			}
			return fmt.Errorf("promo %s: %w", kode, err)
		}
		s.hasil.Promo++
	}
	return nil
}

func (s *demoSeeder) transaksi(ctx context.Context) error {
	today := s.ymd(s.now)
	lalu := s.ymd(s.now.AddDate(0, 0, -40))
	jtMendatang := s.ymd(s.now.AddDate(0, 1, 0))
	jtLewat := s.ymd(s.now.AddDate(0, 0, -10))

	buat := func(kodePlg, tgl string, items []dto.TransaksiItemRequest, jt *string, disc1 string) (*dto.TransaksiResponse, error) {
		req := dto.TransaksiCreateRequest{
			KodePelanggan:     kodePlg,
			Tanggal:           tgl,
			Area:              "Bandung Timur",
			Items:             items,
			PPNPersen:         "11.00",
			NominalDibayar:    "0.00",
			TanggalJatuhTempo: jt,
			Disc1Persen:       disc1,
		}
		return s.trxSvc.Buat(ctx, req, s.sales.ID, s.meta)
	}
	item := func(kode string, qty int, channel string, promos ...string) dto.TransaksiItemRequest {
		h := s.harga[kode].GT
		if strings.Contains(strings.ToLower(channel), "modern trade") {
			h = s.harga[kode].MT
		}
		return dto.TransaksiItemRequest{KodeItem: kode, Qty: qty, Harga: h, KodePromos: promos}
	}

	// 1. Pending — antrian approval.
	if _, err := buat("DEMO-GT01", today, []dto.TransaksiItemRequest{
		item("DEMO-SHS001", 5, "gt"),
	}, &jtMendatang, ""); err != nil {
		return fmt.Errorf("trx pending: %w", err)
	}
	s.hasil.Transaksi++

	// 2. Approved hutang.
	hutang, err := buat("DEMO-GT01", today, []dto.TransaksiItemRequest{
		item("DEMO-NAL001", 4, "gt"),
	}, &jtMendatang, "")
	if err != nil {
		return fmt.Errorf("trx hutang: %w", err)
	}
	if _, err := s.trxSvc.Approve(ctx, hutang.ID, dto.ApproveRequest{}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("approve hutang: %w", err)
	}
	s.hasil.Transaksi++

	// 3. Approved sebagian + bayar 40%.
	sebagian, err := buat("DEMO-MT01", today, []dto.TransaksiItemRequest{
		item("DEMO-PUT001", 6, "modern trade"),
		item("DEMO-NAL001", 2, "modern trade"),
	}, &jtMendatang, "5.00")
	if err != nil {
		return fmt.Errorf("trx sebagian: %w", err)
	}
	if _, err := s.trxSvc.Approve(ctx, sebagian.ID, dto.ApproveRequest{ApprovalNotes: "stok cukup"}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("approve sebagian: %w", err)
	}
	total, err := decimal.NewFromString(sebagian.TotalAkhir)
	if err != nil {
		return err
	}
	nominal := total.Mul(decimal.NewFromInt(2)).Div(decimal.NewFromInt(5)).Round(2)
	if _, _, err := s.bayarS.Catat(ctx, sebagian.ID, dto.PembayaranCreateRequest{
		NominalPembayaran: nominal.StringFixed(2),
		MetodePembayaran:  "Transfer BCA",
		Keterangan:        "cicilan pertama (demo)",
	}, s.admin.ID, "", nil, s.meta); err != nil {
		return fmt.Errorf("bayar sebagian: %w", err)
	}
	s.hasil.Transaksi++
	s.hasil.Pembayaran++

	// 4. Approved lunas + kunci faktur.
	lunas, err := buat("DEMO-SA01", today, []dto.TransaksiItemRequest{
		item("DEMO-SHS001", 12, "gt", "DEMO-BXGY"),
	}, &jtMendatang, "")
	if err != nil {
		return fmt.Errorf("trx lunas: %w", err)
	}
	if _, err := s.trxSvc.Approve(ctx, lunas.ID, dto.ApproveRequest{}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("approve lunas: %w", err)
	}
	if _, _, err := s.bayarS.Catat(ctx, lunas.ID, dto.PembayaranCreateRequest{
		NominalPembayaran: lunas.TotalAkhir,
		MetodePembayaran:  "Tunai",
		Keterangan:        "pelunasan (demo)",
	}, s.admin.ID, "", nil, s.meta); err != nil {
		return fmt.Errorf("bayar lunas: %w", err)
	}
	if _, err := s.fakturS.Ambil(ctx, lunas.ID, s.admin.ID, domain.RoleAdmin, s.meta); err != nil {
		return fmt.Errorf("kunci faktur: %w", err)
	}
	s.hasil.Transaksi++
	s.hasil.Pembayaran++

	// 5. Rejected.
	tolak, err := buat("DEMO-GK01", today, []dto.TransaksiItemRequest{
		item("DEMO-PUT001", 2, "gt"),
	}, &jtMendatang, "")
	if err != nil {
		return fmt.Errorf("trx tolak: %w", err)
	}
	if _, err := s.trxSvc.Reject(ctx, tolak.ID, dto.RejectRequest{ApprovalNotes: "qty tidak sesuai PO (demo)"}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	s.hasil.Transaksi++

	// 6. Overdue (tanggal & JT di masa lalu).
	overdue, err := buat("DEMO-GT01", lalu, []dto.TransaksiItemRequest{
		item("DEMO-SHS001", 3, "gt"),
	}, &jtLewat, "")
	if err != nil {
		return fmt.Errorf("trx overdue: %w", err)
	}
	if _, err := s.trxSvc.Approve(ctx, overdue.ID, dto.ApproveRequest{}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("approve overdue: %w", err)
	}
	s.hasil.Transaksi++

	// 7. Habiskan DEMO-OUT01 agar status HABIS.
	habis, err := buat("DEMO-MI01", today, []dto.TransaksiItemRequest{
		item("DEMO-OUT01", 6, "modern trade"),
	}, &jtMendatang, "")
	if err != nil {
		return fmt.Errorf("trx habis: %w", err)
	}
	if _, err := s.trxSvc.Approve(ctx, habis.ID, dto.ApproveRequest{}, s.admin.ID, s.meta); err != nil {
		return fmt.Errorf("approve habis: %w", err)
	}
	s.hasil.Transaksi++

	return nil
}

func (s *demoSeeder) alertStok() error {
	type al struct {
		kode, lot    string
		tipe         domain.AlertType
		sev          domain.AlertSeverity
		judul, pesan string
		stok, ambang int
		denganBatch  bool
	}
	exp, _ := time.Parse("2006-01-02", s.ymd(s.now.AddDate(0, 0, 18)))
	expD := datatypes.Date(exp)
	rows := []al{
		{"DEMO-LOW01", "LOT-LOW-A", domain.AlertStokRendah, domain.SeverityTinggi, "Stok rendah Serum", "Sisa 8, di bawah min 50 (demo).", 8, 50, false},
		{"DEMO-OUT01", "LOT-OUT-A", domain.AlertStokHabis, domain.SeverityKritis, "Stok habis Toner", "Saldo 0 setelah penjualan demo.", 0, 5, false},
		{"DEMO-EXP01", "LOT-EXP-A", domain.AlertMendekatiExp, domain.SeveritySedang, "Batch dekat kedaluwarsa", "Masker LOT-EXP-A kedaluwarsa dalam < 30 hari (demo).", 25, 30, true},
	}
	for _, a := range rows {
		row := domain.StockAlert{
			BarangID:    s.barang[a.kode],
			AlertType:   a.tipe,
			Severity:    a.sev,
			Judul:       a.judul,
			Pesan:       a.pesan,
			StokSaatItu: intPtr(a.stok),
			NilaiAmbang: intPtr(a.ambang),
		}
		if a.denganBatch {
			id := s.bmID[a.lot]
			row.BarangMasukID = &id
			row.TanggalExp = &expD
		}
		if err := s.db.Create(&row).Error; err != nil {
			return fmt.Errorf("stock_alerts %s: %w", a.kode, err)
		}
		s.hasil.Alert++
	}
	return nil
}

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }
