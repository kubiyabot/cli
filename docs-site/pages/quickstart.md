---
layout: page
title: Quick Start
description: Get up and running with Kubiya CLI in minutes
toc: true
---

## Prerequisites

Before getting started, ensure you have:

1. **Kubiya CLI installed** - [Installation guide]({{ '/pages/installation' | relative_url }})
2. **API Key** - Get one from [Kubiya Console](https://console.kubiya.ai)
3. **Basic familiarity** with command line tools

## Step 1: Initial Setup

### Configure Authentication

Set your API key as an environment variable:

```bash
export KUBIYA_API_KEY="your-api-key-here"
```

Or add it to your shell profile:

```bash
echo 'export KUBIYA_API_KEY="your-api-key-here"' >> ~/.bashrc
source ~/.bashrc
```


This creates `~/.kubiya/config.yaml` with default settings. You can customize it as needed.

### Verify Setup

Test your configuration:

```bash
kubiya version
kubiya agent list
```

## Step 2: Create Your First Serverless Agent

### Interactive Agent Creation

The easiest way to create an agent is interactively:

```bash
kubiya agent create --interactive
```

This will guide you through:
- Agent name and description
- Source selection (optional)
- Environment variables
- Webhook configuration
- Runner selection

### Quick Agent Creation

For a quick start, create a simple agent:

```bash
kubiya agent create \
  --name "My First Agent" \
  --desc "A simple automation agent for testing"
```

### Verify Agent Creation

List your agents to confirm creation:

```bash
kubiya agent list
```

You should see your new agent with a status of "Active".

## Step 3: Execute Your First Workflow

### From a Local File

Create a simple workflow file:

```bash
cat > hello-world.yaml << 'EOF'
name: hello-world
description: A simple hello world workflow
steps:
- name: greet
  executor: command
  command: echo "Hello from Kubiya CLI!"
- name: date
  executor: command
  command: date
  depends: [greet]
EOF
```

Execute it:

```bash
kubiya workflow execute hello-world.yaml
```

### From GitHub Repository

Execute a workflow from a GitHub repository:

```bash
kubiya workflow execute kubiyabot/examples/basic/hello-world.yaml
```

### With Variables

Execute a workflow with custom variables:

```bash
kubiya workflow execute hello-world.yaml \
  --var name="World" \
  --var message="Hello from the CLI"
```

## Step 4: Explore Tool Management

### List Available Tools

```bash
kubiya tool list
```

### Execute a Simple Tool

```bash
kubiya tool exec \
  --name "system-info" \
  --content "uname -a && uptime"
```

### Execute Tool with Docker

```bash
kubiya tool exec \
  --name "python-hello" \
  --type docker \
  --image python:3.11-slim \
  --content "print('Hello from Python in Docker!')"
```

## Step 5: Chat with Your Agent

### Interactive Chat

Start an interactive chat session:

```bash
kubiya chat --interactive
```

### Quick Chat

Send a quick message to your agent:

```bash
kubiya chat \
  --name "My First Agent" \
  --message "Tell me about the system status"
```

### Chat with Context

Provide context files to your agent:

```bash
kubiya chat \
  --name "My First Agent" \
  --message "Review this configuration" \
  --context config.yaml \
  --context Dockerfile
```

## Step 6: Source Management

### Scan Local Directory

Scan your current directory for tools:

```bash
kubiya source scan .
```

### Add a Source

Add a source from a GitHub repository:

```bash
kubiya source add \
  https://github.com/kubiyabot/examples \
  --name "Example Tools"
```

### List Sources

```bash
kubiya source list
```

## Step 7: Advanced Features

### MCP Integration

```bash
kubiya mcp setup
```

### Secret Management

Create and manage secrets:

```bash
kubiya secret create DB_PASSWORD "secure-password-123" \
  --description "Database password for staging"
```

### Webhook Setup

Create a webhook for Slack integration:

```bash
kubiya webhook create \
  --type slack \
  --destination "#alerts" \
  --name "Alert Handler" \
  --prompt "Analyze and respond to this alert"
```

## Common Workflows

### Deploy Application

```bash
# 1. Create deployment agent
kubiya agent create \
  --name "Deployment Agent" \
  --desc "Handles application deployments"

# 2. Execute deployment workflow
kubiya workflow execute myorg/deploy-scripts/production.yaml \
  --var environment=production \
  --var version=v1.2.3

# 3. Monitor deployment
kubiya chat \
  --name "Deployment Agent" \
  --message "Check deployment status for version v1.2.3"
```

### CI/CD Pipeline

```bash
# Execute CI/CD pipeline
kubiya workflow execute myorg/ci-cd-pipeline \
  --var branch=main \
  --var run_tests=true \
  --var deploy_on_success=true
```

### Infrastructure Management

```bash
# Create infrastructure agent
kubiya agent create \
  --name "Infrastructure Agent" \
  --desc "Manages AWS and Kubernetes infrastructure" \
  --secret AWS_ACCESS_KEY_ID \
  --secret AWS_SECRET_ACCESS_KEY

# Execute infrastructure changes
kubiya workflow execute myorg/infrastructure/terraform-apply.yaml \
  --var environment=staging
```

## Best Practices

### Agent Organization

1. **Single Responsibility**: Create agents focused on specific domains
2. **Descriptive Names**: Use clear, descriptive names for agents
3. **Proper Secrets**: Use secret management for sensitive data
4. **Resource Limits**: Configure appropriate resource limits

### Workflow Management

1. **Version Control**: Keep workflows in version control
2. **Environment Variables**: Use variables for environment-specific values
3. **Error Handling**: Include proper error handling in workflows
4. **Documentation**: Document workflow parameters and behavior

### Security

1. **API Key Security**: Never commit API keys to version control
2. **Secret Management**: Use Kubiya's secret management for sensitive data
3. **Runner Security**: Keep runner environments secure and updated
4. **Network Security**: Configure proper network policies

## Troubleshooting

### Common Issues

**Authentication Error**:
```bash
# Verify API key is set
echo $KUBIYA_API_KEY

# Test authentication
kubiya agent list
```

**Workflow Execution Fails**:
```bash
# Run with debug mode
export KUBIYA_DEBUG=true
kubiya workflow execute workflow.yaml --verbose
```

**Agent Creation Issues**:
```bash
# Check runner availability
kubiya runner list --healthy

# Verify source access
kubiya source list
```

### Getting Help

1. **Debug Mode**: Enable with `export KUBIYA_DEBUG=true`
2. **Verbose Output**: Use `--verbose` flag with commands
3. **Documentation**: Check our [full documentation]({{ '/pages/commands' | relative_url }})
4. **Community**: Join our [GitHub discussions](https://github.com/kubiyabot/cli/discussions)

## Next Steps

Now that you have the basics down, explore:

1. [Complete command reference]({{ '/pages/commands' | relative_url }})
2. [Advanced examples]({{ '/pages/examples' | relative_url }})
3. [MCP integration setup]({{ '/pages/mcp' | relative_url }})
4. [API reference]({{ '/pages/api' | relative_url }})
5. [Troubleshooting guide]({{ '/pages/troubleshooting' | relative_url }})

Happy automating! 🚀