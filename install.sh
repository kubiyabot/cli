#!/bin/bash

set -e

# Kubiya CLI Installation Script
# Supports: macOS (Homebrew), Linux (APT, YUM, Binary), Windows (Binary)
# Features: CLI installation, worker setup, configuration

# Default options
VERBOSE=false
INSTALL_WORKER=false
SETUP_CONFIG=false
FORCE_BINARY=false
WORKER_QUEUE_ID=""
WORKER_MODE="local"
START_WORKER=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Print banner
print_banner() {
    echo -e "${CYAN}${BOLD}"
    cat << "EOF"
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   ██╗  ██╗██╗   ██╗██████╗ ██╗██╗   ██╗ █████╗          ║
║   ██║ ██╔╝██║   ██║██╔══██╗██║╚██╗ ██╔╝██╔══██╗         ║
║   █████╔╝ ██║   ██║██████╔╝██║ ╚████╔╝ ███████║         ║
║   ██╔═██╗ ██║   ██║██╔══██╗██║  ╚██╔╝  ██╔══██║         ║
║   ██║  ██╗╚██████╔╝██████╔╝██║   ██║   ██║  ██║         ║
║   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝   ╚═╝   ╚═╝  ╚═╝         ║
║                                                           ║
║         AI-Powered Automation Platform Installer         ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
  -v, --verbose              Enable verbose output
  -w, --worker               Install and setup worker
  -q, --queue-id <id>        Worker queue ID (required with --worker)
  -m, --mode <mode>          Worker mode: local, docker, daemon (default: local)
  -s, --start                Start worker after installation
  -c, --config               Interactive configuration setup
  -f, --force-binary         Force binary installation (skip package managers)
  -h, --help                 Show this help message

Examples:
  # Install CLI only
  $0

  # Install CLI with verbose output
  $0 --verbose

  # Install CLI and setup worker
  $0 --worker --queue-id=my-queue

  # Install CLI, setup worker, and start it
  $0 --worker --queue-id=prod-queue --mode=daemon --start

  # Interactive setup with configuration
  $0 --config --worker
EOF
    exit 0
}

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        -w|--worker) INSTALL_WORKER=true; shift ;;
        -q|--queue-id) WORKER_QUEUE_ID="$2"; shift 2 ;;
        -m|--mode) WORKER_MODE="$2"; shift 2 ;;
        -s|--start) START_WORKER=true; shift ;;
        -c|--config) SETUP_CONFIG=true; shift ;;
        -f|--force-binary) FORCE_BINARY=true; shift ;;
        -h|--help) show_usage ;;
        *) echo "Unknown option: $1"; show_usage ;;
    esac
done

# Log functions
log() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗ ERROR:${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}⚠ WARNING:${NC} $1"
}

log_step() {
    echo -e "\n${CYAN}${BOLD}▶ $1${NC}"
}

# Create log file
LOG_FILE="$HOME/.kubiya/install.log"
mkdir -p "$HOME/.kubiya"
touch "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Kubiya CLI installation started" >> "$LOG_FILE"

# Detect OS and architecture
detect_os() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    log "Detected OS: $OS, Architecture: $ARCH"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Detected OS: $OS, Architecture: $ARCH" >> "$LOG_FILE"

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
}

# Check if Homebrew is installed
has_homebrew() {
    command -v brew >/dev/null 2>&1
}

# Check if APT is available (Debian/Ubuntu)
has_apt() {
    command -v apt-get >/dev/null 2>&1
}

# Check if YUM is available (RHEL/CentOS)
has_yum() {
    command -v yum >/dev/null 2>&1
}

# Check if DNF is available (Fedora)
has_dnf() {
    command -v dnf >/dev/null 2>&1
}

# Install via Homebrew (macOS)
install_via_homebrew() {
    log_step "Installing Kubiya CLI via Homebrew"

    if ! has_homebrew; then
        log_error "Homebrew not found. Please install Homebrew first: https://brew.sh"
        return 1
    fi

    log "Updating Homebrew..."
    brew update >> "$LOG_FILE" 2>&1 || true

    log "Tapping kubiyabot/kubiya..."
    brew tap kubiyabot/kubiya >> "$LOG_FILE" 2>&1

    log "Installing kubiya..."
    brew install kubiya >> "$LOG_FILE" 2>&1

    log_success "Installed via Homebrew"
    return 0
}

# Install via APT (Debian/Ubuntu)
install_via_apt() {
    log_step "Installing Kubiya CLI via APT"

    if ! has_apt; then
        return 1
    fi

    log_warning "APT repository installation is not yet available"
    log "Falling back to binary installation..."
    return 1
}

