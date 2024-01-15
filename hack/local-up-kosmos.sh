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

TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"kosmos-cluster1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"kosmos-cluster2"}
MEMBER_CLUSTER_3_NAME=${PULL_MODE_CLUSTER_NAME:-"kosmos-cluster3"}
#MEMBER_TMP_CONFIG_PREFIX="member-tmp"
#MEMBER_CLUSTER_1_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_1_NAME}.config"
#MEMBER_CLUSTER_2_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_2_NAME}.config"
#PULL_MODE_CLUSTER_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${PULL_MODE_CLUSTER_NAME}.config"
HOST_IPADDRESS=${1:-}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/kosmos"}
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
# make kosmosctl with recently tags
make kosmosctl VERSION="$(git describe --tags --abbrev=0)"
cp "${REPO_ROOT}/_output/bin/${GOOS}/${GOARCH}"/kosmosctl /usr/local/bin/

# delete kosmos create clusters
kind delete clusters "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_3_NAME}" >> "${KIND_LOG_FILE}" 2>&1

if [[ -n "${HOST_IPADDRESS}" ]]; then # If bind the port of clusters(cluster1[as the host cluster], cluster2 and cluster3) to the host IP
  echo "HOST_IPADDRESS" "${HOST_IPADDRESS}" "${TEMP_PATH}"
  cp -rf "${REPO_ROOT}"/deploy/kindClusterConfig/kind-cluster1-config.yml "${TEMP_PATH}"/"${MEMBER_CLUSTER_1_NAME}"-config.yml
  cp -rf "${REPO_ROOT}"/deploy/kindClusterConfig/kind-cluster2-config.yml "${TEMP_PATH}"/"${MEMBER_CLUSTER_2_NAME}"-config.yml
  cp -rf "${REPO_ROOT}"/deploy/kindClusterConfig/kind-cluster3-config.yml "${TEMP_PATH}"/"${MEMBER_CLUSTER_3_NAME}"-config.yml

  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/"${MEMBER_CLUSTER_1_NAME}"-config.yml
  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/"${MEMBER_CLUSTER_2_NAME}"-config.yml
  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/"${MEMBER_CLUSTER_3_NAME}"-config.yml
  else
    echo "HOST_IPADDRESS is required please input the host_ipaddress"
    exit 1
fi
# create cluster use kind
kind create cluster -n "${MEMBER_CLUSTER_1_NAME}" --config "${TEMP_PATH}"/"${MEMBER_CLUSTER_1_NAME}"-config.yml --kubeconfig ~/.kube/kind-config
kind create cluster -n "${MEMBER_CLUSTER_2_NAME}" --config "${TEMP_PATH}"/"${MEMBER_CLUSTER_2_NAME}"-config.yml --kubeconfig ~/.kube/kind-config
kind create cluster -n "${MEMBER_CLUSTER_3_NAME}" --config "${TEMP_PATH}"/"${MEMBER_CLUSTER_3_NAME}"-config.yml --kubeconfig ~/.kube/kind-config

# switch context
export KUBECONFIG=~/.kube/kind-config
kubectl config use-context kind-"${MEMBER_CLUSTER_1_NAME}"
# install kosmos control plane
kosmosctl install  --cni calico --default-nic eth0
# export config to config file
kubectl config view --minify --flatten > ~/.kube/kind-kosmos-cluster1-config
kubectl config use-context kind-"${MEMBER_CLUSTER_2_NAME}"
kubectl config view --minify --flatten > ~/.kube/kind-kosmos-cluster2-config
kubectl config use-context kind-"${MEMBER_CLUSTER_3_NAME}"
kubectl config view --minify --flatten > ~/.kube/kind-kosmos-cluster3-config
# switch context to host-cluster
kubectl config use-context kind-kosmos-cluster1
# join cluster to host-cluster control plane
kosmosctl join cluster --cni calico --default-nic eth0 --name cluster2 --kubeconfig ~/.kube/kind-kosmos-cluster2-config --enable-all
kosmosctl join cluster --cni calico --default-nic eth0 --name cluster3 --kubeconfig ~/.kube/kind-kosmos-cluster3-config --enable-all