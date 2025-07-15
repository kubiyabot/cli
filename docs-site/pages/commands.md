---
layout: page
title: Commands Reference
description: Complete reference for all Kubiya CLI commands
toc: true
---

## Global Options

All commands support these global options:

```bash
--help, -h          Show help information
--version, -v       Show version information
--config            Path to config file (default: ~/.kubiya/config.yaml)
--api-key           Override API key from config
--base-url          Override base URL from config
--debug             Enable debug output
--output, -o        Output format (table, json, yaml)
--quiet, -q         Suppress output
```

## Agent Management

### kubiya agent create

Create a new serverless agent.

```bash
kubiya agent create [OPTIONS]
```

**Options:**
- `--name, -n`: Agent name (required)
- `--desc, -d`: Agent description
- `--interactive, -i`: Interactive mode with prompts
- `--source`: Source UUIDs to attach (can be repeated)
- `--env`: Environment variables in key=value format (can be repeated)
- `--secret`: Secret names to attach (can be repeated)
- `--webhook-method`: Webhook method (slack, teams, http)
- `--webhook-dest`: Webhook destination
- `--webhook-prompt`: Webhook prompt message
- `--runner`: Default runner for the agent
- `--ai-instructions`: Custom AI instructions
- `--llm-model`: LLM model to use

**Examples:**
```bash
# Interactive creation
kubiya agent create --interactive

# Basic agent
kubiya agent create --name "DevOps Agent" --desc "Handles DevOps tasks"

# Agent with environment variables and secrets
kubiya agent create --name "AWS Agent" \
  --env "AWS_REGION=us-west-2" \
  --env "DEBUG=true" \
  --secret AWS_ACCESS_KEY_ID \
  --secret AWS_SECRET_ACCESS_KEY

# Agent with Slack webhook
kubiya agent create --name "Alert Agent" \
  --webhook-method slack \
  --webhook-dest "#alerts" \
  --webhook-prompt "Process this alert"
```

### kubiya agent list

List all serverless agents.

```bash
kubiya agent list [OPTIONS]
```

**Options:**
- `--all, -a`: Show all details
- `--active`: Show only active agents
- `--sort`: Sort by field (name, created, updated)
- `--filter`: Filter by name or description
- `--limit`: Limit number of results
- `--offset`: Offset for pagination

**Examples:**
```bash
# List all agents
kubiya agent list

# Show detailed information
kubiya agent list --all

# Filter active agents
kubiya agent list --active

# Sort by name
kubiya agent list --sort name

# Filter by keyword
kubiya agent list --filter kubernetes
```

### kubiya agent edit

Edit an existing agent.

```bash
kubiya agent edit AGENT_UUID [OPTIONS]
```

**Options:**
- `--name, -n`: Update agent name
- `--desc, -d`: Update agent description
- `--interactive, -i`: Interactive edit mode
- `--add-source`: Add source UUID
- `--remove-source`: Remove source UUID
- `--add-env`: Add environment variable
- `--remove-env`: Remove environment variable
- `--add-secret`: Add secret
- `--remove-secret`: Remove secret
- `--webhook-method`: Update webhook method
- `--webhook-dest`: Update webhook destination
- `--webhook-prompt`: Update webhook prompt

**Examples:**
```bash
# Interactive edit
kubiya agent edit abc-123 --interactive

# Update name and description
kubiya agent edit abc-123 --name "Updated Agent" --desc "New description"

# Add environment variable
kubiya agent edit abc-123 --add-env "LOG_LEVEL=debug"

# Configure webhook
kubiya agent edit abc-123 \
  --webhook-method slack \
  --webhook-dest "#notifications"
```

### kubiya agent delete

Delete an agent.

```bash
kubiya agent delete AGENT_UUID [OPTIONS]
```

**Options:**
- `--force, -f`: Force deletion without confirmation

**Examples:**
```bash
# Delete with confirmation
kubiya agent delete abc-123

# Force delete
kubiya agent delete abc-123 --force
```

## Workflow Management

### kubiya workflow execute

Execute a workflow from various sources.

```bash
kubiya workflow execute SOURCE [OPTIONS]
```

**Options:**
- `--var`: Workflow variables in key=value format (can be repeated)
- `--runner`: Runner to use for execution
- `--timeout`: Execution timeout
- `--save-trace`: Save execution trace
- `--skip-policy-check`: Skip policy validation
- `--verbose`: Verbose output
- `--dry-run`: Validate without executing

