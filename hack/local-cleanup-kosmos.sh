#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
    echo "Usage:"
    echo "    hack/local-down-kosmos.sh [-k] [-h]"
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
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"cluster-host"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"cluster-member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"cluster-member2"}
MEMBER_CLUSTER_3_NAME=${MEMBER_CLUSTER_3_NAME:-"cluster-member3"}
#step1 remove kind clusters
echo -e "\nStart removing kind clusters"
kind delete cluster --name "${HOST_CLUSTER_NAME}"
kind delete cluster --name "${MEMBER_CLUSTER_1_NAME}"
kind delete cluster --name "${MEMBER_CLUSTER_2_NAME}"
kind delete cluster --name "${MEMBER_CLUSTER_3_NAME}"
echo "Remove kind clusters successfully."

ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CLUSTER_DIR="${ROOT}/environments"
source "${ROOT}/hack/cluster.sh"

#step2. remove kubeconfig
echo -e "\nStart removing kubeconfig, kindconfig, cailcoconfig"
HOST_CLUSTER_CONFIG=${HOST_CLUSTER_CONFIG:-"${CLUSTER_DIR}/${HOST_CLUSTER_NAME}"}
MEMBER1_CLUSTER_CONFIG=${MEMBER_CLUSTER_CONFIG:-"${CLUSTER_DIR}/${MEMBER_CLUSTER_1_NAME}"}
MEMBER2_CLUSTER_CONFIG=${MEMBER_CLUSTER_CONFIG:-"${CLUSTER_DIR}/${MEMBER_CLUSTER_2_NAME}"}
MEMBER3_CLUSTER_CONFIG=${MEMBER_CLUSTER_CONFIG:-"${CLUSTER_DIR}/${MEMBER_CLUSTER_3_NAME}"}
delete_cluster "${HOST_CLUSTER_CONFIG}" "${HOST_CLUSTER_CONFIG}"
delete_cluster "${MEMBER1_CLUSTER_CONFIG}" "${MEMBER1_CLUSTER_CONFIG}"
delete_cluster "${MEMBER2_CLUSTER_CONFIG}" "${MEMBER2_CLUSTER_CONFIG}"
delete_cluster "${MEMBER3_CLUSTER_CONFIG}" "${MEMBER3_CLUSTER_CONFIG}"

echo "Remove cluster configs successfully."

#step3. remove docker images
echo -e "\nStart removing images"
version="v0.2.0"
registry="ghcr.io/kosmos-io"
images=(
"${registry}/clusterlink-network-manager:${version}"
"${registry}/clusterlink-controller-manager:${version}"
"${registry}/clusterlink-elector:${version}"
"${registry}/clusterlink-operator:${version}"
"${registry}/clusterlink-agent:${version}"
"${registry}/clusterlink-proxy:${version}"
"${registry}/clustertree-cluster-manager:${version}"
"${registry}/scheduler:${version}"
)
if [[ "${keep_images}" == "false" ]] ; then
  for ((i=0;i<${#images[*]};i++)); do
    docker rmi ${images[i]} || true
  done
  echo "Remove images successfully."
else
  echo "Skip removing images as required."
fi

echo -e "\nLocal Kosmos is removed successfully."
