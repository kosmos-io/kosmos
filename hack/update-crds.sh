#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)

echo "Generating CRDs With controller-gen"
GO11MODULE=on go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.0

GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

controller-gen crd paths=./pkg/apis/kosmos/... output:crd:dir="${REPO_ROOT}/deploy/crds"

#go run "${REPO_ROOT}/hack/generate/generate.go"
