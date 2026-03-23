.PHONY: help lint test build clean docker-build proto schema-validate api-validate dev-up dev-down dev-logs dev-ps

SERVICES := svc-log svc-alert svc-incident svc-cmdb svc-notify svc-ai svc-analytics svc-change svc-orchestration
GO := go
GOFLAGS := -v
DOCKER := docker
COMPOSE := docker compose

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Build ---

build: ## Build all services
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		cd services/$$svc && $(GO) build $(GOFLAGS) -o ../../bin/$$svc ./cmd/server && cd ../..; \
	done

build-%: ## Build a specific service (e.g., make build-svc-log)
	@echo "Building $*..."
	@cd services/$* && $(GO) build $(GOFLAGS) -o ../../bin/$* ./cmd/server

# --- Test ---

test: ## Run all tests
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		cd services/$$svc && $(GO) test ./... && cd ../..; \
	done

test-%: ## Test a specific service (e.g., make test-svc-log)
	@cd services/$* && $(GO) test -race -cover ./...

test-coverage: ## Run tests with coverage report
	@for svc in $(SERVICES); do \
		cd services/$$svc && $(GO) test -coverprofile=../../coverage/$$svc.out ./... && cd ../..; \
	done

# --- Lint ---

lint: ## Run linters on all services
	@for svc in $(SERVICES); do \
		echo "Linting $$svc..."; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
	done

lint-%: ## Lint a specific service (e.g., make lint-svc-log)
	@cd services/$* && golangci-lint run ./...

lint-frontend: ## Lint frontend code
	@cd frontend && pnpm run lint

# --- Docker ---

docker-build: ## Build all Docker images
	@for svc in $(SERVICES); do \
		echo "Building Docker image for $$svc..."; \
		$(DOCKER) build -t opsnexus/$$svc:latest -f services/$$svc/Dockerfile .; \
	done

docker-build-%: ## Build Docker image for a specific service
	@$(DOCKER) build -t opsnexus/$*:latest -f services/$*/Dockerfile .

# --- Infrastructure ---

infra-up: ## Start infrastructure services (Kafka, PostgreSQL, Redis, etc.)
	@$(COMPOSE) -f deployments/docker-compose.infra.yml up -d

infra-down: ## Stop infrastructure services
	@$(COMPOSE) -f deployments/docker-compose.infra.yml down

dev-up: ## Start all dev services
	cd deploy/docker-compose && cp -n .env.example .env && docker compose up -d

dev-down: ## Stop all dev services
	cd deploy/docker-compose && docker compose down

dev-logs: ## Follow logs for all services
	cd deploy/docker-compose && docker compose logs -f

dev-ps: ## Show status of dev services
	cd deploy/docker-compose && docker compose ps

# --- Schema & API Validation ---

schema-validate: ## Validate all Kafka event schemas
	@echo "Validating event schemas..."
	@for schema in schemas/events/*.schema.json; do \
		echo "  Validating $$schema..."; \
		npx ajv validate -s $$schema --valid || exit 1; \
	done
	@echo "All schemas valid."

api-validate: ## Validate all OpenAPI specs
	@echo "Validating OpenAPI specs..."
	@for spec in docs/api/*.yaml; do \
		echo "  Validating $$spec..."; \
		npx @redocly/cli lint $$spec || exit 1; \
	done
	@echo "All API specs valid."

# --- Database Migrations ---

migrate-up-%: ## Run migrations for a service (e.g., make migrate-up-svc-log)
	@cd services/$* && migrate -path migrations -database "$$DATABASE_URL" up

migrate-down-%: ## Rollback migrations for a service
	@cd services/$* && migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-create-%: ## Create a new migration (e.g., make migrate-create-svc-log NAME=add_users)
	@cd services/$* && migrate create -ext sql -dir migrations -seq $(NAME)

# --- Clean ---

clean: ## Clean build artifacts
	@rm -rf bin/ coverage/
	@for svc in $(SERVICES); do \
		cd services/$$svc && $(GO) clean && cd ../..; \
	done

# --- Frontend ---

web-install: ## Install frontend dependencies
	@cd frontend && pnpm install

web-dev: ## Start frontend dev server
	@cd frontend && pnpm run dev

web-build: ## Build frontend for production
	@cd frontend && pnpm run build

web-test: ## Run frontend tests
	@cd frontend && pnpm run test
