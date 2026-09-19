package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "app/docs"
	"app/internal/config"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/middleware"
	"app/internal/service"
)

type Dependencies struct {
	Config             *config.Config
	AuthService        *service.AuthService
	AuthHandler        *AuthHandler
	BarangHandler      *BarangHandler
	BarangMasukHandler *BarangMasukHandler
	PelangganHandler   *PelangganHandler
	UserHandler        *UserHandler
	PromoHandler       *PromoHandler
	TransaksiHandler   *TransaksiHandler
	PembayaranHandler  *PembayaranHandler
	PiutangHandler     *PiutangHandler
	FakturHandler      *FakturHandler
	DashboardHandler   *DashboardHandler
	StokHandler        *StokHandler
	LaporanHandler     *LaporanHandler
	ExportJobs         *service.ExportJobStore
}

func NewRouter(d *Dependencies) *gin.Engine {
	if d.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(d.Config.CORS.AllowedOrigins))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	daftarkanSwagger(r, d.Config)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		loginLimiter := middleware.NewLoginRateLimiter(d.Config.Login.RateLimitPerMinute)
		auth.POST("/login", loginLimiter.Middleware(), d.AuthHandler.Login)
		auth.POST("/refresh", d.AuthHandler.Refresh)
		auth.POST("/logout", d.AuthHandler.Logout)

		protected := v1.Group("")
		protected.Use(middleware.Auth(d.AuthService))
		protected.GET("/me", me)
		if d.DashboardHandler != nil {
			protected.GET("/dashboard", d.DashboardHandler.Ambil)
		}
		protected.GET("/audit",
			middleware.RequirePermission(domain.PermAuditLihat),
			func(c *gin.Context) {
				httpx.BalasOK(c, gin.H{"items": []any{}})
			},
		)

		if d.BarangHandler != nil {
			barang := protected.Group("/barang")
			barang.GET("", middleware.RequirePermission(domain.PermBarangLihat), d.BarangHandler.Daftar)
			// Satu nama wildcard Gin (:kode_barang) — detail juga menerima ID numerik.
			barang.GET("/:kode_barang/batch-tersedia", middleware.RequirePermission(domain.PermBarangLihat), d.BarangHandler.BatchTersedia)
			barang.GET("/:kode_barang/batch", middleware.RequirePermission(domain.PermBarangLihat), d.BarangHandler.DaftarBatch)
			barang.GET("/:kode_barang/riwayat-harga", middleware.RequirePermission(domain.PermHargaKelola), d.BarangHandler.RiwayatHarga)
			barang.POST("/:kode_barang/harga-massal", middleware.RequirePermission(domain.PermHargaKelola), d.BarangHandler.HargaMassal)
			barang.GET("/:kode_barang", middleware.RequirePermission(domain.PermBarangLihat), d.BarangHandler.Detail)
			barang.POST("", middleware.RequirePermission(domain.PermBarangKelola), d.BarangHandler.Buat)
			barang.PATCH("/:kode_barang", middleware.RequirePermission(domain.PermBarangKelola), d.BarangHandler.Ubah)
			barang.PATCH("/:kode_barang/status", middleware.RequirePermission(domain.PermBarangKelola), d.BarangHandler.SetStatus)
		}

		if d.BarangMasukHandler != nil {
			bm := protected.Group("/barang-masuk")
			bm.GET("", middleware.RequirePermission(domain.PermBarangMasukLihat), d.BarangMasukHandler.Daftar)
			bm.GET("/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.BarangMasukHandler.Export)
			bm.GET("/template-impor", middleware.RequirePermission(domain.PermBarangMasukBuat), d.BarangMasukHandler.TemplateImpor)
			bm.POST("/impor", middleware.RequirePermission(domain.PermBarangMasukBuat), d.BarangMasukHandler.Impor)
			bm.GET("/:id", middleware.RequirePermission(domain.PermBarangMasukLihat), d.BarangMasukHandler.Detail)
			bm.POST("", middleware.RequirePermission(domain.PermBarangMasukBuat), d.BarangMasukHandler.Buat)
			bm.PATCH("/:id", middleware.RequirePermission(domain.PermBarangMasukUbah), d.BarangMasukHandler.Ubah)
			bm.DELETE("/:id", middleware.RequirePermission(domain.PermBarangMasukHapus), d.BarangMasukHandler.Hapus)
		}

		if d.PelangganHandler != nil {
			pl := protected.Group("/pelanggan")
			pl.GET("", middleware.RequirePermission(domain.PermPelangganLihat), d.PelangganHandler.Daftar)
			pl.GET("/cari", middleware.RequirePermission(domain.PermPelangganLihat), d.PelangganHandler.CariSelect2)
			pl.GET("/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.PelangganHandler.Export)
			pl.GET("/template-impor", middleware.RequirePermission(domain.PermPelangganBuat), d.PelangganHandler.TemplateImpor)
			pl.POST("/impor", middleware.RequirePermission(domain.PermPelangganBuat), d.PelangganHandler.Impor)
			pl.POST("", middleware.RequirePermission(domain.PermPelangganBuat), d.PelangganHandler.Buat)
			pl.GET("/:id", middleware.RequirePermission(domain.PermPelangganLihat), d.PelangganHandler.Detail)
			pl.GET("/:id/riwayat-transaksi", middleware.RequirePermission(domain.PermPelangganLihat), d.PelangganHandler.RiwayatTransaksi)
			pl.PATCH("/:id", middleware.RequirePermission(domain.PermPelangganUbah), d.PelangganHandler.Ubah)
			pl.PATCH("/:id/status", middleware.RequirePermission(domain.PermPelangganUbah), d.PelangganHandler.SetStatus)
		}

		if d.UserHandler != nil {
			users := protected.Group("/users")
			users.GET("", middleware.RequirePermission(domain.PermUserLihat), d.UserHandler.Daftar)
			users.POST("", middleware.RequirePermission(domain.PermUserKelola), d.UserHandler.Buat)
			users.GET("/:id", middleware.RequirePermission(domain.PermUserLihat), d.UserHandler.Detail)
			users.PATCH("/:id", middleware.RequirePermission(domain.PermUserKelola), d.UserHandler.Ubah)
			users.PATCH("/:id/status", middleware.RequirePermission(domain.PermUserKelola), d.UserHandler.SetStatus)
			users.DELETE("/:id", middleware.RequirePermission(domain.PermUserKelola), d.UserHandler.Hapus)
		}

		if d.PromoHandler != nil {
			promo := protected.Group("/promo")
			promo.GET("", middleware.RequirePermission(domain.PermPromoLihat), d.PromoHandler.Daftar)
			promo.POST("", middleware.RequirePermission(domain.PermPromoKelola), d.PromoHandler.Buat)
			promo.GET("/:id", middleware.RequirePermission(domain.PermPromoLihat), d.PromoHandler.Detail)
			promo.PATCH("/:id", middleware.RequirePermission(domain.PermPromoKelola), d.PromoHandler.Ubah)
			promo.PATCH("/:id/status", middleware.RequirePermission(domain.PermPromoKelola), d.PromoHandler.SetStatus)
			promo.DELETE("/:id", middleware.RequirePermission(domain.PermPromoHapus), d.PromoHandler.Hapus)
		}

		if d.TransaksiHandler != nil {
			trx := protected.Group("/transaksi")
			trx.GET("", middleware.RequireAnyPermission(
				domain.PermTransaksiLihatMilik, domain.PermTransaksiLihatSemua,
			), d.TransaksiHandler.Daftar)
			trx.POST("/pratinjau", middleware.RequirePermission(domain.PermTransaksiBuat), d.TransaksiHandler.Pratinjau)
			trx.POST("/cek-stok", middleware.RequirePermission(domain.PermTransaksiBuat), d.TransaksiHandler.CekStok)
			trx.POST("/approvals/bulk", middleware.RequirePermission(domain.PermApprovalLakukan), d.TransaksiHandler.BulkApprove)
			trx.POST("", middleware.RequirePermission(domain.PermTransaksiBuat), d.TransaksiHandler.Buat)
			trx.POST("/:id/items", middleware.RequirePermission(domain.PermTransaksiBuat), d.TransaksiHandler.TambahItems)
			trx.GET("/:id/ketersediaan-stok", middleware.RequireAnyPermission(
				domain.PermApprovalLakukan, domain.PermTransaksiLihatSemua,
			), d.TransaksiHandler.KetersediaanStok)
			trx.POST("/:id/approve", middleware.RequirePermission(domain.PermApprovalLakukan), d.TransaksiHandler.Approve)
			trx.POST("/:id/reject", middleware.RequirePermission(domain.PermApprovalLakukan), d.TransaksiHandler.Reject)
			if d.PembayaranHandler != nil {
				trx.POST("/:id/pembayaran", middleware.RequirePermission(domain.PermPembayaranCatat), d.PembayaranHandler.Catat)
				trx.GET("/:id/pembayaran", middleware.RequirePermission(domain.PermPembayaranLihat), d.PembayaranHandler.Riwayat)
			}
			if d.FakturHandler != nil {
				trx.GET("/:id/faktur/pdf", middleware.RequirePermission(domain.PermFakturCetak), d.FakturHandler.PDF)
				trx.GET("/:id/faktur", middleware.RequirePermission(domain.PermFakturCetak), d.FakturHandler.Ambil)
			}
			trx.GET("/:id", middleware.RequireAnyPermission(
				domain.PermTransaksiLihatMilik, domain.PermTransaksiLihatSemua,
			), d.TransaksiHandler.Detail)
			// SC-08: tidak ada PUT/PATCH/DELETE transaksi untuk sales.
		}

		if d.StokHandler != nil {
			stok := protected.Group("/stok")
			stok.GET("/pergerakan", middleware.RequirePermission(domain.PermStokLihat), d.StokHandler.DaftarPergerakan)
			stok.GET("/rekonsiliasi", middleware.RequirePermission(domain.PermStokSesuaikan), d.StokHandler.Rekonsiliasi)
			stok.POST("/penyesuaian", middleware.RequirePermission(domain.PermStokSesuaikan), d.StokHandler.Penyesuaian)
		}

		if d.LaporanHandler != nil {
			protected.GET("/ekspor/:job_id", d.LaporanHandler.AmbilEkspor)
			lap := protected.Group("/laporan")
			lap.GET("/penjualan", middleware.RequirePermission(domain.PermLaporanPenjualan), d.LaporanHandler.Penjualan)
			lap.GET("/penjualan/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.LaporanHandler.ExportPenjualan)
			lap.GET("/stok", middleware.RequirePermission(domain.PermLaporanStok), d.LaporanHandler.Stok)
			lap.GET("/stok/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.LaporanHandler.ExportStok)
			lap.GET("/barang-keluar", middleware.RequirePermission(domain.PermLaporanPenjualan), d.LaporanHandler.BarangKeluar)
			lap.GET("/barang-keluar/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.LaporanHandler.ExportBarangKeluar)
			lap.GET("/laba", middleware.RequirePermission(domain.PermLaporanLaba), d.LaporanHandler.Laba)
			lap.GET("/channel-analytics", middleware.RequirePermission(domain.PermLaporanAnalytics), d.LaporanHandler.ChannelAnalytics)
			lap.GET("/channel-analytics/export", middleware.RequirePermission(domain.PermLaporanAnalytics), d.LaporanHandler.ExportChannelAnalytics)
			if d.PiutangHandler != nil {
				lap.GET("/penerimaan", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.Penerimaan)
			}
		}

		if d.PiutangHandler != nil {
			piu := protected.Group("/piutang")
			piu.GET("", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.Daftar)
			piu.GET("/overdue", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.Overdue)
			piu.GET("/export", middleware.RequirePermission(domain.PermLaporanEkspor), d.PiutangHandler.Export)
			piu.GET("/pelanggan/:kode/riwayat", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.RiwayatPelanggan)
			piu.GET("/pelanggan/:kode", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.Pelanggan)

			protected.GET("/notifikasi/piutang", middleware.RequirePermission(domain.PermPembayaranLihat), d.PiutangHandler.Notifikasi)
		}
	}

	return r
}

func daftarkanSwagger(r *gin.Engine, cfg *config.Config) {
	if cfg.App.Env == "production" {
		return
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.DefaultModelsExpandDepth(2),
		ginSwagger.PersistAuthorization(true),
	))
}

func me(c *gin.Context) {
	httpx.BalasOK(c, dto.MeData{
		ID:    c.MustGet(middleware.CtxKeyUserID).(uint64),
		Name:  c.MustGet(middleware.CtxKeyName).(string),
		Email: c.MustGet(middleware.CtxKeyEmail).(string),
		Role:  string(c.MustGet(middleware.CtxKeyRole).(domain.Role)),
	})
}
