# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go.mod from monorepo root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared packages and service code
COPY pkg/ pkg/
COPY services/svc-analytics/ services/svc-analytics/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /app/svc-analytics \
    ./services/svc-analytics/cmd/server/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/svc-analytics .
COPY services/svc-analytics/configs/ configs/
COPY services/svc-analytics/migrations/ migrations/

EXPOSE 8087

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8087/healthz || exit 1

ENTRYPOINT ["./svc-analytics"]
