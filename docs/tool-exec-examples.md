# Tool Execution Examples

The `kubiya tool exec` command allows you to execute tools directly with real-time streaming output.

## Basic Usage

### Simple Command Execution
```bash
# Execute a simple echo command
kubiya tool exec --name "hello-world" --content "echo 'Hello, World!'"

# Execute with a specific runner
kubiya tool exec --name "test" --content "date" --runner core-testing-1
```

### Using Docker Images
```bash
# Python script execution
kubiya tool exec --name "python-test" \
  --type docker \
  --image python:3.11 \
  --content "print('Hello from Python'); import sys; print(f'Python version: {sys.version}')"

# Node.js execution
kubiya tool exec --name "node-test" \
  --type docker \
  --image node:18 \
  --content "console.log('Hello from Node.js'); console.log('Node version:', process.version)"
```

### Long-Running Tasks
```bash
# Progress indicator with timestamps
kubiya tool exec --name "progress" \
  --type docker \
  --image python:3.11 \
  --content 'for i in range(1, 11): print(f"[{i}/10] Processing..."); import time; time.sleep(2)'
```

## Advanced Usage

### Direct JSON Input
```bash
# Execute tool from inline JSON
kubiya tool exec --json '{
  "name": "multi-step",
  "type": "docker",
  "image": "alpine",
  "content": "echo \"Step 1: Initialize\"; sleep 1; echo \"Step 2: Process\"; sleep 1; echo \"Step 3: Complete\""
}'
```

### JSON File Input
Create a file `tool.json`:
```json
{
  "name": "data-processor",
  "description": "Process data with progress updates",
  "type": "docker",
  "image": "python:3.11",
  "content": "import time\nimport datetime\n\nfor i in range(1, 11):\n    timestamp = datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S')\n    print(f'[{timestamp}] [{i}/10] Processing batch...')\n    time.sleep(2)\nprint('\\nProcessing complete!')"
}
```

Then execute:
```bash
kubiya tool exec --json-file tool.json
```

## Advanced Tool Properties

### File Mappings

Map files from your local system to the tool container:

```bash
# Map a single file
kubiya tool exec --name "config-reader" \
  --content "cat /app/config.yaml" \
  --with-file ~/.config/myapp.yaml:/app/config.yaml

# Map multiple files
kubiya tool exec --name "multi-config" \
  --content "ls -la /configs/" \
  --with-file ~/.config/app1.yaml:/configs/app1.yaml \
  --with-file ~/.config/app2.json:/configs/app2.json

# Local files are automatically read and their content included
# Remote paths (like /var/run/secrets/) are preserved as-is
```

### Volume Mappings

Mount volumes into the tool container:

```bash
# Mount Docker socket (read-write)
kubiya tool exec --name "docker-tool" \
  --content "docker ps -a" \
  --with-volume /var/run/docker.sock:/var/run/docker.sock

# Mount volume as read-only
kubiya tool exec --name "docker-inspect" \
  --content "docker inspect nginx" \
  --with-volume /var/run/docker.sock:/var/run/docker.sock:ro
```

### Service Dependencies

Specify services that should be available:

```bash
kubiya tool exec --name "db-tool" \
  --content "mysql -h db -u root -p\$DB_PASSWORD -e 'SHOW DATABASES;'" \
  --with-service mysql:5.7 \
  --env DB_PASSWORD=secret
```

### Tool Arguments

Define arguments that can be passed to the tool:

```bash
# Define tool with arguments
kubiya tool exec --name "ec2-tool" \
  --content 'aws ec2 describe-instances $([[ -n "$instance_ids" ]] && echo "--instance-ids $instance_ids")' \
  --arg instance_ids:string:"Comma-separated instance IDs":false \
  --arg region:string:"AWS region":true

# Arguments format: name:type:description:required
```

### Environment Variables

Pass environment variables to the tool:

```bash
# Single environment variable
kubiya tool exec --name "env-test" \
  --content "echo Hello \$NAME" \
  --env NAME=World

# Multiple environment variables
kubiya tool exec --name "aws-tool" \
  --content "aws s3 ls" \
  --env AWS_ACCESS_KEY_ID=your-key \
  --env AWS_SECRET_ACCESS_KEY=your-secret \
  --env AWS_REGION=us-east-1
```

### Integration Templates

Use predefined integration templates:

```bash
# Kubernetes in-cluster authentication
kubiya tool exec --name "k8s-pods" \
  --content "kubectl get pods -A" \
  --integration k8s/incluster

# AWS credentials with profile
kubiya tool exec --name "s3-list" \
  --content "aws s3 ls" \
  --integration aws/profile \
  --env AWS_PROFILE=production

# Multiple integrations
kubiya tool exec --name "deploy" \
  --content "./deploy.sh" \
  --integration k8s/incluster \
  --integration aws/creds
```

## Output Formats

### Text Output (Default)
Clean, formatted output with visual indicators:
```bash
kubiya tool exec --name "demo" --content "echo 'Starting...'; sleep 2; echo 'Done!'"
```

Output:
```
🚀 Executing tool: demo
📍 Runner: kubiya-hosted

🔧 Creating new Engine session... OK!
› connect DONE [0.2s]
│ Starting...
│ Done!

✅ Tool executed successfully in 2.34s
📊 2 lines of output generated
```

