# Kubiya CLI - Your DevOps Automation Companion 🤖

[![Go Report Card](https://goreportcard.com/badge/github.com/kubiyabot/cli)](https://goreportcard.com/report/github.com/kubiyabot/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Welcome to Kubiya CLI! 👋 A powerful command-line interface for interacting with your Kubiya AI teammates and automating your workflows directly from the terminal.

## Features ✨

- **Interactive Chat** 💬
  - Chat with AI teammates directly from your terminal.
  - Real-time responses with typing indicators.
  - Session management for ongoing conversations.
  - Stream assistant's responses as they are generated.

## Installation 📥

### Prerequisites

- **Go 1.18** or higher.
- **Kubiya account and API key**.

### Quick Install

```bash
# Using go install
go install github.com/kubiyabot/cli/cmd/kubiya@latest

# Or build from source
git clone https://github.com/kubiyabot/cli.git
cd cli
make install
```

### Verify Installation

```bash
kubiya version
```

### Binary Downloads

You can download pre-compiled binaries for your platform from the [GitHub Releases page](https://github.com/kubiyabot/cli/releases/latest).

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

### Environment Variables

```bash
# Required
export KUBIYA_API_KEY="your-api-key"

# Optional
export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"  # Default API URL
export KUBIYA_DEBUG=true                               # Enable debug mode
```

### Configuration File

Create `~/.kubiya/config.yaml`:

```yaml
api_key: your-api-key
base_url: https://api.kubiya.ai/api/v1
debug: false
```

## Quick Start 🚀

### Interactive Chat

```bash
# Start interactive chat
kubiya chat -i

# Chat with specific teammate
kubiya chat -n "DevOps Bot" -m "Deploy to staging"
```

### Source Management

```bash
# List sources
kubiya source list

# Add source from GitHub
kubiya source add --url https://github.com/org/repo

# Sync source
kubiya source sync abc-123
```

### Tool Execution

```bash
# Browse and execute tools interactively
kubiya browse

# Execute specific tool
kubiya tool execute deploy-app --arg key=value
```

### Knowledge Management

```bash
# List knowledge items
kubiya knowledge list

# Create knowledge item
kubiya knowledge create -n "AWS Setup" -f content.md
```

### Runner Management

```bash
# List runners
kubiya runner list

# Get runner manifest
kubiya runner manifest my-runner -o manifest.yaml
```

## Command Reference 📖

### Global Flags

- `--debug`: Enable debug output
- `--output`: Output format (text|json)
- `--help`: Show help for any command

### Main Commands

| Command | Description | Example |
|---------|-------------|---------|
| `chat` | Chat with teammates | `kubiya chat -i` |
| `source` | Manage sources | `kubiya source list` |
| `tool` | Execute tools | `kubiya tool execute deploy` |
| `knowledge` | Manage knowledge base | `kubiya knowledge list` |
| `runner` | Manage runners | `kubiya runner list` |
| `webhook` | Manage webhooks | `kubiya webhook list` |

## Development 👩‍💻

### Building from Source

```bash
# Clone repository
git clone https://github.com/kubiyabot/cli.git
cd cli

# Install dependencies
go mod download

# Run tests
make test

# Build binary
make build
```

### Project Structure

```
.
├── cmd/            # Command line interface
├── internal/       # Internal packages
│   ├── cli/       # CLI implementation
│   ├── config/    # Configuration handling
│   ├── kubiya/    # API client
│   └── tui/       # Terminal UI components
├── docs/          # Documentation
└── test/          # Test files
```

## Contributing 🤝

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support 💬

- Documentation: [docs.kubiya.ai](https://docs.kubiya.ai)
- Issues: [GitHub Issues](https://github.com/kubiyabot/cli/issues)
- Email: [support@kubiya.ai](mailto:support@kubiya.ai)
- Discord: [Join our community](https://discord.gg/kubiya)

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments 🙏

- [Charm](https://charm.sh) for the amazing TUI libraries
- The Go community for inspiration and support
- All our contributors and users

---

Built with ❤️ by the Kubiya team
