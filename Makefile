.PHONY: build lint test test-short cover gen gen-fixtures verify clean

SHELL := /bin/bash
GO := go
LDFLAGS := -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/teranode-acceptance ./cmd/teranode-acceptance
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/teranode-chaos ./cmd/teranode-chaos
	$(GO) build -o bin/gen-traceability ./cmd/gen-traceability
	$(GO) build -o bin/derive-address ./cmd/derive-address
	$(GO) build -o bin/gen-fixtures ./cmd/gen-fixtures

gen-fixtures: build
	./bin/gen-fixtures --out tests/testdata/

lint:
	$(GO) vet ./...
	@diff -u <(echo -n) <(gofmt -l .)
	@command -v staticcheck >/dev/null || (echo "staticcheck not installed; go install honnef.co/go/tools/cmd/staticcheck@latest" && exit 1)
	staticcheck ./...

test:
	$(GO) test -race ./...

test-short: build
	./bin/teranode-acceptance --short --config config.yaml || true

cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

gen: build
	./bin/gen-traceability

verify: gen
	@git diff --exit-code README.md docs/traceability.md \
	  || (echo "README / traceability.md out of sync — run 'make gen' and commit" && exit 1)
	@if [ -f docs/discovery.yaml ] && grep -q "^  - id:" docs/discovery.yaml ; then \
	  go run ./scripts/check-refs.go --discovery docs/discovery.md --yaml docs/discovery.yaml --upstream /Users/oskarsson/gitcheckout/teranode ; \
	fi
	@./bin/gen-fixtures --out tests/testdata/
	@git diff --exit-code tests/testdata/historical_scripts.yaml tests/testdata/historical_utxos.yaml \
	  || (echo "fixture YAML out of sync — run 'make gen-fixtures' and commit" && exit 1)
	@go run ./scripts/check-test-docs.go --tests-dir tests/

clean:
	rm -rf bin/ report.json report.html coverage.out coverage.html

# --- Docker compose targets ---

.PHONY: compose-up compose-down compose-logs compose-test compose-reset

COMPOSE := docker compose -f compose/docker-compose.yml

compose-up: build
	$(COMPOSE) up -d
	@echo "Waiting 10s for services to settle..."
	@sleep 10
	./compose/bootstrap.sh

compose-down:
	./compose/teardown.sh

compose-logs:
	$(COMPOSE) logs -f $${SERVICE:-teranode-1}

compose-test: compose-up
	./bin/teranode-acceptance --short --config config.docker.yaml || true

# Privileged, NON-GATING chaos suite (OPS-1 service failure, OPS-2 network
# partition). Requires the docker mesh to be up and healthy (`make compose-up`,
# confirm with `docker ps`). It is destructive (kills/partitions containers)
# but self-heals the mesh after each test. Results are written to a separate
# scorecard (stdout + chaos-report.json) and NEVER affect the acceptance
# verdict — OPS-1/OPS-2 stay EXCLUDED_PRIVILEGED in the acceptance matrix.
.PHONY: chaos
chaos: build
	./bin/teranode-chaos --config config.docker.yaml

compose-reset: compose-down
