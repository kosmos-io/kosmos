#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

echo "Generating with deepcopy-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen
export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1,github.com/kosmos.io/kosmos/pkg/apis/config,github.com/kosmos.io/kosmos/pkg/apis/config/v1beta1" \
  --output-base="${REPO_ROOT}" \
  --output-package="pkg/apis/kosmos/v1alpha1,pkg/apis/config,pkg/apis/config/v1beta1" \
  --output-file-base=zz_generated.deepcopy

echo "Generating with conversion-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/conversion-gen
conversion-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1,github.com/kosmos.io/kosmos/pkg/apis/config,github.com/kosmos.io/kosmos/pkg/apis/config/v1beta1" \
  --output-base="${REPO_ROOT}" \
  --output-package="pkg/apis/kosmos/v1alpha1,pkg/apis/config,pkg/apis/config/v1beta1" \
  --output-file-base=zz_generated.conversion

echo "Generating with defaults-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/defaulter-gen
defaulter-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1,github.com/kosmos.io/kosmos/pkg/apis/config,github.com/kosmos.io/kosmos/pkg/apis/config/v1beta1" \
  --output-base="${REPO_ROOT}" \
  --output-package="pkg/apis/kosmos/v1alpha1,pkg/apis/config,pkg/apis/config/v1beta1" \
  --output-file-base=zz_generated.defaults

echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1" \
  --output-base="${REPO_ROOT}" \
  --output-package="pkg/apis/kosmos/v1alpha1" \
  --output-file-base=zz_generated.register



echo "Generating with client-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1" \
  --output-base="${REPO_ROOT}" \
  --output-package="github.com/kosmos.io/kosmos/pkg/generated/clientset" \
  --clientset-name=versioned



echo "Generating with lister-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/lister-gen
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1" \
  --output-base="${REPO_ROOT}" \
  --output-package="github.com/kosmos.io/kosmos/pkg/generated/listers"

echo "Generating with informer-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/informer-gen
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1,sigs.k8s.io/mcs-api/pkg/apis/v1alpha1" \
  --versioned-clientset-package="github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned" \
  --listers-package="github.com/kosmos.io/kosmos/pkg/generated/listers" \
  --output-base="${REPO_ROOT}" \
  --output-package="github.com/kosmos.io/kosmos/pkg/generated/informers"

echo "Generating with default-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/defaulter-gen
defaulter-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1" \
  --output-base="${REPO_ROOT}" \
  --output-package="github.com/kosmos.io/kosmos/pkg/generated/default"

echo "Generating with openapi-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/openapi-gen
openapi-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1" \
  --input-dirs "k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version" \
  --output-base="${REPO_ROOT}" \
  --output-package "github.com/kosmos.io/kosmos/pkg/generated/openapi" \
  -O zz_generated.openapi
