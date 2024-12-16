#!/usr/bin/env bash

# This library holds utility functions for building
# and placing Golang binaries.

# os::build::build_binary builds the openshift-builder binary
function os::build::build_binary() {
  # Fetch the version.
  local version_ldflags
  version_ldflags=$(os::build::ldflags)
  go build -mod=vendor -ldflags "${version_ldflags}" -tags "${OS_GOFLAGS_TAGS-}" -o openshift-builder ./cmd
}

readonly -f os::build::build_binary
