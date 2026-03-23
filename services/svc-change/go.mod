module github.com/opsnexus/svc-change

go 1.22.0

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/jackc/pgx/v5 v5.5.5
	github.com/opsnexus/opsnexus v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.27.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/opsnexus/opsnexus => ../../
