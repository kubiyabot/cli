# Kubiya CLI Bootstrap Installer

One-line installation script that sets up Kubiya CLI and worker on Linux, macOS, and Arch Linux.

## 🚀 Quick Start

### Interactive Installation

```bash
curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

This will:
1. Detect your platform (Linux/macOS/Arch)
2. Download and install the latest Kubiya CLI
3. Guide you through authentication (browser login or API key)
4. Optionally configure and start a worker in daemon mode

### Automated Installation with Environment Variables

```bash
KUBIYA_API_KEY="your-api-key" \
KUBIYA_QUEUE_ID="your-queue-id" \
curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

## 📋 Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `KUBIYA_VERSION` | CLI version to install | `v2.5.5` | No |
| `KUBIYA_API_KEY` | Your Kubiya API key (skips interactive login) | - | No* |
| `KUBIYA_QUEUE_ID` | Worker queue ID (skips interactive input) | - | No* |
| `KUBIYA_WORKER_MODE` | Worker mode: `daemon` or `foreground` | `daemon` | No |
| `SKIP_WORKER_START` | Set to `1` to skip worker startup | - | No |
| `INSTALL_DIR` | Installation directory | `/usr/local/bin` | No |
| `KUBIYA_MAX_LOG_SIZE` | Max log file size in bytes | `104857600` (100MB) | No |

\* Required for automated installation without prompts

## 🎯 Usage Examples

### Example 1: Install CLI Only (No Worker)

```bash
SKIP_WORKER_START=1 \
curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

### Example 2: Full Automated Setup for CI/CD

```bash
export KUBIYA_API_KEY="your-api-key"
export KUBIYA_QUEUE_ID="my-worker-queue"
export KUBIYA_WORKER_MODE="daemon"
export KUBIYA_VERSION="v2.5.5"

curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

### Example 3: Install to Custom Directory

```bash
INSTALL_DIR="$HOME/.local/bin" \
curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

### Example 4: Install Specific Version

```bash
KUBIYA_VERSION="v2.5.4" \
curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh | bash
```

### Example 5: Quick Worker Setup (Already Have CLI)

If you already have the CLI installed and just want to start a worker:

```bash
KUBIYA_QUEUE_ID="my-queue" \
KUBIYA_WORKER_MODE="daemon" \
bash <(curl -fsSL https://raw.githubusercontent.com/kubiyabot/cli/main/scripts/install.sh)
```

## 🖥️ Supported Platforms

### Operating Systems
- ✅ Linux (Ubuntu, Debian, CentOS, RHEL, etc.)
- ✅ macOS (Intel & Apple Silicon)
- ✅ Arch Linux

### Architectures
- ✅ x86_64 / amd64
- ✅ arm64 / aarch64
- ✅ armv7

## 📦 Prerequisites

The script automatically checks for required tools:

### Required
- `curl` - For downloading binaries
- `tar` - For extracting archives

### Optional
- `python3` - Required for local worker mode
- `docker` - Required for Docker worker mode
- `sudo` - May be needed for installation to `/usr/local/bin`

## 🔧 What the Script Does

1. **Platform Detection**
   - Identifies OS (Linux/macOS) and architecture
   - Detects Linux distribution for better compatibility

2. **Prerequisites Check**
   - Verifies required tools (curl, tar)
   - Checks for Python 3 (worker requirement)
   - Checks for Docker (optional)

3. **CLI Installation**
   - Downloads specified version from GitHub releases
   - Installs to `/usr/local/bin` (or custom directory)
   - Makes binary executable and verifies installation

4. **Authentication**
   - Interactive: Choose browser login or manual API key entry
   - Automated: Uses `KUBIYA_API_KEY` environment variable
   - Checks for existing authentication

5. **Worker Configuration**
   - Interactive: Prompts for queue ID
   - Automated: Uses `KUBIYA_QUEUE_ID` environment variable
   - Configures daemon mode with supervision

6. **Worker Startup**
   - Starts worker in daemon mode (background) by default
   - Provides management commands (status, stop, logs)
   - Configures crash recovery and log rotation

## 🎛️ Worker Daemon Features

When starting a worker in daemon mode (`-d` flag):

### Supervision & Recovery
- ✅ Automatic crash recovery with exponential backoff
- ✅ Maximum 5 restart attempts with cooldown
- ✅ Process health monitoring

### Logging
- ✅ Rotating logs (100MB default, configurable)
- ✅ Keeps 5 backup files
- ✅ Captures both CLI and Python worker output
- ✅ Timestamped entries

### Management
- ✅ PID file tracking
- ✅ Status command: `kubiya worker status --queue-id=<id>`
- ✅ Stop command: `kubiya worker stop --queue-id=<id>`
- ✅ Graceful shutdown (SIGTERM)

## 📝 Post-Installation

After installation, you'll receive:

```bash
# Check worker status
kubiya worker status --queue-id=<your-queue-id>

# View logs
tail -f ~/.kubiya/workers/<your-queue-id>/worker.log

# Stop worker
kubiya worker stop --queue-id=<your-queue-id>
```

## 🐛 Troubleshooting

### "Failed to download Kubiya CLI"
- Check your internet connection
- Verify the version exists: https://github.com/kubiyabot/cli/releases
- Try specifying a different version: `KUBIYA_VERSION=v2.5.4`

### "Python 3 is not installed"
- Install Python 3.8 or later
- Or use Docker mode: `--type=docker`

### "Installation failed. /usr/local/bin may not be in your PATH"
- Add to PATH: `export PATH="$PATH:/usr/local/bin"`
- Or install to different directory: `INSTALL_DIR="$HOME/.local/bin"`

### "Login failed"
- Try manual API key entry (option 2)
- Get API key from: https://app.kubiya.ai/settings/api-keys

## 🔐 Security Notes

- The script uses HTTPS for all downloads
- API keys are stored securely in `~/.kubiya/config.yaml`
- Worker logs may contain sensitive information - protect accordingly
- Use environment variables in CI/CD to avoid interactive prompts

## 📚 Additional Resources

- [CLI Documentation](https://github.com/kubiyabot/cli)
- [Worker Documentation](https://docs.kubiya.ai/workers)
- [API Documentation](https://docs.kubiya.ai/api)

## 🤝 Contributing

Found a bug or want to improve the installer? PRs welcome!

- Report issues: https://github.com/kubiyabot/cli/issues
- Submit PRs: https://github.com/kubiyabot/cli/pulls

## 📄 License

This script is part of the Kubiya CLI project and follows the same license.
