---
layout: page
title: API Reference
description: Complete API reference for Kubiya CLI integration
toc: true
---

## Base URL and Authentication

### Base URL

```
https://api.kubiya.ai/api/v1
```

### Authentication

All API requests require authentication using an API key:

```bash
# Header-based authentication
curl -H "Authorization: Bearer YOUR_API_KEY" \
  https://api.kubiya.ai/api/v1/agents

# Or using the CLI
export KUBIYA_API_KEY="YOUR_API_KEY"
kubiya agent list
```

## Agents API

### List Agents

```http
GET /agents
```

**Parameters:**
- `limit` (integer): Number of agents to return (default: 50)
- `offset` (integer): Offset for pagination (default: 0)
- `filter` (string): Filter by name or description
- `sort` (string): Sort field (name, created_at, updated_at)
- `active` (boolean): Filter by active status

**Response:**
```json
{
  "agents": [
    {
      "uuid": "abc-123-def",
      "name": "DevOps Agent",
      "description": "Handles DevOps automation tasks",
      "status": "active",
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z",
      "sources": ["source-uuid-1", "source-uuid-2"],
      "environment_variables": {
        "AWS_REGION": "us-west-2",
        "DEBUG": "true"
      },
      "secrets": ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"],
      "runner": "k8s-runner",
      "webhook": {
        "method": "slack",
        "destination": "#alerts",
        "prompt": "Process this alert"
      }
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Get Agent

```http
GET /agents/{uuid}
```

**Response:**
```json
{
  "uuid": "abc-123-def",
  "name": "DevOps Agent",
  "description": "Handles DevOps automation tasks",
  "status": "active",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:00:00Z",
  "sources": ["source-uuid-1"],
  "environment_variables": {
    "AWS_REGION": "us-west-2"
  },
  "secrets": ["AWS_ACCESS_KEY_ID"],
  "runner": "k8s-runner",
  "ai_instructions": "You are a helpful DevOps automation assistant",
  "llm_model": "gpt-4",
  "webhook": {
    "method": "slack",
    "destination": "#alerts",
    "prompt": "Process this alert"
  }
}
```

### Create Agent

```http
POST /agents
```

**Request Body:**
```json
{
  "name": "DevOps Agent",
  "description": "Handles DevOps automation tasks",
  "sources": ["source-uuid-1"],
  "environment_variables": {
    "AWS_REGION": "us-west-2",
    "DEBUG": "true"
  },
  "secrets": ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"],
  "runner": "k8s-runner",
  "ai_instructions": "You are a helpful DevOps automation assistant",
  "llm_model": "gpt-4",
  "webhook": {
    "method": "slack",
    "destination": "#alerts",
    "prompt": "Process this alert"
  }
}
```

**Response:**
```json
{
  "uuid": "abc-123-def",
  "name": "DevOps Agent",
  "description": "Handles DevOps automation tasks",
  "status": "active",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:00:00Z"
}
```

### Update Agent

```http
PUT /agents/{uuid}
```

**Request Body:**
```json
{
  "name": "Updated DevOps Agent",
  "description": "Enhanced DevOps automation tasks",
  "environment_variables": {
    "AWS_REGION": "us-east-1",
    "DEBUG": "false",
    "LOG_LEVEL": "info"
  }
}
```

### Delete Agent

```http
DELETE /agents/{uuid}
```

**Response:**
```json
{
  "message": "Agent deleted successfully"
}
```

## Workflows API

### Execute Workflow

```http
POST /workflows/execute
```

**Request Body:**
```json
{
  "source": "myorg/deploy-scripts",
  "type": "github",
  "variables": {
    "environment": "production",
    "version": "v1.2.3"
  },
  "runner": "k8s-runner",
  "timeout": "1800s",
  "save_trace": true
}
```

**Response:**
```json
{
  "execution_id": "exec-123-456",
  "status": "running",
  "created_at": "2024-01-15T10:00:00Z",
  "workflow": {
    "name": "deploy-application",
    "source": "myorg/deploy-scripts",
    "variables": {
      "environment": "production",
      "version": "v1.2.3"
    }
  }
}
```

### Get Workflow Execution

```http
GET /workflows/executions/{execution_id}
```

**Response:**
```json
{
  "execution_id": "exec-123-456",
  "status": "completed",
  "created_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:15:00Z",
  "duration": "15m",
  "workflow": {
    "name": "deploy-application",
    "source": "myorg/deploy-scripts"
  },
  "steps": [
    {
      "name": "build",
      "status": "completed",
      "duration": "5m",
      "output": "Build completed successfully"
    },
    {
      "name": "deploy",
      "status": "completed", 
      "duration": "10m",
      "output": "Deployment successful"
    }
  ]
}
```

### List Workflow Executions

```http
GET /workflows/executions
```

**Parameters:**
- `limit` (integer): Number of executions to return
- `offset` (integer): Offset for pagination
- `status` (string): Filter by status (running, completed, failed)
- `workflow` (string): Filter by workflow name

**Response:**
```json
{
  "executions": [
    {
      "execution_id": "exec-123-456",
      "status": "completed",
      "created_at": "2024-01-15T10:00:00Z",
      "completed_at": "2024-01-15T10:15:00Z",
      "workflow": {
        "name": "deploy-application",
        "source": "myorg/deploy-scripts"
      }
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Get Workflow Logs

```http
GET /workflows/executions/{execution_id}/logs
```

**Parameters:**
- `step` (string): Filter by step name
- `limit` (integer): Number of log entries
- `since` (string): RFC3339 timestamp

**Response:**
```json
{
  "logs": [
    {
      "timestamp": "2024-01-15T10:00:00Z",
      "level": "info",
      "step": "build",
      "message": "Starting build process"
    },
    {
      "timestamp": "2024-01-15T10:05:00Z",
      "level": "info",
      "step": "build",
      "message": "Build completed successfully"
    }
  ]
}
```

## Tools API

### List Tools

```http
GET /tools
```

**Parameters:**
- `source` (string): Filter by source UUID
- `type` (string): Filter by tool type
- `limit` (integer): Number of tools to return
- `offset` (integer): Offset for pagination

**Response:**
```json
{
  "tools": [
    {
      "uuid": "tool-123-456",
      "name": "kubectl",
      "description": "Kubernetes CLI tool",
      "type": "docker",
      "image": "kubiya/kubectl:latest",
      "source": "source-uuid-1",
      "args": [
        {
          "name": "command",
          "type": "string",
          "description": "kubectl command to execute",
          "required": true
        }
      ]
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Execute Tool

```http
POST /tools/execute
```

**Request Body:**
```json
{
  "name": "kubectl",
  "type": "docker",
  "image": "kubiya/kubectl:latest",
  "content": "get pods -n default",
  "runner": "k8s-runner",
  "timeout": "60s",
  "environment": {
    "KUBECONFIG": "/etc/kubeconfig"
  },
  "integrations": ["kubernetes/incluster"]
}
```

**Response:**
```json
{
  "execution_id": "tool-exec-123",
  "status": "running",
  "created_at": "2024-01-15T10:00:00Z"
}
```

### Get Tool Execution

```http
GET /tools/executions/{execution_id}
```

**Response:**
```json
{
  "execution_id": "tool-exec-123",
  "status": "completed",
  "created_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:01:00Z",
  "duration": "1m",
  "output": "NAME                     READY   STATUS    RESTARTS   AGE\nmy-app-7d4b8c8f9-abc123   1/1     Running   0          1h",
  "exit_code": 0
}
```

## Sources API

### List Sources

```http
GET /sources
```

**Parameters:**
- `limit` (integer): Number of sources to return
- `offset` (integer): Offset for pagination
- `filter` (string): Filter by name or description

**Response:**
```json
{
  "sources": [
    {
      "uuid": "source-123-456",
      "name": "DevOps Tools",
      "description": "Collection of DevOps automation tools",
      "type": "github",
      "url": "https://github.com/org/devops-tools",
      "branch": "main",
      "path": "/tools",
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z",
      "tools_count": 15
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Get Source

```http
GET /sources/{uuid}
```

**Response:**
```json
{
  "uuid": "source-123-456",
  "name": "DevOps Tools",
  "description": "Collection of DevOps automation tools",
  "type": "github",
  "url": "https://github.com/org/devops-tools",
  "branch": "main",
  "path": "/tools",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:00:00Z",
  "tools": [
    {
      "name": "kubectl",
      "type": "docker",
      "description": "Kubernetes CLI tool"
    }
  ]
}
```

### Create Source

```http
POST /sources
```

**Request Body:**
```json
{
  "name": "DevOps Tools",
  "description": "Collection of DevOps automation tools",
  "type": "github",
  "url": "https://github.com/org/devops-tools",
  "branch": "main",
  "path": "/tools",
  "runner": "default-runner"
}
```

**Response:**
```json
{
  "uuid": "source-123-456",
  "name": "DevOps Tools",
  "status": "scanning",
  "created_at": "2024-01-15T10:00:00Z"
}
```

### Scan Source

```http
POST /sources/scan
```

**Request Body:**
```json
{
  "url": "https://github.com/org/devops-tools",
  "branch": "main",
  "path": "/tools",
  "runner": "default-runner"
}
```

**Response:**
```json
{
  "scan_id": "scan-123-456",
  "status": "running",
  "created_at": "2024-01-15T10:00:00Z",
  "tools_found": []
}
```

## Secrets API

### List Secrets

```http
GET /secrets
```

**Parameters:**
- `limit` (integer): Number of secrets to return
- `offset` (integer): Offset for pagination
- `filter` (string): Filter by name

**Response:**
```json
{
  "secrets": [
    {
      "name": "DB_PASSWORD",
      "description": "Database password",
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z",
      "expires_at": "2024-07-15T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Create Secret

```http
POST /secrets
```

**Request Body:**
```json
{
  "name": "DB_PASSWORD",
  "value": "secure-password-123",
  "description": "Database password",
  "expires_in": "30d"
}
```

**Response:**
```json
{
  "name": "DB_PASSWORD",
  "description": "Database password",
  "created_at": "2024-01-15T10:00:00Z",
  "expires_at": "2024-02-15T10:00:00Z"
}
```

### Update Secret

```http
PUT /secrets/{name}
```

**Request Body:**
```json
{
  "value": "new-secure-password-456",
  "description": "Updated database password"
}
```

### Delete Secret

```http
DELETE /secrets/{name}
```

**Response:**
```json
{
  "message": "Secret deleted successfully"
}
```

## Runners API

### List Runners

```http
GET /runners
```

**Parameters:**
- `healthy` (boolean): Filter by health status
- `limit` (integer): Number of runners to return
- `offset` (integer): Offset for pagination

**Response:**
```json
{
  "runners": [
    {
      "name": "k8s-runner",
      "status": "healthy",
      "type": "kubernetes",
      "capacity": {
        "cpu": "4",
        "memory": "8Gi"
      },
      "usage": {
        "cpu": "2",
        "memory": "4Gi"
      },
      "created_at": "2024-01-15T10:00:00Z",
      "last_seen": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Get Runner

```http
GET /runners/{name}
```

**Response:**
```json
{
  "name": "k8s-runner",
  "status": "healthy",
  "type": "kubernetes",
  "capacity": {
    "cpu": "4",
    "memory": "8Gi"
  },
  "usage": {
    "cpu": "2",
    "memory": "4Gi"
  },
  "configuration": {
    "kubernetes_namespace": "kubiya-runners",
    "image_pull_policy": "Always"
  },
  "created_at": "2024-01-15T10:00:00Z",
  "last_seen": "2024-01-15T10:00:00Z"
}
```

### Get Runner Manifest

```http
GET /runners/{name}/manifest
```

**Parameters:**
- `format` (string): Output format (yaml, json)
- `type` (string): Deployment type (kubernetes, docker)

**Response:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubiya-runner
  namespace: kubiya-runners
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubiya-runner
  template:
    metadata:
      labels:
        app: kubiya-runner
    spec:
      containers:
      - name: runner
        image: kubiya/runner:latest
        env:
        - name: KUBIYA_API_KEY
          valueFrom:
            secretKeyRef:
              name: kubiya-secret
              key: api-key
```

## Webhooks API

### List Webhooks

```http
GET /webhooks
```

**Parameters:**
- `type` (string): Filter by webhook type
- `limit` (integer): Number of webhooks to return
- `offset` (integer): Offset for pagination

**Response:**
```json
{
  "webhooks": [
    {
      "uuid": "webhook-123-456",
      "name": "Alert Handler",
      "type": "slack",
      "destination": "#alerts",
      "prompt": "Process this alert",
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### Create Webhook

```http
POST /webhooks
```

**Request Body:**
```json
{
  "name": "Alert Handler",
  "type": "slack",
  "destination": "#alerts",
  "prompt": "Process this alert and take appropriate action"
}
```

**Response:**
```json
{
  "uuid": "webhook-123-456",
  "name": "Alert Handler",
  "type": "slack",
  "destination": "#alerts",
  "webhook_url": "https://api.kubiya.ai/webhooks/webhook-123-456",
  "created_at": "2024-01-15T10:00:00Z"
}
```

### Trigger Webhook

```http
POST /webhooks/{uuid}/trigger
```

**Request Body:**
```json
{
  "message": "Alert: High CPU usage detected on server-01",
  "metadata": {
    "severity": "high",
    "source": "monitoring",
    "timestamp": "2024-01-15T10:00:00Z"
  }
}
```

**Response:**
```json
{
  "execution_id": "webhook-exec-123",
  "status": "running",
  "created_at": "2024-01-15T10:00:00Z"
}
```

## Chat API

### Send Message

```http
POST /chat
```

**Request Body:**
```json
{
  "agent_uuid": "abc-123-def",
  "message": "Check the status of the production deployment",
  "context": [
    {
      "type": "file",
      "content": "apiVersion: apps/v1\nkind: Deployment...",
      "filename": "deployment.yaml"
    },
    {
      "type": "url",
      "url": "https://example.com/config.yaml"
    }
  ],
  "stream": true
}
```

**Response (non-streaming):**
```json
{
  "response": "I'll check the production deployment status for you...",
  "execution_id": "chat-exec-123",
  "created_at": "2024-01-15T10:00:00Z"
}
```

**Response (streaming):**
```
data: {"type": "message", "content": "I'll check the production deployment status"}
data: {"type": "tool_call", "tool": "kubectl", "args": {"command": "get deployments -n production"}}
data: {"type": "tool_result", "result": "NAME     READY   UP-TO-DATE   AVAILABLE   AGE\nmy-app   3/3     3            3           1h"}
data: {"type": "message", "content": "The production deployment is healthy with 3/3 replicas ready."}
data: {"type": "done"}
```

### Get Chat History

```http
GET /chat/{agent_uuid}/history
```

**Parameters:**
- `limit` (integer): Number of messages to return
- `offset` (integer): Offset for pagination
- `since` (string): RFC3339 timestamp

**Response:**
```json
{
  "messages": [
    {
      "id": "msg-123",
      "role": "user",
      "content": "Check the status of the production deployment",
      "timestamp": "2024-01-15T10:00:00Z"
    },
    {
      "id": "msg-124",
      "role": "assistant",
      "content": "The production deployment is healthy with 3/3 replicas ready.",
      "timestamp": "2024-01-15T10:01:00Z"
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0
}
```

## Integrations API

### List Integrations

```http
GET /integrations
```

**Parameters:**
- `type` (string): Filter by integration type
- `status` (string): Filter by status

**Response:**
```json
{
  "integrations": [
    {
      "uuid": "integration-123",
      "name": "GitHub Integration",
      "type": "github",
      "status": "active",
      "configuration": {
        "organization": "myorg",
        "repositories": ["repo1", "repo2"]
      },
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

## Error Responses

All API endpoints return consistent error responses:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": {
      "field": "name",
      "error": "Name is required"
    }
  },
  "request_id": "req-123-456"
}
```

### Common Error Codes

| Code | Description |
|------|-------------|
| `AUTHENTICATION_ERROR` | Invalid or missing API key |
| `AUTHORIZATION_ERROR` | Insufficient permissions |
| `VALIDATION_ERROR` | Invalid request parameters |
| `NOT_FOUND` | Resource not found |
| `CONFLICT` | Resource already exists |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `INTERNAL_ERROR` | Server error |
| `TIMEOUT` | Request timeout |

## Rate Limiting

The API implements rate limiting with the following headers:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1642234567
```

Default limits:
- 1000 requests per hour for authenticated users
- 100 requests per hour for unauthenticated users

## Pagination

List endpoints support pagination with the following parameters:

- `limit`: Number of items per page (default: 50, max: 100)
- `offset`: Number of items to skip (default: 0)

Response includes pagination metadata:

```json
{
  "data": [...],
  "total": 150,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

## Webhooks

### Webhook Events

Kubiya can send webhook events for various activities:

```json
{
  "event": "agent.created",
  "data": {
    "agent": {
      "uuid": "abc-123-def",
      "name": "DevOps Agent",
      "status": "active"
    }
  },
  "timestamp": "2024-01-15T10:00:00Z"
}
```

### Event Types

- `agent.created`
- `agent.updated`
- `agent.deleted`
- `workflow.started`
- `workflow.completed`
- `workflow.failed`
- `tool.executed`
- `source.added`
- `source.updated`

### Webhook Configuration

Configure webhooks in your organization settings or via API:

```json
{
  "url": "https://your-app.com/webhooks/kubiya",
  "events": ["workflow.completed", "workflow.failed"],
  "secret": "your-webhook-secret"
}
```

## SDK Examples

### Python

```python
import requests

class KubiyaClient:
    def __init__(self, api_key, base_url="https://api.kubiya.ai/api/v1"):
        self.api_key = api_key
        self.base_url = base_url
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        }
    
    def list_agents(self):
        response = requests.get(
            f"{self.base_url}/agents",
            headers=self.headers
        )
        return response.json()
    
    def create_agent(self, name, description, **kwargs):
        data = {
            "name": name,
            "description": description,
            **kwargs
        }
        response = requests.post(
            f"{self.base_url}/agents",
            json=data,
            headers=self.headers
        )
        return response.json()

# Usage
client = KubiyaClient("your-api-key")
agents = client.list_agents()
```

### JavaScript

```javascript
class KubiyaClient {
    constructor(apiKey, baseUrl = "https://api.kubiya.ai/api/v1") {
        this.apiKey = apiKey;
        this.baseUrl = baseUrl;
        this.headers = {
            "Authorization": `Bearer ${apiKey}`,
            "Content-Type": "application/json"
        };
    }
    
    async listAgents() {
        const response = await fetch(`${this.baseUrl}/agents`, {
            headers: this.headers
        });
        return response.json();
    }
    
    async createAgent(name, description, options = {}) {
        const response = await fetch(`${this.baseUrl}/agents`, {
            method: "POST",
            headers: this.headers,
            body: JSON.stringify({
                name,
                description,
                ...options
            })
        });
        return response.json();
    }
}

// Usage
const client = new KubiyaClient("your-api-key");
const agents = await client.listAgents();
```

This API reference provides comprehensive documentation for integrating with Kubiya's platform programmatically. For more examples and advanced usage, refer to our [SDK documentation](https://docs.kubiya.ai/sdk).