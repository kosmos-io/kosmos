#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export KUBECONFIG=$KUBECONFIG_PATH/"config"

HOST_CLUSTER_NAME="cluster-host"
HOST_CLUSTER_POD_CIDR="10.233.64.0/18"
HOST_CLUSTER_SERVICE_CIDR="10.233.0.0/18"

MEMBER1_CLUSTER_NAME="cluster-member1"
MEMBER1_CLUSTER_POD_CIDR="10.234.64.0/18"
MEMBER1_CLUSTER_SERVICE_CIDR="10.234.0.0/18"

MEMBER2_CLUSTER_NAME="cluster-member2"
MEMBER2_CLUSTER_POD_CIDR="10.235.64.0/18"
MEMBER2_CLUSTER_SERVICE_CIDR="10.235.0.0/18"

ROOT="$(dirname "${BASH_SOURCE[0]}")"
export VERSION="latest"
source "$(dirname "${BASH_SOURCE[0]}")/install_kind_kubectl.sh"
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"
make images GOOS="linux" --directory="${ROOT}"

make kosmosctl
os=$(go env GOOS)
arch=$(go env GOARCH)
export PATH=$PATH:"$ROOT"/_output/bin/"$os"/"$arch"

#cluster cluster
create_cluster $HOST_CLUSTER_NAME $HOST_CLUSTER_POD_CIDR $HOST_CLUSTER_SERVICE_CIDR
create_cluster $MEMBER1_CLUSTER_NAME $MEMBER1_CLUSTER_POD_CIDR $MEMBER1_CLUSTER_SERVICE_CIDR false
create_cluster $MEMBER2_CLUSTER_NAME $MEMBER2_CLUSTER_POD_CIDR $MEMBER2_CLUSTER_SERVICE_CIDR fasle
#deploy cluster
deploy_cluster_by_ctl $HOST_CLUSTER_NAME
load_cluster_images $MEMBER1_CLUSTER_NAME
load_cluster_images $MEMBER2_CLUSTER_NAME

#join cluster
join_cluster_by_ctl $HOST_CLUSTER_NAME $MEMBER1_CLUSTER_NAME
join_cluster_by_ctl $HOST_CLUSTER_NAME $MEMBER2_CLUSTER_NAME
