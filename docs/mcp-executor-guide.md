# MCP Kubiya Executor Guide

The MCP Kubiya Executor is a wrapper that ensures the Kubiya CLI is available before starting the MCP server. It automatically downloads the latest version of the Kubiya CLI if it's not found or if an update is requested.

## Overview

The executor provides two implementations:
- **Bash script** (`mcp-kubiya-executor`) - For Unix-like systems (Linux, macOS)
- **Python script** (`mcp-kubiya-executor.py`) - Cross-platform support (Windows, Linux, macOS)

Both executors perform the same functions:
1. Check if Kubiya CLI is installed
2. Download it if not found (or if update is requested)
3. Execute the MCP server with the provided arguments

## Installation

### For AI Tools (Claude Desktop, Cursor, etc.)

1. Download the executor script:
   ```bash
   # For Unix-like systems (Linux, macOS)
   curl -L https://raw.githubusercontent.com/kubiyabot/cli/main/mcp-kubiya-executor -o mcp-kubiya-executor
   chmod +x mcp-kubiya-executor
   
   # For Windows or cross-platform
   curl -L https://raw.githubusercontent.com/kubiyabot/cli/main/mcp-kubiya-executor.py -o mcp-kubiya-executor.py
   ```

2. Configure your AI tool to use the executor:

   **Claude Desktop configuration** (`claude_desktop_config.json`):
   ```json
   {
     "mcpServers": {
       "kubiya": {
         "command": "/path/to/mcp-kubiya-executor"
       }
     }
   }
   ```

   **For Windows**:
   ```json
   {
     "mcpServers": {
       "kubiya": {
         "command": "python",
         "args": ["C:\\path\\to\\mcp-kubiya-executor.py"]
       }
     }
   }
   ```

## Usage

The executor accepts all the same arguments as `kubiya mcp serve`:

```bash
# Basic usage (auto-downloads CLI if needed)
./mcp-kubiya-executor

# With MCP server configuration
./mcp-kubiya-executor --config ~/.kubiya/mcp-config.json

# Enable platform APIs
./mcp-kubiya-executor --allow-platform-apis
```

## Environment Variables

The executor respects the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `KUBIYA_CLI_PATH` | Path to existing Kubiya CLI binary | None |
| `KUBIYA_CLI_VERSION` | Specific version to download (e.g., `v1.2.3`) | Latest |
| `KUBIYA_CLI_FORCE_UPDATE` | Force download even if CLI exists (`true`/`false`) | `false` |
| `KUBIYA_API_KEY` | Your Kubiya API key (required for MCP server) | None |

### Examples

```bash
# Use a specific version
export KUBIYA_CLI_VERSION=v1.2.3
./mcp-kubiya-executor

# Force update to latest version
export KUBIYA_CLI_FORCE_UPDATE=true
./mcp-kubiya-executor

# Use existing CLI binary
export KUBIYA_CLI_PATH=/usr/local/bin/kubiya
./mcp-kubiya-executor
```

## Installation Locations

The executor installs the Kubiya CLI to:
- **Unix-like systems**: `~/.kubiya/bin/kubiya`
- **Windows**: `%USERPROFILE%\.kubiya\bin\kubiya.exe`

The installation directory is automatically added to PATH for the current session.

## Platform Support

### Supported Platforms

| OS | Architecture | Binary Name |
|----|--------------|-------------|
| Linux | amd64 | kubiya-cli-linux-amd64 |
| Linux | arm64 | kubiya-cli-linux-arm64 |
| Linux | 386 | kubiya-cli-linux-386 |
| macOS | amd64 | kubiya-cli-darwin-amd64 |
| macOS | arm64 | kubiya-cli-darwin-arm64 |
| Windows | amd64 | kubiya-cli-windows-amd64.exe |
| Windows | 386 | kubiya-cli-windows-386.exe |

## Troubleshooting

### Connection Issues

If the executor can't download the CLI:

1. **Check internet connection**
   ```bash
   curl -I https://github.com/kubiyabot/cli/releases/latest
   ```

2. **Check GitHub API rate limits**
   ```bash
   curl -I https://api.github.com/repos/kubiyabot/cli/releases/latest
   ```

3. **Manually download and set path**
   ```bash
   # Download manually
   curl -L https://github.com/kubiyabot/cli/releases/download/v1.2.3/kubiya-cli-linux-amd64 -o kubiya
   chmod +x kubiya
   
   # Use with executor
   export KUBIYA_CLI_PATH=$(pwd)/kubiya
   ./mcp-kubiya-executor
   ```

### Permission Issues

If you get permission errors:

1. **Ensure executor is executable**
   ```bash
   chmod +x mcp-kubiya-executor
   # or
   chmod +x mcp-kubiya-executor.py
   ```

2. **Check installation directory permissions**
   ```bash
   mkdir -p ~/.kubiya/bin
   chmod 755 ~/.kubiya/bin
   ```

### Windows Specific Issues

1. **Python not found**
   - Install Python 3.7+ from python.org
   - Ensure Python is in PATH

2. **Use Python executor explicitly**
   ```cmd
   python mcp-kubiya-executor.py
   ```

## Advanced Usage

### Custom Download Location

You can modify the executor to use a custom download location:

```bash
# Edit the executor script
INSTALL_DIR="$HOME/.kubiya/bin"  # Change this line
```

### Proxy Support

For environments behind a proxy:

```bash
# For bash executor
export https_proxy=http://proxy.example.com:8080
./mcp-kubiya-executor

# For Python executor
export HTTPS_PROXY=http://proxy.example.com:8080
python mcp-kubiya-executor.py
```

### Offline Installation

For air-gapped environments:

1. Download the CLI manually on a connected machine
2. Transfer to the target machine
3. Set `KUBIYA_CLI_PATH`:
   ```bash
   export KUBIYA_CLI_PATH=/path/to/kubiya
   ./mcp-kubiya-executor
   ```

## Integration Examples

### Claude Desktop (macOS)

1. Install executor:
   ```bash
   mkdir -p ~/bin
   curl -L https://raw.githubusercontent.com/kubiyabot/cli/main/mcp-kubiya-executor -o ~/bin/mcp-kubiya-executor
   chmod +x ~/bin/mcp-kubiya-executor
   ```

2. Configure Claude Desktop:
   ```json
   {
     "mcpServers": {
       "kubiya": {
         "command": "/Users/YOUR_USERNAME/bin/mcp-kubiya-executor",
         "env": {
           "KUBIYA_API_KEY": "your-api-key-here"
         }
       }
     }
   }
   ```

### Cursor

Add to your Cursor MCP configuration:

```json
{
  "mcp": {
    "servers": {
      "kubiya": {
        "command": "python",
        "args": ["/path/to/mcp-kubiya-executor.py"],
        "env": {
          "KUBIYA_API_KEY": "your-api-key-here"
        }
      }
    }
  }
}
```

## Security Considerations

1. **API Key Security**
   - Never commit API keys to version control
   - Use environment variables or secure key management
   - Rotate keys regularly

2. **Binary Verification**
   - The executor downloads from official GitHub releases
   - Consider implementing checksum verification for production use

3. **Network Security**
   - Downloads use HTTPS
   - Consider using a proxy for controlled environments

## Contributing

To contribute to the executor:

1. Fork the repository
2. Make your changes
3. Test on multiple platforms
4. Submit a pull request

## License

The MCP Kubiya Executor is part of the Kubiya CLI project and follows the same license terms. 