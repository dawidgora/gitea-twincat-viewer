# gitea-twincat-viewer — developer Makefile
#
# Conventions:
#   make build      — compile the Go binary into ./bin/
#   make test       — run the full test suite
#   make lint       — run go vet and (when available) gofmt checks
#   make dev        — start the local Gitea + viewer stack
#   make dev-down   — stop the local stack
#   make clean      — remove build artifacts

BIN_DIR        := bin
BIN_NAME       := gitea-twincat-viewer

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w" \
		-o $(BIN_DIR)/$(BIN_NAME) ./cmd/gitea-twincat-viewer

.PHONY: test
test:
	go test ./... -count=1

.PHONY: lint
lint:
	@command -v gofmt >/dev/null && gofmt -l . | tee /tmp/gofmt.out | (! grep .) || (echo "gofmt found unformatted files"; exit 1)
	go vet ./...

.PHONY: dev
dev:
	docker compose up --build

.PHONY: dev-down
dev-down:
	docker compose down -v

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) coverage.out
