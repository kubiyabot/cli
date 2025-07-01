# Kubiya CLI Workflow Commands

The Kubiya CLI provides a comprehensive set of commands for creating, testing, and executing workflows.

## Available Commands

### 1. `kubiya workflow test`
**Status: ✅ Fully Functional**

Test a workflow by executing it with real-time streaming output.

```bash
# Test a workflow from a YAML file
kubiya workflow test my-workflow.yaml

# Test with a specific runner
kubiya workflow test deploy.yaml --runner core-testing-2

# Test with variables
kubiya workflow test backup.yaml --var env=staging --var bucket=backup-staging
```

**Supported formats:**
- YAML workflows with step definitions
- JSON workflows matching the Kubiya API format

### 2. `kubiya workflow execute`
**Status: ✅ Fully Functional**

Execute a workflow from a file with detailed progress tracking.

```bash
# Execute a workflow
kubiya workflow execute production-deploy.yaml

# Execute with variables
kubiya workflow execute deploy.yaml --var version=1.2.3 --var env=prod

# Execute with a specific runner
kubiya workflow execute backup.yaml --runner prod-runner
```

### 3. `kubiya workflow generate`
**Status: ⚠️ Requires Orchestration API Access**

Generate workflows from natural language descriptions.

```bash
# Generate a workflow (requires orchestration API)
kubiya workflow generate "create a workflow to deploy apps to k8s"

# Generate and save to file
kubiya workflow generate "backup database to S3" -o backup.yaml

# Generate with specific mode
kubiya workflow generate "test my application" --mode plan
```

**Note:** This command requires access to the Kubiya Orchestration API, which may need:
- Special authentication tokens
- Orchestration features enabled in your account
- Custom orchestrator URL configuration

**Workaround:** If the orchestration API is not available, the command will fall back to generating a basic template locally.

### 4. `kubiya workflow compose`
**Status: ⚠️ Requires Orchestration API Access**

Compose and execute workflows from natural language in one step.

```bash
# Compose and run a workflow
kubiya workflow compose "check disk usage on all servers and alert if above 80%"

# Compose with variables
kubiya workflow compose "deploy version to environment" --var version=2.0 --var environment=staging
```

## Workflow File Formats

### YAML Format
```yaml
name: example-workflow
description: Example workflow demonstrating features
steps:
  - name: first-step
    description: Run a simple command
    command: echo "Hello from step 1"
    output: STEP1_OUTPUT
    
  - name: second-step
    description: Use output from previous step
    command: echo "Previous output was: ${STEP1_OUTPUT}"
    depends:
      - first-step
```

### JSON Format
```json
{
  "name": "example-workflow",
  "description": "Example workflow in JSON format",
  "steps": [
    {
      "name": "check-status",
      "command": "systemctl status nginx",
      "output": "SERVICE_STATUS"
    },
    {
      "name": "restart-if-needed",
      "command": "[ \"$SERVICE_STATUS\" = \"inactive\" ] && systemctl restart nginx || echo 'Service is running'",
      "depends": ["check-status"]
    }
  ]
}
```

## Authentication

The CLI uses your configured API key for authentication:

```bash
# Set your API key
export KUBIYA_API_KEY="your-api-key"

# Or configure it
kubiya config set api_key your-api-key
```

## Environment Variables

- `KUBIYA_API_KEY` - Your Kubiya API key
- `KUBIYA_API_URL` - Custom API URL (default: https://api.kubiya.ai)
- `KUBIYA_ORCHESTRATOR_URL` - Custom orchestrator URL for workflow generation
- `KUBIYA_USE_SAME_API` - Set to "true" to use the main API for orchestration

## Examples

### Kubernetes Deployment Workflow
```yaml
name: k8s-deployment
description: Deploy application to Kubernetes
steps:
  - name: validate-config
    command: kubectl config current-context
    output: CONTEXT
    
  - name: apply-manifests
    command: kubectl apply -f k8s/
    depends: ["validate-config"]
    output: APPLY_RESULT
    
  - name: wait-for-rollout
    command: kubectl rollout status deployment/{{.app_name}}
    depends: ["apply-manifests"]
```

### Database Backup Workflow
```yaml
name: database-backup
description: Backup database and upload to S3
steps:
  - name: create-backup
    command: pg_dump -h {{.db_host}} -U {{.db_user}} {{.db_name}} > backup.sql
    output: BACKUP_FILE
    
  - name: compress-backup
    command: gzip backup.sql
    depends: ["create-backup"]
    
  - name: upload-to-s3
    command: aws s3 cp backup.sql.gz s3://{{.bucket}}/backups/$(date +%Y%m%d)/
    depends: ["compress-backup"]
```

## Troubleshooting

### Orchestration API Issues
If you encounter 404 or authentication errors with `generate` or `compose` commands:

1. **Check API Access**: Ensure your account has orchestration features enabled
2. **Use Direct Execution**: Use `workflow test` or `workflow execute` with pre-written workflows
3. **Local Templates**: The generate command will fall back to local template generation
4. **Custom URL**: Set `KUBIYA_ORCHESTRATOR_URL` if you have a custom orchestrator endpoint

### Streaming Issues
If you don't see real-time output:
- Ensure your workflow steps output to stdout
- Check that the runner supports streaming
- Verify network connectivity to the Kubiya API

### Authentication Errors
- Verify your API key is correct
- Check if your key has the necessary permissions
- Try regenerating your API key from the Kubiya dashboard 