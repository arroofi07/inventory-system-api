# syntax=docker/dockerfile:1
# Dokploy: tipe Dockerfile.
# Root directory: sistem-barang/be
# Dockerfile: Dockerfile
# Port container: 8080
# Health check path: /healthz
# Env dibaca saat container jalan (lihat .env.example).

FROM golang:1.22-alpine AS base
WORKDIR /app
RUN apk add --no-cache git build-base

FROM base AS deps
COPY go.mod go.sum ./
RUN go mod download

FROM deps AS dev
RUN go install github.com/air-verse/air@v1.61.7
COPY . .
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM deps AS builder
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed /app/seed
COPY --from=builder /app/migrations /app/migrations
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh /app/api /app/migrate /app/seed \
	&& sed -i 's/\r$//' /docker-entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
