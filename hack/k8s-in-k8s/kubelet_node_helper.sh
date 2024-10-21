#!/usr/bin/env bash

source "env.sh"

# args
DNS_ADDRESS=${2:-10.237.0.10}
LOG_NAME=${2:-kubelet}
JOIN_HOST=$2
JOIN_TOKEN=$3
JOIN_CA_HASH=$4

function unjoin() {
    # before unjoin, you need delete node by kubectl
    echo "exec(1/5): kubeadm reset...."
    echo "y" | kubeadm reset
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(2/5): restart cotnainerd...."
    systemctl restart containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(3/5): delete cni...."
    if [ -d "/etc/cni/net.d" ]; then   
        mv /etc/cni/net.d '/etc/cni/net.d.kosmos.back'`date +%Y_%m_%d_%H_%M_%S`
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi

    echo "exec(4/5): delete ca.crt"
    if [ -f "$PATH_KUBERNETES_PKI/ca.crt" ]; then
        echo "y" | rm "$PATH_KUBERNETES_PKI/ca.crt"
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi

    echo "exec(5/5): delete kubelet.conf"
    if [ -f "$PATH_KUBELET_CONF/${KUBELET_CONFIG_NAME}" ]; then
        echo "y" | rm "$PATH_KUBELET_CONF/${KUBELET_CONFIG_NAME}"
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi
}

function beforeRevert() {
    if [ -f "/apps/conf/nginx/nginx.conf" ]; then 
        # modify  hosts
        config_file="/apps/conf/nginx/nginx.conf"

        server_address=$(grep -Po 'server\s+\K[^:]+(?=:6443)' "$config_file" | awk 'NR==1')
        hostname=$(echo $JOIN_HOST | awk -F ":" '{print $1}')
        host_record="$server_address $hostname"
        if grep -qFx "$host_record" /etc/hosts; then
            echo "Record $host_record already exists in /etc/hosts."
        else
            sed -i "1i $host_record" /etc/hosts
            echo "Record $host_record inserted into /etc/hosts."
        fi
    fi
}

function afterRevert() {
    if [ -f "/apps/conf/nginx/nginx.conf" ]; then 
        # modify  hosts
        config_file="/apps/conf/nginx/nginx.conf"

        server_address=$(grep -Po 'server\s+\K[^:]+(?=:6443)' "$config_file" | awk 'NR==1')
        hostname=$(echo $JOIN_HOST | awk -F ":" '{print $1}')
        host_record="$server_address $hostname"
        if grep -qFx "$host_record" /etc/hosts; then
            sudo sed -i "/^$host_record/d" /etc/hosts
        fi

        local_record="127.0.0.1 $hostname"
        if grep -qFx "$local_record" /etc/hosts; then
            echo "Record $local_record already exists in /etc/hosts."
        else
            sed -i "1i $local_record" /etc/hosts
            echo "Record $local_record inserted into /etc/hosts."
        fi

        GenerateStaticNginxProxy
    fi
}

function get_ca_certificate() {
     local output_file="$PATH_KUBERNETES_PKI/ca.crt"
     local kubeconfig_data=$(curl -sS --insecure "https://$JOIN_HOST/api/v1/namespaces/kube-public/configmaps/cluster-info" 2>/dev/null | \
                             grep -oP 'certificate-authority-data:\s*\K.*(?=server:[^[:space:]]*?)' | \
                             sed -e 's/^certificate-authority-data://' -e 's/[[:space:]]//g' -e 's/\\n$//g')

     # verify the kubeconfig data is not empty
     if [ -z "$kubeconfig_data" ]; then
       echo "Failed to extract certificate-authority-data."
       return 1
     fi

     # Base64 decoded and written to a file
     echo "$kubeconfig_data" | base64 --decode > "$output_file"

     # check that the file was created successfully
     if [ -f "$output_file" ]; then
         echo "certificate-authority-data saved to $output_file"
     else
         echo "Failed to save certificate-authority-data to $output_file"
      return 1
     fi
}

function create_kubelet_bootstrap_config() {
   # Checks if the parameters are provided
 if [ -z "$JOIN_HOST" ] || [ -z "$JOIN_TOKEN" ]; then
     echo "Please provide server and token as parameters."
     return 1
 fi

 # Define file contents
 cat << EOF > bootstrap-kubelet.conf
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: $PATH_KUBERNETES_PKI/ca.crt
    server: https://$JOIN_HOST
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
    token: $JOIN_TOKEN
EOF

 # copy the file to the /etc/kubernetes directory
 cp bootstrap-kubelet.conf $PATH_KUBERNETES

 echo "the file bootstrap-kubelet.conf has stored in $PATH_KUBERNETES directory."
}

