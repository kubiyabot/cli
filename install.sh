#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Convert architecture names
case ${ARCH} in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Unsupported architecture: ${ARCH}${NC}"
        exit 1
        ;;
esac

# Get the latest version from GitHub
echo -e "${BLUE}Fetching latest Kubiya CLI version...${NC}"
VERSION=$(curl -s https://api.github.com/repos/kubiyabot/cli/releases/latest | grep '"tag_name":' | cut -d'"' -f4)

# Set binary name based on OS
case ${OS} in
    darwin|linux)
        BINARY="kubiya-${OS}-${ARCH}"
        ;;
    windows)
        BINARY="kubiya-${OS}-${ARCH}.exe"
        ;;
    *)
        echo -e "${RED}Unsupported operating system: ${OS}${NC}"
        exit 1
        ;;
esac

# Create temporary directory
TMP_DIR=$(mktemp -d)
cd "${TMP_DIR}"

# Download binary
echo -e "${BLUE}Downloading Kubiya CLI ${VERSION} for ${OS}/${ARCH}...${NC}"
DOWNLOAD_URL="https://github.com/kubiyabot/cli/releases/download/${VERSION}/${BINARY}"
curl -LO "${DOWNLOAD_URL}"

# Make binary executable
chmod +x "${BINARY}"

# Install binary
echo -e "${BLUE}Installing Kubiya CLI...${NC}"
if [ "${OS}" = "windows" ]; then
    mkdir -p "$HOME/bin"
    mv "${BINARY}" "$HOME/bin/kubiya.exe"
    INSTALL_PATH="$HOME/bin/kubiya.exe"
else
    sudo mv "${BINARY}" "/usr/local/bin/kubiya"
    INSTALL_PATH="/usr/local/bin/kubiya"
fi

# Clean up
cd - > /dev/null
rm -rf "${TMP_DIR}"

# Verify installation
if command -v kubiya >/dev/null 2>&1; then
    echo -e "${GREEN}Kubiya CLI has been installed successfully to: ${INSTALL_PATH}${NC}"
    echo -e "${GREEN}Version: ${VERSION}${NC}"
    echo -e "\nTo get started, run: ${BLUE}kubiya --help${NC}"
else
    echo -e "${RED}Installation failed. Please try again or install manually.${NC}"
    exit 1
fi

# Configuration reminder
echo -e "\n${BLUE}Remember to set your API key:${NC}"
echo -e "export KUBIYA_API_KEY=\"your-api-key\"" 