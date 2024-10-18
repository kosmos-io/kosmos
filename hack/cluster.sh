#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CURRENT="$(dirname "${BASH_SOURCE[0]}")"
ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# true: when cluster is exist, reuse exist one!
REUSE=${REUSE:-false}
VERSION=${VERSION:-latest}

CN_ZONE=${CN_ZONE:-true}
source "$(dirname "${BASH_SOURCE[0]}")/util.sh"

# default cert and key for node server https
CERT=$(util::get_base64_kubeconfig ${ROOT}/pkg/cert/crt.pem)
KEY=$(util::get_base64_kubeconfig ${ROOT}/pkg/cert/crt.pem)

if [ $REUSE == true ]; then
  echo "!!!!!!!!!!!Warning: Setting REUSE to true will not delete existing clusters.!!!!!!!!!!!"
fi

source "${ROOT}/hack/util.sh"

# pull e2e test image
function prepare_test_image() {
  if [ "${CN_ZONE}" == false ]; then
    docker pull bitpoke/mysql-operator-orchestrator:v0.6.3
    docker pull bitpoke/mysql-operator:v0.6.3
    docker pull bitpoke/mysql-operator-sidecar-5.7:v0.6.3
    docker pull nginx
    docker pull percona:5.7
    docker pull prom/mysqld-exporter:v0.13.0
  else
#    todo add bitpoke to m.daocloud.io's Whitelist
    docker pull bitpoke/mysql-operator-orchestrator:v0.6.3
    docker pull bitpoke/mysql-operator:v0.6.3
    docker pull bitpoke/mysql-operator-sidecar-5.7:v0.6.3
    docker pull docker.m.daocloud.io/nginx
    docker pull docker.m.daocloud.io/percona:5.7
    docker pull docker.m.daocloud.io/prom/mysqld-exporter:v0.13.0

    docker tag docker.m.daocloud.io/bitpoke/mysql-operator-orchestrator:v0.6.3 bitpoke/mysql-operator-orchestrator:v0.6.3
    docker tag docker.m.daocloud.io/bitpoke/mysql-operator:v0.6.3 bitpoke/mysql-operator:v0.6.3
    docker tag docker.m.daocloud.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3 bitpoke/mysql-operator-sidecar-5.7:v0.6.3
    docker tag docker.m.daocloud.io/nginx nginx
    docker tag docker.m.daocloud.io/percona:5.7 percona:5.7
    docker tag docker.m.daocloud.io/prom/mysqld-exporter:v0.13.0 prom/mysqld-exporter:v0.13.0
  fi
}

# prepare e2e cluster
function prepare_e2e_cluster() {
  local -r clustername=$1
  CLUSTER_DIR="${ROOT}/environments/${clustername}"

  # load image for kind
  kind load docker-image bitpoke/mysql-operator-orchestrator:v0.6.3 --name "${clustername}"
  kind load docker-image bitpoke/mysql-operator:v0.6.3 --name "${clustername}"
  kind load docker-image bitpoke/mysql-operator-sidecar-5.7:v0.6.3 --name "${clustername}"
  kind load docker-image nginx --name "${clustername}"
  kind load docker-image percona:5.7 --name "${clustername}"
  kind load docker-image prom/mysqld-exporter:v0.13.0 --name "${clustername}"

  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/crds

  # deploy kosmos-scheduler for e2e test case of mysql-operator
  sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/kosmos-scheduler.yml" "$ROOT"/deploy/scheduler/deployment.yaml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${ROOT}/environments/kosmos-scheduler.yml"
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/scheduler/rbac.yaml

  util::wait_for_condition "kosmos scheduler are ready" \
    "kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system get deploy kosmos-scheduler -o jsonpath='{.status.replicas}{\" \"}{.status.readyReplicas}{\"\n\"}' | awk '{if (\$1 == \$2 && \$1 > 0) exit 0; else exit 1}'" \
    300
  echo "cluster $clustername deploy kosmos-scheduler success"

  docker exec ${clustername}-control-plane /bin/sh -c "mv /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes"

  # add the args for e2e test case of mysql-operator
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system patch deployment clustertree-cluster-manager --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/-", "value": "--auto-mcs-prefix=kosmos-e2e"}]'

  util::wait_for_condition "kosmos ${clustername} clustertree are ready" \
    "kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system get deploy clustertree-cluster-manager -o jsonpath='{.status.replicas}{\" \"}{.status.readyReplicas}{\"\n\"}' | awk '{if (\$1 == \$2 && \$1 > 0) exit 0; else exit 1}'" \
    300
}

