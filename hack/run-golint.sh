#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Function to install go-lint if it is not already installed
echo "go lint is not installed. Installing..."

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
bash "${SCRIPT_ROOT}/hack/install-golint.sh -b ${GOPATH}/bin v1.54.2"

echo "go lint is installed and the version is correct."
