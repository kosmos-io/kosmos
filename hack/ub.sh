#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
    echo "Usage:"
    echo "    hack/local-up-kubenest.sh [HOST_IPADDRESS] [-h]"
    echo "Args:"
    echo "    HOST_IPADDRESS: (required) if you want to export clusters' API server port to specific IP address"
    echo "    h: print help information"
}

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "$(dirname "${BASH_SOURCE[0]}")/install_kind_kubectl.sh"
source "$(dirname "${BASH_SOURCE[0]}")/cluster.sh"
source "$(dirname "${BASH_SOURCE[0]}")/util.sh"


function build_kind_image() {
    local image_name=$1

    # 使用 Dockerfile 构建镜像
    echo "Building custom Kind image: ${image_name}"
    
    docker build -t "${image_name}" -<<EOF
    FROM m.daocloud.io/docker.io/kindest/node:v1.27.3
    USER root
    RUN apt-get update && apt-get install -y vim ssh netcat iputils-ping net-tools dstat sudo && useradd -m -s /bin/bash docker && usermod -aG sudo docker
    RUN echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && systemctl enable ssh && printf "kosmos\nkosmos" | passwd && printf "docker\ndocker" | passwd docker

    USER docker
    RUN mkdir -p /home/docker/.ssh

    USER root
    RUN mkdir -p /etc/systemd/system.conf.d/
EOF
    echo "Image ${image_name} built successfully"
}


