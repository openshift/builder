#!/usr/bin/env bash

# This library holds utility functions for determining
# product versions from Git repository state.

# os::build::version::get_vars loads the standard version variables as
# ENV vars
function os::build::version::get_vars() {
	if [[ -n "${OS_VERSION_FILE-}" ]]; then
		if [[ -f "${OS_VERSION_FILE}" ]]; then
			source "${OS_VERSION_FILE}"
			return
		fi
		if [[ ! -d "${OS_ROOT}/.git" ]]; then
			os::log::fatal "No version file at ${OS_VERSION_FILE}"
		fi
		os::log::warning "No version file at ${OS_VERSION_FILE}, falling back to git versions"
	fi
	os::build::version::git_vars
	os::build::version::buildah_vars
}
readonly -f os::build::version::get_vars

# os::build::version::git_vars looks up the current Git vars if they have not been calculated.
function os::build::version::git_vars() {
	if [[ -n "${OS_GIT_VERSION-}" ]]; then
		return 0
 	fi

	local git=(git --work-tree "${OS_ROOT}")

	OS_GIT_COMMIT="${SOURCE_GIT_COMMIT:-${OS_GIT_COMMIT-}}"
	if [[ -z "${OS_GIT_COMMIT}" ]]; then
		OS_GIT_COMMIT=$("${git[@]}" rev-parse --short "HEAD^{commit}" 2>/dev/null)
		if [[ -z ${OS_GIT_TREE_STATE-} ]]; then
			# Check if the tree is dirty.  default to dirty
			if git_status=$("${git[@]}" status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
				OS_GIT_TREE_STATE="clean"
			else
				OS_GIT_TREE_STATE="dirty"
			fi
		fi
	fi

	OS_GIT_VERSION="${BUILD_VERSION:-${OS_GIT_VERSION-}}"
	# Use git describe to find the version based on annotated tags.
	if [[ -n "${OS_GIT_VERSION}" ]] || OS_GIT_VERSION=$("${git[@]}" describe --long --tags --abbrev=7 --match 'v[0-9]*' "${OS_GIT_COMMIT}^{commit}" 2>/dev/null); then
		# Try to match the "git describe" output to a regex to try to extract
		# the "major" and "minor" versions and whether this is the exact tagged
		# version or whether the tree is between two tagged versions.
		if [[ "${OS_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(\.[0-9]+)*([-].*)?$ ]]; then
			OS_GIT_MAJOR=${BASH_REMATCH[1]}
			OS_GIT_MINOR=${BASH_REMATCH[2]}
			OS_GIT_PATCH=${BASH_REMATCH[3]}
			if [[ -n "${BASH_REMATCH[5]}" ]]; then
				OS_GIT_MINOR+="+"
			fi
		fi

		# This translates the "git describe" to an actual semver.org
		# compatible semantic version that looks something like this:
		#   v1.1.0-alpha.0.6+84c76d1-345
		OS_GIT_VERSION=$(echo "${OS_GIT_VERSION}" | sed "s/-\([0-9]\{1,\}\)-g\([0-9a-f]\{7,40\}\)$/\+\2-\1/")
		# If this is an exact tag, remove the last segment.
		OS_GIT_VERSION=$(echo "${OS_GIT_VERSION}" | sed "s/-0$//")
		if [[ "${OS_GIT_TREE_STATE}" == "dirty" ]]; then
			# git describe --dirty only considers changes to existing files, but
			# that is problematic since new untracked .go files affect the build,
			# so use our idea of "dirty" from git status instead.
			OS_GIT_VERSION+="-dirty"
		fi
	fi
}
readonly -f os::build::version::git_vars

function os::build::version::buildah_vars() {
	if [[ -n "${OS_BUILDAH_VERSION-}" ]]; then
		return 0
	fi
	OS_BUILDAH_VERSION=$(go list -mod=mod -m -f '{{.Version}}' github.com/containers/buildah)
}
readonly -f os::build::version::buildah_vars

# os::build::version::save_vars saves the environment flags to $1
function os::build::version::save_vars() {
	cat <<EOF
OS_GIT_COMMIT='${OS_GIT_COMMIT-}'
OS_GIT_TREE_STATE='${OS_GIT_TREE_STATE-}'
OS_GIT_VERSION='${OS_GIT_VERSION-}'
OS_GIT_MAJOR='${OS_GIT_MAJOR-}'
OS_GIT_MINOR='${OS_GIT_MINOR-}'
OS_GIT_PATCH='${OS_GIT_PATCH-}'
EOF
}
readonly -f os::build::version::save_vars

# os::build::ldflags calculates the -ldflags argument for building OpenShift
function os::build::ldflags() {
  # Run this in a subshell to prevent settings/variables from leaking.
  set -o errexit
  set -o nounset
  set -o pipefail

  cd "${OS_ROOT}"

  os::build::version::get_vars

  local buildDate="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  declare -a ldflags=()

  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.majorFromGit" "${OS_GIT_MAJOR}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.minorFromGit" "${OS_GIT_MINOR}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.versionFromGit" "${OS_GIT_VERSION}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.commitFromGit" "${OS_GIT_COMMIT}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.buildDate" "${buildDate}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.buildahVersion" "${OS_BUILDAH_VERSION}"))

  # The -ldflags parameter takes a single string, so join the output.
  echo "${ldflags[*]-}"
}
readonly -f os::build::ldflags

# os::build::ldflag constructs an argument for golang -ldflags
function os::build::ldflag() {
  local key=${1}
  local val=${2}

  echo "-X ${key}=${val}"
}
readonly -f os::build::ldflag
