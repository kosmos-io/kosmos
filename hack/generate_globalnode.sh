#!/bin/bash

if [ -z "$KUBECONFIG" ]; then
    echo "KUBECONFIG环境变量未设置."
    exit 1
fi

# Creating a directory for logs
mkdir -p kube_apply_logs

nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
for node in ${nodes}; do
    nodeIP=$(kubectl get node ${node} -o jsonpath='{.status.addresses[0].address}')
    labels=$(kubectl get node ${node} -o jsonpath='{.metadata.labels}')

    # Use jq to ensure all values are strings, but also explicitly add quotes in the YAML formatting step below
    labelsFormatted=$(echo "$labels" | jq -r 'to_entries | map(.value |= tostring) | .[] | "  \(.key): \"\(.value)\""')

    yamlContent="
apiVersion: kosmos.io/v1alpha1
kind: GlobalNode
metadata:
  name: ${node}
spec:
  state: \"reserved\"
  nodeIP: \"${nodeIP}\"
  labels:
$(echo "${labelsFormatted}" | awk '{print "    " $0}')
"

    # Log the YAML content to a file for inspection
    echo "$yamlContent" > kube_apply_logs/${node}.yaml

    # Apply the YAML
    echo "$yamlContent" | kubectl apply -f -


done
# clear resources
rm -rf kube_apply_logs