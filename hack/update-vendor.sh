#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

echo "running 'go mod tidy'"
go mod tidy

echo "running 'go mod vendor'"
go mod vendor
