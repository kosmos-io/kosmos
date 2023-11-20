#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$(dirname "${BASH_SOURCE[0]}")"
source "${ROOT}/util.sh"

# Make sure go exists and the go version is a viable version.
if command -v go &> /dev/null; then
  util::verify_go_version
else
  source "$(dirname "${BASH_SOURCE[0]}")/install-go.sh"
fi

# Make sure docker exists
util::cmd_must_exist "docker"

# install kind and kubectl
kind_version=v0.20.0
echo -n "Preparing: 'kind' existence check - "
if util::cmd_exist kind; then
  echo "passed"
else
  echo "not pass"
  util::install_tools "sigs.k8s.io/kind" $kind_version
fi
# get arch name and os name in bootstrap
BS_ARCH=$(go env GOARCH)
BS_OS=$(go env GOOS)
# check arch and os name before installing
util::install_environment_check "${BS_ARCH}" "${BS_OS}"
echo -n "Preparing: 'kubectl' existence check - "
if util::cmd_exist kubectl; then
  echo "passed"
else
  echo "not pass"
  util::install_kubectl "" "${BS_ARCH}" "${BS_OS}"
fi
