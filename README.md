# PKB API

Backend Go (Gin + GORM + swaggo) untuk sistem distribusi PKB Web.

Struktur folder mengikuti [docs/02-arsitektur-target.md](../docs/02-arsitektur-target.md) bagian 3.

## Prasyarat

- Go 1.22+
- MySQL 8 (lihat SA-02 untuk Docker Compose)
- Salin `.env.example` → `.env` lalu isi rahasia

## Setup

```bash
cp .env.example .env
go mod tidy
```

Pastikan MySQL jalan (dari root repo):

```bash
docker compose up -d
```

Adminer: http://localhost:8081 — server `mysql`, user `pkb_app`, password `secret`, DB `pkb`.

MySQL dipublish ke host port **3307** (bukan 3306) agar tidak bentrok dengan MySQL/XAMPP lokal.

## Makefile

```bash
make help
make run           # API
make test          # unit test
make migrate-up    # terapkan migrasi
make migrate-down  # rollback 1 langkah
make swagger       # generate OpenAPI (butuh swag)
make lint          # golangci-lint
```

## Menjalankan API

```bash
go run ./cmd/api
# atau
make run
# hot reload:
make dev
```

Health check: `GET http://localhost:8082/healthz` (port dari `APP_PORT`)

Swagger UI (development): http://localhost:8082/swagger/index.html

```bash
make swagger   # regenerate docs dari anotasi
```

## Auth (SA-06)

```bash
make migrate-up
make seed          # super_admin dari SEED_SUPER_ADMIN_*
make run
```

```bash
# Login — access_token di body; refresh_token di cookie HttpOnly
curl -c cookies.txt -X POST http://localhost:8082/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@pkb.test","password":"rahasia123"}'

# Profil
curl http://localhost:8082/api/v1/me -H "Authorization: Bearer <access_token>"

# Refresh / logout (kirim cookie)
curl -b cookies.txt -X POST http://localhost:8082/api/v1/auth/refresh
curl -b cookies.txt -X POST http://localhost:8082/api/v1/auth/logout
```

Middleware terpasang: RequestID, Recovery, Logger, CORS, Auth, RequirePermission (+ peta `izinPerRole` SA-07).

## Test

```bash
make test
# atau
go test ./...
```

## Migrasi skema

```bash
make migrate-up
make migrate-down
make migrate-new name=add_something
```

Skema domain (15 tabel + CHECK) ada di `migrations/000001_init_schema.*.sql` (SA-05).

## Seeder

```bash
make seed
```

Isi `SEED_SUPER_ADMIN_EMAIL` dan `SEED_SUPER_ADMIN_PASSWORD` di `.env` sebelum seed.

## CI lokal

```bash
make ci    # vet + golangci-lint + test
```

Pipeline GitHub: `.github/workflows/ci.yml` (api + web).
