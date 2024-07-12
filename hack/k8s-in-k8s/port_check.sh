#!/bin/bash

function check_port {
    local ip=$1
    local port=$2

    # Check if the IP address is IPv6, then enclose it in square brackets
    if [[ $ip =~ .*:.* ]]; then
         ip="[$ip]"
    fi

    if timeout 1 curl -s --connect-timeout 3 "${ip}:${port}" >/dev/null; then
        return 0
    else
        return 1
    fi
}

nodes=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name} {.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}')

node_array=()

while IFS= read -r line; do
    node_array+=("$line")
done <<< "$nodes"

for node in "${node_array[@]}"; do
    name=$(echo $node | awk '{print $1}')
    ip=$(echo $node | awk '{print $2}')
    
    if check_port $ip 5678; then
      echo ""
    else
        echo "节点: $name, IP: $ip 端口5678不可访问"
    fi
done
