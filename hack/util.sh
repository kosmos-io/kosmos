#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script holds common bash variables and utility functions.

KOSMOS_GO_PACKAGE="github.com/kosmos.io/kosmos"

REPO_ROOT="$(dirname "${BASH_SOURCE[0]}")/.."
HOST_CLUSTER_NAME="cluster-host"

MIN_Go_VERSION=go1.19.0

CLUSTERLINK_TARGET_SOURCE=(
  scheduler=cmd/clustertree/scheduler
  clusterlink-proxy=cmd/clusterlink/proxy
  clusterlink-operator=cmd/clusterlink/clusterlink-operator
  clusterlink-elector=cmd/clusterlink/elector
  clusterlink-agent=cmd/clusterlink/agent
  clusterlink-floater=cmd/clusterlink/floater
  clusterlink-network-manager=cmd/clusterlink/network-manager
  clusterlink-controller-manager=cmd/clusterlink/controller-manager
  clustertree-cluster-manager=cmd/clustertree/cluster-manager
  virtual-cluster-operator=cmd/kubenest/operator
  node-agent=cmd/kubenest/node-agent
  kosmosctl=cmd/kosmosctl
)

#https://textkool.com/en/ascii-art-generator?hl=default&vl=default&font=DOS%20Rebel&text=KOSMOS
KOSMOS_GREETING='
--------------------------------------------------------------------------------------
 █████   ████    ███████     █████████  ██████   ██████    ███████     █████████
░░███   ███░   ███░░░░░███  ███░░░░░███░░██████ ██████   ███░░░░░███  ███░░░░░███
 ░███  ███    ███     ░░███░███    ░░░  ░███░█████░███  ███     ░░███░███    ░░░
 ░███████    ░███      ░███░░█████████  ░███░░███ ░███ ░███      ░███░░█████████
 ░███░░███   ░███      ░███ ░░░░░░░░███ ░███ ░░░  ░███ ░███      ░███ ░░░░░░░░███
 ░███ ░░███  ░░███     ███  ███    ░███ ░███      ░███ ░░███     ███  ███    ░███
 █████ ░░████ ░░░███████░  ░░█████████  █████     █████ ░░░███████░  ░░█████████
░░░░░   ░░░░    ░░░░░░░     ░░░░░░░░░  ░░░░░     ░░░░░    ░░░░░░░     ░░░░░░░░░
---------------------------------------------------------------------------------------
'

function util::get_target_source() {
  local target=$1
  for s in "${CLUSTERLINK_TARGET_SOURCE[@]}"; do
    if [[ "$s" == ${target}=* ]]; then
      echo "${s##${target}=}"
      return
    fi
  done
}

# This function installs a Go tools by 'go install' command.
# Parameters:
#  - $1: package name, such as "sigs.k8s.io/controller-tools/cmd/controller-gen"
#  - $2: package version, such as "v0.4.1"
function util::install_tools() {
	local package="$1"
	local version="$2"
	echo "go install ${package}@${version}"
	GO111MODULE=on go install "${package}"@"${version}"
	GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
	export PATH=$PATH:$GOPATH/bin
}


function util::cmd_exist {
  local CMD=$(command -v ${1})
  if [[ ! -x ${CMD} ]]; then
    return 1
  fi
  return 0
}

# util::cmd_must_exist check whether command is installed.
function util::cmd_must_exist {
    local CMD=$(command -v ${1})
    if [[ ! -x ${CMD} ]]; then
      echo "Please install ${1} and verify they are in \$PATH."
      exit 1
    fi
}

