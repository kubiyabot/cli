---
layout: page
title: Examples
description: Real-world examples and use cases for Kubiya CLI
toc: true
---

## Basic Examples

### Hello World Workflow

Create your first workflow:

```yaml
# hello-world.yaml
name: hello-world
description: A simple hello world workflow
steps:
- name: greet
  executor: command
  command: echo "Hello from Kubiya!"
- name: show_date
  executor: command
  command: date
  depends: [greet]
- name: system_info
  executor: command
  command: uname -a
  depends: [show_date]
```

Execute it:

```bash
kubiya workflow execute hello-world.yaml
```

### Simple Agent Creation

```bash
# Create a basic agent
kubiya agent create \
  --name "Helper Agent" \
  --desc "General purpose automation helper"

# Create agent with environment variables
kubiya agent create \
  --name "AWS Agent" \
  --desc "AWS infrastructure management" \
  --env "AWS_REGION=us-west-2" \
  --env "DEBUG=true"
```

### Tool Execution

```bash
# Execute a simple command
kubiya tool exec \
  --name "disk-usage" \
  --content "df -h"

# Execute with Docker
kubiya tool exec \
  --name "node-version" \
  --type docker \
  --image node:18-alpine \
  --content "node --version && npm --version"
```

## CI/CD Examples {#cicd}

### GitHub Actions Integration

Create a GitHub Actions workflow that uses Kubiya CLI:

```yaml
# .github/workflows/deploy.yml
name: Deploy with Kubiya
on:
  push:
    branches: [main]
    
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout
      uses: actions/checkout@v3
      
    - name: Install Kubiya CLI
      run: curl -fsSL https://cli.kubiya.ai/install.sh | bash
      
    - name: Deploy Application
      env:
        KUBIYA_API_KEY: ${{ secrets.KUBIYA_API_KEY }}
      run: |
        kubiya workflow execute myorg/deploy-workflows/production.yaml \
          --var version=${{ github.sha }} \
          --var environment=production \
          --var branch=${{ github.ref_name }}
```


Execute with parameters:

```bash
kubiya workflow execute deploy-workflow.yaml \
  --var environment=staging \
  --var version=v1.2.3
```

## Kubernetes Examples {#kubernetes}

### Kubernetes Deployment Agent

```bash
# Create Kubernetes-focused agent
kubiya agent create \
  --name "K8s Agent" \
  --desc "Kubernetes cluster management and deployment" \
  --secret KUBECONFIG \
  --env "CLUSTER_NAME=production-cluster"
```

### Kubernetes Deployment Workflow

```yaml
# k8s-deploy.yaml
name: k8s-deployment
description: Deploy application to Kubernetes cluster
params:
  app_name:
    type: string
    description: Application name
    required: true
  image_tag:
    type: string
    description: Docker image tag
    required: true
  namespace:
    type: string
    description: Kubernetes namespace
    default: default
  replicas:
    type: integer
    description: Number of replicas
    default: 3

steps:
- name: check_cluster
  executor: command
  command: |
    kubectl cluster-info
    kubectl get nodes
    
- name: create_namespace
  executor: command
  command: |
    kubectl create namespace $namespace --dry-run=client -o yaml | kubectl apply -f -
  env:
    namespace: ${{ params.namespace }}
  depends: [check_cluster]
  
- name: deploy_app
  executor: command
  command: |
    cat <<EOF | kubectl apply -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: $app_name
      namespace: $namespace
    spec:
      replicas: $replicas
      selector:
        matchLabels:
          app: $app_name
      template:
        metadata:
          labels:
            app: $app_name
        spec:
          containers:
          - name: $app_name
            image: $app_name:$image_tag
            ports:
            - containerPort: 8080
    EOF
  env:
    app_name: ${{ params.app_name }}
    image_tag: ${{ params.image_tag }}
    namespace: ${{ params.namespace }}
    replicas: ${{ params.replicas }}
  depends: [create_namespace]
  
- name: create_service
  executor: command
  command: |
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: Service
    metadata:
      name: $app_name-service
      namespace: $namespace
    spec:
      selector:
        app: $app_name
      ports:
      - port: 80
        targetPort: 8080
      type: LoadBalancer
    EOF
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
  depends: [deploy_app]
  
- name: wait_for_deployment
  executor: command
  command: |
    kubectl rollout status deployment/$app_name -n $namespace
    kubectl get pods -n $namespace -l app=$app_name
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
  depends: [create_service]
```