# prepare docker image
function prepare_docker_image() {
  if [ "${CN_ZONE}" == false ]; then
    # pull calico image
    docker pull calico/apiserver:v3.25.0
    docker pull calico/cni:v3.25.0
    docker pull calico/csi:v3.25.0
    docker pull calico/kube-controllers:v3.25.0
    docker pull calico/node-driver-registrar:v3.25.0
    docker pull calico/node:v3.25.0
    docker pull calico/pod2daemon-flexvol:v3.25.0
    docker pull calico/typha:v3.25.0
    docker pull quay.io/tigera/operator:v1.29.0
  else
    docker pull quay.m.daocloud.io/tigera/operator:v1.29.0
    docker pull docker.m.daocloud.io/calico/apiserver:v3.25.0
    docker pull docker.m.daocloud.io/calico/cni:v3.25.0
    docker pull docker.m.daocloud.io/calico/csi:v3.25.0
    docker pull docker.m.daocloud.io/calico/kube-controllers:v3.25.0
    docker pull docker.m.daocloud.io/calico/node-driver-registrar:v3.25.0
    docker pull docker.m.daocloud.io/calico/node:v3.25.0
    docker pull docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0
    docker pull docker.m.daocloud.io/calico/typha:v3.25.0

    docker tag quay.m.daocloud.io/tigera/operator:v1.29.0 quay.io/tigera/operator:v1.29.0
    docker tag docker.m.daocloud.io/calico/apiserver:v3.25.0 calico/apiserver:v3.25.0
    docker tag docker.m.daocloud.io/calico/cni:v3.25.0 calico/cni:v3.25.0
    docker tag docker.m.daocloud.io/calico/csi:v3.25.0 calico/csi:v3.25.0
    docker tag docker.m.daocloud.io/calico/kube-controllers:v3.25.0 calico/kube-controllers:v3.25.0
    docker tag docker.m.daocloud.io/calico/node-driver-registrar:v3.25.0 calico/node-driver-registrar:v3.25.0
    docker tag docker.m.daocloud.io/calico/node:v3.25.0 calico/node:v3.25.0
    docker tag docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0 calico/pod2daemon-flexvol:v3.25.0
    docker tag docker.m.daocloud.io/calico/typha:v3.25.0 calico/typha:v3.25.0
  fi
}

