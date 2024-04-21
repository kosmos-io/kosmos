sudo systemctl stop node-agent
sudo rm -rf /srv/node-agent
sudo rm /etc/systemd/system/node-agent.service
sudo rm ~/.config/pip/pip.conf
sudo systemctl daemon-reload