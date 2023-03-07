IMAGE ?= docker.io/openshift/origin-docker-builder
TAG ?= latest
PROG  := openshift-builder
CONTAINER_ENGINE := $(shell command -v podman 2> /dev/null || echo docker)


all: build build-image test verify
.PHONY: all

build: ## Build the executable. Example: make build
	hack/build.sh
.PHONY: build

build-image: ## Build the images and push them to the remote registry. Example: make build-image
	rm -f "$(PROG)"
	${CONTAINER_ENGINE} build -t "$(IMAGE):$(TAG)" .
.PHONY: build-image

build-devel-image: build
	${CONTAINER_ENGINE} build -t "$(IMAGE):$(TAG)" -f Dockerfile.dev .
.PHONY: build-devel-image

test: ## Run unit tests. Example: make test
	hack/test.sh
.PHONY: test

verify-gofmt: ## Run gofmt verifications. Example: make verify-gofmt
	hack/verify-gofmt.sh
.PHONY: verify-gofmt

verify-imports: ## Run import verifications. Example: make verify-imports
	hack/verify-imports.sh
.PHONY: verify-imports

verify: verify-gofmt verify-imports ## Run verifications. Example: make verify
.PHONY: verify

imports: ## Organize imports in go files using openshift-goimports. Example: make imports
	go run ./vendor/github.com/openshift-eng/openshift-goimports/ -m github.com/openshift/builder
.PHONY: imports

vendor: ## Vendor Go dependencies. Example: make vendor
	go mod tidy
	go mod vendor
.PHONY: vendor

clean: ## Clean up the workspace. Example: make clean
	rm -- "$(PROG)"
.PHONY: clean

help: ## Print this help. Example: make help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
.PHONY: help
