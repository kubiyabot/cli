---
layout: example
title: Kubernetes Deployment Example
description: Deploy applications to Kubernetes using Kubiya CLI workflows
difficulty: intermediate
category: kubernetes
tags: [kubernetes, deployment, docker, devops]
---

## Overview

This example demonstrates how to deploy applications to a Kubernetes cluster using Kubiya CLI workflows. It covers building Docker images, deploying to Kubernetes, and performing health checks.

## Prerequisites

- Kubiya CLI installed and configured
- Access to a Kubernetes cluster
- Docker registry access
- `kubectl` configured with cluster access

## Agent Setup

First, create a Kubernetes-focused agent:

```bash
kubiya agent create \
  --name "K8s Deployment Agent" \
  --desc "Handles Kubernetes deployments and management" \
  --secret KUBECONFIG \
  --secret DOCKER_REGISTRY_TOKEN \
  --env "CLUSTER_NAME=production-cluster" \
  --env "REGISTRY_URL=registry.example.com"
```

## Workflow Definition

Create `k8s-deployment.yaml`:

```yaml
name: kubernetes-deployment
description: Deploy application to Kubernetes cluster
version: "1.0"

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
    default: "default"
  replicas:
    type: integer
    description: Number of replicas
    default: 3
    min: 1
    max: 10
  port:
    type: integer
    description: Application port
    default: 8080
  environment:
    type: string
    description: Environment (dev, staging, prod)
    default: "dev"
    enum: ["dev", "staging", "prod"]

steps:
- name: validate_cluster_access
  description: Validate Kubernetes cluster access
  executor: command
  command: |
    echo "Checking cluster access..."
    kubectl cluster-info
    kubectl get nodes
    echo "Cluster access validated"
  timeout: 30s

- name: create_namespace
  description: Create namespace if it doesn't exist
  executor: command
  command: |
    echo "Creating namespace: $namespace"
    kubectl create namespace $namespace --dry-run=client -o yaml | kubectl apply -f -
    echo "Namespace ready: $namespace"
  env:
    namespace: ${{ params.namespace }}
  depends: [validate_cluster_access]

- name: create_configmap
  description: Create application configuration
  executor: command
  command: |
    echo "Creating ConfigMap for $app_name..."
    kubectl create configmap $app_name-config \
      --from-literal=ENVIRONMENT=$environment \
      --from-literal=PORT=$port \
      --from-literal=APP_NAME=$app_name \
      --namespace=$namespace \
      --dry-run=client -o yaml | kubectl apply -f -
    echo "ConfigMap created"
  env:
    app_name: ${{ params.app_name }}
    environment: ${{ params.environment }}
    port: ${{ params.port }}
    namespace: ${{ params.namespace }}
  depends: [create_namespace]

- name: create_deployment
  description: Create Kubernetes deployment
  executor: command
  command: |
    echo "Creating deployment for $app_name..."
    cat <<EOF | kubectl apply -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: $app_name
      namespace: $namespace
      labels:
        app: $app_name
        version: $image_tag
        environment: $environment
    spec:
      replicas: $replicas
      selector:
        matchLabels:
          app: $app_name
      template:
        metadata:
          labels:
            app: $app_name
            version: $image_tag
            environment: $environment
        spec:
          containers:
          - name: $app_name
            image: ${REGISTRY_URL}/$app_name:$image_tag
            ports:
            - containerPort: $port
            env:
            - name: ENVIRONMENT
              valueFrom:
                configMapKeyRef:
                  name: $app_name-config
                  key: ENVIRONMENT
            - name: PORT
              valueFrom:
                configMapKeyRef:
                  name: $app_name-config
                  key: PORT
            - name: APP_NAME
              valueFrom:
                configMapKeyRef:
                  name: $app_name-config
                  key: APP_NAME
            resources:
              requests:
                memory: "128Mi"
                cpu: "100m"
              limits:
                memory: "256Mi"
                cpu: "500m"
            livenessProbe:
              httpGet:
                path: /health
                port: $port
              initialDelaySeconds: 30
              periodSeconds: 10
            readinessProbe:
              httpGet:
                path: /ready
                port: $port
              initialDelaySeconds: 5
              periodSeconds: 5
    EOF
    echo "Deployment created"
  env:
    app_name: ${{ params.app_name }}
    image_tag: ${{ params.image_tag }}
    namespace: ${{ params.namespace }}
    replicas: ${{ params.replicas }}
    port: ${{ params.port }}
    environment: ${{ params.environment }}
  depends: [create_configmap]

- name: create_service
  description: Create Kubernetes service
  executor: command
  command: |
    echo "Creating service for $app_name..."
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: Service
    metadata:
      name: $app_name-service
      namespace: $namespace
      labels:
        app: $app_name
        environment: $environment
    spec:
      selector:
        app: $app_name
      ports:
      - port: 80
        targetPort: $port
        protocol: TCP
      type: ClusterIP
    EOF
    echo "Service created"
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
    port: ${{ params.port }}
    environment: ${{ params.environment }}
  depends: [create_deployment]

- name: create_ingress
  description: Create ingress for external access
  executor: command
  command: |
    echo "Creating ingress for $app_name..."
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: $app_name-ingress
      namespace: $namespace
      annotations:
        nginx.ingress.kubernetes.io/rewrite-target: /
        cert-manager.io/cluster-issuer: letsencrypt-prod
    spec:
      tls:
      - hosts:
        - $app_name-$environment.example.com
        secretName: $app_name-tls
      rules:
      - host: $app_name-$environment.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: $app_name-service
                port:
                  number: 80
    EOF
    echo "Ingress created"
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
    environment: ${{ params.environment }}
  depends: [create_service]
  condition: ${{ params.environment != "dev" }}

- name: wait_for_deployment
  description: Wait for deployment to be ready
  executor: command
  command: |
    echo "Waiting for deployment to be ready..."
    kubectl rollout status deployment/$app_name -n $namespace --timeout=300s
    echo "Deployment is ready"
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
  depends: [create_ingress]
  timeout: 320s

- name: verify_deployment
  description: Verify deployment health
  executor: command
  command: |
    echo "Verifying deployment health..."
    
    # Check pod status
    echo "Pod Status:"
    kubectl get pods -n $namespace -l app=$app_name
    
    # Check service
    echo "Service Status:"
    kubectl get service $app_name-service -n $namespace
    
    # Check endpoints
    echo "Endpoints:"
    kubectl get endpoints $app_name-service -n $namespace
    
    # Port forward and health check
    echo "Performing health check..."
    kubectl port-forward service/$app_name-service $port:80 -n $namespace &
    PF_PID=$!
    sleep 5
    
    # Check health endpoint
    if curl -f http://localhost:$port/health; then
      echo "Health check passed"
    else
      echo "Health check failed"
      kill $PF_PID
      exit 1
    fi
    
    kill $PF_PID
    echo "Deployment verification completed"
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
    port: ${{ params.port }}
  depends: [wait_for_deployment]

- name: deployment_summary
  description: Display deployment summary
  executor: command
  command: |
    echo "=== Deployment Summary ==="
    echo "Application: $app_name"
    echo "Version: $image_tag"
    echo "Namespace: $namespace"
    echo "Replicas: $replicas"
    echo "Environment: $environment"
    echo ""
    echo "Resources created:"
    echo "- Deployment: $app_name"
    echo "- Service: $app_name-service"
    echo "- ConfigMap: $app_name-config"
    if [ "$environment" != "dev" ]; then
      echo "- Ingress: $app_name-ingress"
      echo "- URL: https://$app_name-$environment.example.com"
    fi
    echo ""
    echo "🎉 Deployment completed successfully!"
  env:
    app_name: ${{ params.app_name }}
    image_tag: ${{ params.image_tag }}
    namespace: ${{ params.namespace }}
    replicas: ${{ params.replicas }}
    environment: ${{ params.environment }}
  depends: [verify_deployment]
```

