#!/usr/bin/env bash

echo "(1/2) Try to backup tmp dir"
mv /apps/conf/kosmos/tmp /apps/conf/kosmos/tmp.bk
if [ ! $? -eq 0 ]; then
  echo "backup tmp dir failed"
  exit
fi

echo "(2/2) Try to backup kubelet_node_helper"
mv /srv/node-agent/kubelet_node_helper.sh '/srv/node-agent/kubelet_node_helper.sh.'`date +%Y_%m_%d_%H_%M_%S`
if [ ! $? -eq 0 ]; then
  echo "backup kubelet_node_helper.sh failed"
  exit
fi

echo "backup successed"