#clustername podcidr servicecidr
function create_cluster() {
  local -r KIND_IMAGE=$1
  local -r hostIpAddress=$2
  local -r clustername=$3
  local -r podcidr=$4
  local -r servicecidr=$5
  local -r isDual=${6:-false}
  local -r multiNodes=${7:-false}
  local -r isMount=${8:-false}

  local KIND_CONFIG_NAME

  if [ "${multiNodes}" == true ]; then
      KIND_CONFIG_NAME="kubenest_kindconfig"
  else
      KIND_CONFIG_NAME="kindconfig"
  fi
  if [ "${isMount}" == true ]; then
      KIND_CONFIG_NAME="kubenest_kind_config"
  fi

  CLUSTER_DIR="${ROOT}/environments/${clustername}"
  mkdir -p "${CLUSTER_DIR}"

  echo "$CLUSTER_DIR"

  ipFamily=ipv4
  if [ "$isDual" == true ]; then
    ipFamily=dual
    pod_convert=$(printf %x $(echo $podcidr | awk -F "." '{print $2" "$3}'))
    svc_convert=$(printf %x $(echo $servicecidr | awk -F "." '{print $2" "$3}'))
    podcidr_ipv6="fd11:1111:1111:"$pod_convert"::/64"
    servicecidr_ipv6="fd11:1111:1112:"$svc_convert"::/108"
    podcidr_all=${podcidr_ipv6}","${podcidr}
    servicecidr_all=${servicecidr_ipv6}","${servicecidr}
    sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__POD_CIDR_IPV6__|$podcidr_ipv6|g" -e "s|#DUAL||g" -e "w ${CLUSTER_DIR}/calicoconfig" "${CURRENT}/clustertemplete/calicoconfig"
    sed -e "s|__POD_CIDR__|$podcidr_all|g" -e "s|__SERVICE_CIDR__|$servicecidr_all|g" -e "s|__IP_FAMILY__|$ipFamily|g" -e "w ${CLUSTER_DIR}/${KIND_CONFIG_NAME}" "${CURRENT}/clustertemplete/${KIND_CONFIG_NAME}"
  else
    sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__SERVICE_CIDR__|$servicecidr|g" -e "s|__IP_FAMILY__|$ipFamily|g" -e "w ${CLUSTER_DIR}/${KIND_CONFIG_NAME}" "${CURRENT}/clustertemplete/${KIND_CONFIG_NAME}"
    sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__SERVICE_CIDR__|$servicecidr|g" -e "w ${CLUSTER_DIR}/calicoconfig" "${CURRENT}/clustertemplete/calicoconfig"
  fi

  sed -i'' -e "s/__HOST_IPADDRESS__/${hostIpAddress}/g" ${CLUSTER_DIR}/${KIND_CONFIG_NAME}
  if [[ "$(kind get clusters | grep -c "${clustername}")" -eq 1 && "${REUSE}" = true ]]; then
    echo "cluster ${clustername} exist reuse it"
  else
    kind delete clusters $clustername || true
    echo "create cluster ${clustername} with kind image ${KIND_IMAGE}"
    kind create cluster --name "${clustername}" --config "${CLUSTER_DIR}/${KIND_CONFIG_NAME}" --image "${KIND_IMAGE}"
  fi
  # load docker image to kind cluster
  kind load docker-image calico/apiserver:v3.25.0 --name $clustername
  kind load docker-image calico/cni:v3.25.0 --name $clustername
  kind load docker-image calico/csi:v3.25.0 --name $clustername
  kind load docker-image calico/kube-controllers:v3.25.0 --name $clustername
  kind load docker-image calico/node-driver-registrar:v3.25.0 --name $clustername
  kind load docker-image calico/node:v3.25.0 --name $clustername
  kind load docker-image calico/pod2daemon-flexvol:v3.25.0 --name $clustername
  kind load docker-image calico/typha:v3.25.0 --name $clustername
  kind load docker-image quay.io/tigera/operator:v1.29.0 --name $clustername

  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig taint nodes --all node-role.kubernetes.io/control-plane- || true

  # prepare external kubeconfig
  kind get kubeconfig --name "${clustername}" >"${CLUSTER_DIR}/kubeconfig"
  dockerip=$(docker inspect "${clustername}-control-plane" --format "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}")
  echo "get docker ip from pod $dockerip"
  docker exec ${clustername}-control-plane /bin/sh -c "cat /etc/kubernetes/admin.conf" | sed -e "s|${clustername}-control-plane|$dockerip|g" -e "/certificate-authority-data:/d" -e "5s/^/    insecure-skip-tls-verify: true\n/" -e "w ${CLUSTER_DIR}/kubeconfig-nodeIp"

  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig create -f "$CURRENT/calicooperator/tigera-operator.yaml" || $("${REUSE}" -eq "true")
  kind export kubeconfig --name "$clustername"
  util::wait_for_crd installations.operator.tigera.io
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${CLUSTER_DIR}"/calicoconfig
  echo "create cluster ${clustername} success"
  echo "wait all node ready"
  # N = nodeNum + 1
  N=$(kubectl --kubeconfig $CLUSTER_DIR/kubeconfig get nodes --no-headers | wc -l)
  util::wait_for_condition "all nodes are ready" \
    "kubectl --kubeconfig $CLUSTER_DIR/kubeconfig get nodes | awk 'NR>1 {if (\$2 != \"Ready\") exit 1; }' && [ \$(kubectl --kubeconfig $CLUSTER_DIR/kubeconfig get nodes --no-headers | wc -l) -eq ${N} ]" \
    300
  echo "all node ready"
}

function join_cluster() {
  local host_cluster=$1
  local member_cluster=$2
  local kubeconfig_path="${ROOT}/environments/${member_cluster}/kubeconfig"
  local hostConfig_path="${ROOT}/environments/${host_cluster}/kubeconfig"
  local base64_kubeconfig=$(util::get_base64_kubeconfig <"$kubeconfig_path")
  echo " base64 kubeconfig successfully converted: $base64_kubeconfig "

  local common_metadata=""
  if [ "$host_cluster" == "$member_cluster" ]; then
    common_metadata="annotations:
    kosmos.io/cluster-role: root"
  fi

  cat <<EOF | kubectl --kubeconfig "${hostConfig_path}" apply -f -
apiVersion: kosmos.io/v1alpha1
kind: Cluster
metadata:
  $common_metadata
  name: ${member_cluster}
spec:
  imageRepository: "ghcr.io/kosmos-io"
  kubeconfig: "$base64_kubeconfig"
  clusterLinkOptions:
    cni: "calico"
    ipFamily: ipv4
    defaultNICName: eth0
    networkType: "gateway"
  clusterTreeOptions:
    enable: true
EOF
  kubectl --kubeconfig "${hostConfig_path}" apply -f "$ROOT"/deploy/clusterlink-namespace.yml
  kubectl --kubeconfig "${hostConfig_path}" apply -f "$ROOT"/deploy/clusterlink-datapanel-rbac.yml
}

