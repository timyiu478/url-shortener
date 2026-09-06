.PHONY: up down create-table run test e2e build

up:
	docker compose up -d
	docker compose ps

down:
	docker compose down

create-table:
	@echo "Creating DynamoDB table ${DYNAMODB_TABLE_NAME:-Urls} at ${AWS_ENDPOINT:-http://localhost:8000} (using Go helper)"
	go run ./cmd/create_table

run:
	@echo "Run the API locally (requires Go >= 1.26 recommended)."
	export $(shell sed 's/#.*//' .env | xargs)
	OTEL_COLLECTOR_ENDPOINT=$${OTEL_COLLECTOR_ENDPOINT:-}
	go run ./cmd/api

test:
	go test ./... -v

# Run end-to-end tests against a running service (RUN_E2E=1 required by tests)
e2e:
	RUN_E2E=1 go test -run TestEndToEnd -v

build:
	docker build -t url-shortener:local .
