# Kubiya CLI - Your Agentic Automation Companion 🤖

A powerful command-line interface for managing Kubiya sources, teammates, and tools. Automate your engineering workflows and interact with Kubiya AI Agents (Teammates) seamlessly.

## Features ✨

- **Source Management** 📂
  - Scan Git repositories and local directories for tools
  - Add and sync sources with version control
  - Interactive source browsing and management
  - Support for inline tools and dynamic configurations

- **Teammate Management** 👥
  - Create and manage AI teammates
  - Configure capabilities, tools, and permissions
  - Manage environment variables and secrets
  - Set up webhooks for automated interactions

- **Tool Management** 🛠️
  - Execute tools with arguments and flags
  - Interactive tool browser and executor
  - Real-time execution feedback
  - Support for long-running operations

- **Secret Management** 🔒
  - Securely store and manage secrets
  - Integrate with teammates and tools
  - Role-based access control

- **Runner Management** 🚀
  - Manage tool execution environments
  - Monitor runner health and status
  - Configure runner-specific settings

- **Webhook Management** 🔗
  - Create and manage webhooks
  - Support for Slack, Teams, and HTTP
  - Custom webhook configurations

- **MCP Integration** 💻↔️🤖 (Model Context Protocol)
  - Serve MCP server with whitelisted tools and configurations
  - Integrate Kubiya context (API key, Teammates) with local AI tools like **Claude Desktop** and **Cursor IDE**
  - Install and manage a local **MCP Gateway** server that acts as a bridge
  - Automatically configure supported applications during installation
  - List, apply, and edit provider configurations
  - Support for tool execution with streaming output via MCP protocol

## MCP Integration (Model Context Protocol)

The Kubiya CLI provides comprehensive MCP (Model Context Protocol) integration that enables AI-powered applications like Claude Desktop and Cursor IDE to execute Kubiya tools directly within their chat interfaces.

### MCP Server Features

The Kubiya CLI includes a built-in MCP server that provides:

- **Tool Execution**: Execute any Kubiya tool with streaming output via MCP protocol
- **Whitelisted Tools**: Configure specific tools that can be executed through MCP
- **Runner Support**: Execute tools on different runners (local, cloud, custom)
- **Platform APIs**: Optional access to Kubiya platform APIs (list runners, sources, etc.)
- **Integration Templates**: Pre-configured setups for common platforms (AWS, Kubernetes, etc.)
- **Security Controls**: OPA policy enforcement, tool whitelisting, and access controls

### MCP Server vs MCP Gateway

The Kubiya CLI provides two MCP integration approaches:

1. **MCP Server (`kubiya mcp serve`)**: Direct MCP server that serves whitelisted tools
2. **MCP Gateway (`kubiya mcp install`)**: Proxy gateway that provides access to full Kubiya API

Choose MCP Server for:
- Controlled tool execution with specific whitelisted tools
- Production environments with security requirements
- Scenarios where you want to limit available functionality

Choose MCP Gateway for:
- Full access to Kubiya API and all teammates
- Development and exploration
- When you want complete Kubiya functionality

### Auto-Download Executors

For seamless integration with AI tools, we provide **MCP executors** that automatically download and update the Kubiya CLI:

- **`mcp-kubiya-executor`** - Bash script for Unix-like systems (Linux, macOS)
- **`mcp-kubiya-executor.py`** - Python script for cross-platform support

These executors:
- ✅ Automatically download the latest Kubiya CLI if not found
- ✅ Support version pinning via `KUBIYA_CLI_VERSION` environment variable
- ✅ Handle platform-specific binaries (Linux, macOS, Windows)
- ✅ Pass through all MCP server arguments

#### Quick Setup with Executors

**For Claude Desktop:**
```json
{
  "mcpServers": {
    "kubiya": {
      "command": "/path/to/mcp-kubiya-executor",
      "env": {
        "KUBIYA_API_KEY": "your-api-key"
      }
    }
  }
}
```

**For Windows users:**
```json
{
  "mcpServers": {
    "kubiya": {
      "command": "python",
      "args": ["C:\\path\\to\\mcp-kubiya-executor.py"],
      "env": {
        "KUBIYA_API_KEY": "your-api-key"
      }
    }
  }
}
```

See the [MCP Executor Guide](docs/mcp-executor-guide.md) for detailed setup instructions.

### Quick Start

