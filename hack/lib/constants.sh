#!/usr/bin/env bash

# This script provides constants for the Golang binary build process

readonly OS_GO_PACKAGE=github.com/openshift/builder

readonly OS_BUILD_ENV_GOLANG="${OS_BUILD_ENV_GOLANG:-1.10}"
readonly OS_REQUIRED_GO_VERSION="go${OS_BUILD_ENV_GOLANG}"
readonly OS_GLIDE_MINOR_VERSION="13"
readonly OS_REQUIRED_GLIDE_VERSION="0.$OS_GLIDE_MINOR_VERSION"

readonly OS_GOFLAGS_TAGS="include_gcs include_oss containers_image_openpgp containers_image_ostree_stub exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_zfs"
readonly OS_GOFLAGS_TAGS_TEST="containers_image_openpgp containers_image_ostree_stub exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_zfs"
