package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"app/internal/config"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"app/internal/service"
)

// rekonsiliasi-stok membandingkan SUM(stock_movements.qty) vs barang.stok_tersedia.
// Exit 0 bila konsisten; exit 1 bila ada penyimpangan (SC-13 stub job harian).
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	svc := service.NewStockService(
		db,
		repository.NewBarangRepo(),
		repository.NewStockRepo(),
		repository.NewAuditRepo(),
		clock.Real{},
	)
	hasil, err := svc.Rekonsiliasi(context.Background())
	if err != nil {
		log.Fatalf("rekonsiliasi: %v", err)
	}
	fmt.Printf("rekonsiliasi-stok: diperiksa=%d menyimpang=%d\n", hasil.Diperiksa, hasil.Menyimpang)
	for _, p := range hasil.Penyimpangan {
		fmt.Printf("  %s (%s): stok=%d ledger=%d selisih=%d\n",
			p.KodeBarang, p.NamaItem, p.StokTersedia, p.SaldoLedger, p.Selisih)
	}
	if hasil.Menyimpang > 0 {
		os.Exit(1)
	}
}
