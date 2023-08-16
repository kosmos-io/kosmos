#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
_generated_crd="${SCRIPT_ROOT}/deploy/crds"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

_generated_crd_tmp="${_tmp}/crds"
mkdir -p "${_tmp}"
cp -a "${_generated_crd}" "${_generated_crd_tmp}"

"${SCRIPT_ROOT}/hack/update-codegen.sh"

if ! diff -Naupr "${_generated_crd}" "${_generated_crd_tmp}"; then
   echo "Your generated code results are different, Please run hack/update-codegen.sh"
fi