function util::verify_go_version {
    local go_version
    IFS=" " read -ra go_version <<< "$(GOFLAGS='' go version)"
    if [[ "${MIN_Go_VERSION}" != $(echo -e "${MIN_Go_VERSION}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) && "${go_version[2]}" != "devel" ]]; then
      echo "Detected go version: ${go_version[*]}."
      echo "ClusterLink requires ${MIN_Go_VERSION} or greater."
      echo "Please install ${MIN_Go_VERSION} or later."
      exit 1
    fi
}

# util::cmd_must_exist_cfssl downloads cfssl/cfssljson if they do not already exist in PATH
function util::cmd_must_exist_cfssl {
    CFSSL_VERSION=${1}
    if command -v cfssl &>/dev/null && command -v cfssljson &>/dev/null; then
        CFSSL_BIN=$(command -v cfssl)
        CFSSLJSON_BIN=$(command -v cfssljson)
        return 0
    fi

    util::install_tools "github.com/cloudflare/cfssl/cmd/..." ${CFSSL_VERSION}

    GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
    CFSSL_BIN="${GOPATH}/bin/cfssl"
    CFSSLJSON_BIN="${GOPATH}/bin/cfssljson"
    if [[ ! -x ${CFSSL_BIN} || ! -x ${CFSSLJSON_BIN} ]]; then
      echo "Failed to download 'cfssl'. Please install cfssl and cfssljson and verify they are in \$PATH."
      echo "Hint: export PATH=\$PATH:\$GOPATH/bin; go install github.com/cloudflare/cfssl/cmd/..."
      exit 1
    fi
}

# util::install_environment_check will check OS and ARCH before installing
# ARCH support list: amd64,arm64
# OS support list: linux,darwin
function util::install_environment_check {
    local ARCH=${1:-}
    local OS=${2:-}
    if [[ "$ARCH" =~ ^(amd64|arm64)$ ]]; then
        if [[ "$OS" =~ ^(linux|darwin)$ ]]; then
            return 0
        fi
    fi
    echo "Sorry, ClusterLink installation does not support $ARCH/$OS at the moment"
    exit 1
}

# util::install_kubectl will install the given version kubectl
function util::install_kubectl {
    local KUBECTL_VERSION=${1}
    local ARCH=${2}
    local OS=${3:-linux}
    if [ -z "$KUBECTL_VERSION" ]; then
      KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
    fi
    echo "Installing 'kubectl ${KUBECTL_VERSION}' for you"
    curl --retry 5 -sSLo ./kubectl -w "%{http_code}" https://dl.k8s.io/release/"$KUBECTL_VERSION"/bin/"$OS"/"$ARCH"/kubectl | grep '200' > /dev/null
    ret=$?
    if [ ${ret} -eq 0 ]; then
        chmod +x ./kubectl
        mkdir -p ~/.local/bin/
        mv ./kubectl ~/.local/bin/kubectl

        export PATH=$PATH:~/.local/bin
    else
        echo "Failed to install kubectl, can not download the binary file at https://dl.k8s.io/release/$KUBECTL_VERSION/bin/$OS/$ARCH/kubectl"
        exit 1
    fi
}

# util::install_helm will install the helm command
function util::install_helm {
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
}

# util::create_signing_certkey creates a CA, args are sudo, dest-dir, ca-id, purpose
function util::create_signing_certkey {
    local sudo=$1
    local dest_dir=$2
    local id=$3
    local cn=$4
    local purpose=$5
    OPENSSL_BIN=$(command -v openssl)
    # Create ca
    ${sudo} /usr/bin/env bash -e <<EOF
    rm -f "${dest_dir}/${id}.crt" "${dest_dir}/${id}.key"
    ${OPENSSL_BIN} req -x509 -sha256 -new -nodes -days 3650 -newkey rsa:2048 -keyout "${dest_dir}/${id}.key" -out "${dest_dir}/${id}.crt" -subj "/CN=${cn}/"
    echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment",${purpose}]}}}' > "${dest_dir}/${id}-config.json"
EOF
}

# util::create_certkey signs a certificate: args are sudo, dest-dir, ca, filename (roughly), subject, hosts...
function util::create_certkey {
    local sudo=$1
    local dest_dir=$2
    local ca=$3
    local id=$4
    local cn=${5:-$4}
    local og=$6
    local hosts=""
    local SEP=""
    shift 6
    while [[ -n "${1:-}" ]]; do
        hosts+="${SEP}\"$1\""
        SEP=","
        shift 1
    done
    ${sudo} /usr/bin/env bash -e <<EOF
    cd ${dest_dir}
    echo '{"CN":"${cn}","hosts":[${hosts}],"names":[{"O":"${og}"}],"key":{"algo":"rsa","size":2048}}' | ${CFSSL_BIN} gencert -ca=${ca}.crt -ca-key=${ca}.key -config=${ca}-config.json - | ${CFSSLJSON_BIN} -bare ${id}
    mv "${id}-key.pem" "${id}.key"
    mv "${id}.pem" "${id}.crt"
    rm -f "${id}.csr"
EOF
}

# util::append_client_kubeconfig creates a new context including a cluster and a user to the existed kubeconfig file
function util::append_client_kubeconfig {
    local kubeconfig_path=$1
    local client_certificate_file=$2
    local client_key_file=$3
    local api_host=$4
    local api_port=$5
    local client_id=$6
    local token=${7:-}
    kubectl config set-cluster "${client_id}" --server=https://"${api_host}:${api_port}" --insecure-skip-tls-verify=true --kubeconfig="${kubeconfig_path}"
    kubectl config set-credentials "${client_id}" --token="${token}" --client-certificate="${client_certificate_file}" --client-key="${client_key_file}" --embed-certs=true --kubeconfig="${kubeconfig_path}"
    kubectl config set-context "${client_id}" --cluster="${client_id}" --user="${client_id}" --kubeconfig="${kubeconfig_path}"
}


# util::wait_for_condition blocks until the provided condition becomes true
# Arguments:
#  - 1: message indicating what conditions is being waited for (e.g. 'ok')
#  - 2: a string representing an eval'able condition.  When eval'd it should not output
#       anything to stdout or stderr.
#  - 3: optional timeout in seconds. If not provided, waits forever.
# Returns:
#  1 if the condition is not met before the timeout
function util::wait_for_condition() {
  local msg=$1
  # condition should be a string that can be eval'd.
  local condition=$2
  local timeout=${3:-}

  local start_msg="Waiting for ${msg}"
  local error_msg="[ERROR] Timeout waiting for condition ${msg}"

  local counter=0
  while ! eval ${condition}; do
    if [[ "${counter}" = "0" ]]; then
      echo -n "${start_msg}"
    fi

    if [[ -z "${timeout}" || "${counter}" -lt "${timeout}" ]]; then
      counter=$((counter + 1))
      if [[ -n "${timeout}" ]]; then
        echo -n '.'
      fi
      sleep 1
    else
      echo -e "\n${error_msg}"
      return 1
    fi
  done

  if [[ "${counter}" != "0" && -n "${timeout}" ]]; then
    echo ' done'
  fi
}

# util::wait_file_exist checks if a file exists, if not, wait until timeout
function util::wait_file_exist() {
    local file_path=${1}
    local timeout=${2}
    local error_msg="[ERROR] Timeout waiting for file exist ${file_path}"
    for ((time=0; time<${timeout}; time++)); do
        if [[ -e ${file_path} ]]; then
            return 0
        fi
        sleep 1
    done
    echo -e "\n${error_msg}"
    return 1
}

# This function returns the IP address of a docker instance
# Parameters:
#  - $1: docker instance name

function util::get_docker_native_ipaddress(){
  local container_name=$1
  docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${container_name}"
}

# This function returns the IP address and port of a specific docker instance's host IP
# Parameters:
#  - $1: docker instance name
# Note:
#   Use for getting host IP and port for cluster
#   "6443/tcp" assumes that API server port is 6443 and protocol is TCP

function util::get_docker_host_ip_port(){
  local container_name=$1
  docker inspect --format='{{range $key, $value := index .NetworkSettings.Ports "6443/tcp"}}{{if eq $key 0}}{{$value.HostIp}}:{{$value.HostPort}}{{end}}{{end}}' "${container_name}"
}

# util::check_clusters_ready checks if a cluster is ready, if not, wait until timeout
function util::check_clusters_ready() {
  local kubeconfig_path=${1}
  local context_name=${2}

  echo "Waiting for kubeconfig file ${kubeconfig_path} and clusters ${context_name} to be ready..."
  util::wait_file_exist "${kubeconfig_path}" 300
  util::wait_for_condition 'running' "docker inspect --format='{{.State.Status}}' ${context_name}-control-plane &> /dev/null" 300

  kubectl config rename-context "kind-${context_name}" "${context_name}" --kubeconfig="${kubeconfig_path}"

  local os_name
  os_name=$(go env GOOS)
  local container_ip_port
  case $os_name in
    linux) container_ip_port=$(util::get_docker_native_ipaddress "${context_name}-control-plane")":6443"
    ;;
    darwin) container_ip_port=$(util::get_docker_host_ip_port "${context_name}-control-plane")
    ;;
    *)
        echo "OS ${os_name} does NOT support for getting container ip in installation script"
        exit 1
  esac
  kubectl config set-cluster "kind-${context_name}" --server="https://${container_ip_port}" --kubeconfig="${kubeconfig_path}"

  util::wait_for_condition 'ok' "kubectl --kubeconfig ${kubeconfig_path} --context ${context_name} get --raw=/healthz &> /dev/null" 300
}

