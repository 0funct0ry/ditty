# ditty — make is the ONLY supported entry point for build, test, lint, fmt and vet.
#
# Do not call `go build`, `go test` or `npm` directly: the targets below set the
# flags the project depends on (-race, -trimpath, CGO_ENABLED=0, buildinfo ldflags,
# coverage thresholds, bundle budget). Run `make help` for the full list.
#
# See CLAUDE.md for repo constraints and internal-docs/SPEC.md for the design.

# ---- Shell & make behaviour -------------------------------------------------
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --no-print-directory
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:

# ---- Project ----------------------------------------------------------------
BINARY    := ditty
MODULE    := github.com/0funct0ry/ditty
MAIN      := .
BIN_DIR   := bin
TOOLS_DIR := $(BIN_DIR)/tools
DIST_DIR  := dist
COVER_DIR := .cover
WEB_DIR   := web
WEB_DIST  := $(WEB_DIR)/dist
DOCS_DIR  := docs
IMAGE     := ghcr.io/0funct0ry/ditty

# ---- Toolchain --------------------------------------------------------------
GO ?= go
export CGO_ENABLED := 0
export GOFLAGS := -trimpath

# Pinned so CI and laptops agree. Bump deliberately, never with `latest`.
GOLANGCI_VERSION    ?= v2.13.2
STATICCHECK_VERSION ?= v0.8.1
GORELEASER_VERSION  ?= v2.18.0

GOLANGCI    := $(TOOLS_DIR)/golangci-lint
STATICCHECK := $(TOOLS_DIR)/staticcheck
GORELEASER  := $(TOOLS_DIR)/goreleaser

# ---- Version / buildinfo ----------------------------------------------------
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDINFO := $(MODULE)/internal/buildinfo
LDFLAGS   := -s -w \
             -X $(BUILDINFO).Version=$(VERSION) \
             -X $(BUILDINFO).Commit=$(COMMIT) \
             -X $(BUILDINFO).Date=$(DATE)

# ---- Budgets & thresholds (SPEC.md §10.4, §12 M13, §14) ---------------------
BUNDLE_BUDGET_KB ?= 400
BINARY_BUDGET_MB ?= 25
COVER_MIN        ?= 70
COVER_MIN_CRIT   ?= 90
CRITICAL_PKGS    := wire session

# ---- Knobs ------------------------------------------------------------------
PKG      ?= ./...
RUN      ?=
ARGS     ?=
FUZZ     ?= Fuzz
FUZZ_PKG ?= ./internal/wire
FUZZTIME ?= 30s

HAS_WEB := $(wildcard $(WEB_DIR)/package.json)

# Fail with a pointer to the milestone that delivers a missing file, not a raw error.
define need
@[[ -e "$(1)" ]] || { printf '\033[31mmake: %s not found\033[0m — %s\n' "$(1)" "$(2)" >&2; exit 1; }
endef

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} \
	  /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
	  /^[a-zA-Z0-9_\/-]+:.*?##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\n\033[1mVariables\033[0m\n'
	@printf '  %-20s %s\n' 'PKG=./internal/wire' 'narrow test/lint to one package'
	@printf '  %-20s %s\n' 'RUN=TestDecode' 'narrow to one test (with test-run)'
	@printf '  %-20s %s\n' 'ARGS="-w bash"' 'arguments passed to the binary by make run'
	@printf '  %-20s %s\n' 'VERSION=v1.0.0' 'override the version stamped into the binary'
	@printf '\n%s %s (%s)\n\n' 'ditty' '$(VERSION)' '$(COMMIT)'

.PHONY: all
all: check build ## Run every quality gate, then build

.PHONY: version
version: ## Print the version this build would stamp
	@printf 'version %s\ncommit  %s\ndate    %s\n' '$(VERSION)' '$(COMMIT)' '$(DATE)'

