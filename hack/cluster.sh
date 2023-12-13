#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

HOST_CLUSTER_NAME="cluster-host"
CURRENT="$(dirname "${BASH_SOURCE[0]}")"
ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KIND_IMAGE="ghcr.io/kosmos-io/node:v1.25.3"
# true: when cluster is exist, reuse exist one!
REUSE=${REUSE:-false}
VERSION=${VERSION:-latest}

# default cert and key for node server https
CERT=$(cat ${ROOT}/pkg/cert/crt.pem | base64 -w 0)
KEY=$(cat ${ROOT}/pkg/cert/key.pem | base64 -w 0)

CN_ZONE=${CN_ZONE:-false}

if [ $REUSE == true ]; then
    echo "!!!!!!!!!!!Warning: Setting REUSE to true will not delete existing clusters.!!!!!!!!!!!"
fi

source "${ROOT}/hack/util.sh"

#clustername podcidr servicecidr
function create_cluster() {
    local -r clustername=$1
    local -r podcidr=$2
    local -r servicecidr=$3
    local -r isDual=${4:-false}

    CLUSTER_DIR="${ROOT}/environments/${clustername}"
    mkdir -p "${CLUSTER_DIR}"
    ipFamily=ipv4
    if [ "$isDual" == true ]; then
      ipFamily=dual
      pod_convert=$(printf %x $(echo $podcidr | awk -F "." '{print $2" "$3}' ))
      svc_convert=$(printf %x $(echo $servicecidr | awk -F "." '{print $2" "$3}' ))
      podcidr_ipv6="fd11:1111:1111:"$pod_convert"::/64"
      servicecidr_ipv6="fd11:1111:1112:"$svc_convert"::/108"
      podcidr_all=${podcidr_ipv6}","${podcidr}
      servicecidr_all=${servicecidr_ipv6}","${servicecidr}
      sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__POD_CIDR_IPV6__|$podcidr_ipv6|g" -e "s|#DUAL||g" -e "w ${CLUSTER_DIR}/calicoconfig" "${CURRENT}/clustertemplete/calicoconfig"
      sed -e "s|__POD_CIDR__|$podcidr_all|g" -e "s|__SERVICE_CIDR__|$servicecidr_all|g" -e "s|__IP_FAMILY__|$ipFamily|g" -e "w ${CLUSTER_DIR}/kindconfig" "${CURRENT}/clustertemplete/kindconfig"
    else
      sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__SERVICE_CIDR__|$servicecidr|g" -e "s|__IP_FAMILY__|$ipFamily|g" -e "w ${CLUSTER_DIR}/kindconfig" "${CURRENT}/clustertemplete/kindconfig"
      sed -e "s|__POD_CIDR__|$podcidr|g" -e "s|__SERVICE_CIDR__|$servicecidr|g" -e "w ${CLUSTER_DIR}/calicoconfig" "${CURRENT}/clustertemplete/calicoconfig"
    fi

    if [[ "$(kind get clusters | grep -c "${clustername}")" -eq 1 && "${REUSE}" = true ]]; then
      echo "cluster ${clustername} exist reuse it"
    else
      kind delete clusters $clustername || true
      kind create cluster --name $clustername --config ${CLUSTER_DIR}/kindconfig --image $KIND_IMAGE
    fi

    dockerip=$(docker inspect "${clustername}-control-plane" --format "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}")
    kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true

    # prepare external kubeconfig
    docker exec ${clustername}-control-plane /bin/sh -c "cat /etc/kubernetes/admin.conf"| sed -e "s|${clustername}-control-plane|$dockerip|g" -e "/certificate-authority-data:/d" -e "5s/^/    insecure-skip-tls-verify: true\n/"  -e "w ${CLUSTER_DIR}/kubeconfig"

    # install calico
    if [ "${CN_ZONE}" == false ]; then
        docker pull quay.io/tigera/operator:v1.29.0
        docker pull docker.io/calico/cni:v3.25.0
        docker pull docker.io/calico/typha:v3.25.0
        docker pull docker.io/calico/pod2daemon-flexvol:v3.25.0
        docker pull docker.io/calico/kube-controllers:v3.25.0
        docker pull docker.io/calico/node:v3.25.0
        docker pull docker.io/calico/csi:v3.25.0
        docker pull docker.io/percona:5.7
        docker pull docker.io/library/nginx:latest
        docker pull docker.io/library/busybox:latest
    else
        docker pull quay.m.daocloud.io/tigera/operator:v1.29.0
        docker pull docker.m.daocloud.io/calico/cni:v3.25.0
        docker pull docker.m.daocloud.io/calico/typha:v3.25.0
        docker pull docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0
        docker pull docker.m.daocloud.io/calico/kube-controllers:v3.25.0
        docker pull docker.m.daocloud.io/calico/node:v3.25.0
        docker pull docker.m.daocloud.io/calico/csi:v3.25.0
        docker pull docker.m.daocloud.io/percona:5.7
        docker pull docker.m.daocloud.io/library/nginx:latest
        docker pull docker.m.daocloud.io/library/busybox:latest

        docker tag quay.m.daocloud.io/tigera/operator:v1.29.0 quay.io/tigera/operator:v1.29.0
        docker tag docker.m.daocloud.io/calico/cni:v3.25.0 docker.io/calico/cni:v3.25.0
        docker tag docker.m.daocloud.io/calico/typha:v3.25.0 docker.io/calico/typha:v3.25.0
        docker tag docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0 docker.io/calico/pod2daemon-flexvol:v3.25.0
        docker tag docker.m.daocloud.io/calico/kube-controllers:v3.25.0 docker.io/calico/kube-controllers:v3.25.0
        docker tag docker.m.daocloud.io/calico/node:v3.25.0 docker.io/calico/node:v3.25.0
        docker tag docker.m.daocloud.io/calico/csi:v3.25.0 docker.io/calico/csi:v3.25.0
        docker tag docker.m.daocloud.io/percona:5.7 docker.io/percona:5.7
        docker tag docker.m.daocloud.io/library/nginx:latest docker.io/library/nginx:latest
        docker tag docker.m.daocloud.io/library/busybox:latest docker.io/library/busybox:latest
    fi

    kind load docker-image -n "$clustername" quay.io/tigera/operator:v1.29.0
    kind load docker-image -n "$clustername" docker.io/calico/cni:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/typha:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/pod2daemon-flexvol:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/kube-controllers:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/node:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/csi:v3.25.0
    kind load docker-image -n "$clustername" docker.io/percona:5.7
    kind load docker-image -n "$clustername" docker.io/library/nginx:latest
    kind load docker-image -n "$clustername" docker.io/library/busybox:latest

    if "${clustername}" == $HOST_CLUSTER_NAME ; then
        if [ "${CN_ZONE}" == false ]; then
            docker pull docker.io/bitpoke/mysql-operator-orchestrator:v0.6.3
            docker pull docker.io/prom/mysqld-exporter:v0.13.0
            docker pull docker.io/bitpoke/mysql-operator-sidecar-8.0:v0.6.3
            docker pull docker.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3
            docker pull docker.io/bitpoke/mysql-operator:v0.6.3
        else
            docker pull docker.m.daocloud.io/bitpoke/mysql-operator-orchestrator:v0.6.3
            docker pull docker.m.daocloud.io/prom/mysqld-exporter:v0.13.0
            docker pull docker.m.daocloud.io/bitpoke/mysql-operator-sidecar-8.0:v0.6.3
            docker pull docker.m.daocloud.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3
            docker pull docker.m.daocloud.io/bitpoke/mysql-operator:v0.6.3

            docker tag docker.m.daocloud.io/bitpoke/mysql-operator-orchestrator:v0.6.3 docker.io/bitpoke/mysql-operator-orchestrator:v0.6.3
            docker tag docker.m.daocloud.io/prom/mysqld-exporter:v0.13.0 docker.io/prom/mysqld-exporter:v0.13.0
            docker tag docker.m.daocloud.io/bitpoke/mysql-operator-sidecar-8.0:v0.6.3 docker.io/bitpoke/mysql-operator-sidecar-8.0:v0.6.3
            docker tag docker.m.daocloud.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3 docker.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3
            docker tag docker.m.daocloud.io/bitpoke/mysql-operator:v0.6.3 docker.io/bitpoke/mysql-operator:v0.6.3
        fi
            kind load docker-image -n "$clustername" docker.io/bitpoke/mysql-operator-orchestrator:v0.6.3
            kind load docker-image -n "$clustername" docker.io/prom/mysqld-exporter:v0.13.0
            kind load docker-image -n "$clustername" docker.io/bitpoke/mysql-operator-sidecar-8.0:v0.6.3
            kind load docker-image -n "$clustername" docker.io/bitpoke/mysql-operator-sidecar-5.7:v0.6.3
            kind load docker-image -n "$clustername" docker.io/bitpoke/mysql-operator:v0.6.3
    fi
    kubectl --context="kind-${clustername}" create -f "$CURRENT/calicooperator/tigera-operator.yaml" || $("${REUSE}" -eq "true")
    kind export kubeconfig --name "$clustername"
    util::wait_for_crd installations.operator.tigera.io
    kubectl --context="kind-${clustername}" apply -f "${CLUSTER_DIR}"/calicoconfig
    echo "create cluster ${clustername} success"
    echo "wait all node ready"
    # N = nodeNum + 1
    N=$(kubectl get nodes --no-headers | wc -l)
    util::wait_for_condition "all nodes are ready" \
      "kubectl get nodes | awk 'NR>1 {if (\$2 != \"Ready\") exit 1; }' && [ \$(kubectl get nodes --no-headers | wc -l) -eq ${N} ]" \
      300
    echo "all node ready"

    kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/crds/mcs
}

