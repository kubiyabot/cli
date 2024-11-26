# Kubiya CLI - Your DevOps Automation Companion 🤖

[![Go Report Card](https://goreportcard.com/badge/github.com/kubiyabot/cli)](https://goreportcard.com/report/github.com/kubiyabot/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Welcome to **Kubiya CLI**! 👋 A powerful command-line interface for interacting with your **Kubiya AI teammates** and automating your workflows directly from the terminal.

## Features ✨

- **Interactive Chat** 💬
  - Chat with AI teammates directly from your terminal
  - Real-time responses with typing indicators
  - Session management for ongoing conversations
  - Stream assistant's responses as they are generated
  - Context-aware conversations with file support
  - Stdin support for piping data

- **Source Management** 📂
  - Add, list, and sync sources from repositories
  - Manage your code and resources efficiently

- **Tool Execution** 🛠️
  - Browse and execute tools interactively
  - Execute specific tools with arguments

- **Knowledge Management** 📖
  - Create and list knowledge items
  - Manage your team's knowledge base

- **Runner Management** 🚀
  - List runners and manage runner manifests
  - Streamline your automation processes

## Installation 📥

### Prerequisites

- **Go 1.18** or higher
- **Kubiya account and API key**

### Quick Install

Install using `go install`:

```bash
go install github.com/kubiyabot/cli/cmd/kubiya@latest
```

Or build from source:

```bash
git clone https://github.com/kubiyabot/cli.git
cd cli
make install
```

### Install Script

Install the latest version with our installation script:

```bash
# Using curl
curl -sSL https://raw.githubusercontent.com/kubiyabot/cli/main/install.sh | bash

# Using wget
wget -qO- https://raw.githubusercontent.com/kubiyabot/cli/main/install.sh | bash
```

This script will:

- Detect your OS and architecture
- Download the appropriate binary
- Install it to `/usr/local/bin` (or `%USERPROFILE%/bin` on Windows)
- Make it executable
- Verify the installation

### Verify Installation

```bash
kubiya version
```

### Binary Downloads

Download pre-compiled binaries for your platform from the [GitHub Releases](https://github.com/kubiyabot/cli/releases/latest) page.

```bash
# Linux (x86_64)
curl -LO https://github.com/kubiyabot/cli/releases/latest/download/kubiya_Linux_x86_64.tar.gz
tar xzf kubiya_Linux_x86_64.tar.gz

# macOS (Apple Silicon)
curl -LO https://github.com/kubiyabot/cli/releases/latest/download/kubiya_Darwin_arm64.tar.gz
tar xzf kubiya_Darwin_arm64.tar.gz

# Windows (x86_64)
# Download kubiya_Windows_x86_64.zip from the releases page
```

## Configuration 🔧

You can configure Kubiya CLI using environment variables or a configuration file.

### Environment Variables

Set the following environment variables:

```bash
# Required
export KUBIYA_API_KEY="your-api-key"

# Optional
export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"  # Default API URL
export KUBIYA_DEBUG=true                               # Enable debug mode
```

### Configuration File

Create a configuration file at `~/.kubiya/config.yaml`:

```yaml
api_key: your-api-key
base_url: https://api.kubiya.ai/api/v1
debug: false
```

## Getting Started 🚀

### Interactive Chat

Start an interactive chat session with full TUI support:

```bash
# Basic interactive mode
kubiya chat -i

# Interactive mode with a specific teammate
kubiya chat -i -n "DevOps Bot"
```

### Non-Interactive Chat

Send messages directly from the command line:

```bash
# Simple message
kubiya chat -n "DevOps Bot" -m "How do I deploy to staging?"

# Stream the response in real-time
kubiya chat -n "DevOps Bot" -m "Explain the deployment process" --stream

# Continue a previous conversation
kubiya chat -n "DevOps Bot" -m "What about production?" --session-id abc-123
```

### Context-Aware Conversations

Include files, URLs, and patterns for context:

```bash
# Single file context
kubiya chat -n "code-review" -m "Review this code" --context main.go

# Multiple context files
kubiya chat -n "security" \
  --context "Dockerfile" \
  --context "k8s/*.yaml" \
  --context "src/main.go"
```

### Using Stdin

Use stdin for input and process outputs:

```bash
# Analyze logs
tail -f app.log | kubiya chat -n "debug" --stdin

# Review error output
kubectl logs pod-name | kubiya chat -n "DevOps Bot" --stdin

# Save conversation to file
kubiya chat -n "DevOps Bot" -m "Document our API" > api-docs.md
```

### Session Management

Maintain conversation context across interactions:

```bash
# Start a new session
kubiya chat -n "DevOps Bot" -m "Let's plan the deployment"

# Continue the conversation
kubiya chat -n "DevOps Bot" -m "What's the next step?"

# Explicitly use a session
kubiya chat -n "DevOps Bot" -m "Continue from before" -s "session-123"

# Clear saved session
kubiya chat --clear-session
```

### Advanced Usage

Combine multiple features for complex interactions:

```bash
# Code review with multiple files and custom message
kubiya chat -n "code-review" \
  --context "src/*.go,tests/*.go" \
  -m "Review these changes for security issues" \
  --stream

# Process logs with context
tail -100 error.log | kubiya chat -n "debug" \
  --stdin \
  --context "config.yaml,deployment.yaml" \
  -m "What's causing these errors?"

# Generate documentation
kubiya chat -n "docs" \
  --context "**/*.go" \
  -m "Generate API documentation" \
  --stream > docs/api.md
```

## Command Reference 📖

### Global Flags

- `--debug`: Enable debug output
- `--output`: Output format (`text`|`json`)
- `--help`: Show help for any command

### Main Commands

| Command     | Description              | Example                          |
|-------------|--------------------------|----------------------------------|
| `chat`      | Chat with teammates      | `kubiya chat -i`                 |
| `source`    | Manage sources           | `kubiya source list`             |
| `tool`      | Execute tools            | `kubiya tool execute deploy`     |
| `knowledge` | Manage knowledge base    | `kubiya knowledge list`          |
| `runner`    | Manage runners           | `kubiya runner list`             |
| `webhook`   | Manage webhooks          | `kubiya webhook list`            |

## Development 👩‍💻

### Building from Source

```bash
# Clone the repository
git clone https://github.com/kubiyabot/cli.git
cd cli

# Install dependencies
go mod download

# Run tests
make test

# Build the binary
make build
```

### Project Structure

```
.
├── cmd/             # Command-line interface
├── internal/        # Internal packages
│   ├── cli/         # CLI implementation
│   ├── config/      # Configuration handling
│   ├── kubiya/      # API client
│   └── tui/         # Terminal UI components
├── docs/            # Documentation
└── test/            # Test files
```

## Contributing 🤝

We welcome contributions! Please follow these steps:

1. **Fork** the repository
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** (`git commit -m 'Add amazing feature'`)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

## Support 💬

For any questions or concerns:

- **Documentation**: [docs.kubiya.ai](https://docs.kubiya.ai)
- **Issues**: [GitHub Issues](https://github.com/kubiyabot/cli/issues)
- **Email**: [support@kubiya.ai](mailto:support@kubiya.ai)

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments 🙏

- [Charm](https://charm.sh) for the amazing TUI libraries
- The Go community for inspiration and support
- All our contributors and users

---

Built with ❤️ by the Kubiya team
