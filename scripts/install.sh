#!/bin/bash

# Download the latest release
RELEASE_URL="https://github.com/kubiyabot/cli/releases/download/v0.0.03/cli_0.0.03_linux_amd64.deb"
DEB_FILE="kubiya-cli.deb"

echo "Downloading Kubiya CLI..."
curl -L -o $DEB_FILE $RELEASE_URL

# Install the package
echo "Installing Kubiya CLI..."
sudo dpkg -i $DEB_FILE || sudo apt-get install -f -y

# Clean up
rm $DEB_FILE

echo "Installation complete! You can now use 'kubiya-cli' command." 