.DEFAULT_GOAL := help

BIN_DIR := bin
BIN     := $(BIN_DIR)/service
PKG     := ./cmd/service
GO      ?= go

# Current module path, read from go.mod so `rename` has a single source of truth.
MODULE := $(shell $(GO) list -m)
# Bare project name, i.e. the last path segment. Rewritten separately by `rename`:
# it appears on its own in places the module path does not reach, such as the Fiber
# AppName, so replacing only the module path would leave the new service announcing
# itself under the template's name.
NAME := $(notdir $(MODULE))

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Compile the binary into ./bin/service
	$(GO) build -o $(BIN) $(PKG)

.PHONY: run
run: ## Run the service locally
	$(GO) run $(PKG)

.PHONY: test
test: ## Run every test
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run tests with the data race detector (needs CGO + gcc/clang)
	CGO_ENABLED=1 $(GO) test -race ./...

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

# Must stay identical to the version pinned in .github/workflows/ci.yml. A linter
# release changes which findings it reports, so an older local binary yields a green
# `make check` on code CI rejects — the divergence is silent and costs a round trip
# every time. Bump both in the same commit.
GOLANGCI_VERSION := v2.12.2

.PHONY: lint-version
lint-version: ## Check the local golangci-lint matches the version CI pins
	@have="$$(golangci-lint --version 2>/dev/null | grep -oE 'version [0-9]+\.[0-9]+\.[0-9]+' | awk '{print "v"$$2}')"; \
	if [ "$$have" != "$(GOLANGCI_VERSION)" ]; then \
		echo "golangci-lint version mismatch: local $${have:-<not found>}, CI pins $(GOLANGCI_VERSION)."; \
		echo "Install the pinned version, or change GOLANGCI_VERSION here AND in"; \
		echo ".github/workflows/ci.yml — they must agree."; \
		exit 1; \
	fi

.PHONY: lint
lint: lint-version ## golangci-lint run
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## golangci-lint run --fix
	golangci-lint run --fix ./...

.PHONY: fmt
fmt: ## Format the code (golangci-lint fmt)
	golangci-lint fmt ./...

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: check
check: vet lint test ## Full verification (vet + lint + test)

.PHONY: dev
dev: ## Local stack: service + Prometheus + Grafana
	docker compose up --build

.PHONY: dev-down
dev-down: ## Tear the local stack down, volumes included
	docker compose down -v

.PHONY: docker-build
docker-build: ## Build the container image
	docker build -t $(notdir $(MODULE)):dev .

.PHONY: clean
clean: ## Remove build artefacts
	@rm -rf $(BIN_DIR)

.PHONY: rename
rename: ## Rewrite the module path: make rename MODULE_NEW=github.com/org/name
ifndef MODULE_NEW
	$(error MODULE_NEW is required, e.g. make rename MODULE_NEW=github.com/acme/billing-api)
endif
	@echo "Renaming $(MODULE) -> $(MODULE_NEW)"
	@echo "      and $(NAME) -> $(notdir $(MODULE_NEW))"
	@# Full module path first: doing the bare name first would rewrite the tail of
	@# the module path and leave the full-path pass with nothing to match.
	@grep -rl --binary-files=without-match '$(MODULE)' \
		--exclude-dir=.git --exclude-dir=bin --exclude-dir=node_modules . \
		| xargs -r sed -i 's|$(MODULE)|$(MODULE_NEW)|g'
	@grep -rl --binary-files=without-match '$(NAME)' \
		--exclude-dir=.git --exclude-dir=bin --exclude-dir=node_modules . \
		| xargs -r sed -i 's|$(NAME)|$(notdir $(MODULE_NEW))|g'
	@$(GO) mod tidy
	@# Post-condition, identical to the one in rename.ps1. It is what makes the two
	@# implementations interchangeable: whatever route they take, a tree that still
	@# mentions the old name is a failed rename, and it fails loudly instead of
	@# leaving a half-renamed repository to discover three commits later.
	@# MODULE and NAME were expanded when make started, so they still hold the OLD
	@# values here. -F for literal matching (the dots in a module path are regex
	@# wildcards otherwise), -I to skip binaries.
	@if grep -rlI -F -e '$(MODULE)' -e '$(NAME)' \
		--exclude-dir=.git --exclude-dir=bin --exclude-dir=node_modules . ; then \
		echo "Rename incomplete — the files above still mention the old name."; \
		exit 1; \
	fi
	@echo "Done. Run 'make check' to confirm, then commit."
	@echo "Note: README.md and CLAUDE.md still describe the template — /bootstrap-spec rewrites their headers."