# Install via binary download
install_via_binary() {
    log_step "Installing Kubiya CLI via binary download"

    # Get the latest version from GitHub
    log "Fetching latest version..."
    VERSION=$(curl -s https://api.github.com/repos/kubiyabot/cli/releases/latest | grep '"tag_name":' | cut -d'"' -f4)

    if [ -z "$VERSION" ]; then
        log_error "Failed to fetch latest version"
        return 1
    fi

    log "Latest version: $VERSION"

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
            return 1
            ;;
    esac

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    log "Created temporary directory: $TMP_DIR"
    cd "${TMP_DIR}"

    # Download binary
    DOWNLOAD_URL="https://github.com/kubiyabot/cli/releases/download/${VERSION}/${BINARY}"
    log "Downloading from: $DOWNLOAD_URL"

    curl -fsSL "${DOWNLOAD_URL}" -o "${BINARY}" 2>> "$LOG_FILE"

    if [ $? -ne 0 ]; then
        log_error "Failed to download binary"
        cd - > /dev/null
        rm -rf "${TMP_DIR}"
        return 1
    fi

    # Make binary executable
    chmod +x "${BINARY}"

    # Install binary
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

    log_success "Installed to $INSTALL_PATH"
    return 0
}

# Main installation function
install_cli() {
    log_step "Installing Kubiya CLI"

    detect_os

    # Check if already installed
    if command -v kubiya >/dev/null 2>&1 && [ "$FORCE_BINARY" = false ]; then
        CURRENT_VERSION=$(kubiya version 2>/dev/null | head -n 1 | awk '{print $3}' || echo "unknown")
        log_warning "Kubiya CLI is already installed (version: $CURRENT_VERSION)"
        echo -n "Do you want to reinstall? (y/N): "
        read -r REPLY
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Skipping installation"
            return 0
        fi
    fi

    # Try package managers first (unless forced binary)
    if [ "$FORCE_BINARY" = false ]; then
        case ${OS} in
            darwin)
                if install_via_homebrew; then
                    return 0
                fi
                log_warning "Homebrew installation failed, falling back to binary"
                ;;
            linux)
                if has_apt && install_via_apt; then
                    return 0
                elif has_yum || has_dnf; then
                    log_warning "YUM/DNF installation not yet supported, using binary"
                fi
                ;;
        esac
    fi

    # Fall back to binary installation
    install_via_binary
}

# Setup configuration
setup_configuration() {
    log_step "Setting up configuration"

    echo ""
    echo "Let's configure your Kubiya CLI:"
    echo ""

    # API Key
    if [ -z "$KUBIYA_API_KEY" ]; then
        echo -n "Enter your Kubiya API Key: "
        read -r KUBIYA_API_KEY

        if [ -z "$KUBIYA_API_KEY" ]; then
            log_warning "No API key provided. You'll need to set it later."
        else
            # Add to shell profile
            SHELL_PROFILE=""
            if [ -f "$HOME/.zshrc" ]; then
                SHELL_PROFILE="$HOME/.zshrc"
            elif [ -f "$HOME/.bashrc" ]; then
                SHELL_PROFILE="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                SHELL_PROFILE="$HOME/.bash_profile"
            fi

            if [ -n "$SHELL_PROFILE" ]; then
                echo "" >> "$SHELL_PROFILE"
                echo "# Kubiya CLI Configuration" >> "$SHELL_PROFILE"
                echo "export KUBIYA_API_KEY=\"$KUBIYA_API_KEY\"" >> "$SHELL_PROFILE"
                log_success "API key added to $SHELL_PROFILE"
            fi

            export KUBIYA_API_KEY
        fi
    else
        log_success "Using existing API key from environment"
    fi

    # Control Plane URL (optional)
    echo ""
    echo -n "Custom Control Plane URL (leave empty for default): "
    read -r CONTROL_PLANE_URL

    if [ -n "$CONTROL_PLANE_URL" ] && [ -n "$SHELL_PROFILE" ]; then
        echo "export CONTROL_PLANE_GATEWAY_URL=\"$CONTROL_PLANE_URL\"" >> "$SHELL_PROFILE"
        log_success "Control Plane URL configured"
    fi

    echo ""
    log_success "Configuration completed"
}

# Check Python for worker
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        log_error "Python 3 is required for worker mode but not found"
        echo "Please install Python 3.8 or later: https://www.python.org/downloads/"
        return 1
    fi

    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    log "Found Python $PYTHON_VERSION"

    # Check version (need 3.8+)
    PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
    PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)

    if [ "$PYTHON_MAJOR" -lt 3 ] || ([ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 8 ]); then
        log_error "Python 3.8 or later is required (found $PYTHON_VERSION)"
        return 1
    fi

    return 0
}

