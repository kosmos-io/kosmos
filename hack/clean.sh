#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script clean builds tmp files and go cache.
#
# Usage:
#   hack/clean.sh
# Environments:
#   BUILD_PLATFORMS: platforms to build. You can set one or more platforms separated by comma.
#                    e.g.: linux/amd64,linux/arm64
# Examples:
#   hack/clean.sh

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

function clean() {
  echo "${REPO_ROOT}"/_tmp "${REPO_ROOT}"/_output
  rm -rf "${REPO_ROOT}"/_tmp "${REPO_ROOT}"/_output
  util::go_clean_cache
}

clean