---
layout: page
title: Troubleshooting
description: Common issues and solutions for Kubiya CLI
toc: true
---

## General Troubleshooting

### Enable Debug Mode

The first step in troubleshooting is to enable debug mode:

```bash
export KUBIYA_DEBUG=true
export KUBIYA_LOG_LEVEL=debug

# Run your command again
kubiya agent list --verbose
```

### Check Version and Environment

```bash
# Check CLI version
kubiya version --build-info

# Check configuration
kubiya config show

# Test basic connectivity
kubiya agent list
```

## Authentication Issues

### Invalid API Key

**Error:**
```
Error: authentication failed: invalid API key
```

**Solutions:**
```bash
# Verify API key is set
echo $KUBIYA_API_KEY

# Check for extra spaces or characters
echo "$KUBIYA_API_KEY" | cat -A

# Test with fresh API key
export KUBIYA_API_KEY="your-new-api-key"
kubiya agent list
```

### API Key Not Found

**Error:**
```
Error: API key not found
```

**Solutions:**
```bash
# Set API key as environment variable
export KUBIYA_API_KEY="your-api-key"

# Or use config file
kubiya config set api_key "your-api-key"

# Or pass as flag
kubiya --api-key "your-api-key" agent list
```

### Permission Denied

**Error:**
```
Error: permission denied: insufficient privileges
```

**Solutions:**
1. Check your API key has the required permissions
2. Contact your administrator to grant necessary permissions
3. Verify you're using the correct API key for your environment

## Network Issues

### Connection Timeout

**Error:**
```
Error: connection timeout
```

**Solutions:**
```bash
# Check network connectivity
ping api.kubiya.ai

# Test with custom timeout
kubiya --timeout 60s agent list

# Use custom base URL if needed
export KUBIYA_BASE_URL="https://your-custom-api.com/api/v1"
```

### SSL Certificate Issues

**Error:**
```
Error: x509: certificate verify failed
```

**Solutions:**
```bash
# Skip SSL verification (not recommended for production)
export KUBIYA_SKIP_SSL_VERIFY=true

# Or specify custom CA bundle
export KUBIYA_CA_BUNDLE="/path/to/ca-bundle.pem"
```

### Proxy Configuration

**Error:**
```
Error: connection refused
```

**Solutions:**
```bash
# Set proxy environment variables
export HTTP_PROXY="http://proxy.company.com:8080"
export HTTPS_PROXY="http://proxy.company.com:8080"
export NO_PROXY="localhost,127.0.0.1,.company.com"

# Or use kubiya-specific proxy settings
kubiya config set proxy_url "http://proxy.company.com:8080"
```

## Agent Issues

### Agent Creation Failed

**Error:**
```
Error: failed to create agent: runner not available
```

**Solutions:**
```bash
# Check available runners
kubiya runner list --healthy

# Use a specific runner
kubiya agent create --name "Test Agent" --runner available-runner

# Check runner health
kubiya runner describe runner-name
```

### Agent Not Responding

**Error:**
```
Error: agent timeout or not responding
```

**Solutions:**
```bash
# Check agent status
kubiya agent list --filter "agent-name"

# Check runner health
kubiya runner list --healthy

# Try recreating the agent
kubiya agent delete agent-uuid
kubiya agent create --name "New Agent" --desc "Replacement agent"
```

### Agent Source Issues

**Error:**
```
Error: source not found or inaccessible
```

**Solutions:**
```bash
# List available sources
kubiya source list

# Check source health
kubiya source scan source-url

# Re-add source if needed
kubiya source add https://github.com/org/repo --name "Repo Name"
```

## Workflow Issues

### Workflow Execution Failed

**Error:**
```
Error: workflow execution failed: step 'deploy' failed
```

**Solutions:**
```bash
# Run with verbose output
kubiya workflow execute workflow.yaml --verbose

# Validate workflow syntax
kubiya workflow execute workflow.yaml --dry-run

# Check workflow logs
kubiya workflow logs --workflow workflow-name

# Check runner availability
kubiya runner list --healthy
```

### GitHub Authentication Issues

**Error:**
```
Error: failed to clone repository: authentication required
```

**Solutions:**
```bash
# Check GitHub integration
kubiya integration list --type github

# Set up GitHub integration via web console
# Visit https://console.kubiya.ai/integrations

# For public repositories, ensure URL is correct
kubiya workflow execute https://github.com/org/public-repo/blob/main/workflow.yaml
```

