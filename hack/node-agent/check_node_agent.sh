#!/bin/bash

while true; do
  service_status=$(systemctl status node-agent | grep "Active:" | awk '{print $2}')

  if [[ "$service_status" == "active" ]]; then
    status="running"
  else
    status="stopped"
  fi

  current_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  json_patch=$(cat <<EOF
{
  "status": {
    "conditions": [
      {
        "lastHeartbeatTime":"$current_time",
        "status":"$status",
        "type":"NodeAgentStatus"
      }
    ]
  }
}
EOF
)

  #echo "Applying patch: $json_patch"
  #--kubeconfig 
  kubectl patch globalnodes node52 --type=merge --subresource status --patch "$json_patch" 

  sleep 30
done
