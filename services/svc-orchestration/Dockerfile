FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/svc-orchestration ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/svc-orchestration /usr/local/bin/svc-orchestration

EXPOSE 8087

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8087/healthz || exit 1

ENTRYPOINT ["svc-orchestration"]
