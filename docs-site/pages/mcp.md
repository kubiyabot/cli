---
layout: page
title: MCP Integration
description: Model Context Protocol integration for AI-powered development tools
toc: true
---

## What is MCP?

The Model Context Protocol (MCP) is a standardized way for AI applications to access external tools and data sources. Kubiya CLI provides comprehensive MCP integration, allowing you to use Kubiya's powerful automation capabilities directly within AI tools like Claude Desktop and Cursor IDE.

## Features

- **🔧 Tool Execution**: Execute any Kubiya tool with streaming output
- **🤖 Agent Integration**: Access configured serverless agents
- **🏃 Runner Support**: Execute tools on different infrastructure runners
- **🔒 Security Controls**: OPA policy enforcement and tool whitelisting
- **📊 Platform APIs**: Optional access to Kubiya platform APIs
- **⚡ Real-time Streaming**: Live output streaming for long-running operations

## Quick Setup

### Automated Installation

The fastest way to get started:

```bash
# Interactive setup with guided configuration
kubiya mcp setup
```

This will:
1. Download and install the MCP gateway
2. Configure your environment
3. Set up integration with supported applications
4. Create default configurations

### Manual Setup

For more control over the setup process:

```bash
# 1. List available agents
kubiya agent list

# 2. Configure agent UUIDs
export AGENT_UUIDS="abc-123,def-456"

# 3. Set up MCP server
kubiya mcp setup
```

## Configuration

### Basic Configuration

Create a custom MCP configuration file (`~/.kubiya/mcp-config.json`):

```json
{
  "enable_runners": true,
  "allow_platform_apis": false,
  "enable_opa_policies": true,
  "allow_dynamic_tools": false,
  "verbose_logging": false,
  "timeout": "300s",
  "max_concurrent_requests": 5
}
```

### Tool Whitelisting

For security, you can whitelist specific tools:

```json
{
  "enable_runners": true,
  "allow_platform_apis": false,
  "enable_opa_policies": true,
  "allow_dynamic_tools": false,
  "verbose_logging": false,
  "whitelisted_tools": [
    {
      "name": "kubectl",
      "description": "Execute Kubernetes commands",
      "type": "docker",
      "image": "kubiya/kubectl-light:latest",
      "content": "kubectl $command",
      "args": [
        {
          "name": "command",
          "type": "string",
          "description": "The kubectl command to execute",
          "required": true
        }
      ],
      "runner": "k8s-runner"
    },
    {
      "name": "aws-cli",
      "description": "Execute AWS CLI commands",
      "type": "docker",
      "image": "amazon/aws-cli:latest",
      "content": "aws $service $command",
      "args": [
        {
          "name": "service",
          "type": "string",
          "description": "AWS service (e.g., s3, ec2, ecs)",
          "required": true
        },
        {
          "name": "command",
          "type": "string",
          "description": "AWS CLI command",
          "required": true
        }
      ],
      "runner": "aws-runner"
    }
  ]
}
```

### Agent-Specific Configuration

Configure specific agents for MCP access:

```json
{
  "agents": [
    {
      "uuid": "abc-123",
      "name": "DevOps Agent",
      "description": "DevOps automation and deployment",
      "allowed_tools": ["kubectl", "helm", "terraform"],
      "default_runner": "k8s-runner"
    },
    {
      "uuid": "def-456", 
      "name": "Security Agent",
      "description": "Security scanning and compliance",
      "allowed_tools": ["trivy", "snyk", "kube-bench"],
      "default_runner": "security-runner"
    }
  ]
}
```

## Client Configuration

### Claude Desktop

Configure Claude Desktop to use Kubiya MCP server:

```json
{
  "mcpServers": {
    "kubiya": {
      "command": "/usr/local/bin/kubiya",
      "args": ["mcp", "serve", "--config", "/home/user/.kubiya/mcp-config.json"],
      "env": {
        "KUBIYA_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

**Location**: 
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

### Cursor IDE

Configure Cursor IDE MCP integration:

```json
{
  "mcp": {
    "servers": {
      "kubiya": {
        "command": "/usr/local/bin/kubiya",
        "args": ["mcp", "serve", "--config", "/home/user/.kubiya/mcp-config.json"],
        "env": {
          "KUBIYA_API_KEY": "your-api-key-here"
        }
      }
    }
  }
}
```

**Location**: 
- macOS: `~/Library/Application Support/Cursor/User/settings.json`
- Linux: `~/.config/Cursor/User/settings.json`
- Windows: `%APPDATA%\Cursor\User\settings.json`

## Usage Examples

### Basic Tool Execution

Once configured, you can use Kubiya tools directly in your AI applications:

**In Claude Desktop:**
```
Can you check the status of pods in the default namespace?
```

Claude will execute:
```bash
kubectl get pods -n default
```

**In Cursor IDE:**
```
Help me deploy the latest version of my application
```

Cursor will use available Kubiya tools to assist with deployment.

### Advanced Workflows

**Multi-step deployment:**
```
I need to:
1. Build a Docker image for my app
2. Push it to our registry
3. Deploy it to Kubernetes
4. Run health checks
```

The AI will coordinate multiple Kubiya tools to complete this workflow.

**Infrastructure management:**
```
Check our AWS EC2 instances and show me any that are running but not in use
```

The AI will use AWS CLI tools through Kubiya to gather and analyze the information.

## MCP Server Management

### Start MCP Server

```bash
# Start with default configuration
kubiya mcp serve

# Start with custom configuration
kubiya mcp serve --config ~/.kubiya/mcp-config.json

# Start with specific port
kubiya mcp serve --port 8080

