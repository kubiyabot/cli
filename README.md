# Kubiya CLI - Your DevOps Automation Companion 🤖

Welcome to Kubiya CLI! 👋

Kubiya CLI is a powerful tool for interacting with your Kubiya teammates and managing your automation sources. With Kubiya CLI, you can chat with teammates, manage knowledge bases, handle sources, runners, and webhooks—all from your terminal.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Prerequisites](#prerequisites)
  - [Build from Source](#build-from-source)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Chat with Teammates](#chat-with-teammates)
  - [List Teammates](#list-teammates)
  - [Manage Sources](#manage-sources)
    - [List Sources](#list-sources)
    - [Describe a Source](#describe-a-source)
    - [Add a New Source](#add-a-new-source)
    - [Sync a Source](#sync-a-source)
    - [Batch Sync Sources](#batch-sync-sources)
    - [Delete a Source](#delete-a-source)
  - [Manage Knowledge Base](#manage-knowledge-base)
    - [List Knowledge Items](#list-knowledge-items)
    - [Get a Knowledge Item](#get-a-knowledge-item)
    - [Create a Knowledge Item](#create-a-knowledge-item)
    - [Update a Knowledge Item](#update-a-knowledge-item)
    - [Delete a Knowledge Item](#delete-a-knowledge-item)
  - [Manage Runners](#manage-runners)
    - [List Runners](#list-runners)
    - [Get Runner Manifest](#get-runner-manifest)
  - [Manage Webhooks](#manage-webhooks)
    - [List Webhooks](#list-webhooks)
    - [Get a Webhook](#get-a-webhook)
    - [Create a Webhook](#create-a-webhook)
    - [Update a Webhook](#update-a-webhook)
    - [Delete a Webhook](#delete-a-webhook)
- [Contributing](#contributing)
- [License](#license)
- [Support](#support)

---

## Features

- **Interactive Chat**: Chat with your Kubiya teammates directly from the terminal.
- **Manage Sources**: List, describe, add, sync, and delete your tool sources.
- **Knowledge Base**: Create, read, update, and delete knowledge items.
- **Manage Runners**: List runners and retrieve their Kubernetes manifests.
- **Webhooks**: Create, read, update, and delete webhooks in your workspace.

---

## Installation

### Prerequisites

- **Go** version 1.21 or higher.

### Build from Source

1. **Clone the Repository**

   ```bash
   git clone https://github.com/kubiyabot/cli.git
   ```

2. **Navigate to the Project Directory**

   ```bash
   cd cli
   ```

3. **Build the CLI**

   If your `main.go` is in the root directory:

   ```bash
   go build -o kubiya main.go
   ```

   Alternatively, if `main.go` is in `cmd/kubiya/main.go`:

   ```bash
   go build -o kubiya cmd/kubiya/main.go
   ```

4. **Move the Executable to Your PATH**

   ```bash
   sudo mv kubiya /usr/local/bin/
   ```

   Or, for testing purposes, you can add the current directory to your PATH:

   ```bash
   export PATH=$PATH:$(pwd)
   ```

---

## Configuration

Before using Kubiya CLI, you need to set up your configuration.

### Environment Variables

Kubiya CLI uses the following environment variables:

- `KUBIYA_API_KEY`: Your Kubiya API key. **Required**.
- `KUBIYA_BASE_URL`: Base URL for the Kubiya API. Default is `https://api.kubiya.ai/api/v1`.
- `KUBIYA_DEBUG`: Set to `true` to enable debug mode.

Set your API key:

```bash
export KUBIYA_API_KEY=your_api_key_here
```

Optionally, set the base URL and debug mode:

```bash
export KUBIYA_BASE_URL=https://api.kubiya.ai/api/v1
export KUBIYA_DEBUG=true
```

Alternatively, create a configuration file.

### Configuration File

By default, the configuration is loaded from:

- **Unix/Linux/MacOS**: `~/.kubiya/config.yaml`
- **Windows**: `%USERPROFILE%\.kubiya\config.yaml`

Example `config.yaml`:

```yaml
api_key: your_api_key_here
base_url: https://api.kubiya.ai/api/v1
debug: false
```

---

## Usage

Kubiya CLI provides a set of commands to interact with your Kubiya environment.

Type `kubiya --help` to see all available commands.

```bash
kubiya --help
```

**Output:**

```
Kubiya CLI - Your DevOps Automation Companion 🤖

Welcome to Kubiya CLI! 👋

A powerful tool for interacting with your Kubiya teammates and managing your automation sources.
Use 'kubiya --help' to see all available commands.

Usage:
  kubiya [command]

Available Commands:
  chat        💬 Chat with a teammate
  list        👥 List available teammates
  source      📦 Manage sources
  knowledge   🧠 Manage knowledge base
  runner      🏃 Manage runners
  webhook     🔔 Manage webhooks
  help        Help about any command

Flags:
  -h, --help   help for kubiya

Use "kubiya [command] --help" for more information about a command.
```

---

### Chat with Teammates

Start an interactive chat session with a teammate.

```bash
kubiya chat --interactive
```

Or use the shorthand:

```bash
kubiya chat -i
```

Send a direct message to a teammate without interactive mode:

```bash
kubiya chat --name "Teammate Name" --message "Your message here"
```

Example:

```bash
kubiya chat -n "DevOps Bot" -m "How do I deploy to staging?"
```

---

### List Teammates

List all available teammates:

```bash
kubiya list
```

Output in JSON format:

```bash
kubiya list --output json
```

---

### Manage Sources

Work with your Kubiya sources.

#### List Sources

List all sources:

```bash
kubiya source list
```

Output in JSON format:

```bash
kubiya source list --output json
```

#### Describe a Source

Show detailed information about a specific source:

```bash
kubiya source describe [source-uuid]
```

Example:

```bash
kubiya source describe abc-123
```

#### Add a New Source

Add a new source from a GitHub URL or local path.

From GitHub URL:

```bash
kubiya source add --url https://github.com/kubiyabot/community-tools/tree/main/just_in_time_access
```

From local directory (must be a git repository):

```bash
kubiya source add --path ./my-tools
```

Skip confirmation prompt:

```bash
kubiya source add --url https://github.com/org/repo --yes
```

#### Sync a Source

Sync a specific source:

```bash
kubiya source sync [source-uuid]
```

Example:

```bash
kubiya source sync abc-123
```

Skip confirmation prompt:

```bash
kubiya source sync abc-123 --yes
```

#### Batch Sync Sources

Sync sources related to a repository.

From a GitHub repository URL:

```bash
kubiya source sync-batch --repo https://github.com/kubiyabot/community-tools
```

From a local directory:

```bash
kubiya source sync-batch --repo ./my-repo
```

Skip confirmation prompt:

```bash
kubiya source sync-batch --repo ./my-repo --yes
```

#### Delete a Source

Delete a source:

```bash
kubiya source delete [source-uuid]
```

Example:

```bash
kubiya source delete abc-123
```

Force deletion without confirmation:

```bash
kubiya source delete abc-123 --force
```

---

### Manage Knowledge Base

Create, read, update, and delete knowledge items.

#### List Knowledge Items

List all knowledge items:

```bash
kubiya knowledge list
```

Output in JSON format:

```bash
kubiya knowledge list --output json
```

#### Get a Knowledge Item

Retrieve details of a specific knowledge item:

```bash
kubiya knowledge get [uuid]
```

Example:

```bash
kubiya knowledge get abc-123
```

#### Create a Knowledge Item

Create a new knowledge item:

```bash
kubiya knowledge create --name "Item Name" --desc "Description" --labels label1,label2 --content-file path/to/content.md
```

Example:

```bash
kubiya knowledge create --name "Redis Setup" --desc "How to set up Redis" --labels devops,redis --content-file redis.md
```

#### Update a Knowledge Item

Update an existing knowledge item:

```bash
kubiya knowledge update [uuid] --name "New Name" --desc "New Description" --labels newlabel1,newlabel2
```

Example:

```bash
kubiya knowledge update abc-123 --desc "Updated description" --labels updated
```

#### Delete a Knowledge Item

Delete a knowledge item:

```bash
kubiya knowledge delete [uuid]
```

Example:

```bash
kubiya knowledge delete abc-123
```

---

### Manage Runners

Work with Kubiya runners.

#### List Runners

List all runners:

```bash
kubiya runner list
```

Output in JSON format:

```bash
kubiya runner list --output json
```

#### Get Runner Manifest

Retrieve the Kubernetes manifest for a runner.

Save manifest to a file:

```bash
kubiya runner manifest [runner-name] -o manifest.yaml
```

Example:

```bash
kubiya runner manifest my-runner -o manifest.yaml
```

Apply manifest directly to the current kubectl context:

```bash
kubiya runner manifest my-runner --apply
```

Specify a kubectl context:

```bash
kubiya runner manifest my-runner --apply --context my-context
```

---

### Manage Webhooks

Create, read, update, and delete webhooks.

#### List Webhooks

List all webhooks:

```bash
kubiya webhook list
```

Output in JSON format:

```bash
kubiya webhook list --output json
```

#### Get a Webhook

Retrieve details of a specific webhook:

```bash
kubiya webhook get [id]
```

Example:

```bash
kubiya webhook get abc-123
```

#### Create a Webhook

Create a new webhook:

```bash
kubiya webhook create \
  --name "Webhook Name" \
  --source "source" \
  --agent-id "agent-id" \
  --method "POST" \
  --destination "http://your-webhook-url" \
  --filter "filter-condition" \
  --prompt "Your prompt here"
```

Example:

```bash
kubiya webhook create \
  --name "Build Notifier" \
  --source "jenkins" \
  --agent-id "agent-123" \
  --method "POST" \
  --destination "http://example.com/webhook" \
  --filter "status == 'FAILURE'" \
  --prompt "Build failed with status: {{.status}}"
```

#### Update a Webhook

Update an existing webhook:

```bash
kubiya webhook update [id] --name "Updated Name" --prompt "New prompt"
```

Example:

```bash
kubiya webhook update abc-123 --name "Updated Webhook" --prompt "Updated prompt: {{.event}}"
```

#### Delete a Webhook

Delete a webhook:

```bash
kubiya webhook delete [id]
```

Example:

```bash
kubiya webhook delete abc-123
```

---

## Contributing

We welcome contributions! Please follow these steps to contribute:

1. **Fork the Repository**: Click on the "Fork" button on the top right corner of the repository page.

2. **Clone Your Fork**:

   ```bash
   git clone https://github.com/your-username/cli.git
   cd cli
   ```

3. **Create a Branch**:

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make Changes**: Implement your feature or fix.

5. **Commit Changes**:

   ```bash
   git commit -am "Add your commit message here"
   ```

6. **Push to Your Fork**:

   ```bash
   git push origin feature/your-feature-name
   ```

7. **Create a Pull Request**: Go to the original repository and open a pull request.

---

## License

Kubiya CLI is licensed under the **MIT License**. See [LICENSE](LICENSE) for more information.

---

## Support

For more information, visit our [documentation](https://docs.kubiya.ai) or contact support at [support@kubiya.ai](mailto:support@kubiya.ai).

---
