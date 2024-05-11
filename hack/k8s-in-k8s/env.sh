#!/usr/bin/env bash

SCRIPT_VERSION=0.0.1
# save tmp file
PATH_FILE_TMP=/apps/conf/kosmos/tmp
###################################################
# path for kubeadm
PATH_KUBEADM=/usr/bin/kubeadm
##################################################
# path for kubeadm config
PATH_KUBEADM_CONFIG=/etc/kubeadm
##################################################
# path for kubernetes
PATH_KUBERNETES=/apps/conf/kubernetes/
PATH_KUBERNETES_PKI="$PATH_KUBERNETES/ssl"
# scpKCCmd.name
KUBELET_KUBE_CONFIG_NAME=kubelet.conf
##################################################
# path for kubelet
PATH_KUBELET_LIB=/var/lib/kubelet
# scpKubeletConfigCmd.name
KUBELET_CONFIG_NAME=config.yaml