Execute the deployment:

```bash
kubiya workflow execute k8s-deploy.yaml \
  --var app_name=myapp \
  --var image_tag=v1.2.3 \
  --var namespace=production \
  --var replicas=5
```

## Security Examples {#security}

### Security Scanning Agent

```bash
# Create security-focused agent
kubiya agent create \
  --name "Security Agent" \
  --desc "Automated security scanning and compliance" \
  --secret SNYK_TOKEN \
  --secret DOCKER_HUB_TOKEN
```


### Incident Response Workflow

```yaml
# incident-response.yaml
name: incident-response
description: Automated incident response workflow
params:
  incident_type:
    type: string
    description: Type of incident
    required: true
  severity:
    type: string
    description: Incident severity
    default: medium
    
steps:
- name: create_incident_ticket
  executor: http
  method: POST
  url: https://api.example.com/incidents
  headers:
    Authorization: Bearer ${{ secrets.API_TOKEN }}
  body: |
    {
      "title": "Security Incident - ${{ params.incident_type }}",
      "severity": "${{ params.severity }}",
      "status": "open",
      "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    }
    
- name: collect_logs
  executor: command
  command: |
    kubectl logs -l app=myapp --tail=1000 > incident-logs.txt
    journalctl -u myservice --since="1 hour ago" >> incident-logs.txt
  depends: [create_incident_ticket]
  
- name: isolate_affected_systems
  executor: command
  command: |
    kubectl scale deployment/myapp --replicas=0
    kubectl patch service myapp-service -p '{"spec":{"selector":{"app":"maintenance"}}}'
  depends: [collect_logs]
  condition: ${{ params.severity == "high" || params.severity == "critical" }}
  
- name: notify_team
  executor: http
  method: POST
  url: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
  body: |
    {
      "text": "🚨 Security Incident Alert",
      "attachments": [{
        "color": "danger",
        "fields": [{
          "title": "Incident Type",
          "value": "${{ params.incident_type }}",
          "short": true
        }, {
          "title": "Severity",
          "value": "${{ params.severity }}",
          "short": true
        }]
      }]
    }
  depends: [isolate_affected_systems]
```

## Monitoring Examples {#monitoring}

### Monitoring Agent Setup

```bash
# Create monitoring agent
kubiya agent create \
  --name "Monitoring Agent" \
  --desc "System monitoring and alerting" \
  --secret DATADOG_API_KEY \
  --secret PROMETHEUS_URL \
  --env "ALERT_THRESHOLD=80"
```

### Health Check Workflow

```yaml
# health-check.yaml
name: health-check
description: Comprehensive system health check
params:
  environment:
    type: string
    description: Environment to check
    default: production
    
steps:
- name: check_services
  executor: command
  command: |
    kubectl get pods -A | grep -v Running | grep -v Completed || echo "All pods are running"
    kubectl get services -A | grep -v ClusterIP | grep -v LoadBalancer | grep -v NodePort || echo "All services are available"
    
- name: check_resources
  executor: command
  command: |
    kubectl top nodes
    kubectl top pods -A | sort -k3 -nr | head -10
    
- name: check_disk_space
  executor: command
  command: |
    df -h | awk '$5 > 80 {print "WARNING: " $0}'
    
- name: check_memory
  executor: command
  command: |
    free -h
    ps aux --sort=-%mem | head -10
    
- name: check_network
  executor: command
  command: |
    ping -c 3 google.com
    curl -I https://api.example.com/health
    
- name: send_alert
  executor: http
  method: POST
  url: https://api.datadoghq.com/api/v1/events
  headers:
    DD-API-KEY: ${{ secrets.DATADOG_API_KEY }}
  body: |
    {
      "title": "Health Check Alert",
      "text": "System health check completed for ${{ params.environment }}",
      "alert_type": "info",
      "source_type_name": "kubiya-cli"
    }
  depends: [check_services, check_resources, check_disk_space, check_memory, check_network]
```