**Sources:**
- Local file: `workflow.yaml`
- GitHub repo: `org/repo` or `org/repo/path/to/workflow.yaml`
- GitHub URL: `https://github.com/org/repo/blob/main/workflow.yaml`
- Raw URL: `https://raw.githubusercontent.com/org/repo/main/workflow.yaml`

**Examples:**
```bash
# Execute local workflow
kubiya workflow execute deploy.yaml

# Execute from GitHub
kubiya workflow execute myorg/deploy-scripts

# Execute specific file in repo
kubiya workflow execute myorg/workflows/production/deploy.yaml

# Execute with variables
kubiya workflow execute deploy.yaml \
  --var environment=staging \
  --var version=v1.2.3

# Execute with custom runner
kubiya workflow execute deploy.yaml --runner k8s-runner

# Dry run validation
kubiya workflow execute deploy.yaml --dry-run
```

### kubiya workflow describe

Describe a workflow's structure and parameters.

```bash
kubiya workflow describe SOURCE [OPTIONS]
```

**Examples:**
```bash
# Describe local workflow
kubiya workflow describe deploy.yaml

# Describe GitHub workflow
kubiya workflow describe myorg/deploy-scripts/main.yaml
```

### kubiya workflow logs

View workflow execution logs.

```bash
kubiya workflow logs [OPTIONS]
```

**Options:**
- `--execution-id`: Specific execution ID
- `--workflow`: Workflow name filter
- `--limit`: Number of log entries
- `--follow, -f`: Follow log output
- `--since`: Show logs since timestamp

**Examples:**
```bash
# View recent logs
kubiya workflow logs

# View specific execution
kubiya workflow logs --execution-id exec-123

# Follow logs
kubiya workflow logs --follow

# Filter by workflow
kubiya workflow logs --workflow deploy-production
```

## Tool Management

### kubiya tool list

List available tools.

```bash
kubiya tool list [OPTIONS]
```

**Options:**
- `--source`: Filter by source UUID
- `--source-uuid`: Filter by source UUID
- `--type`: Filter by tool type
- `--all, -a`: Show all details
- `--limit`: Limit results
- `--offset`: Offset for pagination

**Examples:**
```bash
# List all tools
kubiya tool list

# Filter by source
kubiya tool list --source abc-123

# Filter by type
kubiya tool list --type docker

# Show detailed information
kubiya tool list --all
```

### kubiya tool exec

Execute a tool.

```bash
kubiya tool exec [OPTIONS]
```

**Options:**
- `--name, -n`: Tool name (required)
- `--content, -c`: Tool content/command
- `--type`: Tool type (command, docker, etc.)
- `--image`: Docker image (for docker type)
- `--runner`: Runner to use
- `--timeout`: Execution timeout
- `--env`: Environment variables (can be repeated)
- `--with-file`: File mappings (can be repeated)
- `--with-volume`: Volume mappings (can be repeated)
- `--source-uuid`: Execute tool from specific source
- `--arg`: Tool arguments (can be repeated)
- `--integration`: Integration to use

**Examples:**
```bash
# Execute simple command
kubiya tool exec --name "hello" --content "echo Hello World"

# Execute with Docker
kubiya tool exec --name "python-script" \
  --type docker \
  --image python:3.11 \
  --content "print('Hello from Python')"

# Execute with timeout
kubiya tool exec --name "long-task" \
  --content "sleep 30 && echo done" \
  --timeout 60s

# Execute with file mapping
kubiya tool exec --name "config-reader" \
  --content "cat /app/config.yaml" \
  --with-file ~/.config/app.yaml:/app/config.yaml

# Execute with integration
kubiya tool exec --name "k8s-pods" \
  --content "kubectl get pods" \
  --integration kubernetes/incluster
```

## Source Management

### kubiya source list

List all sources.

```bash
kubiya source list [OPTIONS]
```

**Options:**
- `--all, -a`: Show all details
- `--sort`: Sort by field (name, created, updated)
- `--filter`: Filter by name or description
- `--limit`: Limit results
- `--offset`: Offset for pagination

**Examples:**
```bash
# List all sources
kubiya source list

# Show detailed information
kubiya source list --all

# Filter by name
kubiya source list --filter kubernetes

# Sort by creation date
kubiya source list --sort created
```

### kubiya source scan

Scan for tools in a directory or repository.

```bash
kubiya source scan PATH [OPTIONS]
```

