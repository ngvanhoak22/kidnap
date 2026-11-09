ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE = docker compose --env-file .env -f infrastructure/compose.yaml

.PHONY: env db up down migrate migration-status api fmt test integration test-docker check backup
env:
	@python3 infrastructure/scripts/create_env.py
db:
	$(COMPOSE) up -d --wait db
up:
	$(COMPOSE) up -d --build --wait
down:
	$(COMPOSE) --profile test down
migrate:
	cd backend && go run ./cmd/migrate up
migration-status:
	cd backend && go run ./cmd/migrate status
api:
	cd backend && go run ./cmd/api
fmt:
	cd backend && gofmt -w .
test:
	./dev test
integration:
	$(COMPOSE) --profile test up -d --wait db-test
	@TEST_DATABASE_URL="postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@127.0.0.1:$${TEST_DB_PORT}/journey_test?sslmode=disable" sh -c 'cd backend && go test -count=1 -v ./tests/integration'
test-docker:
	./dev test-docker
check:
	./dev check
backup:
	@sh infrastructure/scripts/backup.sh
