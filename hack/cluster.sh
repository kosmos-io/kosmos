#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CURRENT="$(dirname "${BASH_SOURCE[0]}")"
ROOT=$(dirname "${BASH_SOURCE[0]}")/..
DEFAULT_NAMESPACE="clusterlink-system"
KIND_IMAGE="ghcr.io/kosmos-io/node:v1.25.3"
# KIND_IMAGE="ghcr.io/kosmos-io/node:v0.1.0"
# true: when cluster is exist, reuse exist one!
REUSE=${REUSE:-false}
VERSION=${VERSION:-latest}

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
        docker pull docker.io/calico/apiserver:v3.25.0
        docker pull docker.io/calico/node-driver-registrar:v3.25.0
    else
        docker pull quay.m.daocloud.io/tigera/operator:v1.29.0
        docker pull docker.m.daocloud.io/calico/cni:v3.25.0
        docker pull docker.m.daocloud.io/calico/typha:v3.25.0
        docker pull docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0
        docker pull docker.m.daocloud.io/calico/kube-controllers:v3.25.0
        docker pull docker.m.daocloud.io/calico/node:v3.25.0
        docker pull docker.m.daocloud.io/calico/csi:v3.25.0

        docker tag quay.m.daocloud.io/tigera/operator:v1.29.0 quay.io/tigera/operator:v1.29.0
        docker tag docker.m.daocloud.io/calico/cni:v3.25.0 docker.io/calico/cni:v3.25.0
        docker tag docker.m.daocloud.io/calico/typha:v3.25.0 docker.io/calico/typha:v3.25.0
        docker tag docker.m.daocloud.io/calico/pod2daemon-flexvol:v3.25.0 docker.io/calico/pod2daemon-flexvol:v3.25.0
        docker tag docker.m.daocloud.io/calico/kube-controllers:v3.25.0 docker.io/calico/kube-controllers:v3.25.0
        docker tag docker.m.daocloud.io/calico/node:v3.25.0 docker.io/calico/node:v3.25.0
        docker tag docker.m.daocloud.io/calico/csi:v3.25.0 docker.io/calico/csi:v3.25.0
    fi

    kind load docker-image -n "$clustername" quay.io/tigera/operator:v1.29.0
    kind load docker-image -n "$clustername" docker.io/calico/cni:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/typha:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/pod2daemon-flexvol:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/kube-controllers:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/node:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/csi:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/apiserver:v3.25.0
    kind load docker-image -n "$clustername" docker.io/calico/node-driver-registrar:v3.25.0

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

}

function join_cluster() {
  local host_cluster=$1
  local member_cluster=$2
  local container_ip_port
  cat <<EOF | kubectl --context="kind-${host_cluster}" apply -f -
apiVersion: kosmos.io/v1alpha1
kind: Cluster
metadata:
  name: ${member_cluster}
spec:
  cni: "calico"
  defaultNICName: eth0
  imageRepository: "ghcr.io/kosmos-io/clusterlink"
  networkType: "gateway"
EOF
  kubectl --context="kind-${member_cluster}" apply -f "$ROOT"/deploy/clusterlink-namespace.yml
  kubectl --context="kind-${member_cluster}" -n clusterlink-system delete secret controlpanel-config || true
  kubectl --context="kind-${member_cluster}" -n clusterlink-system create secret generic controlpanel-config --from-file=kubeconfig="${ROOT}/environments/${host_cluster}/kubeconfig"
  kubectl --context="kind-${member_cluster}" apply -f "$ROOT"/deploy/clusterlink-datapanel-rbac.yml
  sed -e "s|__VERSION__|$VERSION|g" -e "s|__CLUSTER_NAME__|$member_cluster|g" -e "w ${ROOT}/environments/${member_cluster}/clusterlink-operator.yml" "$ROOT"/deploy/clusterlink-operator.yml
  kubectl --context="kind-${member_cluster}" apply -f "${ROOT}/environments/${member_cluster}/clusterlink-operator.yml"
}

function deploy_clusterlink() {
   local -r clustername=$1
   kubectl config use-context "kind-${clustername}"
   load_clusterlink_images "$clustername"

   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/clusterlink-namespace.yml
   kubectl --context="kind-${clustername}" apply -f "$ROOT"/deploy/crds
   util::wait_for_crd clusternodes.kosmos.io clusters.kosmos.io

   sed -e "s|__VERSION__|$VERSION|g" -e "w ${ROOT}/environments/clusterlink-network-manager.yml" "$ROOT"/deploy/clusterlink-network-manager.yml
   kubectl --context="kind-${clustername}" apply -f "${ROOT}/environments/clusterlink-network-manager.yml"

   echo "cluster $clustername deploy clusterlink success"
}

function load_clusterlink_images() {
    local -r clustername=$1

    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-network-manager:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-controller-manager:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-elector:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-operator:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-agent:"${VERSION}"
    kind load docker-image -n "$clustername" ghcr.io/kosmos-io/clusterlink/clusterlink-proxy:"${VERSION}"
}

function delete_cluster() {
    local -r clustername=$1
    kind delete clusters $clustername
    CLUSTER_DIR="${ROOT}/environments/${clustername}"
    rm -rf "$CLUSTER_DIR"
    echo "cluster $clustername delete success"
}
