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
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"
make images GOOS="linux" --directory="${ROOT}"

#cluster cluster
create_cluster $HOST_CLUSTER_NAME $HOST_CLUSTER_POD_CIDR $HOST_CLUSTER_SERVICE_CIDR
create_cluster $MEMBER1_CLUSTER_NAME $MEMBER1_CLUSTER_POD_CIDR $MEMBER1_CLUSTER_SERVICE_CIDR true
#deploy clusterlink
deploy_clusterlink $HOST_CLUSTER_NAME
load_clusterlink_images $MEMBER1_CLUSTER_NAME

#join cluster
join_cluster $HOST_CLUSTER_NAME $HOST_CLUSTER_NAME
join_cluster $HOST_CLUSTER_NAME $MEMBER1_CLUSTER_NAME

echo "e2e test enviroment init success"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/ --
TESTING_RESULT=$?

LOG_PATH=$ROOT/e2e-logs
echo "Collect logs to $LOG_PATH..."
mkdir -p "$LOG_PATH"

echo "Collecting $HOST_CLUSTER_NAME logs..."
mkdir -p "$LOG_PATH/$HOST_CLUSTER_NAME"
kind export logs --name="$HOST_CLUSTER_NAME" "$LOG_PATH/$HOST_CLUSTER_NAME"

echo "Collecting $MEMBER1_CLUSTER_NAME logs..."
mkdir -p "$MEMBER1_CLUSTER_NAME/$MEMBER1_CLUSTER_NAME"
kind export logs --name="$MEMBER1_CLUSTER_NAME" "$LOG_PATH/$MEMBER1_CLUSTER_NAME"

#TODO delete cluster

exit $TESTING_RESULT