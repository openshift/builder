#!/usr/bin/env bash

# This library holds utility functions for building
# and placing Golang binaries.

# os::build::modvendorflag evaluates to either -mod=version or nothing
function os::build::modvendorflag() {
  tmpfile=${TMPDIR:-/tmp}/test${RANDOM}.go
  trap 'rm -f "${tmpfile}"' EXIT
  echo 'package main; func main() {}' > "${tmpfile}"
  if go run -mod=vendor "${tmpfile}" ; then
    echo "-mod=vendor"
  fi
}
readonly -f os::build::modvendorflag

# os::build::build_binary builds the openshift-builder binary
function os::build::build_binary() {
  # Fetch the version.
  local version_ldflags
  version="$(git describe --tags --always --dirty)"
  repo_path="github.com/openshift/builder"
  version_ldflags="-X ${repo_path}/pkg/version.Version=${version}"
  # Fetch additional build flags.
  local mod_vendor_flag
  mod_vendor_flag=$(os::build::modvendorflag)
  go build ${mod_vendor_flag} -ldflags "${version_ldflags}" -tags "${OS_GOFLAGS_TAGS-}" -o openshift-builder ./cmd
}
readonly -f os::build::build_binary
