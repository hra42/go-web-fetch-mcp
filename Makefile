.PHONY: build test lint install clean

build:
	go build -o bin/web-fetch-mcp ./cmd/web-fetch-mcp

test:
	go test ./... -race

lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	fi

install:
	go install ./cmd/web-fetch-mcp

clean:
	rm -rf bin/
