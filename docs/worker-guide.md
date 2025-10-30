# Kubiya Worker Guide

## Overview

The Kubiya Worker is a Temporal-based execution engine that processes agent and team workflows from the Kubiya Control Plane. It provides a scalable, fault-tolerant infrastructure for running AI agent tasks with built-in support for multiple deployment modes.

## Table of Contents

- [Architecture](#architecture)
- [Deployment Modes](#deployment-modes)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Deployment Examples](#deployment-examples)
- [Monitoring and Logging](#monitoring-and-logging)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubiya Control Plane                      │
│            https://control-plane.kubiya.ai                   │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ Task Queue
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                   Temporal Cloud                             │
│              (Managed Workflow Engine)                       │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ Workflows
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                  Kubiya Worker                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Agent Execution Activities                            │ │
│  │  • Tool Integration                                     │ │
│  │  • Event Streaming                                      │ │
│  │  • Session Persistence                                  │ │
│  └────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Team Execution Activities                             │ │
│  │  • Multi-Agent Coordination                            │ │
│  │  • HITL (Human-in-the-Loop)                            │ │
│  │  • Team Event Streaming                                │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Key Features

- **Temporal Integration**: Built on Temporal for reliable, durable workflow execution
- **Control Plane Communication**: Direct integration with Kubiya Control Plane for configuration and monitoring
- **Multiple Deployment Modes**: Support for local Python, Docker, and Kubernetes deployments
- **Automatic Setup**: Self-contained with embedded Python worker files
- **Daemon Mode**: Background execution with automatic crash recovery
- **Health Monitoring**: Built-in heartbeat and health check mechanisms

## Deployment Modes

### Local Mode (Python)

Runs the worker locally using Python 3.8+ with an automatically managed virtual environment.

**Advantages:**
- No Docker required
- Fast startup time
- Easy debugging
- Local development friendly

**Requirements:**
- Python 3.8 or later
- pip and venv support

### Docker Mode

Runs the worker in a Docker container for isolation and portability.

**Advantages:**
- Consistent environment
- Easy to deploy
- Resource isolation
- No local Python setup required

**Requirements:**
- Docker daemon running
- Network access to Temporal and Control Plane

### Daemon Mode

Runs the worker as a background process with automatic supervision and crash recovery.

**Advantages:**
- Automatic restart on crashes
- Log rotation
- Production-ready
- Exponential backoff retry

**Requirements:**
- Linux/macOS system
- Write access to `~/.kubiya/workers/`

## Getting Started

### Quick Start

```bash
# 1. Set your API key
export KUBIYA_API_KEY="your-api-key"

# 2. Start a worker (local mode)
kubiya worker start --queue-id=my-queue-id --type=local

# 3. Start in daemon mode for production
kubiya worker start --queue-id=my-queue-id --type=local -d
```

### Basic Commands

```bash
# Start worker in local mode (default)
kubiya worker start --queue-id=<queue-id>

# Start worker in Docker mode
kubiya worker start --queue-id=<queue-id> --type=docker

# Start worker in daemon mode with supervision
kubiya worker start --queue-id=<queue-id> --type=local -d

# Start with custom image (Docker mode)
kubiya worker start --queue-id=<queue-id> --type=docker --image-tag=v1.2.3

# Disable auto-pull (Docker mode)
kubiya worker start --queue-id=<queue-id> --type=docker --pull=false
```

### Daemon Management

```bash
# View logs
tail -f ~/.kubiya/workers/<queue-id>/logs/worker.log

# Check worker status
ps aux | grep "worker.py"

# Stop worker (send SIGTERM)
kill <pid>

# Force stop worker
kill -9 <pid>
```

## Configuration

### Environment Variables

#### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `KUBIYA_API_KEY` | API key for authentication | `kby_abc123...` |
| `CONTROL_PLANE_URL` | Control Plane API URL | `https://control-plane.kubiya.ai` |
| `QUEUE_ID` | Worker queue identifier | `my-worker-queue` |

#### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CONTROL_PLANE_GATEWAY_URL` | Override control plane URL | Uses `CONTROL_PLANE_URL` |
| `ENVIRONMENT_NAME` | Environment/task queue name | `default` |
| `WORKER_HOSTNAME` | Custom hostname for worker | Auto-detected |
| `HEARTBEAT_INTERVAL` | Seconds between heartbeats | `30` |
| `LOG_LEVEL` | Logging level | `INFO` |
| `PYTHON_PATH` | Custom Python interpreter | `python3` |

### Control Plane URL Override

The worker supports overriding the control plane URL via environment variable:

```bash
# Use custom control plane
export CONTROL_PLANE_GATEWAY_URL="https://custom-control-plane.example.com"
kubiya worker start --queue-id=my-queue
```

Priority order:
1. `CONTROL_PLANE_GATEWAY_URL` environment variable
2. Default: `https://control-plane.kubiya.ai`

### Command-Line Flags

```bash
# Worker start flags
--queue-id string         Worker queue ID (required)
--type string            Deployment type: local or docker (default "local")
-d, --daemon             Run in daemon mode with supervision
--image string           Worker Docker image (default "ghcr.io/kubiyabot/agent-worker")
--image-tag string       Worker image tag (default "latest")
--pull                   Automatically pull latest image (default true)
--max-log-size int       Maximum log file size in bytes (default 104857600)
--max-log-backups int    Maximum number of log backup files (default 5)
```

### Configuration File

For advanced configurations, you can create a worker configuration file:

```yaml
# ~/.kubiya/worker-config.yaml
queue_id: "production-queue"
deployment_type: "local"
daemon_mode: true

control_plane:
  url: "https://control-plane.kubiya.ai"
  api_key: "${KUBIYA_API_KEY}"
  heartbeat_interval: 30

temporal:
  host: "temporal-frontend.temporal:7233"
  namespace: "kubiya"
  task_queue: "agent-tasks"

logging:
  level: "INFO"
  max_size_bytes: 104857600
  max_backups: 5

resources:
  max_concurrent_activities: 10
  activity_timeout: "300s"
```

## Deployment Examples

### Local Development

```bash
# Start worker for local development
export KUBIYA_API_KEY="your-dev-key"
export LOG_LEVEL="DEBUG"

kubiya worker start --queue-id=dev-queue --type=local
```

### Production (Daemon Mode)

```bash
# Start worker in daemon mode with monitoring
export KUBIYA_API_KEY="your-prod-key"
export CONTROL_PLANE_GATEWAY_URL="https://control-plane.kubiya.ai"

kubiya worker start \
  --queue-id=prod-queue \
  --type=local \
  --daemon \
  --max-log-size=104857600 \
  --max-log-backups=10
```

### Docker Standalone

```bash
# Run worker in Docker container
docker run -d \
  --name kubiya-worker \
  --restart unless-stopped \
  -e KUBIYA_API_KEY="your-api-key" \
  -e CONTROL_PLANE_URL="https://control-plane.kubiya.ai" \
  -e QUEUE_ID="docker-queue" \
  -e LOG_LEVEL="INFO" \
  ghcr.io/kubiyabot/agent-worker:latest
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubiya-worker
  namespace: kubiya
  labels:
    app: kubiya-worker
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
        command: ["kubiya", "worker", "start"]
        args:
          - "--queue-id=$(QUEUE_ID)"
          - "--type=local"
        env:
        - name: KUBIYA_API_KEY
          valueFrom:
            secretKeyRef:
              name: kubiya-secrets
              key: api-key
        - name: CONTROL_PLANE_URL
          value: "https://control-plane.kubiya.ai"
        - name: QUEUE_ID
          value: "k8s-production-queue"
        - name: LOG_LEVEL
          value: "INFO"
        - name: WORKER_HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          exec:
            command:
            - cat
            - /tmp/worker-alive
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - cat
            - /tmp/worker-ready
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Secret
metadata:
  name: kubiya-secrets
  namespace: kubiya
type: Opaque
stringData:
  api-key: "your-kubiya-api-key-here"
```

### Kubernetes with Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: kubiya-worker-hpa
  namespace: kubiya
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: kubiya-worker
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 30
```

### Docker Compose

```yaml
version: '3.8'

services:
  kubiya-worker:
    image: ghcr.io/kubiyabot/agent-worker:latest
    container_name: kubiya-worker
    restart: unless-stopped
    environment:
      - KUBIYA_API_KEY=${KUBIYA_API_KEY}
      - CONTROL_PLANE_URL=https://control-plane.kubiya.ai
      - QUEUE_ID=${QUEUE_ID:-default-queue}
      - LOG_LEVEL=INFO
      - WORKER_HOSTNAME=${HOSTNAME}
    volumes:
      - worker-data:/app/data
      - worker-logs:/app/logs
    command: ["worker", "start", "--queue-id=${QUEUE_ID}", "--type=local"]
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 2G
        reservations:
          cpus: '0.25'
          memory: 512M

volumes:
  worker-data:
  worker-logs:
```

## Monitoring and Logging

### Log Locations

| Deployment Mode | Log Location |
|----------------|--------------|
| Local (foreground) | stdout/stderr |
| Local (daemon) | `~/.kubiya/workers/<queue-id>/logs/worker.log` |
| Docker | Container logs (`docker logs kubiya-worker`) |
| Kubernetes | Pod logs (`kubectl logs -f kubiya-worker-xxx`) |

### Log Rotation (Daemon Mode)

Daemon mode includes automatic log rotation:

- **Max Size**: 100 MB (default, configurable)
- **Max Backups**: 5 files (default, configurable)
- **Format**: `worker.log`, `worker.log.1`, `worker.log.2`, etc.

```bash
# Configure log rotation
kubiya worker start \
  --queue-id=my-queue \
  --daemon \
  --max-log-size=52428800 \    # 50 MB
  --max-log-backups=10
```

### Viewing Logs

```bash
# Local daemon mode
tail -f ~/.kubiya/workers/<queue-id>/logs/worker.log

# Docker
docker logs -f kubiya-worker

# Kubernetes
kubectl logs -f deployment/kubiya-worker -n kubiya

# View all worker logs
kubectl logs -l app=kubiya-worker -n kubiya --all-containers=true
```

### Health Checks

Workers send periodic heartbeats to the Control Plane:

- **Interval**: 30 seconds (default, configurable)
- **Metrics**: CPU, memory, uptime, active tasks
- **Status**: Available on Control Plane dashboard

```bash
# Monitor worker health via Control Plane API
curl -H "Authorization: Bearer $KUBIYA_API_KEY" \
  https://control-plane.kubiya.ai/api/v1/workers/<queue-id>/health
```

### Metrics Collection

Workers expose the following metrics:

- **Active Tasks**: Current number of executing tasks
- **Completed Tasks**: Total tasks completed since start
- **Failed Tasks**: Total tasks that failed
- **CPU Usage**: Worker CPU utilization
- **Memory Usage**: Worker memory consumption
- **Uptime**: Time since worker started

## Troubleshooting

### Common Issues

#### Worker Fails to Start

**Symptoms**: Worker exits immediately or fails to connect

**Solutions**:
```bash
# 1. Verify API key
echo $KUBIYA_API_KEY

# 2. Test connectivity to Control Plane
curl https://control-plane.kubiya.ai/health

# 3. Check Python version (local mode)
python3 --version  # Should be 3.8 or later

# 4. Run with debug logging
export LOG_LEVEL=DEBUG
kubiya worker start --queue-id=test-queue
```

#### Connection Errors

**Symptoms**: "Failed to connect to Temporal" or "Control Plane unreachable"

**Solutions**:
```bash
# 1. Check network connectivity
ping control-plane.kubiya.ai

# 2. Verify firewall rules (ports: 443, 7233)
telnet control-plane.kubiya.ai 443

# 3. Check proxy settings
echo $HTTP_PROXY
echo $HTTPS_PROXY

# 4. Use custom control plane URL
export CONTROL_PLANE_GATEWAY_URL="https://custom-cp.example.com"
```

#### Python Dependencies Error

**Symptoms**: "Module not found" or "Import error"

**Solutions**:
```bash
# 1. Clear virtual environment
rm -rf ~/.kubiya/workers/<queue-id>/venv

# 2. Restart worker (will recreate venv)
kubiya worker start --queue-id=<queue-id>

# 3. Manually install dependencies
cd ~/.kubiya/workers/<queue-id>
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

#### High Memory Usage

**Symptoms**: Worker consumes excessive memory

**Solutions**:
```bash
# 1. Limit concurrent activities
export MAX_CONCURRENT_ACTIVITIES=5

# 2. Use Docker mode with memory limits
docker run --memory=1g --memory-swap=2g ...

# 3. Restart worker periodically (Kubernetes)
# Use pod disruption budgets and rolling restarts
```

#### Logs Not Appearing

**Symptoms**: No logs in expected location

**Solutions**:
```bash
# 1. Check permissions
ls -la ~/.kubiya/workers/<queue-id>/logs/

# 2. Verify log level
export LOG_LEVEL=INFO

# 3. Check disk space
df -h

# 4. Test log writing
touch ~/.kubiya/workers/<queue-id>/logs/test.log
```

### Debug Mode

Enable comprehensive debugging:

```bash
# Full debug output
export KUBIYA_DEBUG=true
export LOG_LEVEL=DEBUG
kubiya worker start --queue-id=debug-queue
```

### Worker Status Check

```bash
# Check if worker is running
ps aux | grep "worker.py"

# Check worker directory
ls -la ~/.kubiya/workers/<queue-id>/

# Check worker PID file (daemon mode)
cat ~/.kubiya/workers/<queue-id>/worker.pid

# View worker info (daemon mode)
cat ~/.kubiya/workers/<queue-id>/daemon_info.txt
```

## Best Practices

### Production Deployment

1. **Use Daemon Mode**: For production environments, always use daemon mode for automatic recovery
   ```bash
   kubiya worker start --queue-id=prod-queue -d
   ```

2. **Configure Log Rotation**: Set appropriate log sizes to prevent disk space issues
   ```bash
   --max-log-size=52428800 --max-log-backups=10
   ```

3. **Monitor Health**: Set up monitoring for worker health and performance
   ```bash
   # Use Control Plane API or metrics endpoint
   ```

4. **Resource Limits**: Set appropriate CPU and memory limits (Kubernetes)
   ```yaml
   resources:
     limits:
       memory: "2Gi"
       cpu: "1000m"
   ```

### Scalability

1. **Horizontal Scaling**: Deploy multiple workers for the same queue
   ```yaml
   replicas: 5  # Scale workers based on load
   ```

2. **Queue Separation**: Use different queues for different workload types
   ```bash
   # High priority queue
   kubiya worker start --queue-id=high-priority

   # Low priority queue
   kubiya worker start --queue-id=low-priority
   ```

3. **Auto-scaling**: Implement HPA based on CPU/memory metrics
   ```yaml
   # See Kubernetes HPA example above
   ```

### Security

1. **API Key Management**: Use secrets management for API keys
   ```yaml
   # Kubernetes Secret
   valueFrom:
     secretKeyRef:
       name: kubiya-secrets
       key: api-key
   ```

2. **Network Policies**: Restrict worker network access
   ```yaml
   # Allow only necessary connections
   - control-plane.kubiya.ai:443
   - temporal.kubiya.ai:7233
   ```

3. **Least Privilege**: Run workers with minimal permissions
   ```yaml
   securityContext:
     runAsNonRoot: true
     runAsUser: 1000
     readOnlyRootFilesystem: true
   ```

### Maintenance

1. **Regular Updates**: Keep worker images updated
   ```bash
   # Pull latest image
   docker pull ghcr.io/kubiyabot/agent-worker:latest
   ```

2. **Health Monitoring**: Set up alerts for worker failures
   ```yaml
   # Prometheus/Grafana metrics
   ```

3. **Backup Configurations**: Version control worker configurations
   ```bash
   # Store configs in Git
   git add worker-config.yaml
   ```

4. **Graceful Shutdown**: Always use SIGTERM for worker shutdown
   ```bash
   # Kubernetes handles this automatically
   kill <pid>  # Not kill -9
   ```

## Advanced Topics

### Custom Worker Extensions

Workers support custom activities and workflows:

```python
# custom_activities.py
from temporalio import activity

@activity.defn
async def custom_activity(input: str) -> str:
    # Custom logic
    return f"Processed: {input}"
```

### Multi-Region Deployment

Deploy workers across multiple regions for high availability:

```yaml
# us-east-1
kubiya worker start --queue-id=us-east-1-queue

# eu-west-1
kubiya worker start --queue-id=eu-west-1-queue
```

### Monitoring Integration

Integrate with monitoring systems:

```yaml
# Prometheus ServiceMonitor
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kubiya-worker
spec:
  selector:
    matchLabels:
      app: kubiya-worker
  endpoints:
  - port: metrics
    interval: 30s
```

## Support

For issues and questions:

- **Documentation**: https://docs.kubiya.ai
- **GitHub Issues**: https://github.com/kubiyabot/cli/issues
- **Control Plane**: https://control-plane.kubiya.ai

## Version History

- **v1.0.0**: Initial worker release
- **v1.1.0**: Added daemon mode support
- **v1.2.0**: Docker mode and Kubernetes support
- **v1.3.0**: Custom control plane URL support