### JSON Stream Output
Raw JSON event stream for programmatic consumption:
```bash
kubiya tool exec --name "test" --content "echo 'Hello'" --output stream-json
```

Output:
```json
{"start":false,"end":false,"type":"log","tool_name":"test","content":"Creating new Engine session... OK!","meta":{...}}
{"start":false,"end":false,"type":"tool-output","tool_name":"test","content":"Hello\n","meta":{...}}
{"start":false,"end":true,"type":"status","tool_name":"test","status":"success","meta":{...}}
```

## Flags Reference

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Tool name (required unless using JSON) | - |
| `--content` | Tool script/command to execute | - |
| `--type` | Tool type (docker, python, bash, etc.) | docker |
| `--image` | Docker image to use | - |
| `--description` | Tool description | "CLI executed tool" |
| `--runner` | Runner to execute on | auto |
| `--watch` | Stream output in real-time | true |
| `--output` | Output format: text or stream-json | text |
| `--json` | Tool definition as JSON string | - |
| `--json-file` | Path to JSON file with tool definition | - |
| `--skip-health-check` | Skip runner health check | false |
| `--timeout` | Timeout in seconds (0 for no timeout) | 300 |
| `--integration` | Integration template to apply (can be repeated) | - |
| `--with-file` | File mapping format: source:destination (can be repeated) | - |
| `--with-volume` | Volume mapping format: source:destination[:ro] (can be repeated) | - |
| `--with-service` | Service dependency (can be repeated) | - |
| `--env` | Environment variable format: KEY=VALUE (can be repeated) | - |
| `--arg` | Tool argument format: name:type:description:required (can be repeated) | - |
| `--icon-url` | Icon URL for the tool | - |

## Environment Variables

The following environment variables can be used to set default values:

| Variable | Description | Example |
|----------|-------------|---------|
| `KUBIYA_TOOL_TIMEOUT` | Default timeout in seconds | `export KUBIYA_TOOL_TIMEOUT=600` |
| `KUBIYA_TOOL_RUNNER` | Default runner name | `export KUBIYA_TOOL_RUNNER=my-runner` |
| `KUBIYA_DEFAULT_RUNNER` | Default runner when "default" is used or runner is empty | `export KUBIYA_DEFAULT_RUNNER=core-testing-2` |
| `KUBIYA_TOOL_OUTPUT_FORMAT` | Default output format | `export KUBIYA_TOOL_OUTPUT_FORMAT=stream-json` |
| `KUBIYA_TOOL_TYPE` | Default tool type | `export KUBIYA_TOOL_TYPE=python` |
| `KUBIYA_SKIP_HEALTH_CHECK` | Skip health checks | `export KUBIYA_SKIP_HEALTH_CHECK=true` |

Environment variables are only used if the corresponding flag is not explicitly set. For example:

```bash
# Uses environment variable timeout
export KUBIYA_TOOL_TIMEOUT=60
kubiya tool exec --name "test" --content "date"

# Overrides environment variable with explicit flag
kubiya tool exec --name "test" --content "date" --timeout 30

# Set default runner for all executions
export KUBIYA_DEFAULT_RUNNER=core-testing-2
kubiya tool exec --name "test" --content "date"  # Uses core-testing-2

# Use "default" keyword to reference KUBIYA_DEFAULT_RUNNER
kubiya tool exec --name "test" --content "date" --runner default

# Override default runner with explicit runner
kubiya tool exec --name "test" --content "date" --runner kubiya-hosted
```

## Runner Health Check

By default, the CLI checks if the runner is healthy before executing:
```bash
kubiya tool exec --name "test" --content "date"
# Output:
# ✓ Runner 'kubiya-hosted' is healthy (v2)
# 🚀 Executing tool: test
# ...
```

To skip the health check:
```bash
kubiya tool exec --name "test" --content "date" --skip-health-check
```

## Error Handling

The CLI provides clear error messages:
```bash
# Missing required fields
kubiya tool exec --name "test"
# Error: tool content is required

# Invalid JSON
kubiya tool exec --json '{"invalid"}'
# Error: failed to parse JSON input: ...

# Unhealthy runner
kubiya tool exec --name "test" --content "date" --runner unhealthy-runner
# Error: runner 'unhealthy-runner' is not healthy: offline
```

## Use Cases

### CI/CD Integration
```bash
# Run tests with specific image
kubiya tool exec \
  --name "run-tests" \
  --image "my-app:latest" \
  --content "npm test" \
  --output stream-json | jq -r 'select(.type=="tool-output") | .content'
```

### Data Processing
```bash
# Process CSV with Python
kubiya tool exec \
  --name "csv-processor" \
  --image "python:3.11-slim" \
  --content "import pandas as pd; df = pd.DataFrame({'A': [1,2,3], 'B': [4,5,6]}); print(df.to_csv())"
```

### System Monitoring
```bash
# Check system resources
kubiya tool exec \
  --name "system-check" \
  --image "alpine" \
  --content "echo '=== System Info ==='; uname -a; echo; echo '=== Memory ==='; free -h"
``` 