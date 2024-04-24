#!/bin/bash

if [ -z "$KUBECONFIG" ]; then
    echo "KUBECONFIG环境变量未设置."
    exit 1
fi

nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
for node in ${nodes}; do
    nodeIP=$(kubectl get node ${node} -o jsonpath='{.status.addresses[0].address}')
    labels=$(kubectl get node ${node} -o jsonpath='{.metadata.labels}')
    labelsFormatted=$(echo "$labels" | jq -r 'to_entries | .[] | "  \(.key): \(.value)"')
    echo "
apiVersion: kosmos.io/v1alpha1
kind: GlobalNode
metadata:
  name: ${node}
spec:
  nodeIP: \"${nodeIP}\"
  labels:
$(echo "${labelsFormatted}" | sed 's/=/": "/g' | awk '{print "    " $0}')
" | kubectl apply -f -

done
