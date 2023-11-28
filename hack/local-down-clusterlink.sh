#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

HOST_CLUSTER_NAME="cluster-host-local"

MEMBER1_CLUSTER_NAME="cluster-member1-local"
MEMBER2_CLUSTER_NAME="cluster-member2-local"

ROOT="$(dirname "${BASH_SOURCE[0]}")"
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"

#cluster cluster
delete_cluster $HOST_CLUSTER_NAME
delete_cluster $MEMBER1_CLUSTER_NAME
delete_cluster $MEMBER2_CLUSTER_NAME


echo "clusterlink local down success"