### Workflow Variables Issues

**Error:**
```
Error: required variable 'environment' not provided
```

**Solutions:**
```bash
# Provide required variables
kubiya workflow execute workflow.yaml --var environment=staging

# Check workflow parameters
kubiya workflow describe workflow.yaml

# Use default values in workflow definition
params:
  environment:
    type: string
    default: development
```

## Tool Execution Issues

### Tool Not Found

**Error:**
```
Error: tool 'kubectl' not found
```

**Solutions:**
```bash
# List available tools
kubiya tool list

# Filter by name
kubiya tool list --filter kubectl

# Check source tools
kubiya tool list --source source-uuid

# Add tool source
kubiya source add https://github.com/kubiya/k8s-tools
```

### Tool Execution Timeout

**Error:**
```
Error: tool execution timeout
```

**Solutions:**
```bash
# Increase timeout
kubiya tool exec --name "long-tool" --timeout 600s --content "long-running-command"

# Check runner capacity
kubiya runner list --all

# Use a different runner
kubiya tool exec --name "tool" --runner faster-runner --content "command"
```

### Docker Issues

**Error:**
```
Error: docker: image not found
```

**Solutions:**
```bash
# Use fully qualified image name
kubiya tool exec --name "test" --type docker --image "docker.io/ubuntu:20.04" --content "ls"

# Check if image exists
docker pull ubuntu:20.04

# Use alternative image
kubiya tool exec --name "test" --type docker --image "alpine:latest" --content "ls"
```

## MCP Integration Issues

### MCP Server Not Starting

**Error:**
```
Error: MCP server failed to start
```

**Solutions:**
```bash
# Check configuration
kubiya mcp validate

# Start with verbose logging
kubiya mcp serve --verbose

# Check port availability
lsof -i :8080

# Use different port
kubiya mcp serve --port 8081
```

### Claude Desktop Not Connecting

**Error:**
```
Claude Desktop: Failed to connect to MCP server
```

**Solutions:**
```bash
# Check configuration file location
ls -la ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Verify configuration syntax
jq '.' ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Test MCP server directly
kubiya mcp test

# Check server logs
tail -f ~/.kubiya/logs/mcp-server.log
```

### Tool Execution via MCP Fails

**Error:**
```
MCP: Tool execution failed
```

**Solutions:**
```bash
# Test tool directly
kubiya tool exec --name "problematic-tool" --content "test command"

# Check tool permissions in MCP config
kubiya mcp show --tool tool-name

# Verify whitelisted tools
cat ~/.kubiya/mcp-config.json | jq '.whitelisted_tools'
```

## Runner Issues

### Runner Not Available

**Error:**
```
Error: runner 'my-runner' not available
```

**Solutions:**
```bash
# List available runners
kubiya runner list --healthy

# Check runner status
kubiya runner describe my-runner

# Use default runner
kubiya tool exec --name "test" --content "echo test"  # Will use default runner
```

### Runner Health Issues

**Error:**
```
Error: runner health check failed
```

**Solutions:**
```bash
# Check runner health
kubiya runner list --all

# Get runner manifest for debugging
kubiya runner manifest my-runner > runner-debug.yaml

# Check Kubernetes deployment (if using K8s)
kubectl get pods -l app=kubiya-runner
kubectl logs -l app=kubiya-runner
```

## Secret Management Issues

### Secret Not Found

**Error:**
```
Error: secret 'DB_PASSWORD' not found
```

**Solutions:**
```bash
# List available secrets
kubiya secret list

# Create missing secret
kubiya secret create DB_PASSWORD "your-password"

# Check secret name spelling
kubiya secret list | grep -i password
```

### Secret Permission Issues

**Error:**
```
Error: insufficient permissions to access secret
```

**Solutions:**
1. Check your API key has secret read permissions
2. Verify the secret exists in your organization
3. Contact administrator for permission escalation

## Performance Issues

### Slow Response Times

**Symptoms:**
- Commands taking longer than usual
- Timeouts on normally fast operations

**Solutions:**
```bash
# Check API status
curl -I https://api.kubiya.ai/health

# Use different runner
kubiya tool exec --runner faster-runner --name "test" --content "echo test"

# Reduce concurrent operations
kubiya config set max_concurrent_requests 2

# Check network latency
ping api.kubiya.ai
```

### Memory Issues

