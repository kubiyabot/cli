# Kubiya CLI - Your DevOps Automation Companion 🤖

A command-line interface for managing Kubiya sources, teammates, and tools on the Kubiya platform.

## Features ✨

- **Source Management** 📂
  - Scan repositories for tools and capabilities
  - Add and sync sources from Git repositories
  - List and manage your sources
  - Support for local directory scanning
  - Interactive source browsing

- **Teammate Management** 👥
  - List and manage AI teammates
  - View teammate configurations
  - Manage teammate environment variables

- **Tool Management** 🛠️
  - List available tools
  - Execute tools with arguments
  - Interactive tool execution

- **Secret Management** 🔒
  - Create and manage secrets
  - Update secret values
  - List available secrets

- **Runner Management** 🚀
  - List available runners
  - View runner configurations

- **Webhook Management** 🔗
  - List and manage webhooks
  - View webhook configurations

- **Interactive Mode** 💻
  - TUI-based interface for source browsing
  - Interactive tool execution
  - Real-time updates and feedback

## Installation 📥

### Prerequisites

- Go 1.22 or higher
- [Kubiya API Key](https://docs.kubiya.ai/docs/org-management/api-keys)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/kubiyabot/cli.git
cd cli

# Build
make build

# Install locally
make install
```

## APT Installation (Debian/Ubuntu)

To install Kubiya CLI using APT:

```bash
# Add Kubiya's APT repository
curl -fsSL https://cli.kubiya.ai/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/kubiya-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/kubiya-archive-keyring.gpg] https://cli.kubiya.ai/apt stable main" | sudo tee /etc/apt/sources.list.d/kubiya.list

# Update package list and install Kubiya CLI
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

## Usage 🚀

The Kubiya CLI can be run from any directory - it uses your API key for authentication and doesn't require any specific working directory.

### Interactive Chat

Chat with your AI teammates directly from the terminal:

```bash
# Start an interactive chat session
kubiya chat -i

# Example output:
🤖 Connected to DevOps Assistant
Type your message or /help for commands...

You: How do I deploy to staging?
Assistant: Let me help you with the staging deployment...
```

Chat with specific context:
```bash
# Chat about specific files
kubiya chat -m "Review this deployment" --context k8s/deployment.yaml

# Example output:
🔍 Analyzing k8s/deployment.yaml...
💡 I notice a few things in your deployment:
1. Resource limits are not set
2. Security context is missing
...
```

### Source Management

List your sources with detailed information:
```bash
kubiya source list

# Example output:
📦 SOURCES
UUID                                    NAME              TOOLS  STATUS
f7d8e9c3-4b2a-4f1e-8d9c-1a2b3c4d5e6f  jenkins-tools     12     ✅
a1b2c3d4-5e6f-7g8h-9i0j-k1l2m3n4o5p6  kubernetes-tools  8      ✅
```

Scan a repository with detailed output:
```bash
kubiya source scan https://github.com/org/repo

# Example output:
🔍 Scanning Source: https://github.com/org/repo

✅ Scan completed
URL: https://github.com/org/repo
Name: deployment-tools

📦 Found 3 tools

Available Tools:
• deploy-staging
  Deploys application to staging environment
  Arguments: 2 required, 1 optional

• update-config
  Updates application configuration
  Arguments: 1 required, 2 optional

• rollback
  Rolls back deployment to previous version
  Arguments: 1 required
```

### Teammate Management

View teammate details with their capabilities:
```bash
kubiya teammate get "DevOps Bot"

# Example output:
👤 TEAMMATE DETAILS
Name: DevOps Bot
Description: Specialized in DevOps automation
Sources: 
  • jenkins-tools
  • kubernetes-tools
Environment Variables: 3 configured
Secrets: 2 configured
```

### Tool Execution

Execute tools with arguments:
```bash
kubiya tool execute deploy-staging --app myapp --env staging

# Example output:
🚀 Executing: deploy-staging
Parameters:
  • app: myapp
  • env: staging

📋 Deployment Steps:
1. Validating configuration...
2. Building container...
3. Pushing to registry...
4. Updating deployment...

✅ Deployment successful!
```

### Interactive Tool Browser

The interactive tool browser provides a TUI for exploring and executing tools:
```bash
kubiya tool -i

# Opens an interactive interface:
┌─ Available Tools ───────────────┐
│ > deploy-staging               │
│   update-config               │
│   rollback                    │
│                              │
│ [↑↓] Navigate [Enter] Select │
└──────────────────────────────┘
```

### Secret Management

Create and manage secrets securely:
```bash
# Create a new secret
kubiya secret create DB_PASSWORD "mypassword" --description "Database password"

# Example output:
🔒 Creating secret: DB_PASSWORD
✅ Secret created successfully

# List secrets
kubiya secret list

# Example output:
🔑 SECRETS
NAME         CREATED BY  CREATED AT
DB_PASSWORD  john.doe    2024-03-15 10:30:00
API_KEY      jane.doe    2024-03-14 15:45:00
```

### Working with Local Repositories

The CLI can detect Git information from your local directory:
```bash
# In your project directory
cd my-project
kubiya source scan .

# Example output:
📂 Local Directory Scan
Found repository: https://github.com/org/my-project
Branch: feature/new-tools

🔍 Scanning Source...
```

### CI/CD Integration Examples

#### GitHub Actions
```yaml
name: Kubiya Source Sync

on:
  push:
    branches: [ main ]
    paths:
      - 'tools/**'
      - '.kubiya/**'

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Build Kubiya CLI
        run: |
          git clone https://github.com/kubiyabot/cli.git
          cd cli
          make build
          sudo make install

      - name: Configure Kubiya CLI
        env:
          KUBIYA_API_KEY: ${{ secrets.KUBIYA_API_KEY }}
        run: |
          kubiya source list  # Verify connection
          SOURCE_ID=$(kubiya source list --output json | jq -r '.[] | select(.url | contains("${{ github.repository }}")) | .uuid')
          kubiya source sync $SOURCE_ID --mode ci --auto-commit
```

#### GitLab CI
```yaml
kubiya-sync:
  script:
    - |
      export KUBIYA_API_KEY=${KUBIYA_API_KEY}
      kubiya source scan .
      kubiya source sync ${SOURCE_ID} --mode ci
```

### Advanced Usage

#### JSON Output for Scripting
```bash
# Get source information in JSON format
kubiya source list --output json | jq '.[] | select(.name=="jenkins-tools")'

# Example output:
{
  "uuid": "f7d8e9c3-4b2a-4f1e-8d9c-1a2b3c4d5e6f",
  "name": "jenkins-tools",
  "url": "https://github.com/org/jenkins-tools",
  "tools": [
    {
      "name": "deploy-staging",
      "description": "Deploys application to staging"
    }
  ]
}
```

## Command Reference 📖

### Global Flags
- `--debug`: Enable debug output
- `--output`: Output format (text|json)

### Source Commands
```bash
# List sources
kubiya source list [--output json]

# Scan source
kubiya source scan [url|path] [--local] [--config file.json]

# Add source
kubiya source add [url] [--name "Name"] [--config file.json]

# Sync source
kubiya source sync [uuid] [--mode ci] [--branch main] [--force]

# Describe source
kubiya source describe [uuid] [--output json]

# Delete source
kubiya source delete [uuid] [--force]
```

### Teammate Commands
```bash
# List teammates
kubiya teammate list [--output json]

# Get teammate details
kubiya teammate get [uuid|name]

# Get teammate environment variable
kubiya teammate env get [teammate] [variable]
```

### Tool Commands
```bash
# List tools
kubiya tool list [--output json]

# Execute tool
kubiya tool execute [name] [args...]

# Interactive tool execution
kubiya tool execute -i
```

### Secret Commands
```bash
# List secrets
kubiya secret list

# Get secret value
kubiya secret get [name]

# Create secret
kubiya secret create [name] [value] [--description "desc"]

# Update secret
kubiya secret update [name] [value] [--description "desc"]
```

### Runner Commands
```bash
# List runners
kubiya runner list [--output json]

# Get runner details
kubiya runner get [uuid]
```

### Webhook Commands
```bash
# List webhooks
kubiya webhook list [--output json]

# Get webhook details
kubiya webhook get [id]
```

### Interactive Mode
```bash
# Interactive source browser
kubiya source browse

# Interactive tool execution
kubiya tool -i
```

## Development 👩‍💻

### Project Structure
```
.
├── internal/        # Internal packages
│   ├── cli/         # CLI implementation
│   ├── config/      # Configuration handling
│   ├── kubiya/      # API client
│   ├── style/       # Terminal styling
│   └── tui/         # Terminal UI components
└── main.go         # Entry point
```

## Support 💬

- **Documentation**: [docs.kubiya.ai](https://docs.kubiya.ai)
- **Issues**: [GitHub Issues](https://github.com/kubiyabot/cli/issues)

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

Built with ❤️ by the Kubiya team
