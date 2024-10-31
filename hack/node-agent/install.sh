#!/usr/bin/env bash

# check the input install package file
if [ $# -eq 0 ]; then
  echo "Usage: $0 node-agent.tar.gz"
  exit 1
fi

TAR_FILE="$1"
if [ ! -f "$TAR_FILE" ]; then
  echo "Error: $TAR_FILE does not exist."
  exit 1
fi
INSTALL_DIR=${WORK_DIR:-/srv/node-agent}

echo "Installing $TAR_FILE to $INSTALL_DIR..."

# check dir exists
if [ -d "$INSTALL_DIR" ]; then
  TIMESTAMP=$(date +"%Y%m%d%H%M%S")
  BACKUP_DIR="${INSTALL_DIR}_backup_${TIMESTAMP}"
  echo "Backing up existing directory to $BACKUP_DIR"
  sudo mv "$INSTALL_DIR" "$BACKUP_DIR"
fi
sudo mkdir -p "$INSTALL_DIR"

sudo tar --strip-components=1 -xzf "$TAR_FILE" -C "$INSTALL_DIR"

# Function to prompt for input if environment variables are not set
prompt_for_credentials() {
  if [ -z "$WEB_USER" ]; then
    read -p "Enter WEB_USER: " WEB_USER
  fi

  if [ -z "$WEB_PASS" ]; then
    read -sp "Enter WEB_PASS: " WEB_PASS
    echo
  fi

}

# Prompt for credentials if not already set
prompt_for_credentials

# Update credentials in the configuration file
sudo sed -i 's/^WEB_USER=.*/WEB_USER="'"$WEB_USER"'"/' "${INSTALL_DIR}"/agent.env
sudo sed -i 's/^WEB_PASS=.*/WEB_PASS="'"$WEB_PASS"'"/' "${INSTALL_DIR}"/agent.env
sudo sed -i 's/^WEB_PORT=.*/WEB_PORT="'"$WEB_PORT"'"/' "${INSTALL_DIR}"/agent.env

# Generate SHA256 checksums
sha256sum "${INSTALL_DIR}"/node-agent | sudo tee "${INSTALL_DIR}"/node-agent.sum

# Copy the service file to the systemd directory
sudo cp "${INSTALL_DIR}"/node-agent.service /etc/systemd/system/node-agent.service

# Reload systemd configuration and start the service
sudo systemctl daemon-reload
sudo systemctl enable node-agent
sudo systemctl start node-agent

echo "Installation completed successfully."