function prepare_vc_image() {

    #k8s
    # docker pull docker.m.daocloud.io/kindest/kindnetd:v20230511-dc714da8
    # docker pull docker.m.daocloud.io/kindest/local-path-provisioner:v20230511-dc714da8
    # docker pull docker.m.daocloud.io/kindest/local-path-helper:v20230510-486859a6

    # docker tag docker.m.daocloud.io/kindest/kindnetd:v20230511-dc714da8 docker.io/kindest/kindnetd:v20230511-dc714da8
    # docker tag docker.m.daocloud.io/kindest/local-path-provisioner:v20230511-dc714da8 docker.io/kindest/local-path-provisioner:v20230511-dc714da8
    # docker tag docker.m.daocloud.io/kindest/local-path-helper:v20230510-486859a6 docker.io/kindest/local-path-helper:v20230510-486859a6

    #openebs
    # "openebs/node-disk-manager:2.0.0"
    # "openebs/node-disk-operator:2.0.0"
    # "openebs/linux-utils:3.3.0"
    # "openebs/node-disk-exporter:2.0.0"
    # "openebs/provisioner-localpv:3.3.0"
    docker pull docker.m.daocloud.io/openebs/node-disk-manager:2.0.0
    docker pull docker.m.daocloud.io/openebs/node-disk-operator:2.0.0
    docker pull docker.m.daocloud.io/openebs/linux-utils:3.3.0
    docker pull docker.m.daocloud.io/openebs/node-disk-exporter:2.0.0
    docker pull docker.m.daocloud.io/openebs/provisioner-localpv:3.3.0

    docker tag docker.m.daocloud.io/openebs/node-disk-manager:2.0.0 openebs/node-disk-manager:2.0.0
    docker tag docker.m.daocloud.io/openebs/node-disk-operator:2.0.0 openebs/node-disk-operator:2.0.0
    docker tag docker.m.daocloud.io/openebs/linux-utils:3.3.0 openebs/linux-utils:3.3.0
    docker tag docker.m.daocloud.io/openebs/node-disk-exporter:2.0.0 openebs/node-disk-exporter:2.0.0
    docker tag docker.m.daocloud.io/openebs/provisioner-localpv:3.3.0 openebs/provisioner-localpv:3.3.0

    # 生成githhub 仓库镜像
    cd .. && make image-virtual-cluster-operator && cd hack
    # 根据上一步生成的镜像ID镜像替换，并设置标签
    latest_virtual_cluster_operator_image=$(docker images --format '{{.Repository}}  {{.ID}} {{.CreatedAt}}' | grep 'virtual-cluster-operator' | sort -k3,3 -r | head -n 1)
    # 检查是否找到镜像
    if [[ -n "${latest_virtual_cluster_operator_image}" ]]; then
        # 获取镜像 ID 和名称
        image_name=$(echo "${latest_virtual_cluster_operator_image}" | awk '{print $1}')
        image_id=$(echo "${latest_virtual_cluster_operator_image}" | awk '{print $2}')

        echo "找到最新的镜像: $image_name (ID: $image_id)"

        # 打上 latest 标签
        docker tag "$image_id" "${image_name}:latest"
        echo "已为镜像打上最新标签: ${image_name}:latest"
    else
        echo "未找到 virtual-cluster-operator 的镜像"
    fi
    
    cd .. && make image-node-agent && cd hack
    # 根据上一生成的镜像标签进行替换
    latest_node_agent_image=$(docker images --format '{{.Repository}} {{.ID}} {{.CreatedAt}}' | grep 'node-agent' | sort -k3,3 -r | head -n 1)

    # 检查是否找到镜像
    if [[ -n "${latest_node_agent_image}" ]]; then
        # 获取镜像 ID 和名称
        image_name=$(echo "${latest_node_agent_image}" | awk '{print $1}')
        image_id=$(echo "${latest_node_agent_image}" | awk '{print $2}')

        echo "找到最新的镜像: $image_name (ID: $image_id)"

        # 打上 latest 标签
        docker tag "$image_id" "${image_name}:latest"
        echo "已为镜像打上最新标签: ${image_name}:latest"
    else
        echo "未找到 node-agent 的镜像"
    fi

    

    # docker pull m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    # docker pull m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    # docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504 gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    # docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504 gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504

    # docker pull m.daocloud.io/registry.k8s.io/kube-apiserver:v1.25.7
    # docker pull m.daocloud.io/registry.k8s.io/kube-controller-manager:v1.25

    docker pull m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    docker pull m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    # docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504 gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    # docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504 gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504
    docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-server:v20211105-konnectivity-clientv0.0.25-2-g9e52504 kubenest.io/kas-network-proxy-server:"${VERSION}"
    docker tag m.daocloud.io/gcr.io/k8s-staging-kas-network-proxy/proxy-agent:v20211105-konnectivity-clientv0.0.25-2-g9e52504 kubenest.io/kas-network-proxy-agent:"${VERSION}"

    # 其他eki镜像
    # docker push hidevine/kube-apiserver:v1.25.7-eki.3.0.0
    # docker push hidevine/hidevine/scheduler:v1.25.7-eki.3.0.0
    # docker push hidevine/kube-proxy:v1.25.7-eki.3.0.0
    # docker push hidevine/kube-controller-manager:v1.25.7-eki.3.0.0
    # docker push hidevine/etcd:v1.25.7-eki.3.0.0
    # docker push hidevine/keepalived:v1.25.7-eki.3.0.0
    # docker push hidevine/kubectl:v1.25.7-eki.3.0.0

    docker pull hidevine/kube-apiserver:v1.25.7-eki.3.0.0
    docker pull hidevine/scheduler:v1.25.7-eki.3.0.0
    docker pull hidevine/kube-proxy:v1.25.7-eki.3.0.0
    docker pull hidevine/kube-controller-manager:v1.25.7-eki.3.0.0
    docker pull hidevine/etcd:v1.25.7-eki.3.0.0
    docker pull hidevine/keepalived:v1.25.7-eki.3.0.0
    docker pull hidevine/kubectl:v1.25.7-eki.3.0.0
    
    docker tag hidevine/kube-apiserver:v1.25.7-eki.3.0.0 kubenest.io/kube-apiserver:"${VERSION}"
    docker tag hidevine/scheduler:v1.25.7-eki.3.0.0 kubenest.io/scheduler:"${VERSION}"
    docker tag hidevine/kube-proxy:v1.25.7-eki.3.0.0 kubenest.io/kube-proxy:"${VERSION}"
    docker tag hidevine/kube-controller-manager:v1.25.7-eki.3.0.0 kubenest.io/kube-controller-manager:"${VERSION}"
    docker tag hidevine/etcd:v1.25.7-eki.3.0.0 kubenest.io/etcd:"${VERSION}"
    docker tag hidevine/keepalived:v1.25.7-eki.3.0.0 kubenest.io/keepalived:"${VERSION}"
    docker tag hidevine/kubectl:v1.25.7-eki.3.0.0 kubenest.io/kubectl:"${VERSION}"
    
}

