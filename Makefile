# VERSION defines the project version for the bundle.
include $(CURDIR)/versions.mk

GO      := go
PKG     := ./...
CMD_DIR := ./cmd/rbln-npu-feature-discovery

LOCALBIN ?= $(CURDIR)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

GOFUMPT := $(LOCALBIN)/gofumpt
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

##@ Container Images

# Container build configuration
CONTAINER_TOOL ?= docker

DOCKERFILE ?= $(CURDIR)/Dockerfile
PUSH_ON_BUILD ?= false
BUILD_MULTI_PLATFORM ?= false
DOCKER_BUILD_OPTIONS ?= --output=type=image,push=$(PUSH_ON_BUILD)
BUILDX =

# PLATFORM can be set to a single platform (e.g. linux/amd64, linux/arm64)
# to override the default multi-platform logic.
PLATFORM ?=

ifneq ($(PLATFORM),)
	DOCKER_BUILD_PLATFORM_OPTIONS := --platform=$(PLATFORM)
	BUILDX = buildx
else ifeq ($(BUILD_MULTI_PLATFORM),true)
	DOCKER_BUILD_PLATFORM_OPTIONS ?= --platform=linux/amd64,linux/arm64
	BUILDX = buildx
else
	DOCKER_BUILD_PLATFORM_OPTIONS := --platform=linux/amd64
endif

# Image registry and naming configuration
REGISTRY ?= docker.io/rebellions
IMAGE_NAME ?= $(REGISTRY)/rbln-npu-feature-discovery

# Image tagging configuration
IMAGE_TAG ?= $(VERSION)
IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)

VERSION_LDFLAG := -X github.com/rebellions-sw/rbln-npu-feature-discovery/internal/cmd.version=$(VERSION)

##@ Release versioning

# The release version lives in versions.mk and is mirrored into the static
# manifest, which is what users apply. `bump-version` rewrites both together;
# `verify-version` fails if they ever drift apart. Overriding VERSION turns the
# same check into "does the tree match this tag?", which is how release.yaml
# uses it.
MANIFEST := $(CURDIR)/deployments/static/npu-feature-discovery-daemonset.yaml

.PHONY: bump-version
bump-version: # Set the release version everywhere (make bump-version NEW_VERSION=vX.Y.Z)
	@[ -n "$(NEW_VERSION)" ] || { echo "usage: make bump-version NEW_VERSION=vX.Y.Z"; exit 1; }
	@echo "$(NEW_VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$$' \
		|| { echo "NEW_VERSION must look like vX.Y.Z, got '$(NEW_VERSION)'"; exit 1; }
	@sed -i.bak -E 's|^VERSION[[:space:]]*\?=.*|VERSION ?= $(NEW_VERSION)|' $(CURDIR)/versions.mk
	@sed -i.bak -E \
		-e 's|^([[:space:]]*)app\.kubernetes\.io/version:.*|\1app.kubernetes.io/version: $(NEW_VERSION)|' \
		-e 's|^([[:space:]]*- image:[[:space:]]*[^:]+):.*|\1:$(NEW_VERSION)|' \
		$(MANIFEST)
	@rm -f $(CURDIR)/versions.mk.bak $(MANIFEST).bak
	@$(MAKE) --no-print-directory verify-version VERSION=$(NEW_VERSION)