# Start with debug logging
kubiya mcp serve --verbose
```

### Test MCP Server

```bash
# Test server functionality
kubiya mcp test

# Test with specific configuration
kubiya mcp test --config test/mcp-config.json

# Test specific tools
kubiya mcp test --tool kubectl
```

### Update MCP Gateway

```bash
# Update to latest version
kubiya mcp update

# Update to specific version
kubiya mcp update --version v1.2.3

# Force update (bypass version check)
kubiya mcp update --force
```

## Configuration Management

### Edit Provider Configurations

```bash
# Edit Claude Desktop configuration
kubiya mcp edit claude_desktop

# Edit Cursor IDE configuration
kubiya mcp edit cursor_ide

# Edit MCP server configuration
kubiya mcp edit config
```

### List MCP Configurations

```bash
# List all MCP configurations
kubiya mcp list

# Show detailed configuration
kubiya mcp show --config ~/.kubiya/mcp-config.json
```

### Validate Configuration

```bash
# Validate MCP configuration
kubiya mcp validate

# Validate specific configuration file
kubiya mcp validate --config ~/.kubiya/mcp-config.json
```

## Security and Permissions

### OPA Policy Integration

Enable OPA policies for enhanced security:

```json
{
  "enable_opa_policies": true,
  "opa_policies": [
    {
      "name": "kubectl_restrictions",
      "policy": "package kubiya.kubectl\n\ndefault allow = false\n\nallow {\n  input.tool == \"kubectl\"\n  input.args.command\n  not contains(input.args.command, \"delete\")\n  not contains(input.args.command, \"exec\")\n}"
    }
  ]
}
```

### Tool Permissions

Configure fine-grained tool permissions:

```json
{
  "tool_permissions": {
    "kubectl": {
      "allowed_namespaces": ["default", "monitoring", "logging"],
      "forbidden_commands": ["delete", "exec", "port-forward"],
      "require_approval": ["apply", "create", "patch"]
    },
    "aws-cli": {
      "allowed_services": ["s3", "ec2", "ecs"],
      "forbidden_actions": ["delete", "terminate", "destroy"],
      "require_approval": ["create", "modify"]
    }
  }
}
```

### Audit Logging

Enable comprehensive audit logging:

```json
{
  "audit_logging": {
    "enabled": true,
    "log_level": "info",
    "log_file": "/var/log/kubiya/mcp-audit.log",
    "include_request_body": true,
    "include_response_body": false
  }
}
```

## Advanced Features

### Custom Tool Integration

Add custom tools to your MCP server:

```json
{
  "custom_tools": [
    {
      "name": "custom_deploy",
      "description": "Custom deployment tool",
      "type": "command",
      "content": "/usr/local/bin/custom-deploy.sh",
      "args": [
        {
          "name": "environment",
          "type": "string",
          "description": "Target environment",
          "required": true
        },
        {
          "name": "version",
          "type": "string",
          "description": "Version to deploy",
          "required": true
        }
      ],
      "timeout": "600s"
    }
  ]
}
```

### Streaming Output

Enable real-time streaming for long-running operations:

```json
{
  "streaming": {
    "enabled": true,
    "buffer_size": 1024,
    "flush_interval": "100ms",
    "max_stream_duration": "30m"
  }
}
```

### Caching

Configure response caching for better performance:

```json
{
  "caching": {
    "enabled": true,
    "ttl": "5m",
    "max_size": "100MB",
    "cache_keys": ["tool_name", "args", "runner"]
  }
}
```

## Troubleshooting

### Common Issues

**MCP Server Not Starting:**
```bash
# Check configuration
kubiya mcp validate

# Check logs
kubiya mcp serve --verbose

# Test connectivity
kubiya mcp test
```

**Tool Execution Fails:**
```bash
# Check runner status
kubiya runner list --healthy

# Verify tool permissions
kubiya mcp show --tool kubectl

# Test tool directly
kubiya tool exec --name kubectl --content "get pods"
```

**Claude Desktop Not Connecting:**
```bash
# Verify configuration file location
ls -la ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Check server status
kubiya mcp serve --verbose

# Test MCP protocol
kubiya mcp test --client claude_desktop
```

### Debug Mode

Enable debug mode for troubleshooting:

```bash
# Start server in debug mode
KUBIYA_DEBUG=true kubiya mcp serve --verbose

# Enable debug logging in configuration
{
  "verbose_logging": true,
  "log_level": "debug"
}
```

### Log Analysis

Check MCP server logs:

```bash
# View recent logs
tail -f ~/.kubiya/logs/mcp-server.log

# Search for specific errors
grep -i "error" ~/.kubiya/logs/mcp-server.log

# Analyze performance
grep -i "duration" ~/.kubiya/logs/mcp-server.log
```

## Best Practices

1. **Security First**: Always use tool whitelisting in production
2. **Monitor Performance**: Enable logging and monitoring
3. **Regular Updates**: Keep MCP gateway updated
4. **Configuration Management**: Version control your MCP configurations
5. **Testing**: Test configurations before deploying
6. **Documentation**: Document custom tools and configurations

## Migration and Upgrades

### Upgrading MCP Gateway

```bash
# Check current version
kubiya mcp version

# Update to latest version
kubiya mcp update

# Backup configurations before upgrade
cp ~/.kubiya/mcp-config.json ~/.kubiya/mcp-config.json.backup
```

### Migrating Configurations

```bash
# Export current configuration
kubiya mcp export --output backup.json

# Import configuration on new system
kubiya mcp import --input backup.json
```

The MCP integration provides a powerful bridge between AI development tools and Kubiya's automation capabilities, enabling seamless infrastructure management and deployment workflows directly from your development environment.