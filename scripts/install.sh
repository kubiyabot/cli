#!/usr/bin/env bash
#
# Kubiya CLI & Worker Bootstrap Installer
#
# Quick install:
#   curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
#
# With environment variables:
#   KUBIYA_API_KEY=xxx KUBIYA_QUEUE_ID=yyy curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
#
# Options:
#   KUBIYA_VERSION      - CLI version to install (default: latest)
#   KUBIYA_API_KEY      - API key (skip interactive login)
#   KUBIYA_QUEUE_ID     - Queue ID for worker (skip interactive input)
#   KUBIYA_WORKER_MODE  - Worker mode: daemon or foreground (default: daemon)
#   SKIP_WORKER_START   - Set to 1 to skip worker startup
#   INSTALL_DIR         - Installation directory (default: /usr/local/bin)
#

set -e

# Configuration
VERSION="${KUBIYA_VERSION:-v2.5.5}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
REPO="kubiyabot/cli"
GITHUB_API="https://api.github.com/repos/${REPO}"
GITHUB_RELEASES="https://github.com/${REPO}/releases"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Symbols
CHECK="${GREEN}✓${NC}"
CROSS="${RED}✗${NC}"
ARROW="${CYAN}→${NC}"
STAR="${YELLOW}★${NC}"

# Print functions
print_banner() {
    echo ""
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}                                                                               ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}                    ${BOLD}KUBIYA CLI & WORKER INSTALLER${NC}                           ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}                                                                               ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${BOLD}${BLUE}▶ $1${NC}"
    echo -e "${BLUE}$(printf '─%.0s' {1..80})${NC}"
}

print_success() {
    echo -e "  ${CHECK} $1"
}

print_error() {
    echo -e "  ${CROSS} $1" >&2
}

print_info() {
    echo -e "  ${ARROW} $1"
}

print_warning() {
    echo -e "  ${YELLOW}⚠${NC}  $1"
}

# Error handler
error_exit() {
    print_error "$1"
    exit 1
}

# Detect OS and Architecture
detect_platform() {
    print_section "Detecting Platform"

    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "${OS}" in
        Linux*)
            OS_TYPE="linux"
            # Detect Linux distribution
            if [ -f /etc/os-release ]; then
                . /etc/os-release
                DISTRO="${ID}"
                print_info "Distribution: ${NAME}"
            elif [ -f /etc/arch-release ]; then
                DISTRO="arch"
                print_info "Distribution: Arch Linux"
            else
                DISTRO="unknown"
                print_info "Distribution: Unknown"
            fi
            ;;
        Darwin*)
            OS_TYPE="darwin"
            DISTRO="macos"
            print_info "Distribution: macOS"
            ;;
        *)
            error_exit "Unsupported operating system: ${OS}"
            ;;
    esac

    case "${ARCH}" in
        x86_64|amd64)
            ARCH_TYPE="amd64"
            ;;
        aarch64|arm64)
            ARCH_TYPE="arm64"
            ;;
        armv7l)
            ARCH_TYPE="armv7"
            ;;
        *)
            error_exit "Unsupported architecture: ${ARCH}"
            ;;
    esac

    print_success "Platform: ${OS_TYPE}/${ARCH_TYPE}"

    # Set binary name (matches GitHub release naming)
    BINARY_NAME="kubiya-cli-${OS_TYPE}-${ARCH_TYPE}"
}

# Check prerequisites
check_prerequisites() {
    print_section "Checking Prerequisites"

    # Check curl
    if ! command -v curl &> /dev/null; then
        error_exit "curl is required but not installed. Please install curl first."
    fi
    print_success "curl is available"

    # Check tar
    if ! command -v tar &> /dev/null; then
        error_exit "tar is required but not installed. Please install tar first."
    fi
    print_success "tar is available"

    # Check Python 3 (needed for worker)
    if command -v python3 &> /dev/null; then
        PYTHON_VERSION=$(python3 --version 2>&1 | cut -d' ' -f2)
        print_success "Python 3 is available (${PYTHON_VERSION})"
        HAS_PYTHON=1
    else
        print_warning "Python 3 is not installed (required for local worker mode)"
        HAS_PYTHON=0
    fi

    # Check Docker (optional)
    if command -v docker &> /dev/null; then
        print_success "Docker is available"
        HAS_DOCKER=1
    else
        print_info "Docker is not installed (optional, for container mode)"
        HAS_DOCKER=0
    fi
}

