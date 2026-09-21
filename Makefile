.PHONY: help run dev build vet test test-race test-cover lint swagger swagger-check migrate-up migrate-down migrate-new seed seed-demo seed-demo-reset etl etl-dry etl-framework etl-audit-sumber etl-bersihkan-sumber docker-up docker-down ci

help: ## Tampilkan target yang tersedia
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

run: ## Jalankan API tanpa hot reload
	go run ./cmd/api

dev: ## Jalankan API dengan hot reload (air)
	air -c .air.toml

build: ## Build binary produksi ke bin/api
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/api ./cmd/api

vet: ## go vet
	go vet ./...

test: ## Jalankan semua test
	go test -count=1 ./...

test-race: ## Test dengan race detector (butuh CGO di Windows)
	go test -race -count=1 ./...

test-cover: ## Test dengan laporan coverage HTML
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Jalankan golangci-lint
	golangci-lint run ./...

swagger: ## Generate ulang dokumentasi OpenAPI (swag)
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

swagger-check: ## Gagal bila swagger.json tidak sinkron dengan kode
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	git diff --exit-code docs/swagger.json docs/swagger.yaml docs/docs.go \
		|| (echo "docs/swagger* tidak sinkron. Jalankan 'make swagger' dan commit." && exit 1)

migrate-up: ## Terapkan semua migrasi
	go run ./cmd/migrate up

migrate-down: ## Rollback satu migrasi
	go run ./cmd/migrate down 1

migrate-new: ## Buat file migrasi baru: make migrate-new name=nama_migrasi
	migrate create -ext sql -dir migrations -seq $(name)

seed: ## Isi super_admin dari SEED_SUPER_ADMIN_*
	go run ./cmd/seed

seed-demo: ## Super admin + data demo semua modul (dilarang di production)
	go run ./cmd/seed --demo

seed-demo-reset: ## Hapus data demo lalu isi ulang
	go run ./cmd/seed --demo --reset

etl-dry: ## Cetak rencana tahap ETL tanpa menulis DB
	go run ./cmd/etl -dry-run

etl-framework: ## Uji kerangka SF-01 (truncate stub + checksum 2×; butuh target MySQL)
	go run ./cmd/etl -framework-only -idempotent-check

etl: ## Jalankan ETL penuh (butuh ETL_SUMBER_* + target)
	go run ./cmd/etl

etl-audit-sumber: ## SF-02 audit read-only: duplikat, channel, I7/I8
	go run ./cmd/etl -audit-sumber

etl-bersihkan-sumber: ## SF-02 tulis ke SALINAN sumber (ETL_SUMBER_BOLEH_TULIS=true wajib)
	go run ./cmd/etl -bersihkan-sumber

docker-up: ## MySQL + Adminer (dari root repo)
	docker compose -f ../docker-compose.yml up -d

docker-down: ## Stop MySQL + Adminer
	docker compose -f ../docker-compose.yml down

ci: ## Mirror langkah CI lokal (vet + lint + test + swagger-check)
	$(MAKE) vet
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) swagger-check
