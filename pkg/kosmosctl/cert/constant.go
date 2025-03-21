package cert

const certShell = `
#!/usr/bin/env bash

source "env.sh"

CERT_PATH=/apps/conf/kosmos/cert

function update() {
    echo "exec(1/): copy ca.crt...."
    cp "$CERT_PATH/ca.crt" "$PATH_KUBERNETES_PKI/ca.crt"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(2/): copy kubeconfig...."
    cp "$CERT_PATH/kubelet.conf" "$PATH_KUBERNETES/$KUBELET_KUBE_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi

	KUBELET_PKI_PATH="${PATH_KUBELET_LIB}/pki/*"
    echo "exec(3/): remove pki form kubelet.... ${KUBELET_PKI_PATH}"
    rm -rf $KUBELET_PKI_PATH

    systemctl restart kubelet
    systemctl status kubelet 

    
}

# See how we were called.
case "$1" in
    update)
    update
    ;;
    *)
    echo $"usage: $0 update"
    exit 1
esac
`
