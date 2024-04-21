#!/usr/bin/env bash
# check the param
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <argument>"
    echo "Example: bash install.sh node-agent"
    exit 1
fi
# config pypip packages
mkdir -p ~/.config/pip
cat << EOF > ~/.config/pip/pip.conf
[global]
index-url = https://mirror.sjtu.edu.cn/pypi/web/simple
format = columns
EOF
# upgrade pip for python3.6
sudo pip3 install --upgrade pip
sudo cp "$1"/node-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo mkdir -p /srv/node-agent
sudo cp -r "$1"/* /srv/node-agent
sudo systemctl restart node-agent
sudo systemctl enable node-agent