---
layout: page
title: Installation
description: Multiple ways to install the Kubiya CLI on your system
toc: true
---

## Quick Installation

### Linux/macOS (Recommended)

The fastest way to get started is with our installation script:

```bash
curl -fsSL https://cli.kubiya.ai/install.sh | bash
```

This script will:
- Detect your operating system and architecture
- Download the latest release
- Install the binary to `/usr/local/bin`
- Set up shell completion
- Verify the installation

### Windows

For Windows users, use PowerShell:

```powershell
iwr -useb https://cli.kubiya.ai/install.ps1 | iex
```

Or download the Windows binary directly from our [releases page](https://github.com/kubiyabot/cli/releases).

## Package Managers

### APT (Debian/Ubuntu)

Add our official APT repository:

```bash
# Add Kubiya's GPG key
curl -fsSL https://cli.kubiya.ai/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/kubiya-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/kubiya-archive-keyring.gpg] https://cli.kubiya.ai/apt stable main" | sudo tee /etc/apt/sources.list.d/kubiya.list

# Update package list and install
sudo apt update
sudo apt install kubiya-cli
```

### Homebrew (macOS)

Coming soon! We're working on Homebrew support.

### Snap (Linux)

Coming soon! Snap package will be available shortly.

## Manual Installation

### Download Binary

1. Go to the [releases page](https://github.com/kubiyabot/cli/releases)
2. Download the appropriate binary for your system:
   - `kubiya-linux-amd64` - Linux x86_64
   - `kubiya-linux-arm64` - Linux ARM64
   - `kubiya-darwin-amd64` - macOS Intel
   - `kubiya-darwin-arm64` - macOS Apple Silicon
   - `kubiya-windows-amd64.exe` - Windows x86_64

3. Make it executable and move to your PATH:

```bash
# Linux/macOS
chmod +x kubiya-*
sudo mv kubiya-* /usr/local/bin/kubiya

# Windows
# Move kubiya-windows-amd64.exe to a directory in your PATH and rename to kubiya.exe
```

### Build from Source

Requirements:
- Go 1.19 or later
- Git

```bash
# Clone the repository
git clone https://github.com/kubiyabot/cli.git
cd cli

# Build the binary
make build

# Install locally
make install
```

## Verify Installation

After installation, verify that Kubiya CLI is working correctly:

```bash
# Check version
kubiya version

# Check help
kubiya --help
```

Expected output:
```
Kubiya CLI v2.0.0
Built with Go 1.19
Platform: linux/amd64
```

## Configuration

### Environment Variables

Set up your API key and configuration:

```bash
# Required: Your Kubiya API key
export KUBIYA_API_KEY="your-api-key-here"

# Optional: Custom API URL (defaults to https://api.kubiya.ai/api/v1)
export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"

# Optional: Enable debug mode
export KUBIYA_DEBUG=true

# Optional: Default runner
export KUBIYA_DEFAULT_RUNNER="my-runner"
```

This creates `~/.kubiya/config.yaml` with default settings:

```yaml
api_key: ""
base_url: "https://api.kubiya.ai/api/v1"
debug: false
default_runner: ""
timeout: "300s"
```

## Shell Completion

Enable tab completion for enhanced CLI experience:

### Bash

```bash
# Add to your ~/.bashrc
echo 'source <(kubiya completion bash)' >> ~/.bashrc
source ~/.bashrc
```

### Zsh

```bash
# Add to your ~/.zshrc
echo 'source <(kubiya completion zsh)' >> ~/.zshrc
source ~/.zshrc
```

### Fish

```bash
# Add to Fish config
kubiya completion fish | source
```

### PowerShell

```powershell
# Add to your PowerShell profile
kubiya completion powershell | Out-String | Invoke-Expression
```

## Docker

Run Kubiya CLI in a Docker container:

```bash
# Pull the image
docker pull kubiya/cli:latest

# Run with environment variables
docker run -e KUBIYA_API_KEY="your-api-key" kubiya/cli:latest version

# Run interactively
docker run -it -e KUBIYA_API_KEY="your-api-key" kubiya/cli:latest
```

## Kubernetes

Deploy Kubiya CLI as a Kubernetes job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: kubiya-cli-job
spec:
  template:
    spec:
      containers:
      - name: kubiya-cli
        image: kubiya/cli:latest
        command: ["kubiya"]
        args: ["workflow", "execute", "myorg/deploy-workflow"]
        env:
        - name: KUBIYA_API_KEY
          valueFrom:
            secretKeyRef:
              name: kubiya-secret
              key: api-key
      restartPolicy: Never
```

## Troubleshooting

### Common Issues

#### Permission Denied
```bash
# If you get permission denied, ensure the binary is executable
chmod +x /usr/local/bin/kubiya
```

#### Command Not Found
```bash
# Ensure the binary is in your PATH
export PATH=$PATH:/usr/local/bin
```

#### SSL Certificate Issues
```bash
# If you encounter SSL issues, try:
export KUBIYA_SKIP_SSL_VERIFY=true
```

### Uninstallation

To remove Kubiya CLI:

```bash
# Remove binary
sudo rm /usr/local/bin/kubiya

# Remove configuration
rm -rf ~/.kubiya

# Remove shell completion (if added)
# Edit your shell RC file to remove the completion line
```

For APT installations:
```bash
sudo apt remove kubiya-cli
sudo rm /etc/apt/sources.list.d/kubiya.list
```

## Getting Help

If you encounter issues during installation:

1. Check our [troubleshooting guide]({{ '/pages/troubleshooting' | relative_url }})
2. Search existing [GitHub issues](https://github.com/kubiyabot/cli/issues)
3. Create a new issue with:
   - Your operating system and version
   - Installation method used
   - Complete error messages
   - Output of `kubiya version` (if available)

## Next Steps

Once installed, you can:

1. [Get started with the quick start guide]({{ '/pages/quickstart' | relative_url }})
2. [Explore command examples]({{ '/pages/examples' | relative_url }})
3. [Set up MCP integration]({{ '/pages/mcp' | relative_url }})
4. [Learn about serverless agents]({{ '/pages/commands' | relative_url }}#agent-management)