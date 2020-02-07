#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

NO_SCL=yes go test -tags "${OS_GOFLAGS_TAGS_TEST-}" ./...
