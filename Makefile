# Rancher MCP Server Makefile

.DEFAULT_GOAL := help

PACKAGE = $(shell go list -m)
VERSION_PKG = $(PACKAGE)/pkg/core/version
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_VERSION := $(shell git describe --tags --always --dirty)
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BINARY_NAME = rancher-mcp-server
LD_FLAGS = -s -w \
	-X '$(VERSION_PKG).GitCommit=$(GIT_COMMIT)' \
	-X '$(VERSION_PKG).Version=$(GIT_VERSION)' \
	-X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)'
COMMON_BUILD_ARGS = -ldflags "$(LD_FLAGS)"

GOLANGCI_LINT = $(shell pwd)/_output/tools/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.2.2
GOLANGCI_LINT_PKG = github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GO_VERSION = $(shell go env GOVERSION)

# NPM version should not append the -dirty flag
NPM_VERSION ?= $(shell echo $(shell git describe --tags --always) | sed 's/^v//')
OSES = darwin linux windows
ARCHS = amd64 arm64

CLEAN_TARGETS :=
CLEAN_TARGETS += '$(BINARY_NAME)'
CLEAN_TARGETS += $(foreach os,$(OSES),$(foreach arch,$(ARCHS),$(BINARY_NAME)-$(os)-$(arch)$(if $(findstring windows,$(os)),.exe,)))
CLEAN_TARGETS += $(foreach os,$(OSES),$(foreach arch,$(ARCHS),./npm/$(BINARY_NAME)-$(os)-$(arch)/bin/))
CLEAN_TARGETS += ./npm/rancher-mcp-server/.npmrc ./npm/rancher-mcp-server/LICENSE ./npm/rancher-mcp-server/README.md
CLEAN_TARGETS += $(foreach os,$(OSES),$(foreach arch,$(ARCHS),./npm/$(BINARY_NAME)-$(os)-$(arch)/.npmrc))

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9\/\.-]+:.*?##/ { printf "  \033[36m%-21s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean up all build artifacts
	rm -rf $(CLEAN_TARGETS)

.PHONY: build
build: verify-version-contract tidy ## Build the project
	CGO_ENABLED=0 go build $(COMMON_BUILD_ARGS) -o $(BINARY_NAME) ./cmd/rancher-mcp-server

.PHONY: build-all-platforms
build-all-platforms: verify-version-contract tidy ## Build the project for all platforms
	$(foreach os,$(OSES),$(foreach arch,$(ARCHS), \
		GOOS=$(os) GOARCH=$(arch) go build $(COMMON_BUILD_ARGS) -o $(BINARY_NAME)-$(os)-$(arch)$(if $(findstring windows,$(os)),.exe,) ./cmd/rancher-mcp-server; \
	))

.PHONY: npm-copy-binaries
npm-copy-binaries: build-all-platforms ## Copy the binaries to each npm package
	$(foreach os,$(OSES),$(foreach arch,$(ARCHS), \
		EXECUTABLE=./$(BINARY_NAME)-$(os)-$(arch)$(if $(findstring windows,$(os)),.exe,); \
		DIRNAME=$(BINARY_NAME)-$(os)-$(arch); \
		mkdir -p ./npm/$$DIRNAME/bin; \
		cp $$EXECUTABLE ./npm/$$DIRNAME/bin/; \
	))

.PHONY: npm-publish
npm-publish: npm-copy-binaries ## Publish the npm packages
	$(foreach os,$(OSES),$(foreach arch,$(ARCHS), \
		DIRNAME="$(BINARY_NAME)-$(os)-$(arch)"; \
		cd npm/$$DIRNAME; \
		echo '//registry.npmjs.org/:_authToken=$(NPM_TOKEN)' >> .npmrc; \
		jq '.version = "$(NPM_VERSION)"' package.json > tmp.json && mv tmp.json package.json; \
		npm publish --access=public; \
		cd ../..; \
	))
	cp README.md LICENSE ./npm/rancher-mcp-server/
	echo '//registry.npmjs.org/:_authToken=$(NPM_TOKEN)' >> ./npm/rancher-mcp-server/.npmrc
	jq '.version = "$(NPM_VERSION)"' ./npm/rancher-mcp-server/package.json > tmp.json && mv tmp.json ./npm/rancher-mcp-server/package.json; \
	jq '.optionalDependencies |= with_entries(.value = "$(NPM_VERSION)")' ./npm/rancher-mcp-server/package.json > tmp.json && mv tmp.json ./npm/rancher-mcp-server/package.json; \
	cd npm/rancher-mcp-server && npm publish --access=public

.PHONY: test
test: ## Run the tests
	go test -count=1 -v ./...

.PHONY: verify-version-contract
verify-version-contract: ## Verify build metadata injection targets
	./scripts/verify-version-contract.sh

.PHONY: verify-version-output
verify-version-output: build ## Verify built binary version output
	@binary=./$(BINARY_NAME); \
	if [ -f "./$(BINARY_NAME).exe" ]; then binary=./$(BINARY_NAME).exe; fi; \
	./scripts/verify-version-output.sh "$$binary" "$(GIT_VERSION)" "$(GIT_COMMIT)" "$(BUILD_DATE)"

.PHONY: format
format: ## Format the code
	go fmt ./...

.PHONY: tidy
tidy: ## Tidy up the go modules
	go mod tidy

.PHONY: golangci-lint
golangci-lint: ## Build and install golangci-lint with the local Go toolchain
	@[ -x $(GOLANGCI_LINT) ] && $(GOLANGCI_LINT) version 2>/dev/null | grep -q "built with $(GO_VERSION)" || { \
		set -e; \
		mkdir -p $(shell dirname $(GOLANGCI_LINT)); \
		GOBIN=$(shell dirname $(GOLANGCI_LINT)) go install $(GOLANGCI_LINT_PKG)@$(GOLANGCI_LINT_VERSION); \
	}

.PHONY: lint
lint: golangci-lint ## Lint the code
	$(GOLANGCI_LINT) run --verbose

.PHONY: version
version: ## Show version information
	./$(BINARY_NAME) version