## Execution Examples

### Basic Deployment

Deploy to development environment:

```bash
kubiya workflow execute k8s-deployment.yaml \
  --var app_name=myapp \
  --var image_tag=v1.2.3 \
  --var namespace=development \
  --var replicas=2
```

### Production Deployment

Deploy to production with higher replica count:

```bash
kubiya workflow execute k8s-deployment.yaml \
  --var app_name=myapp \
  --var image_tag=v1.2.3 \
  --var namespace=production \
  --var replicas=5 \
  --var environment=prod
```

### Staging Deployment

Deploy to staging environment:

```bash
kubiya workflow execute k8s-deployment.yaml \
  --var app_name=myapp \
  --var image_tag=v1.2.3-rc.1 \
  --var namespace=staging \
  --var replicas=3 \
  --var environment=staging
```

## Advanced Features

### Blue-Green Deployment

Create `blue-green-deployment.yaml`:

```yaml
name: blue-green-deployment
description: Blue-green deployment strategy
version: "1.0"

params:
  app_name:
    type: string
    required: true
  new_version:
    type: string
    required: true
  namespace:
    type: string
    default: "default"

steps:
- name: deploy_green
  description: Deploy green version
  executor: command
  command: |
    # Deploy new version with green label
    kubectl patch deployment $app_name -n $namespace -p '{"spec":{"template":{"metadata":{"labels":{"version":"green"}}}}}'
    kubectl set image deployment/$app_name $app_name=${REGISTRY_URL}/$app_name:$new_version -n $namespace
    kubectl rollout status deployment/$app_name -n $namespace
  env:
    app_name: ${{ params.app_name }}
    new_version: ${{ params.new_version }}
    namespace: ${{ params.namespace }}

- name: test_green
  description: Test green deployment
  executor: command
  command: |
    # Create temporary service pointing to green version
    kubectl expose deployment $app_name --name=$app_name-green-test --port=80 -n $namespace
    
    # Test the green version
    kubectl port-forward service/$app_name-green-test 8080:80 -n $namespace &
    PF_PID=$!
    sleep 5
    
    if curl -f http://localhost:8080/health; then
      echo "Green version test passed"
      kill $PF_PID
      kubectl delete service $app_name-green-test -n $namespace
    else
      echo "Green version test failed"
      kill $PF_PID
      kubectl delete service $app_name-green-test -n $namespace
      exit 1
    fi
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
  depends: [deploy_green]

- name: switch_traffic
  description: Switch traffic to green version
  executor: command
  command: |
    # Update service selector to point to green version
    kubectl patch service $app_name-service -n $namespace -p '{"spec":{"selector":{"version":"green"}}}'
    echo "Traffic switched to green version"
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
  depends: [test_green]
```