# This function gets api server's ip from kubeconfig by context name
function util::get_apiserver_ip_from_kubeconfig(){
  local context_name=$1
  local cluster_name apiserver_url
  cluster_name=$(kubectl config view --template='{{ range $_, $value := .contexts }}{{if eq $value.name '"\"${context_name}\""'}}{{$value.context.cluster}}{{end}}{{end}}')
  apiserver_url=$(kubectl config view --template='{{range $_, $value := .clusters }}{{if eq $value.name '"\"${cluster_name}\""'}}{{$value.cluster.server}}{{end}}{{end}}')
  echo "${apiserver_url}" | awk -F/ '{print $3}' | sed 's/:.*//'
}

# This function deploys webhook configuration
# Parameters:
#  - $1: k8s context name
#  - $2: CA file
#  - $3: configuration file
# Note:
#   Deprecated: should be removed after helm get on board.
function util::deploy_webhook_configuration() {
  local context_name=$1
  local ca_file=$2
  local conf=$3

  local ca_string=$(cat ${ca_file} | base64 | tr "\n" " "|sed s/[[:space:]]//g)

  local temp_path=$(mktemp -d)
  cp -rf "${conf}" "${temp_path}/temp.yaml"
  sed -i'' -e "s/{{caBundle}}/${ca_string}/g" "${temp_path}/temp.yaml"
  kubectl --context="$context_name" apply -f "${temp_path}/temp.yaml"
  rm -rf "${temp_path}"
}

function util::fill_cabundle() {
  local ca_file=$1
  local conf=$2

  local ca_string=$(cat "${ca_file}" | base64 | tr "\n" " "|sed s/[[:space:]]//g)
  sed -i'' -e "s/{{caBundle}}/${ca_string}/g" "${conf}"
}

# util::wait_service_external_ip give a service external ip when it is ready, if not, wait until timeout
# Parameters:
#  - $1: context name in k8s
#  - $2: service name in k8s
#  - $3: namespace
SERVICE_EXTERNAL_IP=''
function util::wait_service_external_ip() {
  local context_name=$1
  local service_name=$2
  local namespace=$3
  local external_ip
  local tmp
  for tmp in {1..30}; do
    set +e
    ## if .status.loadBalancer does not have `ingress` field, return "".
    ## if .status.loadBalancer has `ingress` field but one of `ingress` field does not have `hostname` or `ip` field, return "<no value>".
    external_host=$(kubectl --context="$context_name" get service "${service_name}" -n "${namespace}" --template="{{range .status.loadBalancer.ingress}}{{.hostname}} {{end}}" | xargs)
    external_ip=$(kubectl --context="$context_name" get service "${service_name}" -n "${namespace}" --template="{{range .status.loadBalancer.ingress}}{{.ip}} {{end}}" | xargs)
    set -e
    if [[ ! -z "$external_host" && "$external_host" != "<no value>" ]]; then # Compatibility with hostname, such as AWS
      external_ip=$external_host
    fi
    if [[ -z "$external_ip" || "$external_ip" = "<no value>" ]]; then
      echo "wait the external ip of ${service_name} ready..."
      sleep 6
      continue
    else
      SERVICE_EXTERNAL_IP="${external_ip}"
      return 0
    fi
  done
  return 1
}

# util::get_load_balancer_ip get a valid load balancer ip from k8s service's 'loadBalancer' , if not, wait until timeout
# call 'util::wait_service_external_ip' before using this function
function util::get_load_balancer_ip() {
  local tmp
  local first_ip
  if [[ -n "${SERVICE_EXTERNAL_IP}" ]]; then
    first_ip=$(echo "${SERVICE_EXTERNAL_IP}" | awk '{print $1}') #temporarily choose the first one
    for tmp in {1..10}; do
      #if it is a host, check dns first
      if [[ ! "${first_ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        if ! nslookup ${first_ip} > /dev/null; then # host dns lookup failed
          sleep 30
          continue
        fi
      fi
      set +e
      connect_test=$(curl -s -k -m 5 https://"${first_ip}":5443/readyz)
      set -e
      if [[ "${connect_test}" = "ok" ]]; then
        echo "${first_ip}"
        return 0
      else
        sleep 3
        continue
      fi
    done
  fi
  return 1
}

# util::add_routes will add routes for given kind cluster
# Parameters:
#  - $1: name of the kind cluster want to add routes
#  - $2: the kubeconfig path of the cluster wanted to be connected
#  - $3: the context in kubeconfig of the cluster wanted to be connected
function util::add_routes() {
  unset IFS
  routes=$(kubectl --kubeconfig ${2} --context ${3} get nodes -o jsonpath='{range .items[*]}ip route add {.spec.podCIDR} via {.status.addresses[?(.type=="InternalIP")].address}{"\n"}{end}')
  echo "Connecting cluster ${1} to ${2}"

  IFS=$'\n'
  for n in $(kind get nodes --name "${1}"); do
    for r in $routes; do
      echo "exec cmd in docker $n $r"
      eval "docker exec $n $r"
    done
  done
  unset IFS
}

function util::get_version() {
  git describe --tags --dirty --always
}

function util::version_ldflags() {
  # set GIT_VERSION from param
  GIT_VERSION=${1:-}
  # If GIT_VERSION is not provided, use util::get_version
  if [ -z "$GIT_VERSION" ]; then
    GIT_VERSION=$(util::get_version)
  fi
  #GIT_VERSION=$(util::get_version)
  GIT_COMMIT_HASH=$(git rev-parse HEAD)
  if git_status=$(git status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
    GIT_TREESTATE="clean"
  else
    GIT_TREESTATE="dirty"
  fi
  BUILDDATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  LDFLAGS="-X github.com/kosmos.io/kosmos/pkg/version.gitVersion=${GIT_VERSION} \
                        -X github.com/kosmos.io/kosmos/pkg/version.gitCommit=${GIT_COMMIT_HASH} \
                        -X github.com/kosmos.io/kosmos/pkg/version.gitTreeState=${GIT_TREESTATE} \
                        -X github.com/kosmos.io/kosmos/pkg/version.buildDate=${BUILDDATE}"
  echo $LDFLAGS
}

# util::create_gopath_tree create the GOPATH tree
# Parameters:
#  - $1: the root path of repo
#  - $2: go path
function util:create_gopath_tree() {
  local repo_root=$1
  local go_path=$2

  local go_pkg_dir="${go_path}/src/${KOSMOS_GO_PACKAGE}"
  go_pkg_dir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_dir}"

  if [[ ! -e "${go_pkg_dir}" || "$(readlink "${go_pkg_dir}")" != "${repo_root}" ]]; then
    ln -snf "${repo_root}" "${go_pkg_dir}"
  fi
}

function util:host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

function wait_deployment_ready() {
  local deployment_name=$1
  local namespace=$2
  local timeout=${3:-300}

  local replica_count=$(kubectl get deployment "${deployment_name}" -n "${namespace}" -o jsonpath='{.spec.replicas}')

  local pod_ready_condition="kubectl get pods -l app=${deployment_name} -n ${namespace} -o 'jsonpath={..status.conditions[?(@.type==\"Ready\")].status}' | tr ' ' '\\n' | sort | uniq -c | grep -c True"

  util::wait_for_condition "Pods are ready" \
    "kubectl get deployment ${deployment_name} -n ${namespace} -o jsonpath='{.status.readyReplicas}' | grep -q '^[1-9]\d*$' && [[ \$(${pod_ready_condition}) -eq ${replica_count} ]]" \
    "${timeout}"
}


function util::wait_deployment_ready() {
  local deployment_name=$1
  local namespace=$2
  kubectl rollout status deployment/"$deployment_name" -n "$namespace" --timeout=30s
}


function util::wait_for_crd() {
  local crd_names=("$@")
  local timeout=500
  local count=0

  local end=$((SECONDS+timeout))
  while [ $SECONDS -lt $end ]; do
    for crd_name in "${crd_names[@]}"; do
      if kubectl get crd "$crd_name"; then
        echo "CRD $crd_name has been stored successfully."
        # delete crd from waiting list
        count=$(($count+1))
      fi
    done

    if [ $count -eq ${#crd_names[@]} ]; then
      echo "All CRDs have been stored successfully."
      return 0
    fi

    sleep 5
  done

  echo "The following CRDs were not stored within the specified timeout of ${timeout}s: ${crd_names[*]}"
  return 1
}

function util::go_clean_cache() {
    set -x

    # clean go cache avoid macos make error
    # vendor/github.com/prometheus/client_golang/prometheus/expvar_collector.go:18:2: open /usr/local/go/src/expvar: too many open files in system
    go clean -cache

    set +x
}

# get base64 from kubeconfig file
function util::get_base64_kubeconfig() {
    local os_type=$(uname)

    if [ "$os_type" == "Linux" ]; then
        # Linux
        base64 -w 0 < "$1"
    elif [ "$os_type" == "Darwin" ]; then
        # macOS
        base64 -b 0 < "$1"
    else
        echo "Unsupported operating system"
        return 1
    fi
}

# verify input ip is valid or not
function util::verify_ip_address {
  IPADDRESS=${1}
  if [[ ! "${IPADDRESS}" =~ ^(([1-9]?[0-9]|1[0-9][0-9]|2([0-4][0-9]|5[0-5]))\.){3}([1-9]?[0-9]|1[0-9][0-9]|2([0-4][0-9]|5[0-5]))$ ]]; then
    echo -e "\nError: invalid IP address"
    exit 1
  fi
}

# util::get_macos_ipaddress will get ip address on macos interactively, store to 'MAC_NIC_IPADDRESS' if available
MAC_NIC_IPADDRESS=''
function util::get_macos_ipaddress() {
  if [[ $(go env GOOS) = "darwin" ]]; then
    tmp_ip=$(ipconfig getifaddr en0 || true)
    echo ""
    echo " Detected that you are installing KOSMOS on macOS "
    echo ""
    echo "It needs a Macintosh IP address to bind Kind Api Server Address,"
    echo "so you can access it from you macOS and the inner kubeconfig for cluster should use --inner-kubeconfig"
    echo "the --inner-kubeconfig should use nodeIp so the host-cluster and member-cluster can be connected"
    echo -n "input an available IP, "
    if [[ -z ${tmp_ip} ]]; then
      echo "you can use the command 'ifconfig' to look for one"
      tips_msg="[Enter IP address]:"
    else
      echo "default IP will be en0 inet addr if exists"
      tips_msg="[Enter for default ${tmp_ip}]:"
    fi
    read -r -p "${tips_msg}" MAC_NIC_IPADDRESS
    MAC_NIC_IPADDRESS=${MAC_NIC_IPADDRESS:-$tmp_ip}
    util::verify_ip_address "${MAC_NIC_IPADDRESS}"
    echo "Using IP address: ${MAC_NIC_IPADDRESS}"
  else # non-macOS
    MAC_NIC_IPADDRESS=${MAC_NIC_IPADDRESS:-}
  fi
}