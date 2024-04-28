#!/usr/bin/env bash
sudo systemctl daemon-reload
sudo systemctl enable node-agent
sudo systemctl stop node-agent
sudo systemctl start node-agent