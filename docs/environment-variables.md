# Kubiya CLI Environment Variables

This document provides a comprehensive reference for all environment variables used by the Kubiya CLI.

## Table of Contents

- [Core Configuration](#core-configuration)
- [Worker Configuration](#worker-configuration)
- [Control Plane Configuration](#control-plane-configuration)
- [Logging and Debugging](#logging-and-debugging)
- [Advanced Configuration](#advanced-configuration)
- [Examples](#examples)

## Core Configuration

### KUBIYA_API_KEY

**Required**: Yes
**Type**: String
**Description**: Your Kubiya API key for authentication with the Kubiya platform.

**Usage**:
```bash
export KUBIYA_API_KEY="kby_abc123..."
```

**How to obtain**:
1. Log in to [Kubiya Composer](https://compose.kubiya.ai)
2. Navigate to Settings → API Keys
3. Generate a new API key

---

### KUBIYA_BASE_URL

**Required**: No
**Type**: String
**Default**: `https://api.kubiya.ai/api/v1`
**Description**: Base URL for the Kubiya API.

**Usage**:
```bash
export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"
```

**Use cases**:
- Connecting to different Kubiya environments (staging, production)
- Using self-hosted Kubiya instances
- Testing against local development servers

---

### KUBIYA_DEBUG

**Required**: No
**Type**: Boolean
**Default**: `false`
**Description**: Enable debug mode for verbose logging throughout the CLI.

**Usage**:
```bash
export KUBIYA_DEBUG=true
```

**Output**:
- Detailed HTTP request/response logs
- Function call traces
- Timing information
- Internal state information

---

### KUBIYA_DEFAULT_RUNNER

**Required**: No
**Type**: String
**Default**: None
**Description**: Default runner to use for tool and workflow execution.

**Usage**:
```bash
export KUBIYA_DEFAULT_RUNNER="production-runner"
```

---

## Worker Configuration

### CONTROL_PLANE_URL

**Required**: Yes (for workers)
**Type**: String
**Default**: `https://control-plane.kubiya.ai`
**Description**: URL of the Kubiya Control Plane API.

**Usage**:
```bash
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
```

**Related**: Can be overridden by `CONTROL_PLANE_GATEWAY_URL`

---

### CONTROL_PLANE_GATEWAY_URL

**Required**: No
**Type**: String
**Default**: None (uses `CONTROL_PLANE_URL`)
**Description**: Override the control plane URL. Takes precedence over `CONTROL_PLANE_URL`.

**Usage**:
```bash
export CONTROL_PLANE_GATEWAY_URL="https://custom-control-plane.example.com"
```

**Priority**: This variable has the highest priority for control plane URL resolution.

**Use cases**:
- Using custom control plane instances
- Testing with staging control plane
- Connecting to regional control plane endpoints

---

### QUEUE_ID

**Required**: Yes (for workers)
**Type**: String
**Description**: Unique identifier for the worker queue.

**Usage**:
```bash
export QUEUE_ID="production-queue-01"
```

**Naming conventions**:
- Use descriptive names: `prod-workers`, `staging-agents`, `dev-queue`
- Separate by environment: `prod-`, `staging-`, `dev-`
- Include region if multi-region: `us-east-1-workers`

---

### ENVIRONMENT_NAME

**Required**: No
**Type**: String
**Default**: `default`
**Description**: Environment or task queue name for the worker.

**Usage**:
```bash
export ENVIRONMENT_NAME="production"
```

**Common values**:
- `production`
- `staging`
- `development`
- `default`

---

### WORKER_HOSTNAME

**Required**: No
**Type**: String
**Default**: Auto-detected (system hostname)
**Description**: Custom hostname for the worker instance.

**Usage**:
```bash
export WORKER_HOSTNAME="worker-pod-01"
```

**Use cases**:
- Kubernetes pod identification
- Multi-worker deployments
- Custom naming schemes

---

### HEARTBEAT_INTERVAL

**Required**: No
**Type**: Integer (seconds)
**Default**: `30`
**Description**: Interval in seconds between worker heartbeats to the control plane.

**Usage**:
```bash
export HEARTBEAT_INTERVAL=60
```

**Recommendations**:
- Production: 30-60 seconds
- Development: 10-30 seconds
- High-load: 60-120 seconds

---

### MAX_CONCURRENT_ACTIVITIES

**Required**: No
**Type**: Integer
**Default**: `10`
**Description**: Maximum number of concurrent activities a worker can execute.

**Usage**:
```bash
export MAX_CONCURRENT_ACTIVITIES=5
```

**Tuning guidelines**:
- Low-resource workers: 1-5
- Standard workers: 5-10
- High-resource workers: 10-20

---

### PYTHON_PATH

**Required**: No
**Type**: String
**Default**: `python3`
**Description**: Path to the Python interpreter for local worker mode.

**Usage**:
```bash
export PYTHON_PATH="/usr/local/bin/python3.11"
```

---

## Control Plane Configuration

### TEMPORAL_HOST_URL

**Required**: No (configured by control plane)
**Type**: String
**Description**: Temporal server address (typically provided by control plane).

**Usage**:
```bash
export TEMPORAL_HOST_URL="temporal.example.com:7233"
```

---

### TEMPORAL_NAMESPACE

**Required**: No (configured by control plane)
**Type**: String
**Default**: `kubiya`
**Description**: Temporal namespace for workflow execution.

**Usage**:
```bash
export TEMPORAL_NAMESPACE="kubiya-production"
```

---

### TEMPORAL_TASK_QUEUE

**Required**: No (configured by control plane)
**Type**: String
**Description**: Temporal task queue name.

**Usage**:
```bash
export TEMPORAL_TASK_QUEUE="agent-execution-queue"
```

---

## Logging and Debugging

### LOG_LEVEL

**Required**: No
**Type**: String
**Default**: `INFO`
**Description**: Logging level for the worker and CLI.

**Valid values**:
- `DEBUG`: Most verbose, includes all details
- `INFO`: Standard operational information
- `WARNING`: Warning messages only
- `ERROR`: Error messages only
- `CRITICAL`: Critical errors only

**Usage**:
```bash
export LOG_LEVEL="DEBUG"
```

---

### KUBIYA_LOG_LEVEL

**Required**: No
**Type**: String
**Default**: `info`
**Description**: CLI-specific log level (alternative to LOG_LEVEL).

**Usage**:
```bash
export KUBIYA_LOG_LEVEL="debug"
```

---

### KUBIYA_LOG_FORMAT

**Required**: No
**Type**: String
**Default**: `text`
**Description**: Log output format.

**Valid values**:
- `text`: Human-readable text format
- `json`: JSON structured logging

**Usage**:
```bash
export KUBIYA_LOG_FORMAT="json"
```

---

## Advanced Configuration

### HTTP_PROXY / HTTPS_PROXY

**Required**: No
**Type**: String
**Description**: HTTP/HTTPS proxy server for CLI requests.

**Usage**:
```bash
export HTTP_PROXY="http://proxy.example.com:8080"
export HTTPS_PROXY="https://proxy.example.com:8443"
```

---

### NO_PROXY

**Required**: No
**Type**: String (comma-separated)
**Description**: Hosts that should bypass the proxy.

**Usage**:
```bash
export NO_PROXY="localhost,127.0.0.1,.local"
```

---

### KUBIYA_TIMEOUT

**Required**: No
**Type**: Integer (seconds)
**Default**: `300`
**Description**: Default timeout for CLI operations.

**Usage**:
```bash
export KUBIYA_TIMEOUT=600
```

---

### KUBIYA_CONFIG_DIR

**Required**: No
**Type**: String
**Default**: `~/.kubiya`
**Description**: Directory for CLI configuration files.

**Usage**:
```bash
export KUBIYA_CONFIG_DIR="/etc/kubiya"
```

---

### SKIP_WORKER_START

**Required**: No
**Type**: Boolean
**Default**: `false`
**Description**: Skip automatic worker startup (used in testing).

**Usage**:
```bash
export SKIP_WORKER_START=1
```

---

### INSTALL_DIR

**Required**: No
**Type**: String
**Description**: Custom installation directory for worker files.

**Usage**:
```bash
export INSTALL_DIR="/opt/kubiya"
```

---

## Examples

### Production Worker

```bash
# Production worker with monitoring
export KUBIYA_API_KEY="kby_prod_key"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export QUEUE_ID="prod-workers-us-east-1"
export ENVIRONMENT_NAME="production"
export WORKER_HOSTNAME="worker-pod-01"
export LOG_LEVEL="INFO"
export HEARTBEAT_INTERVAL=60
export MAX_CONCURRENT_ACTIVITIES=10

kubiya worker start --queue-id=$QUEUE_ID --type=local -d
```

### Development Worker

```bash
# Development worker with debug logging
export KUBIYA_API_KEY="kby_dev_key"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export QUEUE_ID="dev-test-queue"
export ENVIRONMENT_NAME="development"
export LOG_LEVEL="DEBUG"
export KUBIYA_DEBUG=true
export HEARTBEAT_INTERVAL=10

kubiya worker start --queue-id=$QUEUE_ID --type=local
```

### Custom Control Plane

```bash
# Using custom control plane
export KUBIYA_API_KEY="kby_custom_key"
export CONTROL_PLANE_GATEWAY_URL="https://cp.example.com"
export QUEUE_ID="custom-queue"
export LOG_LEVEL="INFO"

kubiya worker start --queue-id=$QUEUE_ID --type=local
```

### Behind Proxy

```bash
# Worker behind corporate proxy
export KUBIYA_API_KEY="kby_corp_key"
export HTTP_PROXY="http://proxy.corp.com:8080"
export HTTPS_PROXY="http://proxy.corp.com:8080"
export NO_PROXY="localhost,127.0.0.1,.local,.corp.com"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export QUEUE_ID="corp-workers"

kubiya worker start --queue-id=$QUEUE_ID --type=local
```

### Multi-Environment Setup

```bash
# Development
cat > ~/.kubiya/env.dev <<EOF
export KUBIYA_API_KEY="kby_dev_key"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export ENVIRONMENT_NAME="development"
export LOG_LEVEL="DEBUG"
export KUBIYA_DEBUG=true
EOF

# Staging
cat > ~/.kubiya/env.staging <<EOF
export KUBIYA_API_KEY="kby_staging_key"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export ENVIRONMENT_NAME="staging"
export LOG_LEVEL="INFO"
EOF

# Production
cat > ~/.kubiya/env.prod <<EOF
export KUBIYA_API_KEY="kby_prod_key"
export CONTROL_PLANE_URL="https://control-plane.kubiya.ai"
export ENVIRONMENT_NAME="production"
export LOG_LEVEL="WARNING"
export HEARTBEAT_INTERVAL=60
export MAX_CONCURRENT_ACTIVITIES=15
EOF

# Usage
source ~/.kubiya/env.prod
kubiya worker start --queue-id=prod-queue --type=local -d
```

### Docker Deployment

```bash
# Docker environment file
cat > worker.env <<EOF
KUBIYA_API_KEY=kby_docker_key
CONTROL_PLANE_URL=https://control-plane.kubiya.ai
QUEUE_ID=docker-workers
ENVIRONMENT_NAME=production
LOG_LEVEL=INFO
WORKER_HOSTNAME=docker-worker-01
HEARTBEAT_INTERVAL=60
EOF

# Run with environment file
docker run -d \
  --name kubiya-worker \
  --env-file worker.env \
  --restart unless-stopped \
  ghcr.io/kubiyabot/agent-worker:latest
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubiya-worker-config
  namespace: kubiya
data:
  CONTROL_PLANE_URL: "https://control-plane.kubiya.ai"
  ENVIRONMENT_NAME: "production"
  LOG_LEVEL: "INFO"
  HEARTBEAT_INTERVAL: "60"
  MAX_CONCURRENT_ACTIVITIES: "10"
---
apiVersion: v1
kind: Secret
metadata:
  name: kubiya-worker-secret
  namespace: kubiya
type: Opaque
stringData:
  KUBIYA_API_KEY: "your-api-key-here"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubiya-worker
  namespace: kubiya
spec:
  replicas: 3
  selector:
    matchLabels:
      app: kubiya-worker
  template:
    metadata:
      labels:
        app: kubiya-worker
    spec:
      containers:
      - name: worker
        image: ghcr.io/kubiyabot/agent-worker:latest
        envFrom:
        - configMapRef:
            name: kubiya-worker-config
        - secretRef:
            name: kubiya-worker-secret
        env:
        - name: QUEUE_ID
          value: "k8s-production-queue"
        - name: WORKER_HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
```

## Best Practices

### Security

1. **Never commit API keys** to version control
2. **Use secrets management** (Kubernetes Secrets, HashiCorp Vault, AWS Secrets Manager)
3. **Rotate API keys** regularly
4. **Use environment-specific keys** (separate dev/staging/prod keys)

### Environment Files

1. **Use .env files** for local development
2. **Add .env to .gitignore**
3. **Document required variables** in `.env.example`
4. **Use secret management** for production

Example `.env.example`:
```bash
# Required
KUBIYA_API_KEY=your-api-key-here
CONTROL_PLANE_URL=https://control-plane.kubiya.ai

# Optional
QUEUE_ID=my-worker-queue
ENVIRONMENT_NAME=development
LOG_LEVEL=INFO
HEARTBEAT_INTERVAL=30
```

### Configuration Hierarchy

Variables are resolved in this order (highest priority first):

1. Command-line flags
2. Environment variables
3. Configuration files
4. Default values

Example:
```bash
# Environment variable
export QUEUE_ID="env-queue"

# Overridden by CLI flag
kubiya worker start --queue-id="cli-queue"  # Uses "cli-queue"
```

## Troubleshooting

### Variable Not Being Read

```bash
# Check if variable is set
echo $KUBIYA_API_KEY

# Export the variable
export KUBIYA_API_KEY="your-key"

# Verify it's exported
env | grep KUBIYA
```

### Debugging Environment

```bash
# Print all Kubiya-related variables
env | grep -i kubiya

# Check worker configuration
kubiya worker start --queue-id=test --type=local --help
```

### Common Issues

**Issue**: Worker can't connect to control plane

**Solution**:
```bash
# Check control plane URL
echo $CONTROL_PLANE_URL
echo $CONTROL_PLANE_GATEWAY_URL

# Test connectivity
curl $CONTROL_PLANE_URL/health

# Use override if needed
export CONTROL_PLANE_GATEWAY_URL="https://control-plane.kubiya.ai"
```

**Issue**: API authentication fails

**Solution**:
```bash
# Verify API key is set
echo $KUBIYA_API_KEY | head -c 20

# Test authentication
kubiya agent list

# Regenerate key if needed (via Kubiya Composer)
```

## Related Documentation

- [Worker Guide](worker-guide.md)
- [README](../README.md)
- [Kubiya Documentation](https://docs.kubiya.ai)
