#!/bin/bash

set -e

# Default to quiet mode
VERBOSE=false

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --verbose|-v) VERBOSE=true; shift ;;
        *) shift ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Log function
log() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')] INFO:${NC} $1"
    fi
}

log_success() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS:${NC} $1"
    fi
}

log_error() {
    # Always show errors regardless of verbose setting
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1"
}

log_warning() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
    fi
}

# Create log file
LOG_FILE="$HOME/kubiya_install.log"
touch "$LOG_FILE"
log "Installation started. Log file: $LOG_FILE"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Kubiya CLI installation started" >> "$LOG_FILE"
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
log "Detected OS: $OS, Architecture: $ARCH"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Detected OS: $OS, Architecture: $ARCH" >> "$LOG_FILE"
fi

# Convert architecture names
case ${ARCH} in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        log_error "Unsupported architecture: ${ARCH}"
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Unsupported architecture: ${ARCH}" >> "$LOG_FILE"
        exit 1
        ;;
esac
log "Normalized architecture: $ARCH"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Normalized architecture: $ARCH" >> "$LOG_FILE"
fi

# Get the latest version from GitHub
log "Fetching latest Kubiya CLI version..."
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Fetching latest version from GitHub" >> "$LOG_FILE"
fi
VERSION=$(curl -s https://api.github.com/repos/kubiyabot/cli/releases/latest | grep '"tag_name":' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
    log_error "Failed to fetch latest version"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Failed to fetch latest version" >> "$LOG_FILE"
    exit 1
fi
log "Latest version: $VERSION"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Latest version: $VERSION" >> "$LOG_FILE"
fi

# Set binary name based on OS
case ${OS} in
    darwin|linux)
        BINARY="kubiya-cli-${OS}-${ARCH}"
        ;;
    windows)
        BINARY="kubiya-cli-${OS}-${ARCH}.exe"
        ;;
    *)
        log_error "Unsupported operating system: ${OS}"
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Unsupported operating system: ${OS}" >> "$LOG_FILE"
        exit 1
        ;;
esac
log "Binary name: $BINARY"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Binary name: $BINARY" >> "$LOG_FILE"
fi

# Create temporary directory
TMP_DIR=$(mktemp -d)
log "Created temporary directory: $TMP_DIR"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Created temporary directory: $TMP_DIR" >> "$LOG_FILE"
fi
cd "${TMP_DIR}"

# Download binary
log "Downloading Kubiya CLI ${VERSION} for ${OS}/${ARCH}..."
DOWNLOAD_URL="https://github.com/kubiyabot/cli/releases/download/${VERSION}/${BINARY}"
log "Download URL: $DOWNLOAD_URL"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Download URL: $DOWNLOAD_URL" >> "$LOG_FILE"
fi

curl -LO "${DOWNLOAD_URL}" 2>> "$LOG_FILE"
if [ $? -ne 0 ]; then
    log_error "Failed to download binary"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Failed to download binary" >> "$LOG_FILE"
    exit 1
fi
log "Download completed"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Download completed" >> "$LOG_FILE"
fi

# Make binary executable
chmod +x "${BINARY}"
log "Made binary executable"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Made binary executable" >> "$LOG_FILE"
fi

# Install binary
log "Installing Kubiya CLI..."
if [ "${OS}" = "windows" ]; then
    mkdir -p "$HOME/bin"
    mv "${BINARY}" "$HOME/bin/kubiya.exe"
    INSTALL_PATH="$HOME/bin/kubiya.exe"
    log "Installed to $INSTALL_PATH"
    if [ "$VERBOSE" = true ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Installed to $INSTALL_PATH" >> "$LOG_FILE"
    fi
else
    sudo mv "${BINARY}" "/usr/local/bin/kubiya"
    INSTALL_PATH="/usr/local/bin/kubiya"
    log "Installed to $INSTALL_PATH"
    if [ "$VERBOSE" = true ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Installed to $INSTALL_PATH" >> "$LOG_FILE"
    fi
fi

# Clean up
cd - > /dev/null
rm -rf "${TMP_DIR}"
log "Cleaned up temporary directory"
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaned up temporary directory" >> "$LOG_FILE"
fi

# Verify installation
if command -v kubiya >/dev/null 2>&1; then
    INSTALLED_VERSION=$(kubiya version | head -n 1 | awk '{print $3}')
    log_success "Kubiya CLI has been installed successfully to: ${INSTALL_PATH}"
    log_success "Version: ${INSTALLED_VERSION}"
    if [ "$VERBOSE" = true ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS: Installation completed. Version: ${INSTALLED_VERSION}" >> "$LOG_FILE"
    fi
    echo -e "\nTo get started, run: ${BLUE}kubiya --help${NC}"
else
    log_error "Installation failed. Please try again or install manually."
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Installation verification failed" >> "$LOG_FILE"
    exit 1
fi

# Configuration reminder
log "Remember to set your API key:"
echo -e "export KUBIYA_API_KEY=\"your-api-key\"" 
if [ "$VERBOSE" = true ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Installation process completed" >> "$LOG_FILE"
fi 