### Canary Deployment

Create `canary-deployment.yaml`:

```yaml
name: canary-deployment
description: Canary deployment with traffic splitting
version: "1.0"

params:
  app_name:
    type: string
    required: true
  canary_version:
    type: string
    required: true
  canary_replicas:
    type: integer
    default: 1
  traffic_percentage:
    type: integer
    default: 10
    min: 1
    max: 100

steps:
- name: deploy_canary
  description: Deploy canary version
  executor: command
  command: |
    # Create canary deployment
    kubectl get deployment $app_name -n $namespace -o yaml | \
      sed 's/name: '$app_name'/name: '$app_name'-canary/' | \
      sed 's/app: '$app_name'/app: '$app_name'-canary/' | \
      kubectl apply -f -
    
    # Update canary deployment
    kubectl set image deployment/$app_name-canary $app_name=${REGISTRY_URL}/$app_name:$canary_version -n $namespace
    kubectl scale deployment/$app_name-canary --replicas=$canary_replicas -n $namespace
    kubectl rollout status deployment/$app_name-canary -n $namespace
  env:
    app_name: ${{ params.app_name }}
    canary_version: ${{ params.canary_version }}
    canary_replicas: ${{ params.canary_replicas }}
    namespace: ${{ params.namespace }}

- name: configure_traffic_split
  description: Configure traffic splitting
  executor: command
  command: |
    # Install Istio VirtualService for traffic splitting
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1alpha3
    kind: VirtualService
    metadata:
      name: $app_name-vs
      namespace: $namespace
    spec:
      hosts:
      - $app_name-service
      http:
      - match:
        - headers:
            canary:
              exact: "true"
        route:
        - destination:
            host: $app_name-service
            subset: canary
      - route:
        - destination:
            host: $app_name-service
            subset: stable
          weight: $((100 - $traffic_percentage))
        - destination:
            host: $app_name-service
            subset: canary
          weight: $traffic_percentage
    EOF
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
    traffic_percentage: ${{ params.traffic_percentage }}
  depends: [deploy_canary]
```

