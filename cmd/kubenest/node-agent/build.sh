#!/usr/bin/env bash
pip install pyinstaller
pyinstaller --onefile app.py
docker build -t cis-hub-huabei-3.cmecloud.cn/node-agent/node-agent:latest .
docker push cis-hub-huabei-3.cmecloud.cn/node-agent/node-agent:latest