# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go.mod from monorepo root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared packages and service code
COPY pkg/ pkg/
COPY services/svc-ai/ services/svc-ai/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /app/svc-ai \
    ./services/svc-ai/cmd/server/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/svc-ai .
COPY services/svc-ai/configs/ configs/
COPY services/svc-ai/migrations/ migrations/

EXPOSE 8086

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8086/healthz || exit 1

ENTRYPOINT ["./svc-ai"]
