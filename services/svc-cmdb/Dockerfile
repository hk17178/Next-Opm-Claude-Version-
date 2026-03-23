# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go.mod from monorepo root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared packages and service code
COPY pkg/ pkg/
COPY services/svc-cmdb/ services/svc-cmdb/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /app/svc-cmdb \
    ./services/svc-cmdb/cmd/server/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/svc-cmdb .
COPY services/svc-cmdb/configs/ configs/
COPY services/svc-cmdb/migrations/ migrations/

EXPOSE 8084 9084

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8084/healthz || exit 1

ENTRYPOINT ["./svc-cmdb"]
