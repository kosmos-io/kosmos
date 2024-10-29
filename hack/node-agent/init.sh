#!/usr/bin/env bash

WEB_USER="$WEB_USER" sed -i 's/^WEB_USER=.*/WEB_USER="'"$WEB_USER"'"/' /app/agent.env
WEB_PASS="$WEB_PASS" sed -i 's/^WEB_PASS=.*/WEB_PASS="'"$WEB_PASS"'"/' /app/agent.env
WEB_PORT="$WEB_PORT" sed -i 's/^WEB_PORT=.*/WEB_PORT="'"$WEB_PORT"'"/' /app/agent.env
sha256sum /app/node-agent > /app/node-agent.sum
sha256sum /host-path/node-agent >> /app/node-agent.sum
rsync -avz /app/ /host-path/
cp /app/node-agent.service /host-systemd/node-agent.service
