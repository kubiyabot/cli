# Kubiya Tool Execution Examples

This document provides comprehensive examples of using the `kubiya tool exec` command to execute tools with various configurations.

## Table of Contents
- [Basic Usage](#basic-usage)
- [Docker Images](#docker-images)
- [Loading Tools from URLs](#loading-tools-from-urls)
- [Loading Tools from Sources](#loading-tools-from-sources)
- [Integration Templates](#integration-templates)
- [Advanced Properties](#advanced-properties)
- [Long-Running Tasks](#long-running-tasks)
- [JSON Input Methods](#json-input-methods)
- [Output Formats](#output-formats)
- [Environment Variables](#environment-variables)

## Basic Usage

### Simple Command
```bash
kubiya tool exec --name "hello" --content "echo Hello World"
```

### With Description
```bash
kubiya tool exec --name "date-check" --description "Check current date and time" --content "date"
```

### Specify Tool Type
```bash
kubiya tool exec --name "python-hello" --type docker --content "print('Hello from Python')"
```

## Docker Images

### Custom Docker Image
```bash
kubiya tool exec --name "python-script" --type docker --image python:3.11 \
  --content "print('Hello from Python 3.11')"
```

### Alpine Linux
```bash
kubiya tool exec --name "alpine-test" --type docker --image alpine:latest \
  --content "echo 'Running in Alpine Linux'"
```

### Node.js
```bash
kubiya tool exec --name "node-version" --type docker --image node:18 \
  --content "node --version && npm --version"
```

## Loading Tools from URLs

### Load Tool Definition from URL
```bash
# Load a tool definition from a public URL
kubiya tool exec --tool-url https://raw.githubusercontent.com/kubiyabot/community-tools/main/aws/tools/ec2_describe_instances.json

# Load and execute with custom runner
kubiya tool exec --tool-url https://example.com/tools/my-tool.json --runner core-testing-1

# Load with timeout override
kubiya tool exec --tool-url https://example.com/tools/long-running-tool.json --timeout 600
```

### Load YAML Tool Definition
```bash
# Load a YAML tool definition (currently parsed as JSON)
kubiya tool exec --tool-url https://raw.githubusercontent.com/kubiyabot/community-tools/main/kubernetes/tools/get_pods.yaml
```

## Loading Tools from Sources

### Execute Tool from Source UUID
```bash
# Execute a specific tool from a source
kubiya tool exec --source-uuid 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56 --name ec2_describe_instances

# With custom arguments
kubiya tool exec --source-uuid 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56 --name ec2_describe_instances \
  --arg instance_ids:string:"i-1234567890abcdef0":false

# With environment override
kubiya tool exec --source-uuid 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56 --name ec2_describe_instances \
  --env AWS_PROFILE=production
```

### List Available Tools in a Source
```bash
# First, list tools in a source
kubiya tool list --source 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56

# Then execute a specific tool
kubiya tool exec --source-uuid 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56 --name rds_describe_instances
```

## Integration Templates

Integration templates provide comprehensive, pre-configured setups for various platforms. They automatically handle authentication, environment setup, and necessary configurations.

### Available Integrations

| Integration | Description | Auto-Configuration |
|-------------|-------------|-------------------|
| `kubernetes/incluster` | In-cluster K8s auth | Service account token, kubectl config |
| `kubernetes/kubeconfig` | Local kubeconfig | Maps ~/.kube/config |
| `kubernetes/eks` | Amazon EKS | AWS creds + EKS kubeconfig update |
| `kubernetes/gke` | Google GKE | GCloud auth + cluster credentials |
| `aws/cli` | AWS CLI with profile | Maps AWS credentials/config |
| `aws/iam-role` | AWS with role assumption | Auto-assumes specified IAM role |
| `aws/env` | AWS from env vars | Uses AWS env variables |
| `gcp/adc` | GCP Application Default | Maps ADC credentials |
| `gcp/service-account` | GCP service account | Activates service account |
| `azure/cli` | Azure CLI | Maps Azure config |
| `azure/sp` | Azure service principal | Auto-login with SP |
| `docker/socket` | Docker socket access | Maps docker.sock |
| `docker/dind` | Docker-in-Docker | Full Docker daemon |
| `terraform/aws` | Terraform + AWS | AWS creds + terraform init |
| `ansible/ssh` | Ansible with SSH | SSH keys + permissions |
| `git/ssh` | Git over SSH | SSH keys + git config |
| `database/postgres` | PostgreSQL client | Auto-waits for DB ready |
| `database/mysql` | MySQL client | Auto-waits for DB ready |
| `cache/redis` | Redis client | Auto-waits for Redis |

### Kubernetes Integrations

#### In-Cluster Authentication
```bash
# Automatically configures kubectl with service account
kubiya tool exec --name "k8s-pods" \
  --content "kubectl get pods -A" \
  --integration kubernetes/incluster

# The integration automatically:
# - Maps service account token and CA cert
# - Configures kubectl context
# - Sets up KUBERNETES_SERVICE_HOST/PORT env vars
```

#### EKS Authentication
```bash
# Automatically updates kubeconfig for EKS
kubiya tool exec --name "eks-nodes" \
  --content "kubectl get nodes" \
  --integration kubernetes/eks \
  --env EKS_CLUSTER_NAME=my-cluster \
  --env AWS_REGION=us-east-1

# The integration automatically:
# - Maps AWS credentials
# - Runs: aws eks update-kubeconfig
# - Configures kubectl for EKS
```

#### GKE Authentication
```bash
# Automatically gets GKE credentials
kubiya tool exec --name "gke-pods" \
  --content "kubectl get pods" \
  --integration kubernetes/gke \
  --env GKE_CLUSTER_NAME=my-cluster \
  --env GKE_CLUSTER_ZONE=us-central1-a \
  --env GCP_PROJECT=my-project

# The integration automatically:
# - Maps gcloud config
# - Runs: gcloud container clusters get-credentials
# - Sets up kubectl for GKE
```

### AWS Integrations

#### AWS CLI with Profile
```bash
# Uses local AWS profile
kubiya tool exec --name "s3-list" \
  --content "aws s3 ls" \
  --integration aws/cli \
  --env AWS_PROFILE=production

# The integration automatically:
# - Maps ~/.aws/credentials and ~/.aws/config
# - Sets default region
# - Uses specified profile
```

#### AWS with IAM Role Assumption
```bash
# Automatically assumes IAM role
kubiya tool exec --name "ec2-admin" \
  --content "aws ec2 describe-instances" \
  --integration aws/iam-role \
  --env AWS_ROLE_ARN=arn:aws:iam::123456789012:role/AdminRole

# The integration automatically:
# - Maps AWS credentials
# - Assumes the specified role using STS
# - Exports temporary credentials
# - Runs your command with assumed role
```

### Database Integrations

#### PostgreSQL with Auto-Ready Check
```bash
# Runs with PostgreSQL service and waits until ready
kubiya tool exec --name "db-query" \
  --content "psql -c 'SELECT version();'" \
  --integration database/postgres \
  --env PGDATABASE=myapp \
  --env PGPASSWORD=secret123

# The integration automatically:
# - Starts PostgreSQL service container
# - Waits for database to be ready (up to 30s)
# - Sets all PG* environment variables
# - Runs your query
```

#### MySQL with Auto-Ready Check
```bash
# Runs with MySQL service
kubiya tool exec --name "mysql-query" \
  --content "mysql -e 'SHOW DATABASES;'" \
  --integration database/mysql \
  --env MYSQL_ROOT_PASSWORD=root123

# The integration automatically:
# - Starts MySQL service container
# - Waits for MySQL to accept connections
# - Sets MySQL environment variables
# - Runs your query
```

### Complex Integration Examples

#### Multi-Cloud Deployment
```bash
# Deploy to K8s on EKS using Terraform
kubiya tool exec --name "deploy-app" \
  --content 'terraform apply -auto-approve && kubectl apply -f k8s/' \
  --integration kubernetes/eks \
  --integration terraform/aws \
  --env EKS_CLUSTER_NAME=prod-cluster \
  --env AWS_REGION=us-east-1 \
  --with-file ./terraform:/workspace \
  --with-file ./k8s:/k8s

# Multiple integrations work together:
# - terraform/aws: Sets up AWS credentials and runs terraform init
# - kubernetes/eks: Updates kubeconfig for the EKS cluster
# - Both auth methods are available in the execution
```

#### Database Migration with Git
```bash
# Pull latest migrations and apply to database
kubiya tool exec --name "db-migrate" \
  --content '
    git pull origin main
    for migration in migrations/*.sql; do
      echo "Applying $migration..."
      psql < "$migration"
    done
  ' \
  --integration git/ssh \
  --integration database/postgres \
  --env PGDATABASE=production

# Integrations provide:
# - git/ssh: SSH keys and git config for pulling
# - database/postgres: Database connection with ready check
```

### How Integrations Work

When you use `--integration`, the system automatically:

1. **Sets the appropriate Docker image** (if not specified)
2. **Runs setup scripts** before your code
3. **Maps required files** (credentials, configs, tokens)
4. **Sets environment variables**
5. **Installs required packages** if needed
6. **Waits for services** to be ready
7. **Runs your script** in the prepared environment
8. **Runs cleanup scripts** if defined

Example of what happens behind the scenes:
```bash
# When you run:
kubiya tool exec --name "k8s-test" \
  --content "kubectl get nodes" \
  --integration kubernetes/incluster

# The system automatically generates:
{
  "name": "k8s-test",
  "image": "bitnami/kubectl:latest",  # Auto-set by integration
  "content": "#!/bin/bash
set -e

# Integration setup
#!/bin/bash
# Setup Kubernetes in-cluster authentication
if [ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]; then
    export KUBE_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    kubectl config set-cluster in-cluster \\
        --server=https://kubernetes.default.svc \\
        --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt > /dev/null 2>&1
    kubectl config set-credentials in-cluster --token=$KUBE_TOKEN > /dev/null 2>&1
    kubectl config set-context in-cluster --cluster=in-cluster --user=in-cluster > /dev/null 2>&1
    kubectl config use-context in-cluster > /dev/null 2>&1
    echo \"✓ Kubernetes in-cluster authentication configured\"
fi

# User script
kubectl get nodes",
  "with_files": [
    {
      "source": "/var/run/secrets/kubernetes.io/serviceaccount/token",
      "destination": "/var/run/secrets/kubernetes.io/serviceaccount/token"
    },
    {
      "source": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
      "destination": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
    }
  ],
  "env": [
    "KUBERNETES_SERVICE_HOST=kubernetes.default.svc",
    "KUBERNETES_SERVICE_PORT=443"
  ]
}
```

### Custom Integrations

You can create custom integrations in `~/.kubiya/tool-integrations.json`:

```json
{
  "mycompany/vault": {
    "name": "mycompany/vault",
    "description": "HashiCorp Vault integration for my company",
    "type": "secrets",
    "default_image": "vault:latest",
    "before_script": "#!/bin/bash\nvault login -method=aws\nexport DB_PASSWORD=$(vault kv get -field=password secret/db)\necho '✓ Vault secrets loaded'\n",
    "env_vars": {
      "VAULT_ADDR": "https://vault.mycompany.com"
    },
    "with_files": [
      {
        "source": "~/.vault-token",
        "destination": "/root/.vault-token"
      }
    ]
  }
}
```

Then use it:
```bash
kubiya tool exec --name "app-with-secrets" \
  --content "echo $DB_PASSWORD | myapp --db-pass-stdin" \
  --integration mycompany/vault
```

## Advanced Properties

### File Mappings
```bash
# Map local file to container
kubiya tool exec --name "config-reader" --content "cat /app/config.yaml" \
  --with-file ~/.config/myapp.yaml:/app/config.yaml

# Multiple file mappings
kubiya tool exec --name "multi-config" --content "ls -la /configs/" \
  --with-file ~/.config/app1.yaml:/configs/app1.yaml \
  --with-file ~/.config/app2.json:/configs/app2.json

# With AWS credentials (manual mapping)
kubiya tool exec --name "aws-manual" --content "aws s3 ls" \
  --with-file ~/.aws/credentials:/root/.aws/credentials \
  --with-file ~/.aws/config:/root/.aws/config \
  --env AWS_PROFILE=production
```

### Volume Mappings
```bash
# Mount Docker socket (read-only)
kubiya tool exec --name "docker-inspect" --content "docker ps -a" \
  --with-volume /var/run/docker.sock:/var/run/docker.sock:ro

# Mount directory
kubiya tool exec --name "analyze-logs" --content "grep ERROR /logs/*.log" \
  --with-volume /var/log/myapp:/logs:ro
```

### Service Dependencies
```bash
# With PostgreSQL service
kubiya tool exec --name "db-migration" --content "psql -h postgres -U admin -d mydb < migration.sql" \
  --with-service postgres:latest \
  --env PGPASSWORD=secret \
  --with-file ./migration.sql:/app/migration.sql

# With Redis service
kubiya tool exec --name "cache-test" --content "redis-cli -h redis ping" \
  --with-service redis:7-alpine
```

### Environment Variables
```bash
# Single environment variable
kubiya tool exec --name "env-test" --content "echo \$MY_VAR" \
  --env MY_VAR=hello

# Multiple environment variables
kubiya tool exec --name "app-config" --content "python app.py" \
  --env DATABASE_URL=postgres://localhost/mydb \
  --env API_KEY=secret123 \
  --env DEBUG=true
```

### Tool Arguments
```bash
# Define tool arguments
kubiya tool exec --name "ec2-describe" --type docker \
  --image amazon/aws-cli:latest \
  --content 'aws ec2 describe-instances $([[ -n "$instance_ids" ]] && echo "--instance-ids $instance_ids")' \
  --arg instance_ids:string:"Comma-separated instance IDs":false \
  --arg region:string:"AWS region":false \
  --env AWS_DEFAULT_REGION=us-east-1
```

### Complex AWS Example
```bash
# Full AWS tool with all properties
kubiya tool exec --name "ec2-operations" --type docker \
  --image amazon/aws-cli:latest \
  --description "Manage EC2 instances" \
  --content 'aws ec2 ${action} --instance-ids ${instance_id}' \
  --arg action:string:"Action to perform (start-instances, stop-instances, etc.)":true \
  --arg instance_id:string:"Instance ID":true \
  --with-file ~/.aws/credentials:/root/.aws/credentials \
  --with-file ~/.aws/config:/root/.aws/config \
  --env AWS_PROFILE=production \
  --icon-url "https://upload.wikimedia.org/wikipedia/commons/9/93/Amazon_Web_Services_Logo.svg"
```

### Kubernetes with Authentication Files
```bash
# Execute with Kubernetes in-cluster token files
kubiya tool exec --name "k8s-resource-usage" \
  --content 'TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) && \
            kubectl --token=$TOKEN --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
            --server=https://kubernetes.default.svc \
            top nodes' \
  --with-file /var/run/secrets/kubernetes.io/serviceaccount/token:/var/run/secrets/kubernetes.io/serviceaccount/token \
  --with-file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt:/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
```

## Long-Running Tasks

### With Custom Timeout
```bash
# 2 minute timeout
kubiya tool exec --name "long-job" --content "sleep 90 && echo 'Job completed'" \
  --timeout 120

# No timeout (run indefinitely)
kubiya tool exec --name "monitoring" --content "while true; do date; sleep 5; done" \
  --timeout 0
```

### Background Processing
```bash
# Process large dataset
kubiya tool exec --name "data-processing" --type docker --image python:3.11 \
  --content "python /app/process_data.py" \
  --with-file ./process_data.py:/app/process_data.py \
  --timeout 3600  # 1 hour
```

## JSON Input Methods

### Direct JSON Input
```bash
kubiya tool exec --json '{
  "name": "test-tool",
  "type": "docker",
  "image": "alpine",
  "content": "echo Hello from JSON"
}'
```

### From JSON File
```bash
# Create a tool definition file
cat > my-tool.json <<EOF
{
  "name": "complex-tool",
  "description": "A complex tool with multiple properties",
  "type": "docker",
  "image": "ubuntu:22.04",
  "content": "apt-get update && apt-get install -y curl && curl https://api.example.com",
  "env": ["API_TOKEN=secret"],
  "timeout": 300
}
EOF

# Execute from file
kubiya tool exec --json-file my-tool.json
```

### Complex Tool Definition
```bash
cat > aws-tool.json <<EOF
{
  "name": "aws-s3-sync",
  "description": "Sync files to S3",
  "type": "docker",
  "image": "amazon/aws-cli:latest",
  "content": "aws s3 sync /data s3://my-bucket/data/",
  "args": [
    {
      "name": "bucket",
      "type": "string",
      "description": "S3 bucket name",
      "required": true
    }
  ],
  "env": ["AWS_PROFILE=production"],
  "with_files": [
    {
      "source": "$HOME/.aws/credentials",
      "destination": "/root/.aws/credentials"
    }
  ],
  "with_volumes": [
    {
      "source": "./data",
      "destination": "/data",
      "read_only": true
    }
  ]
}
EOF

kubiya tool exec --json-file aws-tool.json
```

## Output Formats

### Default Text Output
```bash
# Formatted output with status messages
kubiya tool exec --name "test" --content "echo 'Line 1'; echo 'Line 2'"
```

### Raw JSON Stream
```bash
# Get raw SSE events as JSON
kubiya tool exec --name "test" --content "date" --output stream-json
```

### Quiet Mode
```bash
# Disable watch mode (no streaming)
kubiya tool exec --name "test" --content "date" --watch=false
```

## Environment Variables

### Tool Configuration
```bash
# Set default timeout
export KUBIYA_TOOL_TIMEOUT=600

# Set default runner
export KUBIYA_TOOL_RUNNER=core-testing-1

# Set default output format
export KUBIYA_TOOL_OUTPUT_FORMAT=stream-json

# Set default tool type
export KUBIYA_TOOL_TYPE=docker

# Skip health checks
export KUBIYA_SKIP_HEALTH_CHECK=true
```

### Default Runner Configuration
```bash
# Set default runner for "auto" mode
export KUBIYA_DEFAULT_RUNNER=my-custom-runner

# Execute with auto runner (will use my-custom-runner)
kubiya tool exec --name "test" --content "date"
```

## Complete Examples

### Full-Featured AWS Tool
```bash
kubiya tool exec \
  --name "aws-comprehensive" \
  --description "Comprehensive AWS operations tool" \
  --type docker \
  --image amazon/aws-cli:latest \
  --content 'aws ${service} ${action} ${extra_args}' \
  --arg service:string:"AWS service (ec2, s3, etc.)":true \
  --arg action:string:"Action to perform":true \
  --arg extra_args:string:"Additional arguments":false \
  --integration aws/profile \
  --env AWS_PROFILE=production \
  --env AWS_REGION=us-east-1 \
  --timeout 300 \
  --icon-url "https://aws.amazon.com/favicon.ico"
```

### Kubernetes Deployment Tool
```bash
kubiya tool exec \
  --name "k8s-deploy" \
  --description "Deploy application to Kubernetes" \
  --type docker \
  --image bitnami/kubectl:latest \
  --content 'kubectl apply -f /manifests/${manifest_file}' \
  --arg manifest_file:string:"Manifest filename":true \
  --arg namespace:string:"Target namespace":false \
  --integration k8s/incluster \
  --with-file ./k8s/:/manifests/ \
  --env KUBECTL_NAMESPACE=default
```

### Database Migration Tool
```bash
kubiya tool exec \
  --name "db-migrate" \
  --description "Run database migrations" \
  --type docker \
  --image migrate/migrate:latest \
  --content 'migrate -path /migrations -database ${DATABASE_URL} up' \
  --arg DATABASE_URL:string:"Database connection string":true \
  --with-file ./migrations:/migrations:ro \
  --with-service postgres:14 \
  --env POSTGRES_PASSWORD=secret \
  --timeout 600
```

## Flag Reference

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Tool name | Required |
| `--description` | Tool description | "CLI executed tool" |
| `--type` | Tool type (docker, python, bash, etc.) | docker |
| `--image` | Docker image | - |
| `--content` | Tool content/script | Required* |
| `--runner` | Runner to use | auto |
| `--timeout` | Timeout in seconds (0 = no timeout) | 300 |
| `--output` | Output format (text, stream-json) | text |
| `--watch` | Watch and stream output | true |
| `--json` | Tool definition as JSON string | - |
| `--json-file` | Path to JSON file with tool definition | - |
| `--tool-url` | URL to load tool definition from | - |
| `--source-uuid` | Source UUID to load tool from | - |
| `--integration` | Integration template to apply | - |
| `--with-file` | File mapping (source:destination) | - |
| `--with-volume` | Volume mapping (source:destination[:ro]) | - |
| `--with-service` | Service dependency | - |
| `--env` | Environment variable (KEY=VALUE) | - |
| `--arg` | Tool argument (name:type:description:required) | - |
| `--icon-url` | Icon URL for the tool | - |
| `--skip-health-check` | Skip runner health check | false |
| `--skip-policy-check` | Skip policy validation | false |

*Required unless using `--json`, `--json-file`, `--tool-url`, or `--source-uuid` 