Getting started is designed to be simple, it only has three steps:
1. chose which teammates are to be used by the mcp
2. run mcp setup
3. add the kubiya mcp-server to your favorite mcp client (Cursor od Claude Desktop)

```bash
kubiya teammate list # to list the existing teammates from which your want to pick
export TEAMMATE_UUIDS=...  # a comma-separated list of teammate uuids
kubiya mcp setup # to show the command£
```

# Clone the repository
git clone https://github.com/kubiyabot/cli.git
cd cli

# Build
make build

# Install locally
make install
```

### APT Installation (Debian/Ubuntu)

```bash
# Add Kubiya's APT repository
curl -fsSL https://cli.kubiya.ai/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/kubiya-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/kubiya-archive-keyring.gpg] https://cli.kubiya.ai/apt stable main" | sudo tee /etc/apt/sources.list.d/kubiya.list

# Update and install
sudo apt update
sudo apt install kubiya-cli
```

## Configuration 🔧

### Using Environment Variables

```bash
# Required
export KUBIYA_API_KEY="your-api-key"

# Optional
export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"  # Default API URL
export KUBIYA_DEBUG=true                               # Enable debug mode
```

## Usage Examples 🚀

### Source Management

#### List Sources
```bash
# Basic listing
kubiya source list

# Detailed view with all information
kubiya source list --all

# Filter sources
kubiya source list --filter "kubernetes"

# Sort by name or creation date
kubiya source list --sort name
kubiya source list --sort created
```

#### Scan Sources
```bash
# Scan GitHub repository
kubiya source scan https://github.com/org/repo

# Scan local directory
kubiya source scan .

# Scan with specific runner
kubiya source scan . --runner python

# Scan with branch and path
kubiya source scan https://github.com/org/repo --branch main --path /tools

# Force scan with uncommitted changes
kubiya source scan . --force
```

#### Add Sources
```bash
# Add from repository
kubiya source add https://github.com/org/repo --name "DevOps Tools"

# Add with configuration
kubiya source add https://github.com/org/repo --config config.json --runner python

# Add inline source from file
kubiya source add --inline tools.yaml --name "Custom Tools"

# Add with auto-commit and push
kubiya source add . --add --push --commit-msg "feat: add new tools"
```

### Teammate Management

#### Create Teammates
```bash
# Create interactively
kubiya teammate create --interactive

# Create with basic info
kubiya teammate create --name "DevOps Bot" --desc "Handles DevOps tasks"

# Create with sources and secrets
kubiya teammate create --name "AWS Bot" \
  --source abc-123 --source def-456 \
  --secret AWS_KEY --secret DB_PASSWORD

# Create with environment variables
kubiya teammate create --name "Deploy Bot" \
  --env "ENVIRONMENT=prod" --env "DEBUG=true"

# Create with webhooks
kubiya teammate create --name "Slack Bot" \
  --webhook-method slack --webhook-dest "#alerts" \
  --webhook-prompt "Please analyze this alert"

# Create with HTTP webhook
kubiya teammate create --name "API Bot" \
  --webhook-method http \
  --webhook-prompt "Process this request"
```

#### List Teammates
```bash
# Basic listing
kubiya teammate list

# Show all details
kubiya teammate list --all

# Show only active teammates
kubiya teammate list --active

# Filter teammates
kubiya teammate list --filter "kubernetes"

# Sort by various fields
kubiya teammate list --sort name
kubiya teammate list --sort updated
```

#### Edit Teammates
```bash
# Edit interactively
kubiya teammate edit abc-123 --interactive

# Update basic info
kubiya teammate edit abc-123 --name "New Name" --desc "Updated description"

# Add/remove sources
kubiya teammate edit abc-123 --add-source def-456 --remove-source ghi-789

# Update environment variables
kubiya teammate edit abc-123 --add-env "DEBUG=true" --remove-env LOG_LEVEL

# Add webhooks
kubiya teammate edit abc-123 \
  --webhook-method slack \
  --webhook-dest "#notifications" \
  --webhook-prompt "New alert received"
```

### Tool Management

#### List Tools
```bash
# List all tools
kubiya tool list

# Filter by source
kubiya tool list --source abc-123

# Show detailed info
kubiya tool list --all

# Filter by type
kubiya tool list --type python

# List tools from specific source
kubiya tool list --source-uuid "64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56"
```

#### Execute Tools
```bash
# Basic execution
kubiya tool exec --name "hello" --content "echo Hello World"

