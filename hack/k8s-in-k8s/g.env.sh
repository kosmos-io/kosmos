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
KUBELET_KUBE_CONFIG_NAME=
##################################################
# path for kubelet
PATH_KUBELET_LIB=/var/lib/kubelet
# path for kubelet 
PATH_KUBELET_CONF=.
# name for config file of kubelet
KUBELET_CONFIG_NAME=
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
        token: $1
        unsafeSkipCAVerification: true
kind: JoinConfiguration
nodeRegistration:
    criSocket: /run/containerd/containerd.sock
    kubeletExtraArgs:
    container-runtime: remote
    container-runtime-endpoint: unix:///run/containerd/containerd.sock
    taints: null" > $2/kubeadm.cfg.current
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


