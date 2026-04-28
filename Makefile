.PHONY: build lint test test-short cover gen verify clean

GO := go
LDFLAGS := -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/teranode-acceptance ./cmd/teranode-acceptance
	$(GO) build -o bin/gen-traceability ./cmd/gen-traceability

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
	  || (echo "README / traceability.md out of sync with manifest — run 'make gen' and commit the result" && exit 1)

clean:
	rm -rf bin/ report.json report.html coverage.out coverage.html
