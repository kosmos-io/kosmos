sudo systemctl stop node-agent
sudo rm -rf /srv/node-agent
sudo rm /etc/systemd/system/node-agent.service
sudo systemctl daemon-reload