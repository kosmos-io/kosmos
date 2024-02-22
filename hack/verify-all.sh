#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# Show progress
set -x

# Orders are determined by two factors:
# (1) Less Execution time item should be executed first.
# (2) More likely to fail item should be executed first.
bash "$REPO_ROOT/hack/verify-vendor.sh"
bash "$REPO_ROOT/hack/verify-crds.sh"
bash "$REPO_ROOT/hack/verify-codegen.sh"

set +x
