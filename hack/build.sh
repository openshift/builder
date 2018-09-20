#!/bin/sh

source "$(dirname "${BASH_SOURCE}")/constants.sh"

go build -tags "${OS_GOFLAGS_TAGS-}" -o openshift-builder ./cmd