.PHONY: tools
tools: $(GOLANGCI) $(STATICCHECK) ## Install pinned dev tools into bin/tools

$(GOLANGCI):
	@GOBIN=$(CURDIR)/$(TOOLS_DIR) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

$(STATICCHECK):
	@GOBIN=$(CURDIR)/$(TOOLS_DIR) $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

$(GORELEASER):
	@GOBIN=$(CURDIR)/$(TOOLS_DIR) $(GO) install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

.PHONY: deps
deps: ## Download Go module dependencies
	$(GO) mod download

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: ## Fail if go.mod/go.sum are not tidy (CI gate)
	@cp go.mod go.mod.bak; cp go.sum go.sum.bak 2>/dev/null || true
	@trap 'mv go.mod.bak go.mod; mv go.sum.bak go.sum 2>/dev/null || true' EXIT; \
	  $(GO) mod tidy; \
	  diff -q go.mod go.mod.bak >/dev/null || { echo "make: go.mod is not tidy — run make tidy" >&2; exit 1; }

##@ Build

.PHONY: build
build: $(WEB_DIST) ## Build the binary into bin/ditty
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(MAIN)
	@printf 'built %s (%s)\n' '$(BIN_DIR)/$(BINARY)' '$(VERSION)'

.PHONY: install
install: $(WEB_DIST) ## go install the binary into GOBIN
	$(GO) install -ldflags '$(LDFLAGS)' $(MAIN)

.PHONY: run
run: $(WEB_DIST) ## Run from source — make run ARGS="-w bash"
	$(GO) run -ldflags '$(LDFLAGS)' $(MAIN) $(ARGS)

.PHONY: dev-fixture
dev-fixture: $(WEB_DIST) ## Run from source against a fixture scenario — make dev-fixture NAME=deploy
	$(GO) run -tags fixture -ldflags '$(LDFLAGS)' $(MAIN) --fixture $(or $(NAME),deploy) $(ARGS)

.PHONY: generate
generate: ## Run go generate
	$(GO) generate ./...

# embed.FS needs web/dist to exist at compile time. Until the real UI lands (M4),
# stand in a placeholder rather than failing the build.
$(WEB_DIST):
ifneq ($(HAS_WEB),)
	$(MAKE) web-build
else
	@mkdir -p $(WEB_DIST)
	@printf '<!doctype html><title>ditty</title><p>ditty is booting\n' > $(WEB_DIST)/index.html
	@printf 'make: no %s yet — wrote a placeholder %s (real UI arrives in M4)\n' '$(WEB_DIR)/' '$(WEB_DIST)/index.html'
endif

##@ Web UI

.PHONY: web-deps
web-deps: ## Install frontend dependencies
	$(call need,$(WEB_DIR)/package.json,the web app arrives in M4 (SPEC.md §12))
	@cd $(WEB_DIR) && npm ci

.PHONY: web-dev
web-dev: ## Run the Vite dev server
	$(call need,$(WEB_DIR)/package.json,the web app arrives in M4 (SPEC.md §12))
	@cd $(WEB_DIR) && npm run dev

.PHONY: web-build
web-build: ## Build the frontend into web/dist
	$(call need,$(WEB_DIR)/package.json,the web app arrives in M4 (SPEC.md §12))
	@cd $(WEB_DIR) && npm run build

.PHONY: web-test
web-test: ## Run the Vitest suite
	$(call need,$(WEB_DIR)/package.json,the web app arrives in M4 (SPEC.md §12))
	@cd $(WEB_DIR) && npm run test

.PHONY: e2e
e2e: build ## Run the Playwright E2E smoke against a real binary
	$(call need,$(WEB_DIR)/package.json,the E2E suite arrives with M4/M5 (SPEC.md §14))
	@cd $(WEB_DIR) && npm run test:e2e

##@ Quality

