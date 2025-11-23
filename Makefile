help:
	@echo "Available commands:"
	@echo ""
	@echo "Setup & Initialization:"
	@echo "  make init              - Install swag tool for Swagger documentation generation"
	@echo ""
	@echo "Docker Services:"
	@echo "  make infra             - Start core infrastructure (postgres, clickhouse, kafka, redis, zookeeper)"
	@echo "  make up                - Start all docker compose services in detached mode"
	@echo "  make build-up          - Build and start all services"
	@echo "  make build-no-cache    - Build services without using cache"
	@echo "  make down              - Stop and remove containers (use RUN_ARGS for specific services)"
	@echo "  make purge             - Stop containers and remove volumes (use RUN_ARGS for specific services)"
	@echo ""
	@echo "Monitoring:"
	@echo "  make ps                - Show running docker compose services"
	@echo "  make status            - Show service status (use RUN_ARGS for specific services)"
	@echo "  make log               - Follow logs (use RUN_ARGS for specific services, e.g. RUN_ARGS=app)"
	@echo ""
	@echo "Development:"
	@echo "  make generate-swagger  - Generate Swagger API documentation"
	@echo "  make lint              - Run golangci-lint for code quality checks"
	@echo "  make shell             - Open bash shell in container (required: RUN_ARGS=<service>)"
	@echo ""
	@echo "Testing:"
	@echo "  make tester-up         - Start tester service"
	@echo "  make tester-down       - Stop tester service"
	@echo "  make charge            - Charge balance using simulator"
	@echo "  make stress            - Run stress test using simulator"
	@echo ""
	@echo "Database Migration:"
	@echo "  make migrator-up       - Start migrator service"
	@echo "  make migrator-down     - Stop migrator service"
	@echo "  make migrate           - Run database migrations (up)"
	@echo "  make migrate-up        - Run migrations up"
	@echo "  make migrate-down      - Run migrations down"
	@echo ""
	@echo "Examples:"
	@echo "  make log RUN_ARGS=app            - Follow logs for app service"
	@echo "  make shell RUN_ARGS=app          - Open shell in app service"
	@echo "  make down RUN_ARGS=tester        - Stop only tester service"

init:
	@echo installing swag
	go install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc3

env:
	@[ -e ./.env ] || cp -v ./.env.example ./.env

infra:
	docker compose up -d postgres clickhouse kafka redis zookeeper

up:
	docker compose up -d

ps:
	docker compose ps

build-up:
	docker compose up --build -d

build-no-cache:
	docker compose build --no-cache

status:
	docker compose ps $(RUN_ARGS)

down:
	docker compose down --remove-orphans $(RUN_ARGS)

purge:
	docker compose down --remove-orphans --volumes $(RUN_ARGS)

log:
	docker compose logs -f $(RUN_ARGS)

generate-swagger:
	swag init -q --parseDependency --parseInternal -g router.go -d internal/api

lint:
	golangci-lint run

shell:
	docker compose exec -it $(RUN_ARGS) bash

tester-up:
	docker compose --profile tools up -d tester

tester-down:
	docker compose --profile tools down tester

charge:
	docker compose exec tester sh -c "./simulator charge"

stress:
	docker compose exec tester sh -c "./simulator run"

migrator-up:
	docker compose --profile tools up -d migrator

migrator-down:
	docker compose --profile tools down migrator

migrate-up:
	docker compose exec migrator ./messenger migrate up

migrate-down:
	docker compose exec migrator ./messenger migrate down

migrate:
	$(MAKE) migrate-up