function join_cluster_by_ctl() {
  local host_cluster=$1
  local member_cluster=$2
  local hostClusterDir=$3
  local memberClusterDir=$4
  "${ROOT}"/_output/bin/"$os"/"$arch"/kosmosctl join cluster --name "$member_cluster" --host-kubeconfig "$hostClusterDir/kubeconfig" --kubeconfig "$memberClusterDir/kubeconfig" --inner-kubeconfig "$memberClusterDir/kubeconfig-nodeIp" --enable-all --version ${VERSION}
}

function addTaint() {
  local host_cluster=$1
  local member_cluster=$2
  leafnode="kosmos-${member_cluster}"
  HOST_CLUSTER_DIR="${ROOT}/environments/${host_cluster}"

  sleep 100 && kubectl --kubeconfig $HOST_CLUSTER_DIR/kubeconfig get node -owide
  kubectl --kubeconfig $HOST_CLUSTER_DIR/kubeconfig taint nodes $leafnode test-node/e2e=leafnode:NoSchedule
}

function deploy_cluster_by_ctl() {
  local -r clustername=$1
  local -r kubeconfig=$2
  local -r innerKubeconfig=$3
  load_cluster_images "$clustername"
  CLUSTER_DIR="${ROOT}/environments/${clustername}"

  "${ROOT}"/_output/bin/"$os"/"$arch"/kosmosctl install --version ${VERSION} --kubeconfig "${kubeconfig}" --inner-kubeconfig "${innerKubeconfig}"

  util::wait_for_condition "kosmos ${clustername} clustertree are ready" \
    "kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system get deploy clustertree-cluster-manager -o jsonpath='{.status.replicas}{\" \"}{.status.readyReplicas}{\"\n\"}' | awk '{if (\$1 == \$2 && \$1 > 0) exit 0; else exit 1}'" \
    300
}

function deploy_cluster() {
  local -r clustername=$1
  CLUSTER_DIR="${ROOT}/environments/${clustername}"

  load_cluster_images "$clustername"

  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/clusterlink-namespace.yml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/kosmos-rbac.yml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/crds
  util::wait_for_crd clusternodes.kosmos.io clusters.kosmos.io clusterdistributionpolicies.kosmos.io distributionpolicies.kosmos.io

  sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/clusterlink-network-manager.yml" "$ROOT"/deploy/clusterlink-network-manager.yml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${ROOT}/environments/clusterlink-network-manager.yml"

  echo "cluster $clustername deploy clusterlink success"

  sed -e "s|__VERSION__|$VERSION|g" -e "s|__CERT__|$CERT|g" -e "s|__KEY__|$KEY|g" -e "w ${ROOT}/environments/clustertree-cluster-manager.yml" "$ROOT"/deploy/clustertree-cluster-manager.yml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${ROOT}/environments/clustertree-cluster-manager.yml"

  echo "cluster $clustername deploy clustertree success"

  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system delete secret controlpanel-config || true
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system create secret generic controlpanel-config --from-file=kubeconfig="${ROOT}/environments/cluster-host/kubeconfig"
  sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/clusterlink-operator.yml" "$ROOT"/deploy/clusterlink-operator.yml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${ROOT}/environments/clusterlink-operator.yml"

  echo "cluster $clustername deploy clusterlink-operator success"

  sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/kosmos-scheduler.yml" "$ROOT"/deploy/scheduler/deployment.yaml
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "${ROOT}/environments/kosmos-scheduler.yml"
  kubectl --kubeconfig $CLUSTER_DIR/kubeconfig apply -f "$ROOT"/deploy/scheduler/rbac.yaml

  util::wait_for_condition "kosmos scheduler are ready" \
    "kubectl --kubeconfig $CLUSTER_DIR/kubeconfig -n kosmos-system get deploy kosmos-scheduler -o jsonpath='{.status.replicas}{\" \"}{.status.readyReplicas}{\"\n\"}' | awk '{if (\$1 == \$2 && \$1 > 0) exit 0; else exit 1}'" \
    300
  echo "cluster $clustername deploy kosmos-scheduler success"

  docker exec ${clustername}-control-plane /bin/sh -c "mv /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes"
}

function load_cluster_images() {
  local -r clustername=$1

  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-network-manager:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-controller-manager:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-elector:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-operator:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-agent:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-proxy:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clustertree-cluster-manager:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/scheduler:"${VERSION}"
}

function load_kubenetst_cluster_images() {
  local -r clustername=$1

  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/virtual-cluster-operator:"${VERSION}"
  kind load docker-image -n "$clustername" ghcr.io/kosmos-io/node-agent:"${VERSION}"
}

function delete_cluster() {
  local -r clusterName=$1
  local -r clusterDir=$2

  kind delete clusters "${clusterName}"
  rm -rf "${clusterDir}"
  echo "cluster $clusterName delete success"
}