# Get latest version if not specified
get_latest_version() {
    if [ "${VERSION}" = "latest" ]; then
        print_section "Fetching Latest Version"
        VERSION=$(curl -fsSL "${GITHUB_API}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "${VERSION}" ]; then
            error_exit "Failed to fetch latest version"
        fi
        print_success "Latest version: ${VERSION}"
    fi
}

# Download and install CLI
install_cli() {
    print_section "Installing Kubiya CLI"

    DOWNLOAD_URL="${GITHUB_RELEASES}/download/${VERSION}/${BINARY_NAME}"

    print_info "Downloading from: ${DOWNLOAD_URL}"

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT

    # Download binary
    if ! curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/kubiya"; then
        error_exit "Failed to download Kubiya CLI from ${DOWNLOAD_URL}"
    fi

    print_success "Downloaded successfully"

    # Make executable
    chmod +x "${TMP_DIR}/kubiya"

    # Check if install directory needs sudo
    NEED_SUDO=0
    if [ ! -w "${INSTALL_DIR}" ]; then
        NEED_SUDO=1
        print_warning "Installation directory requires sudo access"
    fi

    # Install binary
    if [ ${NEED_SUDO} -eq 1 ]; then
        if ! command -v sudo &> /dev/null; then
            error_exit "sudo is required to install to ${INSTALL_DIR}"
        fi
        print_info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${TMP_DIR}/kubiya" "${INSTALL_DIR}/kubiya"
        sudo chmod +x "${INSTALL_DIR}/kubiya"
    else
        print_info "Installing to ${INSTALL_DIR}..."
        mv "${TMP_DIR}/kubiya" "${INSTALL_DIR}/kubiya"
    fi

    # Verify installation
    if ! command -v kubiya &> /dev/null; then
        error_exit "Installation failed. ${INSTALL_DIR} may not be in your PATH"
    fi

    INSTALLED_VERSION=$(kubiya version 2>/dev/null || echo "unknown")
    print_success "Kubiya CLI installed successfully"
    print_info "Version: ${INSTALLED_VERSION}"
}

# Configure API key
configure_api_key() {
    print_section "Configuring API Key"

    # Check if API key is already set
    if [ -n "${KUBIYA_API_KEY}" ]; then
        print_success "API key found in environment variable"
        return 0
    fi

    # Check if already logged in
    if kubiya config get api-key &> /dev/null; then
        EXISTING_KEY=$(kubiya config get api-key 2>/dev/null || echo "")
        if [ -n "${EXISTING_KEY}" ]; then
            print_success "Already logged in"
            KUBIYA_API_KEY="${EXISTING_KEY}"
            return 0
        fi
    fi

    # Interactive API key configuration
    echo ""
    echo -e "${BOLD}Choose authentication method:${NC}"
    echo -e "  ${CYAN}1)${NC} Login via browser (recommended)"
    echo -e "  ${CYAN}2)${NC} Enter API key manually"
    echo ""
    read -p "Select option [1-2]: " AUTH_CHOICE

    case "${AUTH_CHOICE}" in
        1)
            print_info "Launching browser login..."
            if kubiya login; then
                print_success "Login successful"
                KUBIYA_API_KEY=$(kubiya config get api-key 2>/dev/null || echo "")
            else
                error_exit "Login failed"
            fi
            ;;
        2)
            echo ""
            read -p "Enter your API key: " INPUT_API_KEY
            if [ -z "${INPUT_API_KEY}" ]; then
                error_exit "API key cannot be empty"
            fi

            # Save API key
            export KUBIYA_API_KEY="${INPUT_API_KEY}"
            if kubiya config set api-key "${INPUT_API_KEY}"; then
                print_success "API key configured"
            else
                error_exit "Failed to configure API key"
            fi
            ;;
        *)
            error_exit "Invalid option"
            ;;
    esac
}

