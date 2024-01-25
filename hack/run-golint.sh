#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Function to install go-lint if it is not already installed
echo "go lint is not installed. Installing..."

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")
#GOLINT_PATH=$(go env GOPATH)/bin
bash "${SCRIPT_ROOT}/install-golint.sh -b /home/runner/go/bin v1.54.2"

echo "go lint is installed and the version is correct."