**Error:**
```
Error: out of memory
```

**Solutions:**
```bash
# Check system resources
free -h
df -h

# Use smaller operations
kubiya tool exec --name "small-task" --content "echo 'small task'"

# Clear cache
rm -rf ~/.kubiya/cache/*
```

## Installation Issues

### Binary Not Found

**Error:**
```
bash: kubiya: command not found
```

**Solutions:**
```bash
# Check if binary exists
ls -la /usr/local/bin/kubiya

# Add to PATH
export PATH=$PATH:/usr/local/bin

# Reinstall
curl -fsSL https://cli.kubiya.ai/install.sh | bash
```

### Permission Denied

**Error:**
```
Error: permission denied: /usr/local/bin/kubiya
```

**Solutions:**
```bash
# Make binary executable
chmod +x /usr/local/bin/kubiya

# Or install to user directory
curl -fsSL https://cli.kubiya.ai/install.sh | bash -s -- --install-dir ~/bin
export PATH=$PATH:~/bin
```

## Configuration Issues

### Config File Not Found

**Error:**
```
Error: config file not found
```

**Solutions:**
```bash
# Create config directory
mkdir -p ~/.kubiya

# Initialize configuration
kubiya config init

# Set config path manually
export KUBIYA_CONFIG=~/.kubiya/config.yaml
```

### Invalid Configuration

**Error:**
```
Error: invalid configuration: yaml: invalid syntax
```

**Solutions:**
```bash
# Validate YAML syntax
yamllint ~/.kubiya/config.yaml

# Or use online validator
cat ~/.kubiya/config.yaml

# Reset to defaults
kubiya config init --force
```

## Logging and Debugging

### Enable Detailed Logging

```bash
# Enable debug mode
export KUBIYA_DEBUG=true

# Set log level
export KUBIYA_LOG_LEVEL=debug

# Log to file
export KUBIYA_LOG_FILE=~/.kubiya/debug.log

# Enable trace logging
export KUBIYA_TRACE=true
```

### Check Log Files

```bash
# View recent logs
tail -f ~/.kubiya/logs/kubiya.log

# Search for errors
grep -i error ~/.kubiya/logs/kubiya.log

# Check MCP logs
tail -f ~/.kubiya/logs/mcp-server.log

# View audit logs
cat ~/.kubiya/logs/audit.log
```

## Getting Help

### Built-in Help

```bash
# General help
kubiya --help

# Command-specific help
kubiya agent create --help

# Show all commands
kubiya --help | grep -A 100 "Available Commands"
```

### Community Support

1. **Documentation**: Check our [complete documentation](https://docs.kubiya.ai)
2. **GitHub Issues**: Search or create issues at [GitHub](https://github.com/kubiyabot/cli/issues)
3. **Discussions**: Join discussions at [GitHub Discussions](https://github.com/kubiyabot/cli/discussions)
4. **Status Page**: Check service status at [status.kubiya.ai](https://status.kubiya.ai)

### Bug Reports

When reporting bugs, include:

1. **Version information**: `kubiya version --build-info`
2. **Environment details**: OS, shell, environment variables
3. **Complete error messages**: Full error output with `--verbose`
4. **Reproduction steps**: Exact commands that cause the issue
5. **Expected vs actual behavior**: What you expected vs what happened

### Example Bug Report

```bash
# Gather information
kubiya version --build-info > bug-report.txt
echo "Environment:" >> bug-report.txt
env | grep KUBIYA >> bug-report.txt
echo "Error:" >> bug-report.txt
kubiya agent create --name "test" --verbose 2>&1 >> bug-report.txt
```

## Common Error Patterns

### JSON/YAML Parsing Errors

```bash
# Pretty print JSON response
kubiya agent list -o json | jq '.'

# Validate YAML workflow
yamllint workflow.yaml

# Check for invisible characters
cat -A problematic-file.yaml
```

### Network Request Failures

```bash
# Test connectivity
curl -v https://api.kubiya.ai/health

# Check DNS resolution
nslookup api.kubiya.ai

# Test with different network
# Try from different location/network
```

### Resource Limits

```bash
# Check system resources
top
free -h
df -h

# Check process limits
ulimit -a

# Monitor resource usage
watch -n 1 'ps aux | grep kubiya'
```

Remember: When in doubt, enable debug mode and check the logs. Most issues can be diagnosed with verbose output and proper logging.