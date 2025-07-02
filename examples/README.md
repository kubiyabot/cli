# Kubiya MCP Server Configuration Examples

This directory contains example configuration files for the Kubiya MCP (Model Context Protocol) server.

## Configuration Files

### 1. `mcp-server-config.json` - Full Production Configuration
This is a comprehensive configuration file that includes all available options for production use:
- Server metadata (name, version)
- Session management settings
- Authentication and security options
- Rate limiting configuration
- Whitelisted tools with detailed configurations
- Tool contexts and examples
- Permission settings
- Feature flags

### 2. `mcp-server-simple.json` - Simple Configuration
A minimal configuration file for basic usage:
- Essential settings only
- Basic tool whitelisting
- Suitable for development and testing

## Usage Examples

### Basic Usage (Platform APIs enabled by default)
```bash
# Start with default settings
kubiya mcp serve

# Start with simple config
kubiya mcp serve --config examples/mcp-server-simple.json
```

### Command Line Configuration
```bash
# Disable platform APIs
kubiya mcp serve --disable-platform-apis

# Add specific tools via command line
kubiya mcp serve --whitelist-tools kubectl,helm,terraform

# Production mode with authentication
kubiya mcp serve --production --require-auth --session-timeout 3600

# Custom server settings
kubiya mcp serve --server-name "My Kubiya Server" --server-version "2.0.0"

# Disable runners and enable OPA policies
kubiya mcp serve --disable-runners --enable-opa-policies
```

### Environment Variables
You can also configure the server using environment variables:
```bash
export KUBIYA_MCP_ENABLE_RUNNERS=true
export KUBIYA_MCP_ALLOW_PLATFORM_APIS=true
export KUBIYA_OPA_ENFORCE=false
export KUBIYA_MCP_REQUIRE_AUTH=false

kubiya mcp serve
```

### Claude Desktop Integration
Add this to your Claude Desktop configuration file (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "kubiya": {
      "command": "kubiya",
      "args": ["mcp", "serve"],
      "env": {
        "KUBIYA_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

For production with custom config:
```json
{
  "mcpServers": {
    "kubiya": {
      "command": "kubiya",
      "args": ["mcp", "serve", "--production", "--config", "/path/to/your/config.json"],
      "env": {
        "KUBIYA_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

## Configuration Options

### Core Flags
- `--disable-platform-apis`: Disable platform management APIs (enabled by default)
- `--disable-runners`: Disable tool runners
- `--enable-opa-policies`: Enable OPA policy enforcement
- `--production`: Run in production mode with full features

### Tool Configuration
- `--whitelist-tools`: Comma-separated list of tools to whitelist

### Production Mode Options
- `--require-auth`: Require authentication
- `--session-timeout`: Session timeout in seconds
- `--server-name`: Custom server name
- `--server-version`: Custom server version

## Configuration Precedence
Settings are applied in the following order (later overrides earlier):
1. Default values
2. Configuration file
3. Environment variables
4. Command line flags

## Default Configuration Location
If no config file is specified, the server looks for configuration at:
`~/.kubiya/mcp-server.json`