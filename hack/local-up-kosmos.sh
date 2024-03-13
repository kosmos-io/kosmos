#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


function usage() {
    echo "Usage:"
    echo "    hack/local-up-kosmos.sh [HOST_IPADDRESS] [-h]"
    echo "Args:"
    echo "    HOST_IPADDRESS: (required) if you want to export clusters' API server port to specific IP address"
    echo "    h: print help information"
}

while getopts 'h' OPT; do
    case $OPT in
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


KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export KUBECONFIG=$KUBECONFIG_PATH/"config"

KIND_IMAGE=${KIND_IMAGE:-"kindest/node:v1.27.2"}
HOST_IPADDRESS=${1:-}
HOST_CLUSTER_NAME="cluster-host"
HOST_CLUSTER_POD_CIDR="10.233.64.0/18"
HOST_CLUSTER_SERVICE_CIDR="10.233.0.0/18"

MEMBER1_CLUSTER_NAME="cluster-member1"
MEMBER1_CLUSTER_POD_CIDR="10.234.64.0/18"
MEMBER1_CLUSTER_SERVICE_CIDR="10.234.0.0/18"

MEMBER2_CLUSTER_NAME="cluster-member2"
MEMBER2_CLUSTER_POD_CIDR="10.235.64.0/18"
MEMBER2_CLUSTER_SERVICE_CIDR="10.235.0.0/18"

MEMBER3_CLUSTER_NAME="cluster-member3"
MEMBER3_CLUSTER_POD_CIDR="10.236.64.0/18"
MEMBER3_CLUSTER_SERVICE_CIDR="10.236.0.0/18"

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
export VERSION="latest"
source "$(dirname "${BASH_SOURCE[0]}")/install_kind_kubectl.sh"
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"
source "$(dirname "${BASH_SOURCE[0]}")/util.sh"

#step1. create host cluster and member clusters in parallel
# host IP address: script parameter ahead of macOS IP
if [[ -z "${HOST_IPADDRESS}" ]]; then
  util::get_macos_ipaddress # Adapt for macOS
  HOST_IPADDRESS=${MAC_NIC_IPADDRESS:-}
fi
make images GOOS="linux" --directory="${REPO_ROOT}"

make kosmosctl
os=$(go env GOOS)
arch=$(go env GOARCH)
export PATH=$PATH:"${REPO_ROOT}"/_output/bin/"$os"/"$arch"

# prepare docker image
prepare_docker_image

#cluster cluster concurrent backend
create_cluster "${KIND_IMAGE}" "$HOST_IPADDRESS" $HOST_CLUSTER_NAME $HOST_CLUSTER_POD_CIDR $HOST_CLUSTER_SERVICE_CIDR &
create_cluster "${KIND_IMAGE}" "$HOST_IPADDRESS" $MEMBER1_CLUSTER_NAME $MEMBER1_CLUSTER_POD_CIDR $MEMBER1_CLUSTER_SERVICE_CIDR false &
create_cluster "${KIND_IMAGE}" "$HOST_IPADDRESS" $MEMBER2_CLUSTER_NAME $MEMBER2_CLUSTER_POD_CIDR $MEMBER2_CLUSTER_SERVICE_CIDR false &
create_cluster "${KIND_IMAGE}" "$HOST_IPADDRESS" $MEMBER3_CLUSTER_NAME $MEMBER3_CLUSTER_POD_CIDR $MEMBER3_CLUSTER_SERVICE_CIDR false &

# wait for finish
wait

#deploy cluster concurrent backend
deploy_cluster_by_ctl $HOST_CLUSTER_NAME "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig" "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig-nodeIp" &
load_cluster_images $MEMBER1_CLUSTER_NAME &
load_cluster_images $MEMBER2_CLUSTER_NAME &
load_cluster_images $MEMBER3_CLUSTER_NAME &

# wait for finish
wait

#join cluster
join_cluster_by_ctl $HOST_CLUSTER_NAME $MEMBER1_CLUSTER_NAME "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}" "${REPO_ROOT}/environments/${MEMBER1_CLUSTER_NAME}"
join_cluster_by_ctl $HOST_CLUSTER_NAME $MEMBER2_CLUSTER_NAME "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}" "${REPO_ROOT}/environments/${MEMBER2_CLUSTER_NAME}"
join_cluster_by_ctl $HOST_CLUSTER_NAME $MEMBER3_CLUSTER_NAME "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}" "${REPO_ROOT}/environments/${MEMBER3_CLUSTER_NAME}"

#add leafnode test taint
addTaint $HOST_CLUSTER_NAME $MEMBER3_CLUSTER_NAME
