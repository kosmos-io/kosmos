#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script builds go components.
# You can set the platform to build with BUILD_PLATFORMS, with format: `<os>/<arch>`
# And binaries will be put in `_output/<os>/<arch>/`
#
# Usage:
#   hack/build.sh <target>
# Args:
#   $1:              target to build
# Environments:
#   BUILD_PLATFORMS: platforms to build. You can set one or more platforms separated by comma.
#                    e.g.: linux/amd64,linux/arm64
#   LDFLAGS          pass to the `-ldflags` parameter of go build
# Examples:
#   hack/build.sh clusterlink-controllermanager
#   BUILD_PLATFORMS=linux/amd64,linux/arm64 hack/build.sh clusterlink-controllermanager

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

LDFLAGS="$(util::version_ldflags "${VERSION:-}") ${LDFLAGS:-}"

function build_binary() {
  local -r target=$1

  IFS="," read -ra platforms <<< "${BUILD_PLATFORMS:-}"
  if [[ ${#platforms[@]} -eq 0 ]]; then
    platforms=("$(util:host_platform)")
  fi

  for platform in "${platforms[@]}"; do
    echo "!!! Building ${target} for ${platform}:"
    build_binary_for_platform "${target}" "${platform}"
  done
}


function build_binary_for_platform() {
  local -r target=$1
  local -r platform=$2
  local -r os=${platform%/*}
  local -r arch=${platform##*/}

  local target_pkg="${KOSMOS_GO_PACKAGE}/$(util::get_target_source "$target")"
  set -x
  CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
      -ldflags "${LDFLAGS:-}" \
      -o "_output/bin/${platform}/$target" \
      "${target_pkg}"
  # copy file for node-agent
  if [[ "${target}" == "node-agent" ]]; then
    echo "${REPO_ROOT}/hack/node-agent" "_output/bin/${platform}/$target"
    mkdir -p "_output/bin/${platform}/agent"
    cp "${REPO_ROOT}/hack/node-agent"/* "_output/bin/${platform}/agent"
    cp "_output/bin/${platform}/$target" "_output/bin/${platform}/agent"
    cd "_output/bin/${platform}" && tar -czvf "agent.tar.gz" "agent"
  fi
  set +x
}

build_binary "$@"
