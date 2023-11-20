#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Function to install Go if it is not already installed
install_go() {
  echo "Go is not installed. Installing..."

  # Specify the Go version you want to install
  GO_VERSION="1.20"  # Change this to the desired Go version

  # Set the Go installation path
  GO_INSTALL_PATH="/usr/local"

  # Download and install Go
  curl -O https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz
  tar -C $GO_INSTALL_PATH -xzf go$GO_VERSION.linux-amd64.tar.gz

  # Set Go environment variables
  export GOROOT=$GO_INSTALL_PATH/go
  export GOPATH=$HOME/go
  export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

  # Cleanup downloaded files
  rm go$GO_VERSION.linux-amd64.tar.gz

  echo "Go installation complete."
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
  install_go
fi

# Verify the Go version
if ! go version | grep -q "go1.20"; then
  echo "Installed Go version does not match the required version (1.20)."
  install_go
fi

echo "Go is installed and the version is correct."
