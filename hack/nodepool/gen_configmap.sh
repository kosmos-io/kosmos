#!/bin/bash

# please check convert.sh is in the same directory
source ./convert.sh

# Run kubectl command to get nodes info
# Note the name of the node master role, in this case control-plane, if the master role is not this name, it needs to be changed here
nodes_info=$(kubectl get nodes -o wide --selector='!node-role.kubernetes.io/control-plane')

# Generate the ConfigMap YAML
configmap_yaml="apiVersion: v1
kind: ConfigMap
metadata:
  name: node-pool
  namespace: kosmos-system
data:
  nodes: |-
    {
"

# Extract node information dynamically
while IFS= read -r line; do
    if [[ $line == NAME* ]]; then
        continue
    fi

    node_name=$(echo $line | awk "{print \$1}")
    node_ip=$(echo $line | awk "{print \$6}")

    # Extract node labels
    node_labels=$(kubectl describe node $node_name | awk '/Labels:/{flag=1;print;next}/Annotations:/{flag=0}flag')
    #cluster_name=$(echo "$node_labels" | awk -F 'cluster=' '{print \$2; exit}' | cut -d',' -f1 | tr -d "',")
    # Format labels as JSON
    formatted_labels=$(convert_to_json "$node_labels")

    configmap_yaml+="
    \"$node_name\":
      {
        \"address\": \"$node_ip\",
        \"labels\": $formatted_labels,
        \"cluster\": \"$cluster_name\",
        \"state\": \"free\"
      },"
done <<< "$nodes_info"

configmap_yaml="${configmap_yaml%?}"  # Remove the trailing comma

configmap_yaml+="
    }
"

# Apply the ConfigMap to the cluster
echo "$configmap_yaml" > node-pool.yaml
kubectl apply -f node-pool.yaml -n kosmos-system