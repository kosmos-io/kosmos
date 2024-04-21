#!/usr/bin/env bash

SCRIPT_VERSION=0.0.1
# save tmp file
PATH_FILE_TMP=/apps/conf/kosmos/tmp
###################################################
# path for kubeadm
PATH_KUBEADM=/usr/bin/kubeadm
##################################################
# path for kubernetes
PATH_KUBERNETES=/etc/kubernetes/
PATH_KUBERNETES_PKI="$PATH_KUBERNETES/pki"
# scpKCCmd.name
KUBELET_KUBE_CONFIG_NAME=kubelet.conf
##################################################
# path for kubelet
PATH_KUBELET_LIB=/var/lib/kubelet
# scpKubeletConfigCmd.name
KUBELET_CONFIG_NAME=config.yaml

# args
DNS_ADDRESS=${2:-10.237.0.10}
LOG_NAME=${2:-kubelet}

function unjoin() {
    # before unjoin, you need delete node by kubectl
    echo "exec(1/1): kubeadm reset...."
    echo "y" | ${PATH_KUBEADM} reset
    if [ $? -ne 0 ]; then
        exit 1
    fi
}


# before join, you need upload ca.crt and kubeconfig to tmp dir!!!
function join() {
    echo "exec(1/7): stop containerd...."
    systemctl stop containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(2/7): copy ca.crt...."
    cp "$PATH_FILE_TMP/ca.crt" "$PATH_KUBERNETES_PKI/ca.crt"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(3/7): copy kubeconfig...."
    cp "$PATH_FILE_TMP/$KUBELET_KUBE_CONFIG_NAME" "$PATH_KUBERNETES/$KUBELET_KUBE_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(4/7): set core dns address...."
    sed -e "s|__DNS_ADDRESS__|$DNS_ADDRESS|g" -e "w ${PATH_KUBELET_LIB}/${KUBELET_CONFIG_NAME}" "$PATH_FILE_TMP"/"$KUBELET_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(5/7): copy kubeadm-flags.env...."
    cp "$PATH_FILE_TMP/kubeadm-flags.env" "$PATH_KUBELET_LIB/kubeadm-flags.env"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(6/7): start containerd"
    systemctl start containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(7/7): start kubelet...."
    systemctl start kubelet
    if [ $? -ne 0 ]; then
        exit 1
    fi
}

function health() {
    result=`systemctl is-active containerd`
    if [[ $result != "active" ]]; then
        echo "health(1/2): containerd is inactive"
        exit 1
    else
        echo "health(1/2): containerd is active"
    fi

    result=`systemctl is-active kubelet`
    if [[ $result != "active" ]]; then
        echo "health(2/2): kubelet is inactive"
        exit 1
    else
        echo "health(2/2): containerd is active"
    fi
}

function log() {
    systemctl status $LOG_NAME
}

# check the environments
function check() {
    if [ ! -d "$PATH_FILE_TMP" ]; then
        echo "check(1/2): try to create $PATH_FILE_TMP"
        mkdir -p "$PATH_FILE_TMP"
        if [ $? -ne 0 ]; then
            exit 1
        fi
        echo "check(2/2): copy  kubeadm-flags.env  to create $PATH_FILE_TMP"
        echo "y" | cp "$PATH_KUBELET_LIB/kubeadm-flags.env" "$PATH_FILE_TMP/"
    fi
    echo "environments is ok"
}

function version() {
    echo "$SCRIPT_VERSION"
}

# See how we were called.
case "$1" in
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
  version)
    version
    ;;
  *)
    echo $"usage: $0 unjoin|join|health|log|check|version"
    exit 1
esac
