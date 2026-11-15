BIN ?= tupinamba

.PHONY: build test fmt vet lint tidy check coverage clean clean-audit

build:
	go build -o $(BIN) ./cmd/tupinamba

test:
	go test ./...

fmt:
	@test -z "$$(gofmt -l .)" || { echo "unformatted:"; gofmt -l .; exit 1; }

vet:
	go vet ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

check: fmt vet test build
	@echo "ok"

coverage:
	./scripts/coverage.sh

clean:
	rm -f $(BIN)

clean-audit:
	./scripts/clean-audit.sh
