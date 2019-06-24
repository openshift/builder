IMAGE ?= docker.io/openshift/origin-docker-builder
TAG ?= latest
PROG  := openshift-builder
CONTAINER_ENGINE := $(shell command -v podman 2> /dev/null || echo docker)

.PHONY: all build build-image build-devel-image clean test verify

all: build build-image test verify

build:
	hack/build.sh

build-image:
	rm -f "$(PROG)"
	${CONTAINER_ENGINE} build -t "$(IMAGE):$(TAG)" .

build-devel-image: build
	${CONTAINER_ENGINE} build -t "$(IMAGE):$(TAG)" -f Dockerfile-dev .

test:
	hack/test.sh

verify:
	hack/verify.sh

clean:
	rm -- "$(PROG)"