.PHONY: fmt
fmt: ## Format Go code
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Fail if any Go file is unformatted (CI gate)
	@out=$$(gofmt -l -e $$(git ls-files '*.go') 2>/dev/null || true); \
	  [[ -z "$$out" ]] || { echo "make: unformatted files — run make fmt:" >&2; echo "$$out" >&2; exit 1; }

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKG)

.PHONY: staticcheck
staticcheck: $(STATICCHECK) ## Run staticcheck
	$(STATICCHECK) $(PKG)

.PHONY: lint
lint: $(GOLANGCI) ## Run golangci-lint
	$(GOLANGCI) run $(PKG)

.PHONY: lint-config
lint-config: $(GOLANGCI) ## Validate .golangci.yml against the v2 schema
	$(call need,.golangci.yml,the lint config arrives with M1 (SPEC.md §12))
	$(GOLANGCI) config verify
	@printf '.golangci.yml is valid golangci-lint v2 config\n'

.PHONY: lint-fix
lint-fix: $(GOLANGCI) ## Run golangci-lint with --fix
	$(GOLANGCI) run --fix $(PKG)

.PHONY: check
check: tidy-check fmt-check vet lint staticcheck test cover-check ## Every gate CI enforces on Go code

.PHONY: ci
ci: check web-build bundle-check size-check release-check ## The full CI pipeline, locally

##@ Test

.PHONY: test
test: ## Run all tests with the race detector
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-short
test-short: ## Run fast tests only (-short, no race)
	$(GO) test -short -count=1 $(PKG)

.PHONY: test-run
test-run: ## Run one test — make test-run PKG=./internal/wire RUN=TestDecode
ifeq ($(strip $(RUN)),)
	@echo 'make: RUN is required, e.g. make test-run PKG=./internal/wire RUN=TestDecode' >&2; exit 1
else
	$(GO) test -race -count=1 -v -run '$(RUN)' $(PKG)
endif

.PHONY: fuzz
fuzz: ## Fuzz — make fuzz FUZZ_PKG=./internal/wire FUZZ=FuzzDecode FUZZTIME=30s
	$(GO) test -run '^$$' -fuzz '$(FUZZ)' -fuzztime $(FUZZTIME) $(FUZZ_PKG)

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -run '^$$' -bench . -benchmem $(PKG)

.PHONY: cover
cover: ## Run tests and write a coverage profile
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -count=1 -covermode=atomic -coverprofile=$(COVER_DIR)/coverage.out $(PKG)
	$(GO) tool cover -func=$(COVER_DIR)/coverage.out | tail -1

.PHONY: cover-html
cover-html: cover ## Open the coverage report in a browser
	$(GO) tool cover -html=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/coverage.html
	@printf 'wrote %s\n' '$(COVER_DIR)/coverage.html'

.PHONY: cover-check
cover-check: cover ## Enforce coverage thresholds (70% overall, 90% for wire & session)
	@if [[ -z "$$(git ls-files '*_test.go')" ]]; then \
	  echo 'coverage: no tests in the module yet — thresholds not enforced'; exit 0; \
	fi; \
	total=$$($(GO) tool cover -func=$(COVER_DIR)/coverage.out | awk '/^total:/ {gsub(/%/,"",$$3); print int($$3)}'); \
	  printf 'coverage: %s%% total (min %s%%)\n' "$$total" '$(COVER_MIN)'; \
	  fail=0; \
	  [[ "$$total" -ge "$(COVER_MIN)" ]] || { echo "make: total coverage below $(COVER_MIN)%" >&2; fail=1; }; \
	  for p in $(CRITICAL_PKGS); do \
	    pct=$$($(GO) tool cover -func=$(COVER_DIR)/coverage.out | awk -v p="internal/$$p/" '$$0 ~ p {gsub(/%/,"",$$3); s+=$$3; n++} END {if (n) print int(s/n); else print -1}'); \
	    if [[ "$$pct" -lt 0 ]]; then printf 'coverage: internal/%s has no coverage data yet — skipping\n' "$$p"; continue; fi; \
	    printf 'coverage: internal/%s %s%% (min %s%%)\n' "$$p" "$$pct" '$(COVER_MIN_CRIT)'; \
	    [[ "$$pct" -ge "$(COVER_MIN_CRIT)" ]] || { printf 'make: internal/%s below %s%%\n' "$$p" '$(COVER_MIN_CRIT)' >&2; fail=1; }; \
	  done; \
	  exit $$fail

