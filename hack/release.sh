#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

LDFLAGS="$(util::version_ldflags) ${LDFLAGS:-}"

function release_binary() {
  local -r target=$1
  local -r os=$2
  local -r arch=$3

  release_binary_for_platform "${target}" "${os}" "${arch}"
}

function release_binary_for_platform() {
  local -r target=$1
  local -r os=$2
  local -r arch=$3
  local -r platform="${os}-${arch}"

  local target_pkg="${KOSMOS_GO_PACKAGE}/$(util::get_target_source "$target")"
  set -x
  CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
      -ldflags "${LDFLAGS:-}" \
      -o "_output/release/$target-${platform}" \
      "${target_pkg}"
  set +x
}

release_binary "$@"