# Configure worker
configure_worker() {
    print_section "Configuring Worker"

    # Check if we should skip worker setup
    if [ "${SKIP_WORKER_START}" = "1" ]; then
        print_warning "Skipping worker setup (SKIP_WORKER_START=1)"
        return 0
    fi

    # Get queue ID
    if [ -z "${KUBIYA_QUEUE_ID}" ]; then
        echo ""
        echo -e "${BOLD}Worker Configuration${NC}"
        echo -e "${BLUE}$(printf '─%.0s' {1..80})${NC}"
        echo ""
        read -p "Enter worker queue ID: " INPUT_QUEUE_ID

        if [ -z "${INPUT_QUEUE_ID}" ]; then
            print_warning "No queue ID provided, skipping worker setup"
            return 0
        fi

        KUBIYA_QUEUE_ID="${INPUT_QUEUE_ID}"
    fi

    print_success "Queue ID: ${KUBIYA_QUEUE_ID}"

    # Determine worker mode
    WORKER_MODE="${KUBIYA_WORKER_MODE:-daemon}"

    if [ "${WORKER_MODE}" != "daemon" ] && [ "${WORKER_MODE}" != "foreground" ]; then
        print_warning "Invalid KUBIYA_WORKER_MODE (${WORKER_MODE}), using 'daemon'"
        WORKER_MODE="daemon"
    fi

    print_info "Worker mode: ${WORKER_MODE}"

    # Check if Python is available for local mode
    if [ ${HAS_PYTHON} -eq 0 ]; then
        print_warning "Python 3 is not available, cannot start local worker"
        if [ ${HAS_DOCKER} -eq 1 ]; then
            print_info "Consider using Docker mode: kubiya worker start --queue-id=${KUBIYA_QUEUE_ID} --type=docker"
        fi
        return 0
    fi
}

# Start worker
start_worker() {
    if [ "${SKIP_WORKER_START}" = "1" ] || [ -z "${KUBIYA_QUEUE_ID}" ]; then
        return 0
    fi

    print_section "Starting Worker"

    # Build worker command
    WORKER_CMD="kubiya worker start --queue-id=${KUBIYA_QUEUE_ID} --type=local"

    if [ "${WORKER_MODE}" = "daemon" ]; then
        WORKER_CMD="${WORKER_CMD} -d"
    fi

    print_info "Executing: ${WORKER_CMD}"
    echo ""

    # Start worker
    if ${WORKER_CMD}; then
        echo ""
        print_success "Worker started successfully"

        if [ "${WORKER_MODE}" = "daemon" ]; then
            echo ""
            echo -e "${BOLD}Worker Management Commands:${NC}"
            echo -e "  ${CYAN}Status:${NC}  kubiya worker status --queue-id=${KUBIYA_QUEUE_ID}"
            echo -e "  ${CYAN}Stop:${NC}    kubiya worker stop --queue-id=${KUBIYA_QUEUE_ID}"
            echo -e "  ${CYAN}Logs:${NC}    tail -f ~/.kubiya/workers/${KUBIYA_QUEUE_ID}/worker.log"
        fi
    else
        print_error "Failed to start worker"
        return 1
    fi
}

# Print completion message
print_completion() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}                                                                               ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}                    ${BOLD}${GREEN}INSTALLATION COMPLETED SUCCESSFULLY!${NC}                        ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}                                                                               ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BOLD}What's Next?${NC}"
    echo ""
    echo -e "  ${STAR} View CLI help:           ${CYAN}kubiya --help${NC}"
    echo -e "  ${STAR} Check worker status:     ${CYAN}kubiya worker status --queue-id=${KUBIYA_QUEUE_ID:-<queue-id>}${NC}"
    echo -e "  ${STAR} View documentation:      ${CYAN}https://github.com/kubiyabot/cli${NC}"
    echo ""

    if [ -n "${KUBIYA_QUEUE_ID}" ] && [ "${WORKER_MODE}" = "daemon" ]; then
        echo -e "${BOLD}Worker Information:${NC}"
        echo -e "  ${ARROW} Queue ID:              ${KUBIYA_QUEUE_ID}"
        echo -e "  ${ARROW} Mode:                  Daemon (background)"
        echo -e "  ${ARROW} Log file:              ~/.kubiya/workers/${KUBIYA_QUEUE_ID}/worker.log"
        echo ""
    fi

    echo -e "${BOLD}Environment Variables:${NC}"
    echo -e "  ${ARROW} KUBIYA_API_KEY         Your API key"
    echo -e "  ${ARROW} KUBIYA_QUEUE_ID        Worker queue ID"
    echo -e "  ${ARROW} KUBIYA_WORKER_MODE     daemon or foreground"
    echo -e "  ${ARROW} KUBIYA_MAX_LOG_SIZE    Max log file size (bytes)"
    echo ""
}

# Main installation flow
main() {
    print_banner

    detect_platform
    check_prerequisites
    get_latest_version
    install_cli
    configure_api_key
    configure_worker
    start_worker
    print_completion
}

# Run main function
main "$@"
