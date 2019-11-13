#!/usr/bin/env bash

# This library holds utility functions for building
# and placing Golang binaries.

# os::build::build_binary builds the openshift-builder binary
function os::build::build_binary() {
  # Fetch the version.
  local version_ldflags
  version_ldflags=$(os::build::ldflags)
  # test for go module support
  local go_build
  if go help mod >/dev/null 2>&1 ; then
    go_build="env GO111MODULE=on go build -mod=vendor"
  else
    go_build="go build"
  fi
  $go_build -ldflags "${version_ldflags}" -tags "${OS_GOFLAGS_TAGS-}" -o openshift-builder ./cmd
}
readonly -f os::build::build_binary