function join_cluster() {
  local host_cluster=$1
  local member_cluster=$2
  local kubeconfig_path="${ROOT}/environments/${member_cluster}/kubeconfig"
  local base64_kubeconfig=$(base64 -w 0 < "$kubeconfig_path")
  echo " base64 kubeconfig successfully converted: $base64_kubeconfig "

  local common_metadata=""
  if [ "$host_cluster" == "$member_cluster" ]; then
    common_metadata="annotations:
    kosmos.io/cluster-role: root"
  fi

  cat <<EOF | kubectl --context="kind-${host_cluster}" apply -f -
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
  kubectl --context="kind-${member_cluster}" apply -f "$ROOT"/deploy/clusterlink-namespace.yml
  kubectl --context="kind-${member_cluster}" apply -f "$ROOT"/deploy/clusterlink-datapanel-rbac.yml
}

function deploy_cluster() {
   local -r clustername=$1
   kubectl config use-context "kind-${clustername}"
   load_cluster_images "$clustername"

   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/clusterlink-namespace.yml
   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/kosmos-rbac.yml
   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/crds
   util::wait_for_crd clusternodes.kosmos.io clusters.kosmos.io

   sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/clusterlink-network-manager.yml" "$ROOT"/deploy/clusterlink-network-manager.yml
   kubectl --context="kind-${clustername}" apply -f "${ROOT}/environments/clusterlink-network-manager.yml"

   echo "cluster $clustername deploy clusterlink success"

   sed -e "s|__VERSION__|$VERSION|g" -e "s|__CERT__|$CERT|g" -e "s|__KEY__|$KEY|g" -e "w ${ROOT}/environments/clustertree-cluster-manager.yml" "$ROOT"/deploy/clustertree-cluster-manager.yml
   kubectl --context="kind-${clustername}" apply -f "${ROOT}/environments/clustertree-cluster-manager.yml"

   echo "cluster $clustername deploy clustertree success"

   kubectl --context="kind-${clustername}" -n kosmos-system delete secret controlpanel-config || true
   kubectl --context="kind-${clustername}" -n kosmos-system create secret generic controlpanel-config --from-file=kubeconfig="${ROOT}/environments/cluster-host/kubeconfig"
   sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/kosmos-operator.yml" "$ROOT"/deploy/kosmos-operator.yml
   kubectl --context="kind-${clustername}" apply -f "${ROOT}/environments/kosmos-operator.yml"

   echo "cluster $clustername deploy kosmos-operator success"

   sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/kosmos-scheduler.yml" "$ROOT"/deploy/scheduler/deployment.yaml
   kubectl --context="kind-${clustername}" apply -f "${ROOT}/environments/kosmos-scheduler.yml"
   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/scheduler/rbac.yaml

   util::wait_for_condition "kosmos scheduler are ready" \
     "kubectl -n kosmos-system get deploy kosmos-scheduler -o jsonpath='{.status.replicas}{\" \"}{.status.readyReplicas}{\"\n\"}' | awk '{if (\$1 == \$2 && \$1 > 0) exit 0; else exit 1}'" \
     300
   echo "cluster $clustername deploy kosmos-scheduler success"

   docker exec ${clustername}-control-plane /bin/sh -c "mv /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes"
}

function load_cluster_images() {
    local -r clustername=$1

    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-network-manager:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-controller-manager:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-elector:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/kosmos-operator:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-agent:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink-proxy:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clustertree-cluster-manager:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/scheduler:"${VERSION}"
}

function delete_cluster() {
    local -r clustername=$1
    kind delete clusters $clustername
    CLUSTER_DIR="${ROOT}/environments/${clustername}"
    rm -rf "$CLUSTER_DIR"
    echo "cluster $clustername delete success"
}