function revert() {
    echo "exec(1/5): update kubeadm.cfg..."
    if [ ! -f "$PATH_KUBEADM_CONFIG/kubeadm.cfg" ]; then
        GenerateKubeadmConfig  $JOIN_TOKEN $PATH_FILE_TMP
    else
      sed -e "s|token: .*$|token: $JOIN_TOKEN|g" -e "w $PATH_FILE_TMP/kubeadm.cfg.current" "$PATH_KUBEADM_CONFIG/kubeadm.cfg"
    fi
    
    # add taints
    echo "exec(2/5): update kubeadm.cfg tanits..."
    sed -i "/kubeletExtraArgs/a \    register-with-taints: node.kosmos.io/unschedulable:NoSchedule"  "$PATH_FILE_TMP/kubeadm.cfg.current" 
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(3/5): update kubelet-config..."
    sed -e "s|__DNS_ADDRESS__|$HOST_CORE_DNS|g" -e "w ${PATH_KUBELET_CONF}/${KUBELET_CONFIG_NAME}" "$PATH_FILE_TMP"/"$KUBELET_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi

    beforeRevert
    if [ $? -ne 0 ]; then
        exit 1
    fi


    echo "exec(4/5): execute join cmd...."
    if [ -z "$USE_KUBEADM" ]; then
      # if "USE_KUBEADM is not set, default set to true"
      export USE_KUBEADM=true
    fi
    if [ "$USE_KUBEADM" = true ]; then
       echo "use kubeadm to join node to host"
       kubeadm join --config "$PATH_FILE_TMP/kubeadm.cfg.current"
       if [ $? -ne 0 ]; then
          exit 1
       fi
    else
       echo "NONONO use kubeadm to join node to host"
       get_ca_certificate $JOIN_HOST
       create_kubelet_bootstrap_config $JOIN_HOST $JOIN_TOKEN
       if [ -f "${PATH_FILE_TMP}/kubeadm-flags.env.origin" ]; then
          cp "${PATH_FILE_TMP}/kubeadm-flags.env.origin" "${PATH_KUBELET_LIB}" && \
          mv "${PATH_KUBELET_LIB}/kubeadm-flags.env.origin" "${PATH_KUBELET_LIB}/kubeadm-flags.env"
       else
          cp "${PATH_FILE_TMP}/kubeadm-flags.env" "${PATH_KUBELET_LIB}"
       fi
    fi

    echo "exec(5/5): restart cotnainerd...."
    systemctl restart containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi

    if [ "$USE_KUBEADM" = false ]; then
           systemctl start kubelet
           elapsed_time=0

           while [ $elapsed_time -lt $KUBELET_CONF_TIMEOUT ]; do
             if [ -f "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" ]; then
                rm -f "${PATH_KUBERNETES}/bootstrap-kubelet.conf"
                echo "Deleted bootstrap-kubelet.conf file as kubelet.conf exists."
                break
             fi
             sleep 2
             elapsed_time=$((elapsed_time + 2))
           done

           if [ $elapsed_time -ge $KUBELET_CONF_TIMEOUT ]; then
              echo "Timeout: kubelet.conf was not generated within $KUBELET_CONF_TIMEOUT seconds. Continuing script execution."
           fi
        fi
    afterRevert
    if [ $? -ne 0 ]; then
        exit 1
    fi


}

