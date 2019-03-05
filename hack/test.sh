#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

go test -tags "${OS_GOFLAGS_TAGS_TEST-}" ./...
