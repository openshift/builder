IMAGE ?= docker.io/openshift/origin-docker-builder:latest
PROG  := openshift-builder

.PHONY: all build clean test build-image build-devel-image

all: generate build build-image

build:
	go build -o $(PROG) "./cmd"

build-image:
	docker build -t "$(IMAGE)" .

build-devel-image: build
	docker build -t "$(IMAGE)" -f Dockerfile-dev .

test:
	go test ./...

clean:
	rm -- "$(PROG)"
