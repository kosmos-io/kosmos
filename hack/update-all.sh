#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# vendor should be updated first because we build code-gen tools from vendor.
bash "$REPO_ROOT/hack/update-codegen.sh"
bash "$REPO_ROOT/hack/update-crds.sh"

