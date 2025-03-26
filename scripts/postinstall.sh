#!/bin/sh

# Ensure the binary is executable
chmod +x /usr/local/bin/kubiya-cli

# Create completion directory if it doesn't exist
mkdir -p /etc/bash_completion.d

# Generate and install bash completion
KUBIYA_API_KEY=dummy kubiya-cli completion bash > /etc/bash_completion.d/kubiya-cli

# Create zsh completion directory if it doesn't exist
mkdir -p /usr/local/share/zsh/site-functions

# Generate and install zsh completion
KUBIYA_API_KEY=dummy kubiya-cli completion zsh > /usr/local/share/zsh/site-functions/_kubiya-cli 