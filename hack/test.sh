#!/bin/sh

source "$(dirname "${BASH_SOURCE}")/constants.sh"

go test -tags "${OS_GOFLAGS_TAGS_TEST-}" ./...