# Execute with Docker image
kubiya tool exec --name "python-script" --type docker --image python:3.11 \
  --content "print('Hello from Python')"

# Execute with integration
kubiya tool exec --name "k8s-pods" \
  --content "kubectl get pods -A" \
  --integration kubernetes/incluster

# Execute from source
kubiya tool exec --source-uuid "abc-123" --name "my-tool"

# Execute with JSON definition
kubiya tool exec --json '{
  "name": "test-tool",
  "type": "docker",
  "image": "alpine",
  "content": "echo Hello from JSON"
}'
```

### Secret Management

```bash
# Create secret
kubiya secret create DB_PASSWORD "mypassword" --description "Database password"

# Create with expiration
kubiya secret create API_KEY "secretkey" --expires-in 30d

# List secrets
kubiya secret list

# Update secret
kubiya secret update DB_PASSWORD "newpassword"

# Delete secret
kubiya secret delete DB_PASSWORD
```

### Webhook Management

```bash
# Create Slack webhook
kubiya webhook create --type slack --destination "#alerts" \
  --name "Alert Handler" --prompt "Process this alert"

# Create HTTP webhook
kubiya webhook create --type http \
  --name "API Endpoint" --prompt "Handle this request"

# List webhooks
kubiya webhook list

# Get webhook details
kubiya webhook get abc-123

# Delete webhook
kubiya webhook delete abc-123
```

### Agent Chat

#### Interactive Chat
```bash
# Start interactive chat session
kubiya chat --interactive

# Chat with context files
kubiya chat -m "Review this deployment" --context k8s/deployment.yaml --context src/**/*.go

# Chat with specific agent by name
kubiya chat -n "DevOps Bot" -m "Help me troubleshoot this error"

# Chat with agent by ID
kubiya chat -t "abc-123" -m "Show me the logs"

# Chat with stdin input
cat error.log | kubiya chat -n "Debug Assistant" --stdin
```

#### Inline Agent Chat
```bash
# Create and chat with inline agent using tools from file
kubiya chat --inline --tools-file tools.json \
  --ai-instructions "You are a helpful DevOps assistant" \
  --description "Custom DevOps Agent" \
  --runners "kubiya-prod" \
  -m "kubectl get pods"

# Inline agent with tools from JSON string
kubiya chat --inline --tools-json '[{
  "name": "echo-test",
  "description": "Simple echo tool",
  "content": "echo hello",
  "image": "alpine:latest"
}]' -m "Run echo test"

# Inline agent with environment variables and secrets
kubiya chat --inline --tools-file tools.json \
  --env-vars "ENV1=value1" --env-vars "ENV2=value2" \
  --secrets "SECRET1" --integrations "jira" \
  --llm-model "azure/gpt-4-32k" --debug-mode \
  -m "Use the configured tools"
```

#### Agent Chat with Context
```bash
# Chat with multiple context files using wildcards
kubiya chat -n "security" -m "Review this code" \
  --context "src/*.go" --context "tests/**/*_test.go"

# Chat with URLs as context
kubiya chat -n "security" -m "Check this configuration" \
  --context https://raw.githubusercontent.com/org/repo/main/config.yaml

# Multiple context sources
kubiya chat -n "devops" \
  --context "k8s/*.yaml" \
  --context "https://example.com/deployment.yaml" \
  --context "Dockerfile" \
  -m "Review deployment"
```

#### Session Management
```bash
# Resume previous session
kubiya chat --session "session-id-123" -m "Continue our conversation"

# Auto-resume last session
export KUBIYA_AUTO_SESSION=true
kubiya chat -m "What were we discussing?"

# Clear current session
kubiya chat --clear-session
```

### MCP Integration (Examples)

*These commands are also detailed in the dedicated MCP section above.*

```bash
# Install MCP gateway and configure defaults interactively
kubiya mcp install

# List configured application providers
kubiya mcp list

# Manually apply/re-apply configuration for Cursor
kubiya mcp apply cursor_ide 

# Update the MCP gateway code
kubiya mcp update

# Edit the Claude Desktop provider config
kubiya mcp edit claude_desktop

# Serve MCP server with custom configuration
kubiya mcp serve --config ~/.kubiya/mcp-config.json

