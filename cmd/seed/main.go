package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"app/internal/config"
	"app/internal/repository"
	"app/internal/service"
)

func main() {
	demo := flag.Bool("demo", false, "isi data demo semua modul (dilarang di production)")
	reset := flag.Bool("reset", false, "hapus data demo lalu isi ulang (hanya bersama --demo)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	if err := service.SeedSuperAdmin(db, cfg.Seed); err != nil {
		log.Fatalf("seed super_admin: %v", err)
	}
	fmt.Printf("super_admin: %s\n", cfg.Seed.SuperAdminEmail)

	if !*demo {
		if *reset {
			log.Fatal("--reset hanya berlaku bersama --demo")
		}
		return
	}

	if err := service.BolehSeedDemo(cfg.App.Env); err != nil {
		log.Fatal(err)
	}

	ada, err := service.DemoSudahAda(db)
	if err != nil {
		log.Fatalf("cek demo: %v", err)
	}
	if ada && !*reset {
		log.Fatal("data demo sudah ada. Jalankan: make seed-demo-reset  (atau go run ./cmd/seed --demo --reset)")
	}
	if *reset {
		if err := service.HapusDataDemo(db); err != nil {
			log.Fatalf("hapus demo: %v", err)
		}
		fmt.Println("data demo lama dihapus")
	}

	pass := strings.TrimSpace(os.Getenv("SEED_DEMO_PASSWORD"))
	if pass == "" {
		pass = service.DemoPasswordDefault
	}

	hasil, err := service.SeedDemo(context.Background(), db, cfg, pass)
	if err != nil {
		log.Fatalf("seed demo: %v", err)
	}

	fmt.Printf("demo siap: %d user, %d barang, %d pelanggan, %d promo, %d transaksi, %d pembayaran, %d alert stok\n",
		hasil.Users, hasil.Barang, hasil.Pelanggan, hasil.Promo, hasil.Transaksi, hasil.Pembayaran, hasil.Alert)
	fmt.Println("login demo (password dari SEED_DEMO_PASSWORD, default rahasia123):")
	fmt.Printf("  admin     demo.admin@pkb.test\n")
	fmt.Printf("  sales     demo.sales@pkb.test\n")
	fmt.Printf("  afiliasi  demo.afiliasi@pkb.test\n")
	fmt.Printf("  super_admin %s (SEED_SUPER_ADMIN_*)\n", cfg.Seed.SuperAdminEmail)
}
