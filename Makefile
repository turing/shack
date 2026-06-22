.PHONY: build test test-integration install fmt vet clean check-deps

BIN := shack
PREFIX ?= $(shell brew --prefix 2>/dev/null || echo /usr/local)

check-deps:
	@command -v go >/dev/null 2>&1 || { \
		echo "error: go not on PATH. Install with: brew install go"; \
		exit 1; \
	}
	@command -v caddy >/dev/null 2>&1 || { \
		echo "error: caddy not on PATH. Install with: brew install caddy"; \
		exit 1; \
	}
	@command -v tmux >/dev/null 2>&1 || { \
		echo "error: tmux not on PATH. Install with: brew install tmux"; \
		exit 1; \
	}

build: check-deps
	go build -o $(BIN) ./cmd/shack

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

install: build
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BIN)
