#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

HOST_CLUSTER_NAME="cluster-host-local"
HOST_CLUSTER_POD_CIDR="10.233.64.0/18"
HOST_CLUSTER_SERVICE_CIDR="10.233.0.0/18"

MEMBER1_CLUSTER_NAME="cluster-m-local"
MEMBER1_CLUSTER_POD_CIDR="10.234.64.0/18"
MEMBER1_CLUSTER_SERVICE_CIDR="10.234.0.0/18"

export VERSION="0.2.0"
ROOT="$(dirname "${BASH_SOURCE[0]}")"
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"
make images GOOS="linux" --directory="${ROOT}"

#cluster cluster
create_cluster $HOST_CLUSTER_NAME $HOST_CLUSTER_POD_CIDR $HOST_CLUSTER_SERVICE_CIDR false
create_cluster $MEMBER1_CLUSTER_NAME $MEMBER1_CLUSTER_POD_CIDR $MEMBER1_CLUSTER_SERVICE_CIDR true
#deploy clusterlink
deploy_clusterlink $HOST_CLUSTER_NAME
load_clusterlink_images $MEMBER1_CLUSTER_NAME

#join cluster
join_cluster $HOST_CLUSTER_NAME $HOST_CLUSTER_NAME
join_cluster $HOST_CLUSTER_NAME $MEMBER1_CLUSTER_NAME

echo "clusterlink local start success enjoy it!"