### Log Analysis Workflow

```yaml
# log-analysis.yaml
name: log-analysis
description: Automated log analysis and alerting
params:
  log_level:
    type: string
    description: Minimum log level to analyze
    default: ERROR
  time_range:
    type: string
    description: Time range for log analysis
    default: "1h"
    
steps:
- name: collect_application_logs
  executor: command
  command: |
    kubectl logs -l app=myapp --since=$time_range > app-logs.txt
    grep -i "$log_level" app-logs.txt > filtered-logs.txt || echo "No $log_level logs found"
  env:
    time_range: ${{ params.time_range }}
    log_level: ${{ params.log_level }}
    
- name: analyze_error_patterns
  executor: command
  command: |
    echo "Error Pattern Analysis:" > analysis-report.txt
    echo "======================" >> analysis-report.txt
    grep -i "error" filtered-logs.txt | cut -d' ' -f3- | sort | uniq -c | sort -nr | head -10 >> analysis-report.txt
    
- name: check_error_rate
  executor: command
  command: |
    total_lines=$(wc -l < app-logs.txt)
    error_lines=$(wc -l < filtered-logs.txt)
    error_rate=$(echo "scale=2; $error_lines * 100 / $total_lines" | bc)
    echo "Error rate: $error_rate%" >> analysis-report.txt
    
    if (( $(echo "$error_rate > 5" | bc -l) )); then
      echo "HIGH_ERROR_RATE=true" >> analysis-report.txt
    fi
    
- name: create_dashboard
  executor: http
  method: POST
  url: https://api.datadoghq.com/api/v1/dashboard
  headers:
    DD-API-KEY: ${{ secrets.DATADOG_API_KEY }}
  body: |
    {
      "title": "Log Analysis Dashboard",
      "widgets": [{
        "definition": {
          "type": "timeseries",
          "requests": [{
            "q": "logs(source:myapp status:error).rollup(count).by(host)",
            "display_type": "line"
          }]
        }
      }]
    }
  depends: [collect_application_logs, analyze_error_patterns, check_error_rate]
```

## Advanced Integration Examples

### Slack Integration

```bash
# Create Slack integration agent
kubiya agent create \
  --name "Slack Bot Agent" \
  --desc "Slack integration for notifications and commands" \
  --webhook-method slack \
  --webhook-dest "#devops" \
  --webhook-prompt "Process this Slack message and take appropriate action"
```



## Tips and Best Practices

### Workflow Organization

1. **Use descriptive names** for workflows and steps
2. **Include proper documentation** in the description field
3. **Use parameters** for reusable workflows
4. **Implement error handling** with proper exit codes
5. **Use dependencies** to control execution order

### Security Best Practices

1. **Never hardcode secrets** in workflows
2. **Use Kubiya's secret management** for sensitive data
3. **Implement proper access controls** for agents
4. **Regular security scans** of your workflows
5. **Audit trail** for all operations

### Performance Optimization

1. **Use parallel execution** where possible
2. **Optimize Docker images** for faster execution
3. **Cache dependencies** when appropriate
4. **Monitor resource usage** of workflows
5. **Use appropriate timeouts** for long-running operations

These examples provide a solid foundation for automating your infrastructure and development workflows with Kubiya CLI. Adapt them to your specific needs and requirements.