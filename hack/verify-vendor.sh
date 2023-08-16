#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

_vendor_root="${SCRIPT_ROOT}/vendor"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

_vendor_root_tmp="${_tmp}/vendor"
mkdir -p "${_vendor_root_tmp}"
cp -a "${_vendor_root}"/* "${_vendor_root_tmp}"
cp "${SCRIPT_ROOT}"/go.mod "$_tmp"/go.mod
cp "${SCRIPT_ROOT}"/go.sum "$_tmp"/go.sum

bash "${SCRIPT_ROOT}/hack/update-vendor.sh"

if ! diff -Naupr "${_vendor_root}" "${_vendor_root_tmp}"; then
   echo "Your vendored results are different, Please run hack/update-vendor.sh"
fi

if ! diff -Naupr "${SCRIPT_ROOT}"/go.mod "${_tmp}"/go.mod; then
   echo "Your vendored results are different, Please run hack/update-vendor.sh"
fi

echo "${_vendor_root} up to date."