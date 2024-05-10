#!/usr/bin/env bash
set -e
sed -i 's/^WEB_USER=.*/WEB_USER=$(WEB_USER)/' /app/agent.env
sed -i 's/^WEB_PASS=.*/WEB_PASS=$(WEB_PASS)/' /app/agent.env
sha256sum /app/node-agent > node-agent.sum
sha256sum /host-path/node-agent >> node-agent.sum
rsync -avz /app/ /host-path/
cp /app/node-agent.service /host-systemd/node-agent.service