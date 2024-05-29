#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

VERSION=${VERSION:-"latest"}

function usage() {
    echo "Usage:"
    echo "    hack/local-down-kosmos_kubenest.sh [-k] [-h]"
    echo "Args:"
    echo "    k: keep the local images"
    echo "    h: print help information"
}

keep_images="false"
while getopts 'kh' OPT; do
    case $OPT in
        k) keep_images="true";;
        h)
          usage
          exit 0
          ;;
        ?)
          usage
          exit 1
          ;;
    esac
done

KUBE_NEST_CLUSTER_NAME=${KUBE_NEST_CLUSTER_NAME:-"kubenest-cluster"}

#step1 remove kind clusters
echo -e "\nStart removing kind clusters"
kind delete cluster --name "${KUBE_NEST_CLUSTER_NAME}"
echo "Remove kind clusters successfully."

ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CLUSTER_DIR="${ROOT}/environments"
source "${ROOT}/hack/cluster.sh"

#step2. remove kubeconfig
echo -e "\nStart removing kubeconfig, kindconfig, cailcoconfig"
KUBE_NEST_CLUSTER_CONFIG=${KUBE_NEST_CLUSTER_CONFIG:-"${CLUSTER_DIR}/${KUBE_NEST_CLUSTER_NAME}"}
delete_cluster "${KUBE_NEST_CLUSTER_CONFIG}" "${KUBE_NEST_CLUSTER_CONFIG}"

echo "Remove cluster configs successfully."

#step3. remove docker images
echo -e "\nStart removing images"
registry="ghcr.io/kosmos-io"
images=(
"${registry}/virtual-cluster-operator:${VERSION}"
"${registry}/node-agent:${VERSION}"
)
if [[ "${keep_images}" == "false" ]] ; then
  for ((i=0;i<${#images[*]};i++)); do
    docker rmi ${images[i]} || true
  done
  echo "Remove images successfully."
else
  echo "Skip removing images as required."
fi

echo -e "\nLocal Kubenest is removed successfully."