**Options:**
- `--runner`: Runner to use for scanning
- `--branch`: Git branch to scan
- `--path`: Specific path within repository
- `--force`: Force scan with uncommitted changes
- `--output`: Output format (table, json, yaml)

**Examples:**
```bash
# Scan current directory
kubiya source scan .

# Scan GitHub repository
kubiya source scan https://github.com/org/repo

# Scan specific branch
kubiya source scan https://github.com/org/repo --branch develop

# Scan with specific runner
kubiya source scan . --runner python-runner
```

### kubiya source add

Add a new source.

```bash
kubiya source add SOURCE [OPTIONS]
```

**Options:**
- `--name, -n`: Source name
- `--config`: Configuration file
- `--runner`: Default runner
- `--inline`: Add inline source from file
- `--add`: Add files to git
- `--push`: Push to remote
- `--commit-msg`: Commit message

**Examples:**
```bash
# Add from GitHub
kubiya source add https://github.com/org/tools --name "DevOps Tools"

# Add with configuration
kubiya source add https://github.com/org/tools \
  --config config.json \
  --runner python-runner

# Add inline source
kubiya source add --inline tools.yaml --name "Custom Tools"

# Add with git operations
kubiya source add . --add --push --commit-msg "Add monitoring tools"
```

## Chat Interface

### kubiya chat

Chat with agents.

```bash
kubiya chat [OPTIONS]
```

**Options:**
- `--interactive, -i`: Interactive chat mode
- `--name, -n`: Agent name
- `--agent-uuid, -t`: Agent UUID
- `--message, -m`: Message to send
- `--context`: Context files or URLs (can be repeated)
- `--stdin`: Read message from stdin
- `--inline`: Create temporary inline agent
- `--tools-file`: Tools file for inline agent
- `--ai-instructions`: AI instructions for inline agent
- `--description`: Description for inline agent
- `--runners`: Runners for inline agent
- `--env-vars`: Environment variables for inline agent
- `--secrets`: Secrets for inline agent
- `--integrations`: Integrations for inline agent
- `--llm-model`: LLM model for inline agent

**Examples:**
```bash
# Interactive chat
kubiya chat --interactive

# Chat with specific agent
kubiya chat --name "DevOps Agent" --message "Check system status"

# Chat with context files
kubiya chat --name "Security Agent" \
  --message "Review this deployment" \
  --context k8s/deployment.yaml \
  --context Dockerfile

# Chat with URL context
kubiya chat --name "Config Agent" \
  --message "Analyze this configuration" \
  --context https://raw.githubusercontent.com/org/repo/main/config.yaml

# Inline agent creation
kubiya chat --inline \
  --tools-file devops-tools.json \
  --ai-instructions "You are a DevOps automation assistant" \
  --description "Temporary DevOps Agent" \
  --message "Deploy the application"
```

## Secret Management

### kubiya secret create

Create a new secret.

```bash
kubiya secret create NAME VALUE [OPTIONS]
```

**Options:**
- `--description, -d`: Secret description
- `--expires-in`: Expiration time (e.g., 30d, 1y)

**Examples:**
```bash
# Create basic secret
kubiya secret create DB_PASSWORD "secure-password-123"

# Create with description and expiration
kubiya secret create API_TOKEN "token-xyz" \
  --description "API access token" \
  --expires-in 30d
```

### kubiya secret list

List all secrets.

```bash
kubiya secret list [OPTIONS]
```

**Options:**
- `--all, -a`: Show all details (values are still hidden)
- `--filter`: Filter by name or description

**Examples:**
```bash
# List all secrets
kubiya secret list

# Show detailed information
kubiya secret list --all

# Filter by name
kubiya secret list --filter database
```

### kubiya secret update

Update a secret value.

```bash
kubiya secret update NAME VALUE [OPTIONS]
```

**Examples:**
```bash
# Update secret value
kubiya secret update DB_PASSWORD "new-secure-password-456"
```

### kubiya secret delete

Delete a secret.

```bash
kubiya secret delete NAME [OPTIONS]
```

**Options:**
- `--force, -f`: Force deletion without confirmation

**Examples:**
```bash
# Delete with confirmation
kubiya secret delete TEMP_TOKEN

# Force delete
kubiya secret delete TEMP_TOKEN --force
```

## Runner Management

### kubiya runner list

List all runners.

```bash
kubiya runner list [OPTIONS]
```

