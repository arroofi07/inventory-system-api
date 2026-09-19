package main

import (
	"fmt"
	"log"

	"app/internal/config"
	"app/internal/repository"
	"app/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	if err := service.SeedSuperAdmin(db, cfg.Seed); err != nil {
		log.Fatalf("seed: %v", err)
	}

	fmt.Printf("super_admin seeded: %s\n", cfg.Seed.SuperAdminEmail)
}