.PHONY: smoke
smoke: build ## Run the integration smoke script
	$(call need,scripts/smoke.sh,the smoke script arrives with M13 (SPEC.md §14))
	DITTY_BIN=$(CURDIR)/$(BIN_DIR)/$(BINARY) bash scripts/smoke.sh

##@ Release

.PHONY: bundle-check
bundle-check: ## Fail if the embedded UI exceeds 400 KB gzipped (SPEC.md §10.4)
	$(call need,$(WEB_DIST),run make web-build first)
	@total=$$(find $(WEB_DIST) -type f -exec sh -c 'gzip -c "$$1" | wc -c' _ {} \; | awk '{s+=$$1} END {print int(s/1024)}'); \
	  printf 'bundle: %s KB gzipped (budget %s KB)\n' "$$total" '$(BUNDLE_BUDGET_KB)'; \
	  [[ "$$total" -le "$(BUNDLE_BUDGET_KB)" ]] || { echo "make: bundle budget exceeded" >&2; exit 1; }

.PHONY: size-check
size-check: build ## Fail if the binary exceeds 25 MB (SPEC.md §12 M13)
	@mb=$$(( $$(wc -c < $(BIN_DIR)/$(BINARY)) / 1048576 )); \
	  printf 'binary: %s MB (budget %s MB)\n' "$$mb" '$(BINARY_BUDGET_MB)'; \
	  [[ "$$mb" -le "$(BINARY_BUDGET_MB)" ]] || { echo "make: binary size budget exceeded" >&2; exit 1; }

.PHONY: release-check
release-check: $(GORELEASER) ## Validate the GoReleaser config
	$(call need,.goreleaser.yaml,GoReleaser config arrives with M13 (SPEC.md §15))
	$(GORELEASER) check

.PHONY: release-snapshot
release-snapshot: $(GORELEASER) ## Build a local snapshot release into dist/
	$(call need,.goreleaser.yaml,GoReleaser config arrives with M13 (SPEC.md §15))
	$(GORELEASER) release --snapshot --clean

.PHONY: docker
docker: ## Build the distroless container image
	$(call need,Dockerfile,the Dockerfile arrives with M13 (SPEC.md §15))
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
	  -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

.PHONY: docker-alpine
docker-alpine: ## Build the alpine container image
	$(call need,Dockerfile,the Dockerfile arrives with M13 (SPEC.md §15))
	docker build --target alpine --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION)-alpine .

##@ Docs

.PHONY: docs-dev
docs-dev: ## Run the docs site locally
	$(call need,$(DOCS_DIR)/package.json,the docs site arrives with M14 (SPEC.md §12))
	@cd $(DOCS_DIR) && npm run dev

.PHONY: docs-build
docs-build: ## Build the docs site
	$(call need,$(DOCS_DIR)/package.json,the docs site arrives with M14 (SPEC.md §12))
	@cd $(DOCS_DIR) && npm run build

##@ Housekeeping

.PHONY: clean
clean: ## Remove build and test artifacts
	@rm -rf $(BIN_DIR)/$(BINARY) $(DIST_DIR) $(COVER_DIR)
	$(GO) clean -testcache

.PHONY: clean-all
clean-all: clean ## Also remove installed tools, web build output and node_modules
	@rm -rf $(TOOLS_DIR) $(WEB_DIST) $(WEB_DIR)/node_modules $(DOCS_DIR)/node_modules