## Monitoring and Observability

### Deployment Monitoring

Create `deployment-monitoring.yaml`:

```yaml
name: deployment-monitoring
description: Monitor deployment health and metrics
version: "1.0"

params:
  app_name:
    type: string
    required: true
  namespace:
    type: string
    default: "default"
  duration:
    type: string
    default: "5m"

steps:
- name: check_deployment_status
  description: Check deployment status
  executor: command
  command: |
    echo "=== Deployment Status ==="
    kubectl get deployment $app_name -n $namespace -o wide
    
    echo "=== Pod Status ==="
    kubectl get pods -n $namespace -l app=$app_name -o wide
    
    echo "=== Recent Events ==="
    kubectl get events -n $namespace --field-selector involvedObject.name=$app_name --sort-by='.lastTimestamp'
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}

- name: monitor_metrics
  description: Monitor deployment metrics
  executor: command
  command: |
    echo "=== Resource Usage ==="
    kubectl top pods -n $namespace -l app=$app_name
    
    echo "=== Service Endpoints ==="
    kubectl get endpoints $app_name-service -n $namespace -o wide
    
    echo "=== Monitoring for $duration ==="
    end_time=$(($(date +%s) + $(echo $duration | sed 's/m/*60/g; s/s//g' | bc)))
    
    while [ $(date +%s) -lt $end_time ]; do
      ready_pods=$(kubectl get pods -n $namespace -l app=$app_name -o jsonpath='{.items[*].status.containerStatuses[*].ready}' | tr ' ' '\n' | grep -c true)
      total_pods=$(kubectl get pods -n $namespace -l app=$app_name --no-headers | wc -l)
      
      echo "$(date): Ready pods: $ready_pods/$total_pods"
      sleep 30
    done
  env:
    app_name: ${{ params.app_name }}
    namespace: ${{ params.namespace }}
    duration: ${{ params.duration }}
  depends: [check_deployment_status]
```

## Best Practices

### 1. Resource Management

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "500m"
```

### 2. Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

### 3. Security

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 2000
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
```

### 4. ConfigMaps and Secrets

```yaml
env:
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: app-secrets
      key: database-url
- name: API_KEY
  valueFrom:
    secretKeyRef:
      name: app-secrets
      key: api-key
```

## Troubleshooting

### Common Issues

1. **Image Pull Errors**
   ```bash
   # Check image registry access
   kubectl create secret docker-registry regcred \
     --docker-server=registry.example.com \
     --docker-username=username \
     --docker-password=password
   ```

2. **Pod Startup Issues**
   ```bash
   # Check pod logs
   kubectl logs -n namespace deployment/app-name
   
   # Describe pod for events
   kubectl describe pod -n namespace -l app=app-name
   ```

3. **Service Connectivity**
   ```bash
   # Test service connectivity
   kubectl run test-pod --rm -i --tty --image=busybox -- /bin/sh
   # Inside the pod:
   # wget -qO- http://app-name-service:80/health
   ```

## Next Steps

- Try [Helm Chart Deployment](helm-deployment)
- Learn about [GitOps with ArgoCD](gitops-argocd)
- Explore [Service Mesh Integration](service-mesh)
- Check out [Monitoring and Alerting](monitoring-alerting)