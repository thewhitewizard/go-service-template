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

.PHONY: lint
lint: ## golangci-lint run
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
	@echo "Done. Run 'make check' to confirm, then commit."
	@echo "Note: README.md and CLAUDE.md still describe the template — /bootstrap-spec rewrites their headers."
