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
  - Integrate Kubiya context (API key, Teammates) with local AI tools like **Claude Desktop** and **Cursor IDE**.
  - Install and manage a local **MCP Gateway** server that acts as a bridge.
  - Automatically configure supported applications during installation.
  - List, apply, and edit provider configurations.

## MCP Integration (Model Context Protocol)

The Kubiya CLI can bridge your Kubiya environment (API Key, Teammate context) with local AI-powered applications that support the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/). This allows applications like Claude Desktop or Cursor IDE to access information about your Kubiya teammates directly within their chat interfaces.

This integration works by:

1.  **Installing a local MCP Gateway server:** A small Python server (`mcp-gateway`) is installed locally.
2.  **Configuring applications:** The CLI automatically updates the configuration files of supported applications (e.g., `~/.cursor/mcp.json` for Cursor) to point to this local gateway server.
3.  **Proxying requests:** The local gateway receives requests from the application, injects your Kubiya API key and selected teammate context, and forwards the requests to the actual Kubiya API.

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

#### Execute Tools
```bash
# Basic execution
kubiya tool execute deploy-app --app myapp --env staging

# Interactive execution
kubiya tool execute -i

# Execute with JSON input
kubiya tool execute update-config --input config.json

# Execute with environment variables
kubiya tool execute backup-db --env "BACKUP_PATH=/data"

# Long-running tool execution
kubiya tool execute monitor-logs --follow
```

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

### Interactive Chat

```bash
# Start chat session
kubiya chat -i

# Chat with context
kubiya chat -m "Review this deployment" --context k8s/deployment.yaml

# Chat with specific teammate
kubiya chat -i --teammate "DevOps Bot"

# Chat with file attachments
kubiya chat -i --attach "error.log" --attach "config.yaml"
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
```

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

- Documentation: [https://docs.kubiya.ai](https://docs.kubiya.ai)
- Issues: [GitHub Issues](https://github.com/kubiyabot/cli/issues)
- Community: [Join our Slack](https://join.slack.com/t/kubiya/shared_invite/zt-1234567890)

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
