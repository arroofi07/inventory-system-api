# syntax=docker/dockerfile:1

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

FROM gcr.io/distroless/static-debian12 AS prod
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /app/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
