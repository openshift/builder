IMAGE ?= docker.io/openshift/origin-docker-builder:latest
PROG  := openshift-builder

.PHONY: all build build-image build-devel-image clean test verify

all: build build-image test verify

build:
	go build -o $(PROG) "./cmd"

build-image:
	docker build -t "$(IMAGE)" .

build-devel-image: build
	docker build -t "$(IMAGE)" -f Dockerfile-dev .

test:
	go test ./...

verify:
	hack/verify.sh

clean:
	rm -- "$(PROG)"
