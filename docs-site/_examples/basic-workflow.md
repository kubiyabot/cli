---
layout: example
title: Basic Workflow Example
description: A simple workflow demonstrating basic Kubiya CLI workflow execution
difficulty: beginner
category: workflow
tags: [workflow, basic, getting-started]
---

## Overview

This example demonstrates how to create and execute a basic workflow using the Kubiya CLI. It covers the fundamental concepts of workflow definition and execution.

## Prerequisites

- Kubiya CLI installed and configured
- Valid API key set in environment
- Basic understanding of YAML syntax

## Workflow Definition

Create a file named `basic-workflow.yaml`:

```yaml
name: basic-hello-world
description: A simple workflow that demonstrates basic functionality
version: "1.0"

params:
  name:
    type: string
    description: Name to greet
    default: "World"
  message:
    type: string
    description: Custom message
    default: "Hello"

steps:
- name: greet
  description: Print greeting message
  executor: command
  command: echo "$message, $name!"

- name: show_date
  description: Show current date and time
  executor: command
  command: date '+%Y-%m-%d %H:%M:%S'
  depends: [greet]

- name: system_info
  description: Display basic system information
  executor: command
  command: |
    echo "Hostname: $(hostname)"
    echo "User: $(whoami)"
    echo "Uptime: $(uptime)"
  depends: [show_date]

- name: final_message
  description: Display completion message
  executor: command
  command: echo "Workflow completed successfully!"
  depends: [system_info]
```

## Execution

### Basic Execution

Execute the workflow with default parameters:

```bash
kubiya workflow execute basic-workflow.yaml
```

Expected output:
```
🚀 Executing workflow: basic-hello-world
Source: Local File
File: basic-workflow.yaml
Runner: default-runner

🚀 Starting workflow execution...

▶️ [1/4] 🔄 Running: greet
  ✅ Step completed in 0.5s
  📤 Output: Hello, World!

▶️ [2/4] 🔄 Running: show_date
  ✅ Step completed in 0.3s
  📤 Output: 2024-01-15 10:30:45

▶️ [3/4] 🔄 Running: system_info
  ✅ Step completed in 0.7s
  📤 Output: 
  Hostname: runner-host
  User: kubiya
  Uptime: 10:30:45 up 1 day, 2:15, 1 user, load average: 0.01, 0.02, 0.00

▶️ [4/4] 🔄 Running: final_message
  ✅ Step completed in 0.2s
  📤 Output: Workflow completed successfully!

🎉 Workflow completed successfully!
```

### Execution with Parameters

Execute with custom parameters:

```bash
kubiya workflow execute basic-workflow.yaml \
  --var name="Developer" \
  --var message="Welcome"
```

Expected output:
```
🚀 Executing workflow: basic-hello-world

▶️ [1/4] 🔄 Running: greet
  ✅ Step completed in 0.5s
  📤 Output: Welcome, Developer!

# ... rest of execution
```

### Dry Run

Validate the workflow without executing:

```bash
kubiya workflow execute basic-workflow.yaml --dry-run
```

Expected output:
```
✅ Workflow validation successful
📋 Workflow: basic-hello-world
📄 Description: A simple workflow that demonstrates basic functionality
🔧 Parameters:
  - name: "World" (string)
  - message: "Hello" (string)
📝 Steps:
  1. greet → show_date
  2. show_date → system_info
  3. system_info → final_message
  4. final_message
```

## Advanced Usage

### Using Environment Variables

Create a workflow that uses environment variables:

```yaml
name: env-workflow
description: Workflow demonstrating environment variable usage

steps:
- name: check_env
  executor: command
  command: |
    echo "API Key present: $([ -n "$KUBIYA_API_KEY" ] && echo "Yes" || echo "No")"
    echo "Debug mode: ${KUBIYA_DEBUG:-false}"
    echo "Runner: ${KUBIYA_DEFAULT_RUNNER:-default}"
```


## Common Use Cases

### Development Workflow

```yaml
name: development-workflow
description: Common development tasks

params:
  branch:
    type: string
    description: Git branch to work with
    default: "main"

steps:
- name: checkout
  executor: command
  command: git checkout $branch

- name: install_deps
  executor: command
  command: npm install
  depends: [checkout]

- name: run_tests
  executor: command
  command: npm test
  depends: [install_deps]

- name: build
  executor: command
  command: npm run build
  depends: [run_tests]
```

### System Maintenance

```yaml
name: system-maintenance
description: Basic system maintenance tasks

steps:
- name: disk_cleanup
  executor: command
  command: |
    echo "Cleaning temporary files..."
    rm -rf /tmp/*
    echo "Disk cleanup completed"

- name: log_rotation
  executor: command
  command: |
    echo "Rotating logs..."
    find /var/log -name "*.log" -size +100M -exec gzip {} \;
    echo "Log rotation completed"
  depends: [disk_cleanup]

- name: system_update
  executor: command
  command: |
    echo "Checking for updates..."
    apt list --upgradable
    echo "Update check completed"
  depends: [log_rotation]
```

## Best Practices

### 1. Use Descriptive Names

```yaml
steps:
- name: validate_input_parameters
  description: Validate all required input parameters
  # ... rest of step
```

### 2. Handle Dependencies

```yaml
steps:
- name: setup_environment
  # ... setup step

- name: run_application
  depends: [setup_environment]
  # ... application step

- name: cleanup_resources
  depends: [run_application]
  run_on_failure: true
  # ... cleanup step
```

### 3. Use Parameters for Flexibility

```yaml
params:
  environment:
    type: string
    description: Target environment
    default: "development"
    enum: ["development", "staging", "production"]
```

### 4. Add Proper Error Handling

```yaml
steps:
- name: critical_operation
  executor: command
  command: important-command
  timeout: 300s
  retry:
    attempts: 3
    delay: 10s
```

## Troubleshooting

### Common Issues

1. **Workflow file not found**
   ```bash
   # Ensure file exists and has correct permissions
   ls -la basic-workflow.yaml
   ```

2. **Parameter validation errors**
   ```bash
   # Check parameter types and values
   kubiya workflow describe basic-workflow.yaml
   ```

3. **Step execution failures**
   ```bash
   # Run with verbose output
   kubiya workflow execute basic-workflow.yaml --verbose
   ```

### Debug Mode

Enable debug mode for detailed execution information:

```bash
export KUBIYA_DEBUG=true
kubiya workflow execute basic-workflow.yaml --verbose
```

## Next Steps

- Try the [Docker Workflow Example](docker-workflow)
- Learn about [Agent Creation](agent-creation)
- Explore [Advanced Workflows](advanced-workflows)
- Check out [CI/CD Integration](cicd-integration)