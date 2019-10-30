#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

cd "$(dirname "${BASH_SOURCE}")/.."
#source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

#os::golang::verify_glide_version

# fail early if any of the staging dirs is checked out
for pkg in "$GOPATH/src/k8s.io/kubernetes/staging/src/k8s.io/"*; do
  dir=$(basename $pkg)
  if [ -d "$GOPATH/src/k8s.io/$dir" ]; then
    echo "Conflicting $GOPATH/src/k8s.io/$dir found. Please remove from GOPATH." 1>&2
    exit 1
  fi
done

export GO111MODULE=on
go mod tidy
go mod vendor
go mod verify
