# Every target here is also what CI runs. A command that only exists in the CI
# configuration is a command developers cannot reproduce locally, which is how
# "works on my machine" starts.

SHELL := /bin/bash
.DEFAULT_GOAL := help

MODULE      := github.com/example/go-enterprise
BINARY      := bin/api
GOBIN       := $(shell go env GOPATH)/bin
MIGRATIONS  := migrations

# Read by the migrate targets. Sourced from .env so that a developer does not
# have to keep the URL in two places.
DATABASE_URL ?= $(shell grep -s '^DATABASE_URL=' .env .env.example 2>/dev/null | head -1 | cut -d= -f2-)

# The SQL helper targets prefer a local psql and otherwise run one inside the
# database container, so they work on a machine that has Docker but no
# PostgreSQL client installed. Inside the container the server is always on
# localhost:5432 whichever port the host published, so the host and port are
# rewritten for that case. The container is addressed by name rather than
# through `docker compose exec` so the fallback also works for a database
# someone started by hand.
PG_CONTAINER := goscaffold-pg
CONTAINER_DATABASE_URL = $(shell echo '$(DATABASE_URL)' | sed -E 's,@[^/]+/,@localhost:5432/,')
PSQL = $(shell command -v psql >/dev/null 2>&1 \
	&& echo 'psql "$(DATABASE_URL)"' \
	|| echo 'docker exec -i $(PG_CONTAINER) psql "$(CONTAINER_DATABASE_URL)"')

# Tool versions are pinned. An unpinned linter changes its opinion between
# machines, and the resulting failures look like code problems.
GOLANGCI_LINT_VERSION := v2.13.2
SQLC_VERSION          := v1.31.1
MIGRATE_VERSION       := v4.19.0
GOVULNCHECK_VERSION   := latest

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------------ build/run

.PHONY: run
run: ## Run the API server
	go run ./cmd/api

.PHONY: build
build: ## Build the API binary into bin/
	go build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/api

.PHONY: clean
clean: ## Remove build and coverage output
	rm -rf bin coverage.out coverage.html

# ---------------------------------------------------------------------- tests

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: test-race
test-race: ## Run tests with the race detector
	go test -race ./...

.PHONY: test-integration
test-integration: ## Run tests that need Docker (testcontainers)
	go test -race -tags=integration -count=1 ./...

.PHONY: cover
cover: ## Run tests and open the coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "wrote coverage.html"

# ------------------------------------------------------------------- quality

.PHONY: fmt
fmt: ## Format the code
	go fmt ./...
	$(GOBIN)/goimports -local $(MODULE) -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	$(GOBIN)/golangci-lint run

.PHONY: vuln
vuln: ## Check dependencies for known vulnerabilities
	$(GOBIN)/govulncheck ./...

.PHONY: openapi-lint
openapi-lint: ## Validate api/openapi.yaml (requires npx)
	@command -v npx >/dev/null 2>&1 \
		|| { echo "SKIPPED: npx is not installed"; exit 0; }
	npx --yes @redocly/cli@latest lint api/openapi.yaml

.PHONY: check
check: fmt vet lint test-race vuln sqlc-check ## Run every gate that CI runs

# ------------------------------------------------------------- code generation

.PHONY: sqlc
sqlc: ## Regenerate database code from queries/
	$(GOBIN)/sqlc generate

.PHONY: sqlc-check
sqlc-check: ## Fail if committed generated code is stale
	@$(GOBIN)/sqlc diff \
		|| { echo "generated code is out of date; run 'make sqlc' and commit the result"; exit 1; }

.PHONY: generate
generate: sqlc ## Run all code generation

# ------------------------------------------------------------------ migrations

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" down 1

.PHONY: migrate-status
migrate-status: ## Show the current schema version
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" version

.PHONY: migrate-create
migrate-create: ## Create a migration: make migrate-create name=add_users
	@test -n "$(name)" || { echo "usage: make migrate-create name=<snake_case_name>"; exit 1; }
	$(GOBIN)/migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

# ----------------------------------------------------------------------- admin

.PHONY: bootstrap-admin
bootstrap-admin: ## Create the first administrator: make bootstrap-admin email=you@example.com
	@test -n "$(email)" || { echo "usage: make bootstrap-admin email=<address> [name=<name>]"; exit 1; }
	go run ./cmd/adminctl -email "$(email)" $(if $(name),-name "$(name)",)

# ------------------------------------------------------------------- messaging

.PHONY: worker
worker: ## Run the outbox relay and event consumer
	go run ./cmd/worker

.PHONY: broker-up
broker-up: ## Start the message broker (NATS JetStream)
	docker compose --profile messaging up -d nats

.PHONY: broker-down
broker-down: ## Stop the message broker
	docker compose --profile messaging stop nats

.PHONY: outbox-status
outbox-status: ## Show how many events are waiting to be delivered
	@$(PSQL) -v ON_ERROR_STOP=1 -c "\
	  SELECT count(*) FILTER (WHERE published_at IS NULL) AS pending, \
	         count(*) FILTER (WHERE published_at IS NOT NULL) AS published, \
	         count(*) FILTER (WHERE published_at IS NULL AND attempts > 0) AS failing, \
	         max(attempts) FILTER (WHERE published_at IS NULL) AS worst_attempts \
	  FROM outbox_events;"

.PHONY: dlq-list
dlq-list: ## List dead-lettered events awaiting triage
	@$(PSQL) -v ON_ERROR_STOP=1 -c "\
	  SELECT id, event_id, event_type, consumer, attempts, \
	         failed_at, left(last_error, 80) AS last_error \
	  FROM dead_letter_events \
	  WHERE replayed_at IS NULL \
	  ORDER BY failed_at DESC LIMIT 50;"

.PHONY: dlq-replay
dlq-replay: ## Put one dead letter back in the outbox: make dlq-replay id=<dead_letter_id>
	@test -n "$(id)" || { echo "usage: make dlq-replay id=<dead_letter_id>"; exit 1; }
	@$(PSQL) -v ON_ERROR_STOP=1 -c "\
	  WITH letter AS ( \
	    UPDATE dead_letter_events SET replayed_at = now() \
	    WHERE id = '$(id)' AND replayed_at IS NULL \
	    RETURNING event_id, event_type, payload, traceparent \
	  ) \
	  INSERT INTO outbox_events (id, event_type, payload, traceparent, created_at) \
	  SELECT event_id, event_type, payload, traceparent, now() FROM letter \
	  ON CONFLICT (id) DO NOTHING;"

# ----------------------------------------------------------------------- tools

.PHONY: tools
tools: ## Install the pinned developer tools into GOPATH/bin
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	go install golang.org/x/tools/cmd/goimports@latest