# before join, you need upload ca.crt and kubeconfig to tmp dir!!!
function join() {
    echo "exec(1/8): stop containerd...."
    systemctl stop containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(2/8): copy ca.crt...."
    cp "$PATH_FILE_TMP/ca.crt" "$PATH_KUBERNETES_PKI/ca.crt"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(3/8): copy kubeconfig...."
    cp "$PATH_FILE_TMP/$KUBELET_KUBE_CONFIG_NAME" "$PATH_KUBERNETES/$KUBELET_KUBE_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(4/8): set core dns address...."
    sed -e "s|__DNS_ADDRESS__|$DNS_ADDRESS|g" -e "w ${PATH_KUBELET_CONF}/${KUBELET_CONFIG_NAME}" "$PATH_FILE_TMP"/"$KUBELET_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    echo "exec(5/8): copy kubeadm-flags.env...."
    cp "$PATH_FILE_TMP/kubeadm-flags.env" "$PATH_KUBELET_LIB/kubeadm-flags.env"
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(6/8): delete cni...."
    if [ -d "/etc/cni/net.d" ]; then   
        mv /etc/cni/net.d '/etc/cni/net.d.back'`date +%Y_%m_%d_%H_%M_%S`
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi

    echo "exec(7/8): start containerd"
    systemctl start containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(8/8): start kubelet...."
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
    # TODO: create env file
    echo "check(1/2): try to create $PATH_FILE_TMP"
    if [ ! -d "$PATH_FILE_TMP" ]; then   
        mkdir -p "$PATH_FILE_TMP"
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi
    
    echo "check(2/2): copy  kubeadm-flags.env  to create $PATH_FILE_TMP , remove args[cloud-provider] and taints"
    # Since this function is used both to detach nodes, we need to make sure we haven't copied kubeadm-flags.env before
    if [ ! -f "${PATH_FILE_TMP}/kubeadm-flags.env.origin" ]; then
       cp "${PATH_KUBELET_LIB}/kubeadm-flags.env" "${PATH_FILE_TMP}/kubeadm-flags.env.origin"
    fi
    sed -e "s| --cloud-provider=external | |g" -e "w ${PATH_FILE_TMP}/kubeadm-flags.env" "$PATH_KUBELET_LIB/kubeadm-flags.env"
    sed -i "s| --register-with-taints=node.kosmos.io/unschedulable:NoSchedule||g" "${PATH_FILE_TMP}/kubeadm-flags.env"
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "environments is ok"
}

function version() {
    echo "$SCRIPT_VERSION"
}


function is_ipv6() {
    if [[ "$1" =~ : ]]; then
        return 0 
    else
        return 1
    fi
}

function install_lb() {
    if [ -z "$USE_NGINX" ]; then
      export USE_KUBEADM=false
    fi

    if [ "$USE_NGINX" = false ]; then
        exit 0
    fi

    echo "exec(1/6): get port of apiserver...."

    PORT=$(grep 'server:' "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" | awk -F '[:/]' '{print $NF}')

    if [ -z "$PORT" ]; then
        echo "can not get port"
        exit 1
    else
        echo "port is $PORT"
    fi

    if [ "$LOCAL_PORT" -eq "$PORT" ]; then
        echo "Error: LOCAL_PORT ($LOCAL_PORT) cannot be the same as the backend port ($PORT)."
        exit 0
    fi

    # Start generating nginx.conf
    echo "exec(1/6): generate nginx.conf...."
    cat <<EOL > "$PATH_FILE_TMP/nginx.conf"
error_log stderr notice;
worker_processes 1;
events {
  multi_accept on;
  use epoll;
  worker_connections 1024;
}

stream {
        upstream kube_apiserver {
            least_conn;
EOL

    # Loop through the array and append each server to the nginx.conf file
    for SERVER in "${SERVERS[@]}"; do
        if is_ipv6 "$SERVER"; then
            echo "            server [$SERVER]:$PORT;" >> "$PATH_FILE_TMP/nginx.conf"
        else
            echo "            server $SERVER:$PORT;" >> "$PATH_FILE_TMP/nginx.conf"
        fi
    done

    # Continue writing the rest of the nginx.conf
    cat <<EOL >> "$PATH_FILE_TMP/nginx.conf"
        }
        server {
            listen        [::]:$LOCAL_PORT;
            listen        6443;
            proxy_pass    kube_apiserver;
            proxy_timeout 10m;
            proxy_connect_timeout 10s;
        }
}
EOL

    echo "exec(1/6): create static pod"
    GenerateStaticNginxProxy true


    echo "exec(1/6): restart static pod"
    mv "${PATH_KUBERNETES}/manifests/nginx-proxy.yaml" "${PATH_KUBERNETES}/nginx-proxy.yaml"
    sleep 2
    mv "${PATH_KUBERNETES}/nginx-proxy.yaml" "${PATH_KUBERNETES}/manifests/nginx-proxy.yaml"

    echo "exec(1/6): update kubelet.conf"
    cp "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}.bak"
    sed -i "s|server: .*|server: https://${LOCAL_IP}:${LOCAL_PORT}|" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}"

    echo "exec(1/6): restart kubelet"
    systemctl restart kubelet
}

# See how we were called.
case "$1" in
    unjoin)
    unjoin
    ;;
    install_lb)
    install_lb
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
    echo $"usage: $0 unjoin|join|health|log|check|version|revert"
    exit 1
esac