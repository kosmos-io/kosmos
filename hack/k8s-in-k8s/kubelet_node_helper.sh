#!/usr/bin/env bash

source "env.sh"

# args
DNS_ADDRESS=${2:-10.237.0.10}
LOG_NAME=${2:-kubelet}
JOIN_HOST=$2
JOIN_TOKEN=$3
JOIN_CA_HASH=$4
NODE_LOCAL_DNS_ADDRESS=$3


function cri_runtime_clean() {
    criSocket=unix://$CRI_SOCKET
    containers=($(crictl -r $criSocket pods -q))

    if [ ${#containers[@]} -eq 0 ]; then
        echo "No containers found in containerd"
        return 0
    fi

    for container in "${containers[@]}"; do
        echo "Stopping container: $container"
        crictl -r $criSocket stopp "$container"
        echo "Removing container: $container"
        crictl -r $criSocket rmp "$container"
    done
}


function docker_runtime_clean() {
    containers=($(docker ps -a --filter name=k8s_ -q))

    if [ ${#containers[@]} -eq 0 ]; then
        echo "No containers found matching the filter 'k8s_'"
        return 0
    fi

    for container in "${containers[@]}"; do
        echo "Stopping container: $container"
        docker stop "$container"
        echo "Removing container: $container"
        docker rm "$container"
    done

}

# Function to unmount all directories under a given directory
function unmount_kubelet_directory() {
    kubelet_dir="$1"

    if [ -z "$kubelet_dir" ]; then
        echo "Error: kubelet directory not specified."
        exit 1
    fi

    # Ensure the directory has a trailing slash
    if [[ "$kubelet_dir" != */ ]]; then
        kubelet_dir="${kubelet_dir}/"
    fi


    mounts=($(awk -v dir="$kubelet_dir" '$0 ~ dir {print $2}' /proc/mounts))

    for mount in "${mounts[@]}"; do
        echo "Unmounting $mount..."
        if ! umount "$mount"; then
            echo "Warning: Failed to unmount $mount" >&2
        fi
    done
}

function clean_dirs() {
   files_to_delete=(
        "${PATH_KUBELET_LIB}/*"
        "${PATH_KUBERNETES}/manifests/*"
        "${PATH_KUBERNETES_PKI}/*"
        "${PATH_KUBERNETES}/admin.conf"
        "${PATH_KUBERNETES}/kubelet.conf"
        "${PATH_KUBERNETES}/bootstrap-kubelet.conf"
        "${PATH_KUBERNETES}/controller-manager.conf"
        "${PATH_KUBERNETES}/scheduler.conf"
        "/var/lib/dockershim"
        "/var/run/kubernetes"
        "/var/lib/cni"
    )
    for file in "${files_to_delete[@]}"; do
        echo "Deleting file: $file"
        rm -rf $file
    done
}


# similar to the reset function of kubeadm. kubernetes/cmd/kubeadm/app/cmd/phases/reset/cleanupnode.go
function node_reset() {
    echo "exec node_reset(1/4): stop kubelet...."
    systemctl stop kubelet

    echo "exec node_reset(2/4): remove container of kubernetes...."
    if [[ "$CRI_SOCKET" == *"docker"* ]]; then
        docker_runtime_clean
    elif [[ "$CRI_SOCKET" == *"containerd"* ]]; then
        cri_runtime_clean
    else 
        echo "Unknown runtime: $CRI_SOCKET"
        exit 1
    fi
    
    echo "exec node_reset(3/4): unmount kubelet lib...."
    # /kubernetes/cmd/kubeadm/app/cmd/phases/reset/cleanupnode.go:151 CleanDir
    unmount_kubelet_directory "${PATH_KUBELET_LIB}"

    echo "exec node_reset(4/4): clean file for kubernetes...."
    clean_dirs
}

function unjoin() {
    # before unjoin, you need delete node by kubectl
    echo "exec(1/5): kubeadm reset...."
    node_reset
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

function before_revert() {
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

function after_revert() {
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
    echo "exec(1/6): update kubeadm.cfg..."
    if [ ! -f "$PATH_KUBEADM_CONFIG/kubeadm.cfg" ]; then
        GenerateKubeadmConfig  $JOIN_TOKEN $PATH_FILE_TMP
    else
      sed -e "s|token: .*$|token: $JOIN_TOKEN|g" -e "w $PATH_FILE_TMP/kubeadm.cfg.current" "$PATH_KUBEADM_CONFIG/kubeadm.cfg"
    fi
    
    # add taints
    echo "exec(2/6): update kubeadm.cfg tanits..."
    sed -i "/kubeletExtraArgs/a \    register-with-taints: node.kosmos.io/unschedulable:NoSchedule"  "$PATH_FILE_TMP/kubeadm.cfg.current" 
    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(3/6): update kubelet-config..."
    sed -e "s|__DNS_ADDRESS__|$HOST_CORE_DNS|g" -e "w ${PATH_KUBELET_CONF}/${KUBELET_CONFIG_NAME}" "$PATH_FILE_TMP"/"$KUBELET_CONFIG_NAME"
    if [ $? -ne 0 ]; then
        exit 1
    fi

    before_revert
    if [ $? -ne 0 ]; then
        exit 1
    fi


    echo "exec(4/6): execute join cmd...."

    echo "NONONO use kubeadm to join node to host"
    get_ca_certificate $JOIN_HOST
    if [ $? -ne 0 ]; then
        exit 1
    fi
    create_kubelet_bootstrap_config $JOIN_HOST $JOIN_TOKEN
    if [ -f "${PATH_FILE_TMP}/kubeadm-flags.env.origin" ]; then
        cp "${PATH_FILE_TMP}/kubeadm-flags.env.origin" "${PATH_KUBELET_LIB}" && \
        mv "${PATH_KUBELET_LIB}/kubeadm-flags.env.origin" "${PATH_KUBELET_LIB}/kubeadm-flags.env"
    else
        cp "${PATH_FILE_TMP}/kubeadm-flags.env" "${PATH_KUBELET_LIB}"
    fi

    echo "exec(5/6): restart cotnainerd...."
    systemctl restart containerd
    if [ $? -ne 0 ]; then
        exit 1
    fi

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

    after_revert

    if [ $? -ne 0 ]; then
        exit 1
    fi

    echo "exec(6/6): revert manifests...."
    if [ -d "$PATH_FILE_TMP/manifests.origin" ]; then  
        if [[ -n "$(ls -A ${PATH_FILE_TMP}/manifests.origin/ 2>/dev/null)" ]]; then
            cp -r ${PATH_FILE_TMP}/manifests.origin/* "${PATH_KUBERNETES}/manifests/" 
        else
            echo "No files in ${PATH_FILE_TMP}/manifests.origin"
        fi
        
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
    if [ -n "$NODE_LOCAL_DNS_ADDRESS" ]; then
        sed -e "/__DNS_ADDRESS__/i - ${NODE_LOCAL_DNS_ADDRESS}" \
            -e "s|__DNS_ADDRESS__|${DNS_ADDRESS}|g" \
            "$PATH_FILE_TMP/$KUBELET_CONFIG_NAME" \
            > "${PATH_KUBELET_CONF}/${KUBELET_CONFIG_NAME}"
    else
        sed -e "s|__DNS_ADDRESS__|$DNS_ADDRESS|g" -e "w ${PATH_KUBELET_CONF}/${KUBELET_CONFIG_NAME}" "$PATH_FILE_TMP"/"$KUBELET_CONFIG_NAME"
    fi
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

function backup_manifests() {
    echo "backup_manifests(1/1): backup manifests"
    if [ ! -d "$PATH_FILE_TMP/manifests.origin" ]; then   
        mkdir -p "$PATH_FILE_TMP/manifests.origin"
        if [ $? -ne 0 ]; then
            exit 1
        fi
        if [[ -n "$(ls -A ${PATH_KUBERNETES}/manifests/ 2>/dev/null)" ]]; then
            cp -rf ${PATH_KUBERNETES}/manifests/* ${PATH_FILE_TMP}/manifests.origin/
        else
            echo "No files in ${PATH_KUBERNETES}/manifests/"
        fi
    fi
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

    backup_manifests
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

function wait_api_server_proxy_ready() {
    local retries=0
    local max_retries=10
    local sleep_duration=6

    while true; do
        response=$(curl -k --connect-timeout 5 --max-time 10 https://${LOCAL_IP}:${LOCAL_PORT}/healthz)
        
        if [ "$response" == "ok" ]; then
            echo "apiserver proxy is ready!"
            return 0 
        else
            retries=$((retries + 1))
            echo "apiserver proxy is not ready. Retrying(${retries}/${max_retries})..."
            if [ "$retries" -ge "$max_retries" ]; then
                echo "Max retries reached. apiserver proxy did not become ready."
                return 1
            fi
            sleep $sleep_duration
        fi
    done
}

function install_nginx_lb() {
    echo "exec(1/7): get port of apiserver...."

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
    echo "exec(2/7): generate nginx.conf...."
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

    echo "exec(3/7): create static pod"
    GenerateStaticNginxProxy true


    echo "exec(4/7): restart static pod"
    mv "${PATH_KUBERNETES}/manifests/nginx-proxy.yaml" "${PATH_KUBERNETES}/nginx-proxy.yaml"
    sleep 2
    mv "${PATH_KUBERNETES}/nginx-proxy.yaml" "${PATH_KUBERNETES}/manifests/nginx-proxy.yaml"

    echo "exec(5/7): wati nginx ready"
    if wait_api_server_proxy_ready; then
        echo "nginx is ready"
    else
        echo "nginx is not ready"
        exit 1
    fi

    echo "exec(6/7): update kubelet.conf"
    cp "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}.bak"
    sed -i "s|server: .*|server: https://${LOCAL_IP}:${LOCAL_PORT}|" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}"

    echo "exec(7/7): restart kubelet"
    systemctl restart kubelet
}

function install_lvscare_lb() {
    echo "exec(1/7): get port of apiserver...."

    PORT=$(grep 'server:' "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" | awk -F '[:/]' '{print $NF}')

    if [ -z "$PORT" ]; then
        echo "can not get port"
        exit 1
    else
        echo "port is $PORT"
    fi

    # Start generating kube-lvscare.yaml
    echo "exec(2/7): generate kube-lvscare.yaml...."

    cat <<EOL > $PATH_KUBERNETES/manifests/kube-lvscare.yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: kube-lvscare
  name: kube-lvscare
  namespace: kube-system
spec:
  containers:
  - args:
    - care
    - --vs
    - ${LOCAL_IP}:${LOCAL_PORT}
    - --health-path
    - /healthz
    - --health-schem
    - https
EOL

    # Loop through the array and append each server to the kube-lvscare.yaml file
    for SERVER in "${SERVERS[@]}"; do
        if is_ipv6 "$SERVER"; then
            echo "    - --rs" >> "$PATH_KUBERNETES/manifests/kube-lvscare.yaml"
            echo "    - [$SERVER]:$PORT" >> "$PATH_KUBERNETES/manifests/kube-lvscare.yaml"
        else
            echo "    - --rs" >> "$PATH_KUBERNETES/manifests/kube-lvscare.yaml"
            echo "    - $SERVER:$PORT" >> "$PATH_KUBERNETES/manifests/kube-lvscare.yaml"
        fi
    done

    # Continue writing the rest of the kube-lvscare.yaml file
    cat <<EOL >> "$PATH_KUBERNETES/manifests/kube-lvscare.yaml"
    command:
    - /usr/bin/lvscare
    image: $DOCKER_IMAGE_LVSCARE
    imagePullPolicy: Always
    name: kube-lvscare
    resources: {}
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /lib/modules
      name: lib-modules
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /lib/modules
    name: lib-modules
status: {}
EOL

    echo "exec(3/7): restart static pod"
    mv "${PATH_KUBERNETES}/manifests/kube-lvscare.yaml" "${PATH_KUBERNETES}/kube-lvscare.yaml"
    sleep 2
    mv "${PATH_KUBERNETES}/kube-lvscare.yaml" "${PATH_KUBERNETES}/manifests/kube-lvscare.yaml"

    echo "exec(4/7): wait lvscare ready"
    if wait_api_server_proxy_ready; then
        echo "lvscare is ready"
    else
        echo "lvscare is not ready"
        exit 1
    fi

    echo "exec(5/7): update kubelet.conf"
    cp "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}.bak"
    sed -i "s|server: .*|server: https://apiserver.virtual-cluster-system.svc:${LOCAL_PORT}|" "${PATH_KUBERNETES}/${KUBELET_KUBE_CONFIG_NAME}"
    
    echo "exec(6/7): update /etc/hosts"
    local_record="${LOCAL_IP} apiserver.virtual-cluster-system.svc"
    if grep -qFx "$local_record" /etc/hosts; then
        echo "Record $local_record already exists in /etc/hosts."
    else
        sed -i "1i $local_record" /etc/hosts
        echo "Record $local_record inserted into /etc/hosts."
    fi

    echo "exec(7/7): restart kubelet"
    systemctl restart kubelet
}

function install_lb() {
    if [ -z "$USE_PROXY" ]; then
      export USE_PROXY="LVSCARE"
    fi

    if [ "$USE_PROXY" = "NGINX" ]; then
        install_nginx_lb
    elif [ "$USE_PROXY" = "LVSCARE" ]; then
        install_lvscare_lb
    else
        exit 0
    fi
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