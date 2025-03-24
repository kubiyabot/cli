#!/bin/bash

# Add GitHub Pages repository as apt source
echo "Adding Kubiya repository..."
echo "deb [trusted=yes] https://kubiyabot.github.io/cli/apt-repo stable main" | sudo tee /etc/apt/sources.list.d/kubiya.list

# Update package list
echo "Updating package list..."
sudo apt-get update

# Install the package
echo "Installing Kubiya CLI..."
sudo apt-get install -y kubiya-cli

echo "Installation complete! You can now use 'kubiya-cli' command." 