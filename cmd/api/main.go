package main

import (
	"fmt"
	"log"

	"app/internal/config"
	"app/internal/handler"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"app/internal/service"
)

// @title                      sistem-barang API
// @version                    1.0
// @description                API sistem distribusi FMCG PKB: inventory berbasis batch,
// @description                penjualan dengan approval stok, promo, faktur, dan piutang.
// @description
// @description                Semua nilai uang dan persentase dikirim sebagai string desimal,
// @description                bukan number, agar presisi tidak hilang saat diproses JavaScript.
// @description
// @description                Istilah domain memakai bahasa Indonesia.

// @contact.name               Tim Engineering PKB
// @contact.email              dev@pkb.example.com

// @license.name               Proprietary

// @host                       localhost:8082
// @BasePath                   /api/v1
// @schemes                    http https

// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Access token JWT. Format: "Bearer <token>"

// @tag.name                   Auth
// @tag.description            Autentikasi dan pengelolaan sesi
// @tag.name                   Transaksi
// @tag.description            Pratinjau, simpan pending, add-items, approval, dan detail penjualan
// @tag.name                   Pembayaran
// @tag.description            Catat nominal dan riwayat pembayaran transaksi
// @tag.name                   Piutang
// @tag.description            Daftar piutang, aging, overdue, notifikasi, penerimaan
// @tag.name                   Faktur
// @tag.description            Data faktur siap cetak dan kunci sekali cetak
// @tag.name                   Dashboard
// @tag.description            Ringkasan metrik per role
// @tag.name                   Stok
// @tag.description            Ledger pergerakan stok dan penyesuaian
// @tag.name                   Laporan
// @tag.description            Laporan dan ekspor CSV
// @tag.name                   Sistem
// @tag.description            Health check dan utilitas
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	userRepo := repository.NewUserRepo()
	tokenRepo := repository.NewTokenRepo()
	auditRepo := repository.NewAuditRepo()
	barangRepo := repository.NewBarangRepo()
	barangMasukRepo := repository.NewBarangMasukRepo()
	pelangganRepo := repository.NewPelangganRepo()
	stockRepo := repository.NewStockRepo()
	priceRepo := repository.NewPriceChangeRepo()
	authSvc := service.NewAuthService(db, cfg.JWT, clock.Real{}, userRepo, tokenRepo, auditRepo)
	authHandler := handler.NewAuthHandler(authSvc, cfg.App.Env == "production", cfg.JWT.RefreshTTL)
	barangSvc := service.NewBarangService(
		db, barangRepo, barangMasukRepo, stockRepo, priceRepo, auditRepo, clock.Real{}, cfg.Bisnis.PPNPersenDefault,
	)
	barangHandler := handler.NewBarangHandler(barangSvc)
	barangMasukSvc := service.NewBarangMasukService(
		db, barangMasukRepo, barangRepo, stockRepo, priceRepo, auditRepo, cfg.Bisnis.PPNPersenDefault,
	)
	exportSvc := service.NewExportService(db, barangMasukRepo, pelangganRepo, repository.NewTransaksiRepo())
	pelangganSvc := service.NewPelangganService(db, pelangganRepo, auditRepo)
	importSvc := service.NewImportService(db, barangMasukSvc, pelangganSvc)
	barangMasukHandler := handler.NewBarangMasukHandler(barangMasukSvc, exportSvc, importSvc)
	pelangganHandler := handler.NewPelangganHandler(pelangganSvc, exportSvc, importSvc)
	userSvc := service.NewUserService(db, userRepo, tokenRepo, auditRepo, clock.Real{})
	userHandler := handler.NewUserHandler(userSvc)
	promoRepo := repository.NewPromoRepo()
	promoSvc := service.NewPromoService(db, promoRepo, barangRepo, auditRepo, clock.Real{})
	promoHandler := handler.NewPromoHandler(promoSvc)
	transaksiSvc := service.NewTransaksiService(
		db,
		pelangganRepo,
		barangRepo,
		stockRepo,
		promoRepo,
		repository.NewTransaksiRepo(),
		repository.NewPembayaranRepo(),
		auditRepo,
		clock.Real{},
		cfg.Bisnis.PPNPersenDefault,
	)
	transaksiHandler := handler.NewTransaksiHandler(transaksiSvc)
	bayarSvc := service.NewPembayaranService(
		db,
		repository.NewTransaksiRepo(),
		repository.NewPembayaranRepo(),
		repository.NewIdempotencyRepo(),
		auditRepo,
		clock.Real{},
	)
	pembayaranHandler := handler.NewPembayaranHandler(bayarSvc)
	piutangSvc := service.NewPiutangService(db, repository.NewTransaksiRepo(), repository.NewPembayaranRepo(), clock.Real{})
	piutangHandler := handler.NewPiutangHandler(piutangSvc)
	fakturSvc := service.NewFakturService(
		db,
		repository.NewTransaksiRepo(),
		userRepo,
		auditRepo,
		clock.Real{},
		cfg.Bisnis,
		cfg.Perusahaan,
	)
	fakturHandler := handler.NewFakturHandler(fakturSvc)
	dashboardSvc := service.NewDashboardService(db, repository.NewTransaksiRepo(), clock.Real{})
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc)
	stockSvc := service.NewStockService(db, barangRepo, stockRepo, auditRepo, clock.Real{})
	stokHandler := handler.NewStokHandler(stockSvc)
	laporanSvc := service.NewLaporanService(db, repository.NewLaporanRepo())
	exportJobs := service.NewExportJobStore()
	laporanHandler := handler.NewLaporanHandler(exportSvc, laporanSvc, exportJobs)

	r := handler.NewRouter(&handler.Dependencies{
		Config:             cfg,
		AuthService:        authSvc,
		AuthHandler:        authHandler,
		BarangHandler:      barangHandler,
		BarangMasukHandler: barangMasukHandler,
		PelangganHandler:   pelangganHandler,
		UserHandler:        userHandler,
		PromoHandler:       promoHandler,
		TransaksiHandler:   transaksiHandler,
		PembayaranHandler:  pembayaranHandler,
		PiutangHandler:     piutangHandler,
		FakturHandler:      fakturHandler,
		DashboardHandler:   dashboardHandler,
		StokHandler:        stokHandler,
		LaporanHandler:     laporanHandler,
	})

	addr := ":" + cfg.App.Port
	fmt.Printf("pkb-api listening on %s (env=%s)\n", addr, cfg.App.Env)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