**Options:**
- `--all, -a`: Show all details
- `--healthy`: Show only healthy runners
- `--sort`: Sort by field
- `--filter`: Filter by name

**Examples:**
```bash
# List all runners
kubiya runner list

# Show only healthy runners
kubiya runner list --healthy

# Show detailed information
kubiya runner list --all
```

### kubiya runner describe

Describe a specific runner.

```bash
kubiya runner describe RUNNER_NAME [OPTIONS]
```

**Examples:**
```bash
# Describe runner
kubiya runner describe my-k8s-runner
```

### kubiya runner manifest

Generate deployment manifest for a runner.

```bash
kubiya runner manifest RUNNER_NAME [OPTIONS]
```

**Options:**
- `--format`: Output format (yaml, json)
- `--type`: Deployment type (kubernetes, docker)

**Examples:**
```bash
# Generate Kubernetes manifest
kubiya runner manifest my-runner > runner-manifest.yaml

# Generate Helm chart
kubiya runner helm-chart my-runner > runner-chart.yaml
```

## Webhook Management

### kubiya webhook create

Create a new webhook.

```bash
kubiya webhook create [OPTIONS]
```

**Options:**
- `--type`: Webhook type (slack, teams, http)
- `--destination`: Webhook destination
- `--name`: Webhook name
- `--prompt`: Webhook prompt message

**Examples:**
```bash
# Create Slack webhook
kubiya webhook create --type slack \
  --destination "#alerts" \
  --name "Alert Handler" \
  --prompt "Process this alert"

# Create HTTP webhook
kubiya webhook create --type http \
  --destination "https://api.example.com/webhook" \
  --name "API Integration"
```

### kubiya webhook list

List all webhooks.

```bash
kubiya webhook list [OPTIONS]
```

**Options:**
- `--all, -a`: Show all details
- `--type`: Filter by webhook type

**Examples:**
```bash
# List all webhooks
kubiya webhook list

# Filter by type
kubiya webhook list --type slack
```

### kubiya webhook delete

Delete a webhook.

```bash
kubiya webhook delete WEBHOOK_UUID [OPTIONS]
```

**Options:**
- `--force, -f`: Force deletion without confirmation

## MCP Integration

### kubiya mcp setup

Configure MCP integration.

```bash
kubiya mcp setup [OPTIONS]
```

**Options:**
- `--force`: Force reinstall
- `--version`: Install specific version

**Examples:**
```bash
# Interactive installation
kubiya mcp setup

```

### kubiya mcp serve

Start MCP server.

```bash
kubiya mcp serve [OPTIONS]
```

**Options:**
- `--config`: Configuration file path
- `--port`: Server port
- `--verbose`: Verbose logging

**Examples:**
```bash
# Start with default config
kubiya mcp serve

# Start with custom config
kubiya mcp serve --config ~/.kubiya/mcp-config.json

# Start with verbose logging
kubiya mcp serve --verbose
```

## Integration Management

### kubiya integration list

List available integrations.

```bash
kubiya integration list [OPTIONS]
```

**Options:**
- `--type`: Filter by integration type
- `--status`: Filter by status

**Examples:**
```bash
# List all integrations
kubiya integration list

# Filter by type
kubiya integration list --type github
```

## Utility Commands

### kubiya completion

Generate shell completion scripts.

```bash
kubiya completion SHELL
```

**Supported shells:**
- `bash`
- `zsh`
- `fish`
- `powershell`

**Examples:**
```bash
# Generate bash completion
kubiya completion bash

# Install bash completion
echo 'source <(kubiya completion bash)' >> ~/.bashrc
```

### kubiya version

Show version information.

```bash
kubiya version [OPTIONS]
```

**Options:**
- `--short`: Show short version only
- `--build-info`: Show build information

**Examples:**
```bash
# Show version
kubiya version

# Show short version
kubiya version --short

# Show build info
kubiya version --build-info
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KUBIYA_API_KEY` | API key for authentication | Required |
| `KUBIYA_BASE_URL` | Base URL for API | `https://api.kubiya.ai/api/v1` |
| `KUBIYA_DEBUG` | Enable debug logging | `false` |
| `KUBIYA_DEFAULT_RUNNER` | Default runner name | None |
| `KUBIYA_TIMEOUT` | Default timeout | `300s` |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Authentication error |
| 4 | Not found |
| 5 | Permission denied |
| 6 | Timeout |
| 7 | Network error |