include .env
export

.PHONY: run test migrate-up migrate-down seed infra-up infra-down

run:
	go run main.go

test:
	go test ./...

test-integration:
	go test ./internal/store/postgres/... -v -tags integration

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

seed:
	docker compose exec postgres psql -U coffee -d coffeedb -f /dev/stdin < migrations/seed.sql

infra-up:
	docker compose up -d
	@echo "Waiting for postgres..."
	@until docker compose exec postgres pg_isready -U coffee -d coffeedb > /dev/null 2>&1; do sleep 1; done
	@echo "Postgres ready."

infra-down:
	docker compose down
