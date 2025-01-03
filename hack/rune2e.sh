#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export KUBECONFIG=$KUBECONFIG_PATH/"config"
E2E_NAMESPACE="kosmos-e2e"
ARTIFACTS_PATH=${ARTIFACTS_PATH:-"${HOME}/${E2E_NAMESPACE}"}

HOST_CLUSTER_NAME="cluster-host"
MEMBER1_CLUSTER_NAME="cluster-member1"
MEMBER2_CLUSTER_NAME="cluster-member2"
MEMBER3_CLUSTER_NAME="cluster-member3"

ROOT="$(dirname "${BASH_SOURCE[0]}")"
REPO_ROOT="$(dirname "${BASH_SOURCE[0]}")/.."
source "$(dirname "${BASH_SOURCE[0]}")/install_kind_kubectl.sh"
source "${ROOT}/util.sh"
source "${ROOT}/cluster.sh"
mkdir -p "$ARTIFACTS_PATH"

# pull e2e test image
prepare_test_image

# prepare for e2e test
prepare_e2e_cluster "${HOST_CLUSTER_NAME}" &
prepare_e2e_cluster "${MEMBER1_CLUSTER_NAME}" &
prepare_e2e_cluster "${MEMBER2_CLUSTER_NAME}" &
prepare_e2e_cluster "${MEMBER3_CLUSTER_NAME}" &

wait

# e2e for nginx and mcs
kubectl --kubeconfig "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig" apply -f "${REPO_ROOT}"/test/e2e/deploy/nginx
util::wait_for_condition "nginx are ready" \
  "kubectl --kubeconfig ${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig -n ${E2E_NAMESPACE} get pod -l app=nginx | awk 'NR>1 {if (\$3 == \"Running\") exit 0; else exit 1; }'" \
  300

util::wait_for_condition "mcs of member1 are ready" \
  "[ \$(kubectl --kubeconfig ${REPO_ROOT}/environments/${MEMBER1_CLUSTER_NAME}/kubeconfig -n ${E2E_NAMESPACE} get endpointslices.discovery.k8s.io --no-headers -l kubernetes.io\/service-name=nginx-service | wc -l) -eq 1 ] " \
  300

util::wait_for_condition "mcs of member2 are ready" \
  "[ \$(kubectl --kubeconfig ${REPO_ROOT}/environments/${MEMBER2_CLUSTER_NAME}/kubeconfig -n ${E2E_NAMESPACE} get endpointslices.discovery.k8s.io --no-headers -l kubernetes.io\/service-name=nginx-service | wc -l) -eq 1 ] " \
  300

util::wait_for_condition "mcs of member3 are ready" \
  "[ \$(kubectl --kubeconfig ${REPO_ROOT}/environments/${MEMBER3_CLUSTER_NAME}/kubeconfig -n ${E2E_NAMESPACE} get endpointslices.discovery.k8s.io --no-headers -l kubernetes.io\/service-name=nginx-service | wc -l) -eq 1 ] " \
  300

nginx_service_ip=$(kubectl --kubeconfig ${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig -n kosmos-e2e get svc nginx-service -o=jsonpath='{.spec.clusterIP}')

# e2e test for access nginx service
sleep 100 && docker exec -i ${HOST_CLUSTER_NAME}-control-plane sh -c "curl -sSf -m 5 ${nginx_service_ip}:80" && echo "success" || {
  echo "fail"
  exit 1
}

# e2e for mysql-operator
echo "apply mysql-operator on cluster ${HOST_CLUSTER_NAME} with files in path ${REPO_ROOT}/test/e2e/deploy/mysql-operator"
kubectl --kubeconfig "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig" apply -f "${REPO_ROOT}"/test/e2e/deploy/mysql-operator
kubectl create secret generic mysql-operator-orc \
  --from-literal=TOPOLOGY_PASSWORD="$(openssl rand -base64 12)" \
  --from-literal=TOPOLOGY_USER="$(openssl rand -base64 16)" \
  --namespace=mysql-operator
util::wait_for_condition "mysql operator are ready" \
  "kubectl --kubeconfig "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig" get pods -n mysql-operator mysql-operator-0 | awk 'NR>1 {if (\$3 == \"Running\") exit 0; else exit 1; }'" \
  300
kubectl --kubeconfig "${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig" apply -f "${REPO_ROOT}"/test/e2e/deploy/cr
kubectl create secret generic my-secret \
  --from-literal=ROOT_PASSWORD="$(openssl rand -base64 12)" \
  --namespace=kosmos-e2e

util::wait_for_condition "mysql cr are ready" \
  "[ \$(kubectl --kubeconfig ${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig get pods -n kosmos-e2e -l app.kubernetes.io/name=mysql |grep \"4/4\"| grep \"Running\" | wc -l) -eq 2 ]" \
  1200


echo "E2e test of mysql-operator success"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/ --
TESTING_RESULT=$?

# Collect logs
echo "Collect logs to $ARTIFACTS_PATH..."
cp -r "${REPO_ROOT}/environments" "$ARTIFACTS_PATH"

echo "Collecting Kind logs..."
mkdir -p "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"
kind export logs --name="$HOST_CLUSTER_NAME" "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"

mkdir -p "$ARTIFACTS_PATH/$MEMBER1_CLUSTER_NAME"
kind export logs --name="$MEMBER1_CLUSTER_NAME" "$ARTIFACTS_PATH/$MEMBER1_CLUSTER_NAME"

mkdir -p "$ARTIFACTS_PATH/$MEMBER2_CLUSTER_NAME"
kind export logs --name="$MEMBER2_CLUSTER_NAME" "$ARTIFACTS_PATH/$MEMBER2_CLUSTER_NAME"

mkdir -p "$ARTIFACTS_PATH/$MEMBER3_CLUSTER_NAME"
kind export logs --name="$MEMBER3_CLUSTER_NAME" "$ARTIFACTS_PATH/$MEMBER3_CLUSTER_NAME"

echo "Collected logs at $ARTIFACTS_PATH:"
ls -al "$ARTIFACTS_PATH"

exit $TESTING_RESULT