#!/usr/bin/env bash

# This script will generate an g.env.sh file, like the following:
# #!/usr/bin/env bash

# # #####
# # Generate by script generate_env.sh
# # #####

# SCRIPT_VERSION=0.0.1
# # tmp dir of kosmos
# PATH_FILE_TMP=/apps/conf/kosmos/tmp
# ##################################################
# # path for kubeadm config
# PATH_KUBEADM_CONFIG=/etc/kubeadm
# ##################################################
# # path for kubernetes, from kubelet args --config
# PATH_KUBERNETES=/etc/kubernetes
# PATH_KUBERNETES_PKI=/etc/kubernetes/pki
# # name for kubelet kubeconfig file
# KUBELET_KUBE_CONFIG_NAME=kubelet.conf
# ##################################################
# # path for kubelet
# PATH_KUBELET_LIB=/var/lib/kubelet
# # path for kubelet
# PATH_KUBELET_CONF=/var/lib/kubelet
# # name for config file of kubelet
# KUBELET_CONFIG_NAME=config.yaml

# function GenerateKubeadmConfig() {
#     echo "---
# apiVersion: kubeadm.k8s.io/v1beta2
# discovery:
#     bootstrapToken:
#         apiServerEndpoint: apiserver.cluster.local:6443
#         token: $1
#         unsafeSkipCAVerification: true
# kind: JoinConfiguration
# nodeRegistration:
#     criSocket: /run/containerd/containerd.sock
#     kubeletExtraArgs:
#     container-runtime: remote
#     container-runtime-endpoint: unix:///run/containerd/containerd.sock
#     taints: null" > $2/kubeadm.cfg.current
# }




SCRIPT_VERSION=0.0.1
# save tmp file
PATH_FILE_TMP=/apps/conf/kosmos/tmp
# path for kubeadm config
PATH_KUBEADM_CONFIG=/etc/kubeadm
# path for kubelet lib
PATH_KUBELET_LIB=/var/lib/kubelet


function GetKubeletConfigFilePath() {
    systemctl status kubelet | grep -o '\--config=[^ ]*' | awk -F= '{print $2}'
}

function GetKubeletKubeConfigFilePath() {
    systemctl status kubelet | grep -o '\--kubeconfig=[^ ]*' | awk -F= '{print $2}'
}

function GetKubernetesCaPath() {
    kubectl get cm kubelet-config -nkube-system -oyaml  | awk '/clientCAFile:/{print $2}'
}

function GetKubeDnsClusterIP() {
    kubectl get svc -nkube-system kube-dns  -o jsonpath='{.spec.clusterIP}'
}

function GetFileName() {
    local fullpath="$1"
    local filename=$(basename "$fullpath")
    echo "$filename"
}

function GetDirectory() {
    local fullpath="$1"
    if [ -z "$fullpath" ]; then
        echo "Error: No directory found."
        exit 1
    fi
    local directory=$(dirname "$fullpath")
    echo "$directory"
}

function GetMasterNodeIPs() {
  kubectl get nodes -l node-role.kubernetes.io/master="" -o jsonpath='{range .items[*]}{.status.addresses[?(@.type=="InternalIP")].address}{" "}{end}'
}

# kubelet config name
KUBELET_CONFIG_NAME=$(GetFileName "$(GetKubeletConfigFilePath)")
# path for kubelet 
PATH_KUBELET_CONF=$(GetDirectory "$(GetKubeletConfigFilePath)")
# kubelet  kubeconfig  file  name
KUBELET_KUBE_CONFIG_NAME=$(GetFileName "$(GetKubeletKubeConfigFilePath)")

# ca.crt path
PATH_KUBERNETES_PKI=$(GetDirectory "$(GetKubernetesCaPath)")
# length=${#PATH_KUBERNETES_PKI}
PATH_KUBERNETES=$(GetDirectory $PATH_KUBERNETES_PKI)
HOST_CORE_DNS=$(GetKubeDnsClusterIP)

DOCKER_IMAGE_NGINX="registry.paas/cmss/nginx:1.21.4"
SERVERS=$(GetMasterNodeIPs)
if [ -z "$SERVERS" ]; then
    echo "Error: No master nodes found or failed to retrieve node IPs."
    exit 1
fi
LOCAL_PORT="6443"
LOCAL_IP="127.0.0.1"  # [::1]

echo "#!/usr/bin/env bash

# #####
# Generate by script generate_env.sh
# #####

SCRIPT_VERSION=$SCRIPT_VERSION
# tmp dir of kosmos
PATH_FILE_TMP=$PATH_FILE_TMP
##################################################
# path for kubeadm config
PATH_KUBEADM_CONFIG=$PATH_KUBEADM_CONFIG
##################################################
# path for kubernetes, from kubelet args --config
PATH_KUBERNETES=$PATH_KUBERNETES
PATH_KUBERNETES_PKI=$PATH_KUBERNETES_PKI
# name for kubelet kubeconfig file
KUBELET_KUBE_CONFIG_NAME=$KUBELET_KUBE_CONFIG_NAME
##################################################
# path for kubelet
PATH_KUBELET_LIB=$PATH_KUBELET_LIB
# path for kubelet 
PATH_KUBELET_CONF=$PATH_KUBELET_CONF
# name for config file of kubelet
KUBELET_CONFIG_NAME=$KUBELET_CONFIG_NAME
HOST_CORE_DNS=$HOST_CORE_DNS
# kubeadm switch
USE_KUBEADM=false
# Generate kubelet.conf TIMEOUT
KUBELET_CONF_TIMEOUT=30

# load balance
DOCKER_IMAGE_NGINX=$DOCKER_IMAGE_NGINX
SERVERS=($SERVERS)
LOCAL_PORT="6443"
LOCAL_IP="127.0.0.1"  # [::1]
USE_NGINX=true

function GenerateKubeadmConfig() {
    echo \"---
apiVersion: kubeadm.k8s.io/v1beta2
caCertPath: $PATH_KUBERNETES_PKI/ca.crt
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
    taints: null\" > \$2/kubeadm.cfg.current
}

function GenerateStaticNginxProxy() {
    config_path=/apps/conf/nginx
    if [ "\$1" == \"true\" ]; then
      config_path=\$PATH_FILE_TMP
    fi
    echo \"apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: nginx-proxy
  namespace: kube-system
spec:
  containers:
  - image: \$DOCKER_IMAGE_NGINX
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
      path: \$config_path
      type: 
    name: etc-nginx
status: {}\" > $PATH_KUBERNETES/manifests/nginx-proxy.yaml
}

" > g.env.sh


cat g.env.sh  