# Setup worker
setup_worker() {
    log_step "Setting up Kubiya Worker"

    # Check if worker installation was requested
    if [ "$INSTALL_WORKER" = false ]; then
        return 0
    fi

    # Validate queue ID
    if [ -z "$WORKER_QUEUE_ID" ]; then
        echo ""
        echo -n "Enter Worker Queue ID: "
        read -r WORKER_QUEUE_ID

        if [ -z "$WORKER_QUEUE_ID" ]; then
            log_error "Worker queue ID is required"
            return 1
        fi
    fi

    # Check Python for local mode
    if [ "$WORKER_MODE" = "local" ] || [ "$WORKER_MODE" = "daemon" ]; then
        if ! check_python; then
            return 1
        fi
    fi

    # Check Docker for docker mode
    if [ "$WORKER_MODE" = "docker" ]; then
        if ! command -v docker >/dev/null 2>&1; then
            log_error "Docker is required for docker mode but not found"
            echo "Please install Docker: https://docs.docker.com/get-docker/"
            return 1
        fi

        if ! docker ps >/dev/null 2>&1; then
            log_error "Docker daemon is not running"
            return 1
        fi

        log "Docker is available and running"
    fi

    # Verify API key
    if [ -z "$KUBIYA_API_KEY" ]; then
        log_error "KUBIYA_API_KEY is required for worker"
        echo "Please set your API key: export KUBIYA_API_KEY='your-api-key'"
        return 1
    fi

    log_success "Worker prerequisites verified"

    # Start worker if requested
    if [ "$START_WORKER" = true ]; then
        start_worker
    else
        echo ""
        echo "Worker is configured but not started."
        echo "To start the worker, run:"
        echo ""
        if [ "$WORKER_MODE" = "daemon" ]; then
            echo -e "  ${CYAN}kubiya worker start --queue-id=$WORKER_QUEUE_ID --type=local -d${NC}"
        else
            echo -e "  ${CYAN}kubiya worker start --queue-id=$WORKER_QUEUE_ID --type=$WORKER_MODE${NC}"
        fi
        echo ""
    fi
}

# Start worker
start_worker() {
    log_step "Starting Kubiya Worker"

    WORKER_CMD="kubiya worker start --queue-id=$WORKER_QUEUE_ID"

    case "$WORKER_MODE" in
        local)
            WORKER_CMD="$WORKER_CMD --type=local"
            ;;
        docker)
            WORKER_CMD="$WORKER_CMD --type=docker"
            ;;
        daemon)
            WORKER_CMD="$WORKER_CMD --type=local -d"
            ;;
        *)
            log_error "Invalid worker mode: $WORKER_MODE"
            return 1
            ;;
    esac

    log "Executing: $WORKER_CMD"
    echo ""

    eval "$WORKER_CMD"
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}${BOLD}  Installation Complete! 🎉${NC}"
    echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Verify installation
    if command -v kubiya >/dev/null 2>&1; then
        INSTALLED_VERSION=$(kubiya version 2>/dev/null | head -n 1 | awk '{print $3}' || echo "unknown")
        echo -e "  ${CYAN}Version:${NC} $INSTALLED_VERSION"
        echo -e "  ${CYAN}Location:${NC} $(which kubiya)"
    fi

    echo ""
    echo -e "${BOLD}Next Steps:${NC}"
    echo ""

    if [ -z "$KUBIYA_API_KEY" ]; then
        echo "  1. Set your API key:"
        echo -e "     ${CYAN}export KUBIYA_API_KEY='your-api-key'${NC}"
        echo ""
    fi

    echo "  2. Verify installation:"
    echo -e "     ${CYAN}kubiya version${NC}"
    echo ""

    echo "  3. Get help:"
    echo -e "     ${CYAN}kubiya --help${NC}"
    echo ""

    if [ "$INSTALL_WORKER" = false ]; then
        echo "  4. Start a worker (optional):"
        echo -e "     ${CYAN}kubiya worker start --queue-id=my-queue --type=local${NC}"
        echo ""
    fi

    echo -e "${BOLD}Documentation:${NC}"
    echo -e "  • Main docs:    ${BLUE}https://docs.kubiya.ai${NC}"
    echo -e "  • Worker guide: ${BLUE}docs/worker-guide.md${NC}"
    echo -e "  • GitHub:       ${BLUE}https://github.com/kubiyabot/cli${NC}"
    echo ""

    echo -e "${YELLOW}Note:${NC} You may need to restart your shell or run 'source ~/.zshrc' (or ~/.bashrc)"
    echo "      for environment changes to take effect."
    echo ""
}

# Main execution
main() {
    print_banner

    # Install CLI
    if ! install_cli; then
        log_error "CLI installation failed"
        exit 1
    fi

    # Setup configuration if requested
    if [ "$SETUP_CONFIG" = true ]; then
        setup_configuration
    fi

    # Setup worker if requested
    if [ "$INSTALL_WORKER" = true ]; then
        setup_worker
    fi

    # Print next steps
    print_next_steps

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Installation completed successfully" >> "$LOG_FILE"
}

# Run main function
main