function load_openebs_images() {
    local -r clustername=$1
    kind load docker-image openebs/node-disk-manager:2.0.0 --name "$clustername"
    kind load docker-image openebs/node-disk-operator:2.0.0 --name "$clustername"
    kind load docker-image openebs/linux-utils:3.3.0 --name "$clustername"
    kind load docker-image openebs/node-disk-exporter:2.0.0 --name "$clustername"
    kind load docker-image openebs/provisioner-localpv:3.3.0 --name "$clustername"
}

function load_vc_images() {
    local -r clustername=$1
    kind load docker-image  kubenest.io/kube-apiserver:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/scheduler:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/kube-proxy:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/kube-controller-manager:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/etcd:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/keepalived:"${VERSION}"   --name "$clustername"
    kind load docker-image  kubenest.io/kubectl:"${VERSION}"   --name "$clustername"
}


build_kind_image "kindest/node:v1.27.3.1"
KIND_IMAGE=${KIND_IMAGE:-"kindest/node:v1.27.3.1"}
HOST_IPADDRESS=${1:-}
HOST_CLUSTER_NAME="kubenest"
HOST_CLUSTER_POD_CIDR="10.233.64.0/18"
HOST_CLUSTER_SERVICE_CIDR="10.233.0.0/18"
VERSION=${VERSION:-"latest"}
CLUSTER_DIR="${REPO_ROOT}/environments/${HOST_CLUSTER_NAME}"

if [[ -z "${HOST_IPADDRESS}" ]]; then
  util::get_macos_ipaddress # Adapt for macOS
  HOST_IPADDRESS=${MAC_NIC_IPADDRESS:-}
fi
create_cluster "${KIND_IMAGE}" "$HOST_IPADDRESS" $HOST_CLUSTER_NAME $HOST_CLUSTER_POD_CIDR $HOST_CLUSTER_SERVICE_CIDR false true true

dockerip=$(docker inspect "${HOST_CLUSTER_NAME}-control-plane" --format "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}")
host_config_base64=$(cat ${CLUSTER_DIR}/kubeconfig-nodeIp | base64  -w0)

prepare_vc_image
load_kubenetst_cluster_images $HOST_CLUSTER_NAME  && load_openebs_images $HOST_CLUSTER_NAME && load_vc_images $HOST_CLUSTER_NAME
hostConfig_path="${ROOT}/environments/${HOST_CLUSTER_NAME}/kubeconfig-nodeIp"
kubectl --kubeconfig $hostConfig_path apply -f ${REPO_ROOT}/hack/k8s-in-k8s/openebs-hostpath.yaml

cat <<EOF >vc-operator.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: kosmos-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtual-cluster-operator
rules:
  - apiGroups: ['*']
    resources: ['*']
    verbs: ["*"]
  - nonResourceURLs: ['*']
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: virtual-cluster-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: virtual-cluster-operator
subjects:
  - kind: ServiceAccount
    name: virtual-cluster-operator
    namespace: kosmos-system
