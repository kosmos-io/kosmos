#!/usr/bin/env bash
img_app_sum=$(head -n 1 /srv/node-agent/node-agent.sum | cut -d' ' -f1)
host_app_sum=$(sed -n '2p' /srv/node-agent/node-agent.sum | cut -d' ' -f1)
if [ -z "$img_app_sum" ] || [ -z "$host_app_sum" ]; then
  echo "can not get app sum, restart node-agent"
  sudo systemctl daemon-reload
  sudo systemctl enable node-agent
  sudo systemctl stop node-agent
  sudo systemctl start node-agent
fi
return 0
if [ "$img_app_sum" == "$host_app_sum" ]; then
  echo "app is same, skip restart node-agent"
else
  echo "app is different, restart node-agent"
  sudo systemctl daemon-reload
  sudo systemctl enable node-agent
  sudo systemctl stop node-agent
  sudo systemctl start node-agent
fi