# Test MCP server functionality
kubiya mcp test --config test/test-mcp-config.json
```

#### MCP Configuration Examples

Create a custom MCP configuration file (`~/.kubiya/mcp-config.json`):

```json
{
  "enable_runners": true,
  "allow_platform_apis": false,
  "enable_opa_policies": false,
  "allow_dynamic_tools": false,
  "verbose_logging": true,
  "whitelisted_tools": [
    {
      "name": "kubectl",
      "alias": "",
      "description": "Executes kubectl commands. For namespace-scoped resources, include '-n <namespace>' in the command.",
      "type": "docker",
      "content": "set -eu\nTOKEN_LOCATION=\"/tmp/kubernetes_context_token\"\nCERT_LOCATION=\"/tmp/kubernetes_context_cert\"\nif [ -f $TOKEN_LOCATION ] && [ -f $CERT_LOCATION ]; then\n    KUBE_TOKEN=$(cat $TOKEN_LOCATION)\n    kubectl config set-cluster in-cluster --server=https://kubernetes.default.svc --certificate-authority=$CERT_LOCATION > /dev/null 2>&1\n    kubectl config set-credentials in-cluster --token=$KUBE_TOKEN > /dev/null 2>&1\n    kubectl config set-context in-cluster --cluster=in-cluster --user=in-cluster > /dev/null 2>&1\n    kubectl config use-context in-cluster > /dev/null 2>&1\nfi\n\necho \"🔧 Executing: kubectl $command\"\n\nif eval \"kubectl $command\"; then\n    echo \"✅ Command executed successfully\"\nelse\n    echo \"❌ Command failed: kubectl $command\"\n    exit 1\nfi",
      "args": [
        {
          "name": "command",
          "type": "string",
          "description": "The full kubectl command to execute. Examples: 'get pods -n default', 'get nodes', 'describe service my-service'",
          "required": true
        }
      ],
      "env": null,
      "with_files": [
        {
          "source": "/var/run/secrets/kubernetes.io/serviceaccount/token",
          "destination": "/tmp/kubernetes_context_token"
        },
        {
          "source": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
          "destination": "/tmp/kubernetes_context_cert"
        }
      ],
      "with_volumes": null,
      "icon_url": "https://kubernetes.io/icons/icon-128x128.png",
      "image": "kubiya/kubectl-light:latest",
      "runner": "core-testing-2"
    }
  ]
}
```

#### MCP Client Setup

**Claude Desktop (`claude_desktop_config.json`):**
```json
{
  "mcpServers": {
    "kubiya": {
      "command": "/path/to/kubiya",
      "args": ["mcp", "serve", "--config", "/path/to/config.json"],
      "env": {
        "KUBIYA_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

**Cursor (`~/.cursor/mcp.json`):**
```json
{
  "mcp": {
    "servers": {
      "kubiya": {
        "command": "/path/to/kubiya",
        "args": ["mcp", "serve", "--config", "/path/to/config.json"],
        "env": {
          "KUBIYA_API_KEY": "your-api-key-here"
        }
      }
    }
  }
}
```

For detailed MCP setup and configuration, see the [MCP Comprehensive Guide](docs/mcp-comprehensive-guide.md).

### Tool Execution

#### Direct Tool Execution
```bash
# Execute a simple tool
kubiya tool exec --name "hello" --content "echo Hello World"

# Execute with Docker image
kubiya tool exec --name "python-script" --type docker --image python:3.11 \
  --content "print('Hello from Python')"

# Execute with timeout
kubiya tool exec --name "long-task" --content "sleep 90 && echo done" \
  --timeout 120

# Execute with custom runner
kubiya tool exec --name "test" --content "date" --runner "my-runner"
```

#### Tool Execution with Integrations
```bash
# Execute kubectl with in-cluster authentication
kubiya tool exec --name "k8s-pods" \
  --content "kubectl get pods -A" \
  --integration kubernetes/incluster

# Execute AWS CLI with profile
kubiya tool exec --name "s3-list" \
  --content "aws s3 ls" \
  --integration aws/cli \
  --env AWS_PROFILE=production

# Execute with multiple integrations
kubiya tool exec --name "deploy-app" \
  --content 'terraform apply -auto-approve && kubectl apply -f k8s/' \
  --integration terraform/aws \
  --integration kubernetes/eks \
  --env EKS_CLUSTER_NAME=prod-cluster
```

#### Tool Execution with Files and Volumes
```bash
# Execute with file mappings
kubiya tool exec --name "config-reader" \
  --content "cat /app/config.yaml" \
  --with-file ~/.config/myapp.yaml:/app/config.yaml

# Execute with volume mappings
kubiya tool exec --name "docker-inspect" \
  --content "docker ps -a" \
  --with-volume /var/run/docker.sock:/var/run/docker.sock:ro

# Execute with service dependencies
kubiya tool exec --name "db-query" \
  --content "psql -h postgres -U admin -c 'SELECT version();'" \
  --with-service postgres:14 \
  --env PGPASSWORD=secret
```

#### Tool Execution from Sources
```bash
# Execute tool from source UUID
kubiya tool exec --source-uuid "abc-123" --name "my-tool"

# Execute with custom arguments
kubiya tool exec --source-uuid "abc-123" --name "aws-tool" \
  --arg instance_ids:string:"i-1234567890abcdef0":false

# Load tool from URL
kubiya tool exec --tool-url https://raw.githubusercontent.com/org/repo/main/tool.json
```

#### Tool Execution with JSON
```bash
# Execute with JSON definition
kubiya tool exec --json '{
  "name": "test-tool",
  "type": "docker",
  "image": "alpine",
  "content": "echo Hello from JSON"
}'

# Execute from JSON file
kubiya tool exec --json-file my-tool.json
```

#### Tool Execution Output Formats
```bash
# Default formatted output
kubiya tool exec --name "test" --content "date"

# Raw JSON stream
kubiya tool exec --name "test" --content "date" --output stream-json

# Disable streaming (quiet mode)
kubiya tool exec --name "test" --content "date" --watch=false
```

### Workflow Commands

#### Create and Manage Workflows
```bash
# Create a new workflow
kubiya workflow create --name "deploy-pipeline" --description "Deployment pipeline"

# List workflows
kubiya workflow list

# Execute a workflow
kubiya workflow run --name "deploy-pipeline" --env "TARGET_ENV=production"

# Get workflow status
kubiya workflow status --id "workflow-123"

# Get workflow logs
kubiya workflow logs --id "workflow-123"
```

#### Workflow with Tools
```bash
# Create workflow with tool steps
kubiya workflow create --name "k8s-deployment" \
  --step "build:docker build -t myapp:latest ." \
  --step "deploy:kubectl apply -f k8s/deployment.yaml" \
  --step "verify:kubectl get pods -l app=myapp"

# Execute workflow with parameters
kubiya workflow run --name "k8s-deployment" \
  --param "image_tag=v1.2.3" \
  --param "namespace=production"
```

For more workflow examples, see the [Workflow Commands Guide](docs/workflow-commands.md).

## Tips and Tricks 💡

1. Use `--help` with any command to see detailed usage:
   ```bash
   kubiya source --help
   kubiya teammate create --help
   ```

2. Enable debug mode for verbose output:
   ```bash
   export KUBIYA_DEBUG=true
   kubiya source scan .
   ```

3. Use tab completion (bash/zsh):
   ```bash
   # For bash
   source <(kubiya completion bash)
   
   # For zsh
   source <(kubiya completion zsh)
   ```

4. Save common configurations in a config file:
   ```bash
   kubiya config init
   kubiya config set default_runner python
   ```

## Support 🤝

### Documentation
- **Main Documentation**: [https://docs.kubiya.ai](https://docs.kubiya.ai)
- **MCP Integration**: [MCP Comprehensive Guide](docs/mcp-comprehensive-guide.md)
- **MCP Executor**: [MCP Executor Guide](docs/mcp-executor-guide.md)
- **MCP Testing**: [MCP Server Testing Guide](docs/mcp-server-testing.md)
- **Tool Execution**: [Tool Execution Examples](docs/tool-exec-examples.md)
- **Workflow Commands**: [Workflow Commands Guide](docs/workflow-commands.md)

### Community & Support
- **Issues**: [GitHub Issues](https://github.com/kubiyabot/cli/issues)
- **Community**: [Join our Slack](https://join.slack.com/t/kubiya/shared_invite/zt-1234567890)
- **Examples**: [Community Examples](examples/README.md)

## Development 👩‍💻

### Project Structure
```
.
├── internal/        # Internal packages
│   ├── cli/         # CLI implementation
│   ├── config/      # Configuration handling
│   ├── kubiya/      # API client
│   ├── mcp/         # MCP integration logic & defaults
│   ├── style/       # Terminal styling
│   └── tui/         # Terminal UI components
└── main.go         # Entry point
```

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

Built with ❤️ by the Kubiya team
