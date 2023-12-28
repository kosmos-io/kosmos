#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export KUBECONFIG=$KUBECONFIG_PATH/"config"

E2E_NAMESPACE="kosmos-e2e"
HOST_CLUSTER_NAME="cluster-host"
MEMBER1_CLUSTER_NAME="cluster-member1"
MEMBER2_CLUSTER_NAME="cluster-member2"

ROOT="$(dirname "${BASH_SOURCE[0]}")"
source "${ROOT}/util.sh"

# e2e for nginx and mcs
kubectl --context="kind-${HOST_CLUSTER_NAME}" apply -f "${ROOT}"/../test/e2e/deploy/nginx
util::wait_for_condition "nginx are ready" \
  "kubectl --context=kind-${HOST_CLUSTER_NAME} -n ${E2E_NAMESPACE} get pod -l app=nginx | awk 'NR>1 {if (\$3 == \"Running\") exit 0; else exit 1; }'" \
  120

util::wait_for_condition "mcs of member1 are ready" \
  "[ \$(kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n ${E2E_NAMESPACE} get endpointslices.discovery.k8s.io --no-headers -l kubernetes.io\/service-name=nginx-service | wc -l) -eq 1 ] " \
  120

util::wait_for_condition "mcs of member2 are ready" \
  "[ \$(kubectl --context=kind-${MEMBER2_CLUSTER_NAME} -n ${E2E_NAMESPACE} get endpointslices.discovery.k8s.io --no-headers -l kubernetes.io\/service-name=nginx-service | wc -l) -eq 1 ] " \
  120

nginx_service_ip=$(kubectl -n kosmos-e2e get svc nginx-service -o=jsonpath='{.spec.clusterIP}')

# e2e test for access nginx service
sleep 100 && docker exec -i ${HOST_CLUSTER_NAME}-control-plane sh -c "curl -sSf -m 5 ${nginx_service_ip}:80" && echo "success" || {
  echo "fail"
  exit 1
}

# e2e for mysql-operator
kubectl --context="kind-cluster-host" apply -f "${ROOT}"/../test/e2e/deploy/mysql-operator
util::wait_for_condition "mysql operator are ready" \
  "kubectl --context=kind-${HOST_CLUSTER_NAME} get pods -n mysql-operator mysql-operator-0 | awk 'NR>1 {if (\$3 == \"Running\") exit 0; else exit 1; }'" \
  300

#kubectl --context="kind-cluster-host" exec -it /bin/sh -c
kubectl --context="kind-${HOST_CLUSTER_NAME}" apply -f "${ROOT}"/../test/e2e/deploy/cr

sleep 240
echo "主集群e2e pod"
kubectl --context=kind-${HOST_CLUSTER_NAME} get pods -n kosmos-e2e -o wide
echo "主集群所有 pod"
kubectl --context=kind-${HOST_CLUSTER_NAME} get pods -A
echo "集群2 servicimport"
kubectl --context=kind-${MEMBER2_CLUSTER_NAME} -n kosmos-e2e get serviceimports
echo "集群1 servicimport"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e get serviceimports
echo "集群2 esp"
kubectl --context=kind-${MEMBER2_CLUSTER_NAME} -n kosmos-e2e get endpointslices
echo "集群1 esp"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e get endpointslices
echo "主集群 esp"
kubectl --context=kind-${HOST_CLUSTER_NAME} -n kosmos-e2e get endpointslices

echo "集群1 mysql init容器日志"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e logs mysql-cluster-e2e-mysql-0 init
echo "集群1 mysql mysql容器日志"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e logs mysql-cluster-e2e-mysql-0 mysql
echo "集群1 mysql init容器旧日志"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e logs -p mysql-cluster-e2e-mysql-0 init
echo "集群1 mysql mysql容器旧日志"
kubectl --context=kind-${MEMBER1_CLUSTER_NAME} -n kosmos-e2e logs -p mysql-cluster-e2e-mysql-0 mysql


#util::wait_for_condition "mysql cr are ready" \
#  "[ \$(kubectl get pods -n kosmos-e2e --field-selector=status.phase=Running --no-headers | wc -l) -eq 2 ]" \
#  1200

echo "E2e test of mysql-operator success"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/ --
TESTING_RESULT=$?

LOG_PATH=$ROOT/../e2e-logs
echo "Collect logs to $LOG_PATH..."
mkdir -p "$LOG_PATH"

echo "Collecting $HOST_CLUSTER_NAME logs..."
mkdir -p "$LOG_PATH/$HOST_CLUSTER_NAME"
kind export logs --name="$HOST_CLUSTER_NAME" "$LOG_PATH/$HOST_CLUSTER_NAME"

echo "Collecting $MEMBER1_CLUSTER_NAME logs..."
mkdir -p "$MEMBER1_CLUSTER_NAME/$MEMBER1_CLUSTER_NAME"
kind export logs --name="$MEMBER1_CLUSTER_NAME" "$LOG_PATH/$MEMBER1_CLUSTER_NAME"

echo "Collecting $MEMBER2_CLUSTER_NAME logs..."
mkdir -p "$MEMBER2_CLUSTER_NAME/$MEMBER2_CLUSTER_NAME"
kind export logs --name="$MEMBER2_CLUSTER_NAME" "$LOG_PATH/$MEMBER2_CLUSTER_NAME"

#TODO delete cluster

exit $TESTING_RESULT