---
apiVersion: v1
kind: Secret
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
type: Opaque
data:
  kubeconfig: $host_config_base64
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
data:
  env.sh: |
    #!/usr/bin/env bash
    
    # #####
    # Generate by script generate_env.sh
    # #####
    
    SCRIPT_VERSION=0.0.1
    # tmp dir of kosmos
    PATH_FILE_TMP=/apps/conf/kosmos/tmp
    ##################################################
    # path for kubeadm config
    PATH_KUBEADM_CONFIG=/etc/kubeadm
    ##################################################
    # path for kubernetes, from kubelet args --config
    PATH_KUBERNETES=/etc/kubernetes
    PATH_KUBERNETES_PKI=/etc/kubernetes/pki
    # name for kubelet kubeconfig file
    KUBELET_KUBE_CONFIG_NAME=kubelet.conf
    ##################################################
    # path for kubelet
    PATH_KUBELET_LIB=/var/lib/kubelet
    # path for kubelet
    PATH_KUBELET_CONF=/var/lib/kubelet
    # name for config file of kubelet
    KUBELET_CONFIG_NAME=config.yaml
    HOST_CORE_DNS=10.96.0.10
    # kubeadm switch
    USE_KUBEADM=false
    # Generate kubelet.conf TIMEOUT
    KUBELET_CONF_TIMEOUT=30
    
    function GenerateKubeadmConfig() {
        echo "---
    apiVersion: kubeadm.k8s.io/v1beta2
    caCertPath: /etc/kubernetes/pki/ca.crt
    discovery:
        bootstrapToken:
            apiServerEndpoint: apiserver.cluster.local:6443
            token: \$1
            unsafeSkipCAVerification: true
    kind: JoinConfiguration
    nodeRegistration:
        criSocket: /run/containerd/containerd.sock
        kubeletExtraArgs:
        container-runtime: remote
        container-runtime-endpoint: unix:///run/containerd/containerd.sock
        taints: null" > \$2/kubeadm.cfg.current
    }
    
    function GenerateStaticNginxProxy() {
        echo "apiVersion: v1
    kind: Pod
    metadata:
      creationTimestamp: null
      name: nginx-proxy
      namespace: kube-system
    spec:
      containers:
      - image: registry.paas/cmss/nginx:1.21.4
        imagePullPolicy: IfNotPresent
        name: nginx-proxy
        resources:
          limits:
            cpu: 300m
            memory: 512M
          requests:
            cpu: 25m
            memory: 32M
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /etc/nginx
          name: etc-nginx
          readOnly: true
      hostNetwork: true
      priorityClassName: system-node-critical
      volumes:
      - hostPath:
          path: /apps/conf/nginx
          type:
        name: etc-nginx
    status: {}" > /etc/kubernetes/manifests/nginx-proxy.yaml
    }
  kubelet_node_helper.sh: |
    #!/usr/bin/env bash
    
    source "env.sh"
    
    # args
    DNS_ADDRESS=\${2:-10.237.0.10}
    LOG_NAME=\${2:-kubelet}
    JOIN_HOST=\$2
    JOIN_TOKEN=\$3
    JOIN_CA_HASH=\$4
    
    function unjoin() {
        # before unjoin, you need delete node by kubectl
        echo "exec(1/5): kubeadm reset...."
        echo "y" | kubeadm reset
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(2/5): restart cotnainerd...."
        systemctl restart containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(3/5): delete cni...."
        if [ -d "/etc/cni/net.d" ]; then
            mv /etc/cni/net.d '/etc/cni/net.d.kosmos.back'\`date +%Y_%m_%d_%H_%M_%S\`
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(4/5): delete ca.crt"
        if [ -f "\$PATH_KUBERNETES_PKI/ca.crt" ]; then
            echo "y" | rm "\$PATH_KUBERNETES_PKI/ca.crt"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(5/5): delete kubelet.conf"
        if [ -f "\$PATH_KUBELET_CONF/\${KUBELET_CONFIG_NAME}" ]; then
            echo "y" | rm "\$PATH_KUBELET_CONF/\${KUBELET_CONFIG_NAME}"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    }
    
    function beforeRevert() {
        if [ -f "/apps/conf/nginx/nginx.conf" ]; then
            # modify  hosts
            config_file="/apps/conf/nginx/nginx.conf"
    
            server_address=\$(grep -Po 'server\s+\K[^:]+(?=:6443)' "\$config_file" | awk 'NR==1')
            hostname=\$(echo \$JOIN_HOST | awk -F ":" '{print \$1}')
            host_record="\$server_address \$hostname"
            if grep -qFx "\$host_record" /etc/hosts; then
                echo "Record \$host_record already exists in /etc/hosts."
            else
                sed -i "1i \$host_record" /etc/hosts
                echo "Record \$host_record inserted into /etc/hosts."
            fi
        fi
    }
    
    function afterRevert() {
        if [ -f "/apps/conf/nginx/nginx.conf" ]; then
            # modify  hosts
            config_file="/apps/conf/nginx/nginx.conf"
    
            server_address=\$(grep -Po 'server\s+\K[^:]+(?=:6443)' "\$config_file" | awk 'NR==1')
            hostname=\$(echo \$JOIN_HOST | awk -F ":" '{print \$1}')
            host_record="\$server_address \$hostname"
            if grep -qFx "\$host_record" /etc/hosts; then
                sudo sed -i "/^\$host_record/d" /etc/hosts
            fi
    
            local_record="127.0.0.1 \$hostname"
            if grep -qFx "\$local_record" /etc/hosts; then
                echo "Record \$local_record already exists in /etc/hosts."
            else
                sed -i "1i \$local_record" /etc/hosts
                echo "Record \$local_record inserted into /etc/hosts."
            fi
    
            GenerateStaticNginxProxy
        fi
    }

    function get_ca_certificate() {
         local output_file="\$PATH_KUBERNETES_PKI/ca.crt"
         local kubeconfig_data=\$(curl -sS --insecure "https://\$JOIN_HOST/api/v1/namespaces/kube-public/configmaps/cluster-info" 2>/dev/null | \
                               \ grep -oP 'certificate-authority-data:\s*\K.*(?=server:[^[:space:]]*?)' | \
                               \  sed -e 's/^certificate-authority-data://' -e 's/[[:space:]]//g' -e 's/\\n$//g')

         # verify the kubeconfig data is not empty
         if [ -z "\$kubeconfig_data" ]; then
           echo "Failed to extract certificate-authority-data."
           return 1
         fi

         # Base64 decoded and written to a file
         echo "\$kubeconfig_data" | base64 --decode > "\$output_file"

         # check that the file was created successfully
         if [ -f "\$output_file" ]; then
             echo "certificate-authority-data saved to \$output_file"
         else
             echo "Failed to save certificate-authority-data to \$output_file"
          return 1
         fi
    }

    function create_kubelet_bootstrap_config() {
       # Checks if the parameters are provided
     if [ -z "\$JOIN_HOST" ] || [ -z "\$JOIN_TOKEN" ]; then
         echo "Please provide server and token as parameters."
         return 1
     fi

     # Define file contents
     cat << EOF > bootstrap-kubelet.conf
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        certificate-authority: \$PATH_KUBERNETES_PKI/ca.crt
        server: https://\$JOIN_HOST
      name: kubernetes
    contexts:
    - context:
        cluster: kubernetes
        user: kubelet-bootstrap
      name: kubelet-bootstrap-context
    current-context: kubelet-bootstrap-context
    preferences: {}
    users:
    - name: kubelet-bootstrap
      user:
        token: \$JOIN_TOKEN
    EOF

     # copy the file to the /etc/kubernetes directory
     cp bootstrap-kubelet.conf \$PATH_KUBERNETES

     echo "the file bootstrap-kubelet.conf has stored in \$PATH_KUBERNETES directory."
    }

    
    function revert() {
        echo "exec(1/5): update kubeadm.cfg..."
        if [ ! -f "\$PATH_KUBEADM_CONFIG/kubeadm.cfg" ]; then
            GenerateKubeadmConfig  \$JOIN_TOKEN \$PATH_FILE_TMP
        else
          sed -e "s|token: .*\$|token: \$JOIN_TOKEN|g" -e "w \$PATH_FILE_TMP/kubeadm.cfg.current" "\$PATH_KUBEADM_CONFIG/kubeadm.cfg"
        fi
    
        # add taints
        echo "exec(2/5): update kubeadm.cfg tanits..."
        sed -i "/kubeletExtraArgs/a \    register-with-taints: node.kosmos.io/unschedulable:NoSchedule"  "\$PATH_FILE_TMP/kubeadm.cfg.current"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(3/5): update kubelet-config..."
        sed -e "s|__DNS_ADDRESS__|\$HOST_CORE_DNS|g" -e "w \${PATH_KUBELET_CONF}/\${KUBELET_CONFIG_NAME}" "\$PATH_FILE_TMP"/"\$KUBELET_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
    #    beforeRevert
    #    if [ \$? -ne 0 ]; then
    #        exit 1
    #    fi
    
    
        echo "exec(4/5): execute join cmd...."
        kubeadm join --config "\$PATH_FILE_TMP/kubeadm.cfg.current"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(5/5): restart cotnainerd...."
        systemctl restart containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
    #    afterRevert
    #    if [ \$? -ne 0 ]; then
    #        exit 1
    #    fi
    
    
    }
    
    # before join, you need upload ca.crt and kubeconfig to tmp dir!!!
    function join() {
        echo "exec(1/8): stop containerd...."
        systemctl stop containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(2/8): copy ca.crt...."
        cp "\$PATH_FILE_TMP/ca.crt" "\$PATH_KUBERNETES_PKI/ca.crt"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(3/8): copy kubeconfig...."
        cp "\$PATH_FILE_TMP/\$KUBELET_KUBE_CONFIG_NAME" "\$PATH_KUBERNETES/\$KUBELET_KUBE_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(4/8): set core dns address...."
        sed -e "s|__DNS_ADDRESS__|\$DNS_ADDRESS|g" -e "w \${PATH_KUBELET_CONF}/\${KUBELET_CONFIG_NAME}" "\$PATH_FILE_TMP"/"\$KUBELET_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(5/8): copy kubeadm-flags.env...."
        cp "\$PATH_FILE_TMP/kubeadm-flags.env" "\$PATH_KUBELET_LIB/kubeadm-flags.env"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(6/8): delete cni...."
        if [ -d "/etc/cni/net.d" ]; then
            mv /etc/cni/net.d '/etc/cni/net.d.back'\`date +%Y_%m_%d_%H_%M_%S\`
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(7/8): start containerd"
        systemctl start containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(8/8): start kubelet...."
        systemctl start kubelet
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    }
    
    function health() {
        result=\`systemctl is-active containerd\`
        if [[ \$result != "active" ]]; then
            echo "health(1/2): containerd is inactive"
            exit 1
        else
            echo "health(1/2): containerd is active"
        fi
    
        result=\`systemctl is-active kubelet\`
        if [[ \$result != "active" ]]; then
            echo "health(2/2): kubelet is inactive"
            exit 1
        else
            echo "health(2/2): containerd is active"
        fi
    }
    
    function log() {
        systemctl status \$LOG_NAME
    }
    
    # check the environments
    function check() {
        # TODO: create env file
        echo "check(1/2): try to create \$PATH_FILE_TMP"
        if [ ! -d "\$PATH_FILE_TMP" ]; then
            mkdir -p "\$PATH_FILE_TMP"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "check(2/2): copy  kubeadm-flags.env  to create \$PATH_FILE_TMP , remove args[cloud-provider] and taints"
        sed -e "s| --cloud-provider=external | |g" -e "w \${PATH_FILE_TMP}/kubeadm-flags.env" "\$PATH_KUBELET_LIB/kubeadm-flags.env"
        sed -i "s| --register-with-taints=node.kosmos.io/unschedulable:NoSchedule||g" "\${PATH_FILE_TMP}/kubeadm-flags.env"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "environments is ok"
    }
    
    function version() {
        echo "\$SCRIPT_VERSION"
    }
    
    # See how we were called.
    case "\$1" in
        unjoin)
        unjoin
        ;;
        join)
        join
        ;;
        health)
        health
        ;;
        check)
        check
        ;;
        log)
        log
        ;;
        revert)
        revert
        ;;
        version)
        version
        ;;
        *)
        echo $"usage: \$0 unjoin|join|health|log|check|version|revert"
        exit 1
    esac
  config.yaml: |
    apiVersion: kubelet.config.k8s.io/v1beta1
    authentication:
      anonymous:
        enabled: false
      webhook:
        cacheTTL: 0s
        enabled: true
      x509:
        clientCAFile: /etc/kubernetes/pki/ca.crt
    authorization:
      mode: Webhook
      webhook:
        cacheAuthorizedTTL: 0s
        cacheUnauthorizedTTL: 0s
    cgroupDriver: systemd
    clusterDNS:
    - __DNS_ADDRESS__
    clusterDomain: cluster.local
    cpuManagerReconcilePeriod: 0s
    evictionHard:
      imagefs.available: 15%
      memory.available: 100Mi
      nodefs.available: 10%
      nodefs.inodesFree: 5%
    evictionPressureTransitionPeriod: 5m0s
    fileCheckFrequency: 0s
    healthzBindAddress: 127.0.0.1
    healthzPort: 10248
    httpCheckFrequency: 0s
    imageMinimumGCAge: 0s
    kind: KubeletConfiguration
    kubeAPIBurst: 100
    kubeAPIQPS: 100
    kubeReserved:
      cpu: 140m
      memory: 1.80G
    logging:
      flushFrequency: 0
      options:
        json:
          infoBufferSize: "0"
      verbosity: 0
    memorySwap: {}
    nodeStatusReportFrequency: 0s
    nodeStatusUpdateFrequency: 0s
    rotateCertificates: true
    runtimeRequestTimeout: 0s
    shutdownGracePeriod: 0s
    shutdownGracePeriodCriticalPods: 0s
    staticPodPath: /etc/kubernetes/manifests
    streamingConnectionIdleTimeout: 0s
    syncFrequency: 0s
    volumeStatsAggPeriod: 0s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
  labels:
    app: virtual-cluster-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: virtual-cluster-operator
  template:
    metadata:
      labels:
        app: virtual-cluster-operator
    spec:
      # Enter the name of the node where the virtual cluster operator is deployed
      nodeName: kubenest-control-plane
      serviceAccountName: virtual-cluster-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - virtual-cluster-operator
                topologyKey: kubernetes.io/hostname
      containers:
        - name: virtual-cluster-operator
          image: ghcr.io/kosmos-io/virtual-cluster-operator:${VERSION}
          imagePullPolicy: IfNotPresent
          env:
            - name: WAIT_NODE_READ_TIME
              value: "120"
            - name: IMAGE_REPOSITIRY
              value: kubenest.io
            - name: IMAGE_VERSION
              value: ${VERSION}
            - name: EXECTOR_HOST_MASTER_NODE_IP
              value: $dockerip
            - name: EXECTOR_SHELL_NAME
              value: kubelet_node_helper_1.sh
            - name: WEB_USER
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: username
            - name: WEB_PASS
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: password
          volumeMounts:
          - name: credentials
            mountPath: /etc/virtual-cluster-operator
            readOnly: true
          - name: shellscript
            mountPath: /etc/vc-node-dir/env.sh
            subPath: env.sh
          - name: shellscript
            mountPath: /etc/vc-node-dir/kubelet_node_helper_1.sh
            subPath: kubelet_node_helper.sh
          - name: shellscript
            mountPath: /etc/vc-node-dir/config.yaml
            subPath: config.yaml
          - mountPath: /kosmos/manifest
            name: components-manifest
          command:
          - virtual-cluster-operator
          - --kubeconfig=/etc/virtual-cluster-operator/kubeconfig
          - --kube-nest-anp-mode=uds
          - --v=4
      volumes:
        - name: credentials
          secret:
            secretName: virtual-cluster-operator
        - name: shellscript
          configMap:
            name: virtual-cluster-operator
        - hostPath:
            path: /apps/vc-operator/manifest
            type: DirectoryOrCreate
          name: components-manifest
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: node-agent
  namespace: kosmos-system
spec:
  selector:
    matchLabels:
      app: node-agent-service
  template:
    metadata:
      labels:
        app: node-agent-service
    spec:
      hostPID: true # access host pid
      hostIPC: true # access host ipc
      hostNetwork: true # access host network
      tolerations:
        - operator: Exists # run on all nodes
      initContainers:
        - name: init-agent
          image: ghcr.io/kosmos-io/node-agent:${VERSION}
          securityContext:
            privileged: true
          env:
            - name: WEB_USER
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: username
            - name: WEB_PASS
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: password
          command: ["/bin/bash"]
          args:
            - "/app/init.sh"
          volumeMounts:
            - mountPath: /host-path
              name: node-agent
              readOnly: false
            - mountPath: /host-systemd
              name: systemd-path
              readOnly: false
      containers:
        - name: install-agent
          image: ghcr.io/kosmos-io/node-agent:${VERSION}
          securityContext:
            privileged: true # container privileged
          command:
            - nsenter
            - --target
            - "1"
            - --mount
            - --uts
            - --ipc
            - --net
            - --pid
            - --
            - bash
            - -l
            - -c
            - "/srv/node-agent/start.sh && sleep infinity"
      volumes:
        - name: node-agent
          hostPath:
            path: /srv/node-agent
            type: DirectoryOrCreate
        - name: systemd-path
          hostPath:
            path: /etc/systemd/system
            type: DirectoryOrCreate
---
apiVersion: v1
kind: Secret
metadata:
  name: node-agent-secret
  namespace: kosmos-system
type: kubernetes.io/basic-auth
stringData:
  username: "kosmos-node-agent"
  password: "bdp_dspt_202X_pA@Min1a"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kosmos-hostports
  namespace: kosmos-system
data:
  config.yaml: |
    # ports allocate for virtual cluster api server,from 33001, increment by 1 for each virtual cluster.Be careful not to use ports that are already in use
    portsPool:
      - 33001
      - 33002
      - 33003
      - 33004
      - 33005
      - 33006
      - 33007
      - 33008
      - 33009
      - 33010
      - 33011
      - 33012
      - 33013
      - 33014
      - 33015
      - 33016
      - 33017
      - 33018
      - 33019
      - 33020
      - 33021
      - 33022
      - 33023
      - 33024
      - 33025
      - 33026
      - 33027
      - 33028
      - 33029
      - 33030
      - 33031
      - 33032
      - 33033
      - 33034
      - 33035
      - 33036
      - 33037
      - 33038
      - 33039
      - 33040
      - 33041
      - 33042
      - 33043
      - 33044
      - 33045
      - 33046
      - 33037
      - 33048
      - 33049
      - 33050
---
apiVersion: v1
data:
  components: |
    [
      {"name": "kube-proxy", "path": "/kosmos/manifest/kube-proxy/*.yaml"},
      {"name": "calico", "path": "/kosmos/manifest/calico/*.yaml"},
      {"name": "keepalived", "path": "/kosmos/manifest/keepalived/*.yaml"},
    ]
  host-core-dns-components: |
    [
      {"name": "core-dns-host", "path": "/kosmos/manifest/core-dns/host/*.yaml"},
    ]
  virtual-core-dns-components: |
    [
      {"name": "core-dns-virtual", "path": "/kosmos/manifest/core-dns/virtualcluster/*.yaml"},
    ]
kind: ConfigMap
metadata:
  name: components-manifest-cm
  namespace: kosmos-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kosmos-vip-pool
  namespace: kosmos-system
data:
  vip-config.yaml: |
    # can be use for vc, the ip formate is 192.168.0.1 and 192.168.0.2-192.168.0.10
    vipPool:
      - 192.168.6.110-192.168.6.120
EOF

kubectl --kubeconfig $hostConfig_path apply -f vc-operator.yaml

echo "create vc-operator success"
echo "wait all kosmos pod ready"
# N = nodeNum + 1
N=$(kubectl --kubeconfig $hostConfig_path get pod -n kosmos-system --no-headers | wc -l)
util::wait_for_condition "all pod are ready" \
  "kubectl --kubeconfig $hostConfig_path get pod -n kosmos-system | awk 'NR>1 {if (\$2 != \"Running\") exit 1; }' && [ \$(kubectl --kubeconfig $hostConfig_path get pod -n kosmos-system --no-headers | wc -l) -eq ${N} ]" \
  300
echo "all pod ready"

kubectl --kubeconfig $hostConfig_path apply -f ${REPO_ROOT}/deploy/crds/kosmos.io_virtualclusters.yaml
kubectl --kubeconfig $hostConfig_path apply -f ${REPO_ROOT}/deploy/crds/kosmos.io_globalnodes.yaml

export KUBECONFIG=$hostConfig_path
bash bash ${REPO_ROOT}/hack/k8s-in-k8s/generate_globalnode.sh