.PHONY: verify-version
verify-version: # Fail if versions.mk and the static manifest disagree on the version
	@rc=0; \
	declared=$$(sed -n 's|^VERSION[[:space:]]*?=[[:space:]]*\(.*\)$$|\1|p' $(CURDIR)/versions.mk); \
	image=$$(sed -n 's|^[[:space:]]*- image:[[:space:]]*[^:]*:\(.*\)$$|\1|p' $(MANIFEST)); \
	labels=$$(sed -n 's|^[[:space:]]*app\.kubernetes\.io/version:[[:space:]]*\(.*\)$$|\1|p' $(MANIFEST)); \
	[ -n "$$image" ] || { echo "no image tag found in $(MANIFEST)"; rc=1; }; \
	[ -n "$$labels" ] || { echo "no app.kubernetes.io/version found in $(MANIFEST)"; rc=1; }; \
	for found in $$declared $$image $$labels; do \
		[ "$$found" = "$(VERSION)" ] || { echo "found $$found, expected $(VERSION)"; rc=1; }; \
	done; \
	[ $$rc -eq 0 ] || { \
		echo "Version drift. Run: make bump-version NEW_VERSION=$(VERSION)"; \
		exit 1; \
	}; \
	echo "Version $(VERSION) consistent across versions.mk and the static manifest."

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build -ldflags "$(VERSION_LDFLAG)" -o bin/$(BINARY) $(CMD_DIR)

.PHONY: clean
clean:
	rm -rf bin

.PHONY: test
test:
	$(GO) test $(PKG)

.PHONY: verify-deps
verify-deps:
	@echo "Verifying that all Go dependencies and vendor files are consistent..."
	go mod verify
	@echo "Go mod verify completed."
	go mod tidy
	@git diff --exit-code -- go.sum go.mod
	@echo "Go mod tidy completed."
	go mod vendor
	@git diff --exit-code -- vendor
	@echo "Go vendor completed."

.PHONY: fmt
fmt: ensure-gofumpt ## Run go fmt against code.
	@echo "Running go fmt..."
	@out="$$( $(GOFUMPT) -l . )"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		echo "Formatting issues found"; \
		exit 1; \
	fi
	@echo "Go fmt completed."

.PHONY: fmt-fix
fmt-fix:
	$(GOFUMPT) -l -w .

.PHONY: ensure-gofumpt
ensure-gofumpt:
	@echo "Ensuring gofumpt is installed..."
	GOBIN=$(LOCALBIN) GO111MODULE=on $(GO) install mvdan.cc/gofumpt@latest
	@echo "gofumpt installation complete."

.PHONY: vet
vet: # Run go vet against code.
	@echo "Running go vet..."
	go vet ./...
	@echo "Go vet completed."


.PHONY: lint
lint: ensure-golangci-lint # Run golangci-lint linter
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT) run
	@echo "golangci-lint completed."

.PHONY: lint-fix
lint-fix: ensure-golangci-lint # Run golangci-lint linter and perform fixes
	GOTOOLCHAIN=$(GOLANGCI_LINT_TOOLCHAIN) $(GOLANGCI_LINT) run --fix

.PHONY: build-image
build-image: # Build the RBLN npu feature discovery image
	DOCKER_BUILDKIT=1 \
		$(CONTAINER_TOOL) $(BUILDX) build --pull \
		$(DOCKER_BUILD_OPTIONS) \
		$(DOCKER_BUILD_PLATFORM_OPTIONS) \
		--tag $(IMAGE) \
		--build-arg VERSION="$(VERSION)" \
		--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
		--file $(DOCKERFILE) $(CURDIR)

.PHONY: scan-image
scan-image: build-image # Scan the built image for fixable HIGH/CRITICAL vulnerabilities
	@command -v trivy >/dev/null || { \
		echo "trivy not found: https://trivy.dev/latest/getting-started/installation/"; \
		exit 1; \
	}
	trivy image --scanners vuln,secret --severity HIGH,CRITICAL --ignore-unfixed \
		--ignorefile $(CURDIR)/.trivyignore.yaml --exit-code 1 $(IMAGE)

.PHONY: ensure-golangci-lint
ensure-golangci-lint:
	@echo "Ensuring golangci-lint is installed..."
	GOBIN=$(LOCALBIN) GO111MODULE=on $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "golangci-lint installation complete."

.PHONY: code-check
code-check: vet fmt lint verify-deps verify-version

.PHONY: pre-commit-install
pre-commit-install:
	pre-commit install

.PHONY: pre-commit-run
pre-commit-run:
	pre-commit run --all-files
