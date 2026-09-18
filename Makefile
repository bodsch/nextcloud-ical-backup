
BINARY      := nextcloud-ical-backup
CMD_PKG     := ./cmd/nextcloud-ical-backup
BIN_DIR     := bin
DIST_DIR    := dist

MODULE      := bodsch.me/nextcloud-ical-backup
VERSION_PKG := main

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)


LDFLAGS     := -s -w \
	-X $(VERSION_PKG).version=$(VERSION) \
	-X $(VERSION_PKG).buildDate=$(BUILD_DATE)

# Static binaries: no cgo (see design decision Q26).
export CGO_ENABLED := 0

# Release target platforms.
PLATFORMS      := linux/amd64 linux/arm64 darwin/arm64

GO             := go
GOLANGCI       := golangci-lint

.DEFAULT_GOAL  := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the nextcloud-ical-backup binary for the local platform
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD_PKG)

.PHONY: fmt
fmt: ## Format all Go source.
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet.
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (config in .golangci.yml; gosec enabled).
	$(GOLANGCI) run ./...

.PHONY: vuln
vuln: ## Scan deps and reachable code for known vulnerabilities (govulncheck).
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: sec
sec: lint vuln ## Security checks: golangci-lint (incl. gosec) + govulncheck.

.PHONY: tidy
tidy: ## Tidy and verify go.mod/go.sum.
	$(GO) mod tidy
	$(GO) mod verify

.PHONY: test
test: ## Run all tests with race detector
	CGO_ENABLED=1 go test -race -cover ./...

.PHONY: release
release: ## Cross-compile static release binaries into dist/.
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=$(DIST_DIR)/$(BINARY)-$$os-$$arch; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $$out $(CMD_PKG) || exit 1; \
	done

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR)
