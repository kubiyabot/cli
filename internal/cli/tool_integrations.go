package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// ToolIntegrationTemplate defines a reusable integration configuration
type ToolIntegrationTemplate struct {
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description" yaml:"description"`
	Type         string                 `json:"type" yaml:"type"` // kubernetes, aws, gcp, etc.
	EnvVars      map[string]string      `json:"env_vars,omitempty" yaml:"env_vars,omitempty"`
	WithFiles    []FileMapping          `json:"with_files,omitempty" yaml:"with_files,omitempty"`
	WithVolumes  []VolumeMapping        `json:"with_volumes,omitempty" yaml:"with_volumes,omitempty"`
	WithServices []string               `json:"with_services,omitempty" yaml:"with_services,omitempty"`
	Secrets      []string               `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	// New fields for more comprehensive integration
	EntrypointWrapper string            `json:"entrypoint_wrapper,omitempty" yaml:"entrypoint_wrapper,omitempty"`
	BeforeScript      string            `json:"before_script,omitempty" yaml:"before_script,omitempty"`
	AfterScript       string            `json:"after_script,omitempty" yaml:"after_script,omitempty"`
	RequiredPackages  []string          `json:"required_packages,omitempty" yaml:"required_packages,omitempty"`
	DefaultImage      string            `json:"default_image,omitempty" yaml:"default_image,omitempty"`
	ImageOverrides    map[string]string `json:"image_overrides,omitempty" yaml:"image_overrides,omitempty"`
}

// FileMapping represents a file to be mapped into the tool container
type FileMapping struct {
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	Content     string `json:"content,omitempty" yaml:"content,omitempty"` // Optional inline content
}

// VolumeMapping represents a volume to be mounted into the tool container
type VolumeMapping struct {
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	ReadOnly    bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

// PredefinedToolIntegrations contains built-in integration templates
var PredefinedToolIntegrations = map[string]ToolIntegrationTemplate{
	"kubernetes/incluster": {
		Name:        "kubernetes/incluster",
		Description: "Kubernetes in-cluster authentication with automatic kubectl configuration",
		Type:        "kubernetes",
		WithFiles: []FileMapping{
			{
				Source:      "/var/run/secrets/kubernetes.io/serviceaccount/token",
				Destination: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			},
			{
				Source:      "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
				Destination: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			},
		},
		EnvVars: map[string]string{
			"KUBERNETES_SERVICE_HOST": "kubernetes.default.svc",
			"KUBERNETES_SERVICE_PORT": "443",
		},
		BeforeScript: `#!/bin/bash
# Setup Kubernetes in-cluster authentication
if [ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]; then
    export KUBE_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    kubectl config set-cluster in-cluster \
        --server=https://kubernetes.default.svc \
        --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt > /dev/null 2>&1
    kubectl config set-credentials in-cluster --token=$KUBE_TOKEN > /dev/null 2>&1
    kubectl config set-context in-cluster --cluster=in-cluster --user=in-cluster > /dev/null 2>&1
    kubectl config use-context in-cluster > /dev/null 2>&1
    echo "✓ Kubernetes in-cluster authentication configured"
fi
`,
		DefaultImage: "bitnami/kubectl:latest",
	},
	"kubernetes/kubeconfig": {
		Name:        "kubernetes/kubeconfig",
		Description: "Kubernetes authentication using local kubeconfig file",
		Type:        "kubernetes",
		WithFiles: []FileMapping{
			{
				Source:      "~/.kube/config",
				Destination: "/root/.kube/config",
			},
		},
		EnvVars: map[string]string{
			"KUBECONFIG": "/root/.kube/config",
		},
		DefaultImage: "bitnami/kubectl:latest",
	},
	"kubernetes/eks": {
		Name:        "kubernetes/eks",
		Description: "Amazon EKS authentication with aws-cli",
		Type:        "kubernetes",
		WithFiles: []FileMapping{
			{
				Source:      "~/.aws/credentials",
				Destination: "/root/.aws/credentials",
			},
			{
				Source:      "~/.aws/config",
				Destination: "/root/.aws/config",
			},
			{
				Source:      "~/.kube/config",
				Destination: "/root/.kube/config",
			},
		},
		EnvVars: map[string]string{
			"AWS_PROFILE": "${AWS_PROFILE:-default}",
			"KUBECONFIG":  "/root/.kube/config",
		},
		RequiredPackages: []string{"aws-cli", "kubectl"},
		BeforeScript: `#!/bin/bash
# Update kubeconfig for EKS
if [ -n "$EKS_CLUSTER_NAME" ] && [ -n "$AWS_REGION" ]; then
    aws eks update-kubeconfig --name $EKS_CLUSTER_NAME --region $AWS_REGION
    echo "✓ EKS kubeconfig updated for cluster: $EKS_CLUSTER_NAME"
fi
`,
		DefaultImage: "amazon/aws-cli:latest",
	},
	"kubernetes/gke": {
		Name:        "kubernetes/gke",
		Description: "Google Kubernetes Engine authentication",
		Type:        "kubernetes",
		WithFiles: []FileMapping{
			{
				Source:      "~/.config/gcloud",
				Destination: "/root/.config/gcloud",
			},
		},
		EnvVars: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json",
			"CLOUDSDK_CORE_PROJECT":          "${GCP_PROJECT}",
		},
		BeforeScript: `#!/bin/bash
# Get GKE credentials
if [ -n "$GKE_CLUSTER_NAME" ] && [ -n "$GKE_CLUSTER_ZONE" ]; then
    gcloud container clusters get-credentials $GKE_CLUSTER_NAME --zone $GKE_CLUSTER_ZONE
    echo "✓ GKE credentials configured for cluster: $GKE_CLUSTER_NAME"
fi
`,
		DefaultImage: "google/cloud-sdk:slim",
	},
	"aws/cli": {
		Name:        "aws/cli",
		Description: "AWS CLI with credentials from local AWS config",
		Type:        "aws",
		WithFiles: []FileMapping{
			{
				Source:      "~/.aws/credentials",
				Destination: "/root/.aws/credentials",
			},
			{
				Source:      "~/.aws/config",
				Destination: "/root/.aws/config",
			},
		},
		EnvVars: map[string]string{
			"AWS_PROFILE":        "${AWS_PROFILE:-default}",
			"AWS_DEFAULT_REGION": "${AWS_DEFAULT_REGION:-us-east-1}",
		},
		DefaultImage: "amazon/aws-cli:latest",
	},
	"aws/iam-role": {
		Name:        "aws/iam-role",
		Description: "AWS CLI with IAM role assumption",
		Type:        "aws",
		WithFiles: []FileMapping{
			{
				Source:      "~/.aws/credentials",
				Destination: "/root/.aws/credentials",
			},
			{
				Source:      "~/.aws/config",
				Destination: "/root/.aws/config",
			},
		},
		EnvVars: map[string]string{
			"AWS_PROFILE":        "${AWS_PROFILE:-default}",
			"AWS_DEFAULT_REGION": "${AWS_DEFAULT_REGION:-us-east-1}",
		},
		BeforeScript: `#!/bin/bash
# Assume IAM role if specified
if [ -n "$AWS_ROLE_ARN" ]; then
    CREDS=$(aws sts assume-role --role-arn $AWS_ROLE_ARN --role-session-name kubiya-session)
    export AWS_ACCESS_KEY_ID=$(echo $CREDS | jq -r '.Credentials.AccessKeyId')
    export AWS_SECRET_ACCESS_KEY=$(echo $CREDS | jq -r '.Credentials.SecretAccessKey')
    export AWS_SESSION_TOKEN=$(echo $CREDS | jq -r '.Credentials.SessionToken')
    echo "✓ Assumed IAM role: $AWS_ROLE_ARN"
fi
`,
		RequiredPackages: []string{"jq"},
		DefaultImage:     "amazon/aws-cli:latest",
	},
	"aws/env": {
		Name:        "aws/env",
		Description: "AWS CLI with credentials from environment variables",
		Type:        "aws",
		EnvVars: map[string]string{
			"AWS_ACCESS_KEY_ID":     "${AWS_ACCESS_KEY_ID}",
			"AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}",
			"AWS_SESSION_TOKEN":     "${AWS_SESSION_TOKEN}",
			"AWS_DEFAULT_REGION":    "${AWS_DEFAULT_REGION:-us-east-1}",
		},
		DefaultImage: "amazon/aws-cli:latest",
	},
	"gcp/adc": {
		Name:        "gcp/adc",
		Description: "Google Cloud with Application Default Credentials",
		Type:        "gcp",
		WithFiles: []FileMapping{
			{
				Source:      "~/.config/gcloud/application_default_credentials.json",
				Destination: "/root/.config/gcloud/application_default_credentials.json",
			},
		},
		EnvVars: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json",
			"CLOUDSDK_CORE_PROJECT":          "${GCP_PROJECT}",
		},
		DefaultImage: "google/cloud-sdk:slim",
	},
	"gcp/service-account": {
		Name:        "gcp/service-account",
		Description: "Google Cloud with service account key file",
		Type:        "gcp",
		WithFiles: []FileMapping{
			{
				Source:      "${GCP_SERVICE_ACCOUNT_KEY_FILE}",
				Destination: "/tmp/gcp-key.json",
			},
		},
		EnvVars: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": "/tmp/gcp-key.json",
			"CLOUDSDK_CORE_PROJECT":          "${GCP_PROJECT}",
		},
		BeforeScript: `#!/bin/bash
# Activate service account
if [ -f /tmp/gcp-key.json ]; then
    gcloud auth activate-service-account --key-file=/tmp/gcp-key.json
    echo "✓ GCP service account activated"
fi
`,
		DefaultImage: "google/cloud-sdk:slim",
	},
	"azure/cli": {
		Name:        "azure/cli",
		Description: "Azure CLI with local credentials",
		Type:        "azure",
		WithFiles: []FileMapping{
			{
				Source:      "~/.azure",
				Destination: "/root/.azure",
			},
		},
		EnvVars: map[string]string{
			"AZURE_SUBSCRIPTION_ID": "${AZURE_SUBSCRIPTION_ID}",
		},
		DefaultImage: "mcr.microsoft.com/azure-cli:latest",
	},
	"azure/sp": {
		Name:        "azure/sp",
		Description: "Azure CLI with service principal authentication",
		Type:        "azure",
		EnvVars: map[string]string{
			"AZURE_CLIENT_ID":       "${AZURE_CLIENT_ID}",
			"AZURE_CLIENT_SECRET":   "${AZURE_CLIENT_SECRET}",
			"AZURE_TENANT_ID":       "${AZURE_TENANT_ID}",
			"AZURE_SUBSCRIPTION_ID": "${AZURE_SUBSCRIPTION_ID}",
		},
		BeforeScript: `#!/bin/bash
# Login with service principal
if [ -n "$AZURE_CLIENT_ID" ] && [ -n "$AZURE_CLIENT_SECRET" ] && [ -n "$AZURE_TENANT_ID" ]; then
    az login --service-principal -u $AZURE_CLIENT_ID -p $AZURE_CLIENT_SECRET --tenant $AZURE_TENANT_ID
    echo "✓ Azure service principal authenticated"
fi
`,
		DefaultImage: "mcr.microsoft.com/azure-cli:latest",
	},
	"docker/socket": {
		Name:        "docker/socket",
		Description: "Docker CLI with socket access",
		Type:        "docker",
		WithVolumes: []VolumeMapping{
			{
				Source:      "/var/run/docker.sock",
				Destination: "/var/run/docker.sock",
			},
		},
		DefaultImage: "docker:cli",
	},
	"docker/dind": {
		Name:        "docker/dind",
		Description: "Docker-in-Docker with full Docker daemon",
		Type:        "docker",
		WithServices: []string{
			"docker:dind",
		},
		EnvVars: map[string]string{
			"DOCKER_HOST":       "tcp://docker:2376",
			"DOCKER_TLS_VERIFY": "1",
			"DOCKER_CERT_PATH":  "/certs/client",
		},
		WithVolumes: []VolumeMapping{
			{
				Source:      "docker-certs-client",
				Destination: "/certs/client",
				ReadOnly:    true,
			},
		},
		DefaultImage: "docker:cli",
	},
	"terraform/aws": {
		Name:        "terraform/aws",
		Description: "Terraform with AWS provider",
		Type:        "terraform",
		WithFiles: []FileMapping{
			{
				Source:      "~/.aws/credentials",
				Destination: "/root/.aws/credentials",
			},
			{
				Source:      "~/.aws/config",
				Destination: "/root/.aws/config",
			},
		},
		EnvVars: map[string]string{
			"AWS_PROFILE":        "${AWS_PROFILE:-default}",
			"AWS_DEFAULT_REGION": "${AWS_DEFAULT_REGION:-us-east-1}",
		},
		BeforeScript: `#!/bin/bash
# Initialize Terraform
terraform init -upgrade
echo "✓ Terraform initialized"
`,
		DefaultImage: "hashicorp/terraform:latest",
	},
	"ansible/ssh": {
		Name:        "ansible/ssh",
		Description: "Ansible with SSH key authentication",
		Type:        "ansible",
		WithFiles: []FileMapping{
			{
				Source:      "~/.ssh/id_rsa",
				Destination: "/root/.ssh/id_rsa",
			},
			{
				Source:      "~/.ssh/id_rsa.pub",
				Destination: "/root/.ssh/id_rsa.pub",
			},
			{
				Source:      "~/.ssh/known_hosts",
				Destination: "/root/.ssh/known_hosts",
			},
		},
		BeforeScript: `#!/bin/bash
# Set proper SSH key permissions
chmod 600 /root/.ssh/id_rsa
chmod 644 /root/.ssh/id_rsa.pub
echo "✓ SSH keys configured"
`,
		DefaultImage: "ansible/ansible:latest",
	},
	"git/ssh": {
		Name:        "git/ssh",
		Description: "Git with SSH authentication",
		Type:        "git",
		WithFiles: []FileMapping{
			{
				Source:      "~/.ssh/id_rsa",
				Destination: "/root/.ssh/id_rsa",
			},
			{
				Source:      "~/.ssh/known_hosts",
				Destination: "/root/.ssh/known_hosts",
			},
			{
				Source:      "~/.gitconfig",
				Destination: "/root/.gitconfig",
			},
		},
		BeforeScript: `#!/bin/bash
# Configure Git SSH
chmod 600 /root/.ssh/id_rsa
git config --global core.sshCommand "ssh -o StrictHostKeyChecking=no"
echo "✓ Git SSH configured"
`,
		DefaultImage: "alpine/git:latest",
	},
	"database/postgres": {
		Name:        "database/postgres",
		Description: "PostgreSQL client with database service",
		Type:        "database",
		WithServices: []string{
			"postgres:15",
		},
		EnvVars: map[string]string{
			"PGHOST":     "${PGHOST:-postgres}",
			"PGPORT":     "${PGPORT:-5432}",
			"PGDATABASE": "${PGDATABASE:-postgres}",
			"PGUSER":     "${PGUSER:-postgres}",
			"PGPASSWORD": "${PGPASSWORD:-postgres}",
		},
		BeforeScript: `#!/bin/bash
# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if pg_isready -h $PGHOST -p $PGPORT -U $PGUSER; then
        echo "✓ PostgreSQL is ready"
        break
    fi
    echo "Waiting for PostgreSQL... ($i/30)"
    sleep 1
done
`,
		DefaultImage: "postgres:15-alpine",
	},
	"database/mysql": {
		Name:        "database/mysql",
		Description: "MySQL client with database service",
		Type:        "database",
		WithServices: []string{
			"mysql:8",
		},
		EnvVars: map[string]string{
			"MYSQL_HOST":          "${MYSQL_HOST:-mysql}",
			"MYSQL_PORT":          "${MYSQL_PORT:-3306}",
			"MYSQL_DATABASE":      "${MYSQL_DATABASE:-test}",
			"MYSQL_USER":          "${MYSQL_USER:-root}",
			"MYSQL_PASSWORD":      "${MYSQL_PASSWORD:-root}",
			"MYSQL_ROOT_PASSWORD": "${MYSQL_ROOT_PASSWORD:-root}",
		},
		BeforeScript: `#!/bin/bash
# Wait for MySQL to be ready
for i in {1..30}; do
    if mysqladmin ping -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASSWORD --silent; then
        echo "✓ MySQL is ready"
        break
    fi
    echo "Waiting for MySQL... ($i/30)"
    sleep 1
done
`,
		DefaultImage: "mysql:8",
	},
	"cache/redis": {
		Name:        "cache/redis",
		Description: "Redis client with cache service",
		Type:        "cache",
		WithServices: []string{
			"redis:7-alpine",
		},
		EnvVars: map[string]string{
			"REDIS_HOST": "${REDIS_HOST:-redis}",
			"REDIS_PORT": "${REDIS_PORT:-6379}",
		},
		BeforeScript: `#!/bin/bash
# Wait for Redis to be ready
for i in {1..30}; do
    if redis-cli -h $REDIS_HOST -p $REDIS_PORT ping > /dev/null 2>&1; then
        echo "✓ Redis is ready"
        break
    fi
    echo "Waiting for Redis... ($i/30)"
    sleep 1
done
`,
		DefaultImage: "redis:7-alpine",
	},
	"monitoring/prometheus": {
		Name:        "monitoring/prometheus",
		Description: "Prometheus monitoring stack",
		Type:        "monitoring",
		WithServices: []string{
			"prom/prometheus:latest",
			"grafana/grafana:latest",
		},
		EnvVars: map[string]string{
			"PROMETHEUS_URL": "http://prometheus:9090",
			"GRAFANA_URL":    "http://grafana:3000",
		},
		DefaultImage: "prom/prometheus:latest",
	},
}

// LoadCustomToolIntegrations loads user-defined integrations from config directory
func LoadCustomToolIntegrations(fs afero.Fs) (map[string]ToolIntegrationTemplate, error) {
	customIntegrations := make(map[string]ToolIntegrationTemplate)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return customIntegrations, nil // Return empty map if no home dir
	}

	configDir := filepath.Join(homeDir, ".kubiya")
	integrationsFile := filepath.Join(configDir, "tool-integrations.json")

	data, err := afero.ReadFile(fs, integrationsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return customIntegrations, nil // No custom integrations file
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &customIntegrations); err != nil {
		return nil, fmt.Errorf("failed to parse tool-integrations.json: %w", err)
	}

	return customIntegrations, nil
}

// GetAllToolIntegrations returns both predefined and custom integrations
func GetAllToolIntegrations(fs afero.Fs) (map[string]ToolIntegrationTemplate, error) {
	allIntegrations := make(map[string]ToolIntegrationTemplate)

	// Add predefined integrations
	for k, v := range PredefinedToolIntegrations {
		allIntegrations[k] = v
	}

	// Load and merge custom integrations
	customIntegrations, err := LoadCustomToolIntegrations(fs)
	if err != nil {
		return nil, err
	}

	for k, v := range customIntegrations {
		allIntegrations[k] = v
	}

	return allIntegrations, nil
}

// ApplyIntegrationToTool applies integration template to tool definition
func ApplyIntegrationToTool(toolDef map[string]interface{}, integration ToolIntegrationTemplate) error {
	// Apply default image if tool doesn't have one
	if integration.DefaultImage != "" {
		if _, hasImage := toolDef["image"]; !hasImage {
			toolDef["image"] = integration.DefaultImage
		}
	}

	// Wrap content with before/after scripts
	originalContent, _ := toolDef["content"].(string)
	if originalContent != "" {
		var wrappedContent strings.Builder

		// Add shebang if not present
		if !strings.HasPrefix(strings.TrimSpace(originalContent), "#!") {
			wrappedContent.WriteString("#!/bin/bash\nset -e\n\n")
		}

		// Add before script
		if integration.BeforeScript != "" {
			wrappedContent.WriteString("# Integration setup\n")
			wrappedContent.WriteString(integration.BeforeScript)
			wrappedContent.WriteString("\n\n# User script\n")
		}

		// Add original content
		wrappedContent.WriteString(originalContent)

		// Add after script
		if integration.AfterScript != "" {
			wrappedContent.WriteString("\n\n# Integration cleanup\n")
			wrappedContent.WriteString(integration.AfterScript)
		}

		toolDef["content"] = wrappedContent.String()
	}

	// Apply environment variables
	if len(integration.EnvVars) > 0 {
		envList, ok := toolDef["env"].([]interface{})
		if !ok {
			envList = []interface{}{}
		}

		// Convert map to list of KEY=VALUE strings
		for k, v := range integration.EnvVars {
			// Expand environment variables in values
			v = os.ExpandEnv(v)
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		toolDef["env"] = envList
	}

	// Apply file mappings
	if len(integration.WithFiles) > 0 {
		withFiles, ok := toolDef["with_files"].([]interface{})
		if !ok {
			withFiles = []interface{}{}
		}

		for _, file := range integration.WithFiles {
			// Expand home directory and environment variables
			source := expandPath(os.ExpandEnv(file.Source))
			dest := expandPath(os.ExpandEnv(file.Destination))

			fileMap := map[string]interface{}{
				"source":      source,
				"destination": dest,
			}

			// Add inline content if specified
			if file.Content != "" {
				fileMap["content"] = file.Content
			} else if IsLocalPath(source) {
				// Try to read local file content
				content, err := os.ReadFile(source)
				if err == nil {
					fileMap["content"] = string(content)
				}
			}

			withFiles = append(withFiles, fileMap)
		}

		toolDef["with_files"] = withFiles
	}

	// Apply volume mappings
	if len(integration.WithVolumes) > 0 {
		withVolumes, ok := toolDef["with_volumes"].([]interface{})
		if !ok {
			withVolumes = []interface{}{}
		}

		for _, vol := range integration.WithVolumes {
			volumeMap := map[string]interface{}{
				"source":      expandPath(os.ExpandEnv(vol.Source)),
				"destination": expandPath(os.ExpandEnv(vol.Destination)),
			}
			if vol.ReadOnly {
				volumeMap["read_only"] = true
			}
			withVolumes = append(withVolumes, volumeMap)
		}

		toolDef["with_volumes"] = withVolumes
	}

	// Apply service dependencies
	if len(integration.WithServices) > 0 {
		withServices, ok := toolDef["with_services"].([]interface{})
		if !ok {
			withServices = []interface{}{}
		}

		for _, service := range integration.WithServices {
			withServices = append(withServices, service)
		}

		toolDef["with_services"] = withServices
	}

	// Apply required packages (add to content if needed)
	if len(integration.RequiredPackages) > 0 && integration.DefaultImage != "" {
		// Check if we need to install packages
		content, _ := toolDef["content"].(string)

		// For Alpine-based images
		if strings.Contains(integration.DefaultImage, "alpine") {
			installCmd := fmt.Sprintf("apk add --no-cache %s", strings.Join(integration.RequiredPackages, " "))
			if !strings.Contains(content, installCmd) {
				// Prepend package installation
				toolDef["content"] = fmt.Sprintf("#!/bin/sh\n%s\n\n%s", installCmd, content)
			}
		}
		// For Debian/Ubuntu-based images
		if strings.Contains(integration.DefaultImage, "ubuntu") || strings.Contains(integration.DefaultImage, "debian") {
			installCmd := fmt.Sprintf("apt-get update && apt-get install -y %s", strings.Join(integration.RequiredPackages, " "))
			if !strings.Contains(content, installCmd) {
				// Prepend package installation
				toolDef["content"] = fmt.Sprintf("#!/bin/bash\n%s\n\n%s", installCmd, content)
			}
		}
	}

	// Apply secrets
	if len(integration.Secrets) > 0 {
		secrets, ok := toolDef["secrets"].([]interface{})
		if !ok {
			secrets = []interface{}{}
		}

		for _, secret := range integration.Secrets {
			secrets = append(secrets, secret)
		}

		toolDef["secrets"] = secrets
	}

	// Apply additional config
	if integration.Config != nil {
		for k, v := range integration.Config {
			// Don't overwrite existing values
			if _, exists := toolDef[k]; !exists {
				toolDef[k] = v
			}
		}
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

// newToolIntegrationsCommand creates the tool integrations management command
func newToolIntegrationsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "integrations",
		Aliases: []string{"int"},
		Short:   "🔌 Manage tool integration templates",
		Long:    `List and manage integration templates for tool execution.`,
	}

	cmd.AddCommand(
		newListToolIntegrationsCommand(cfg),
		newShowToolIntegrationCommand(cfg),
	)

	return cmd
}

// newListToolIntegrationsCommand lists available integrations
func newListToolIntegrationsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List available integration templates",
		Example: `  # List all tool integrations
  kubiya tool integrations list

  # Output in JSON format
  kubiya tool integrations list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fs := afero.NewOsFs()
			integrations, err := GetAllToolIntegrations(fs)
			if err != nil {
				return fmt.Errorf("failed to load integrations: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(integrations)
			case "text":
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔌 Tool Integration Templates "))

				if len(integrations) == 0 {
					fmt.Println("No integrations found")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Group by type
				typeGroups := make(map[string][]ToolIntegrationTemplate)
				for _, integration := range integrations {
					typeGroups[integration.Type] = append(typeGroups[integration.Type], integration)
				}

				// Display by type
				for integrationType, templates := range typeGroups {
					fmt.Fprintf(w, "\n%s\n", style.SubtitleStyle.Render(fmt.Sprintf("%s Integrations:", strings.Title(integrationType))))

					for _, tmpl := range templates {
						fmt.Fprintf(w, "  %s\t%s\n",
							style.HighlightStyle.Render(tmpl.Name),
							style.DimStyle.Render(tmpl.Description))
					}
				}

				fmt.Fprintln(w, "\n\nUsage:")
				fmt.Fprintln(w, "  • Use 'kubiya tool integrations show <name>' for details")
				fmt.Fprintln(w, "  • Use 'kubiya tool exec --integration <name>' to apply to tool execution")
				fmt.Fprintln(w, "\nExample:")
				fmt.Fprintln(w, "  kubiya tool exec --name my-k8s-tool --content 'kubectl get pods' --integration k8s/incluster")

				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

// newShowToolIntegrationCommand shows details of a specific integration
func newShowToolIntegrationCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "📖 Show integration template details",
		Args:  cobra.ExactArgs(1),
		Example: `  # Show k8s in-cluster integration
  kubiya tool integrations show k8s/incluster

  # Show AWS credentials integration
  kubiya tool integrations show aws/creds`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fs := afero.NewOsFs()
			integrations, err := GetAllToolIntegrations(fs)
			if err != nil {
				return fmt.Errorf("failed to load integrations: %w", err)
			}

			name := args[0]
			integration, ok := integrations[name]
			if !ok {
				return fmt.Errorf("integration '%s' not found", name)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(integration)
			case "text":
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 🔌 Integration: %s ", integration.Name)))
				fmt.Printf("%s %s\n", style.SubtitleStyle.Render("Type:"), integration.Type)
				fmt.Printf("%s %s\n\n", style.SubtitleStyle.Render("Description:"), integration.Description)

				// Environment variables
				if len(integration.EnvVars) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Environment Variables:"))
					for k, v := range integration.EnvVars {
						fmt.Printf("  • %s=%s\n", style.HighlightStyle.Render(k), v)
					}
					fmt.Println()
				}

				// File mappings
				if len(integration.WithFiles) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("File Mappings:"))
					for _, file := range integration.WithFiles {
						fmt.Printf("  • %s → %s\n", file.Source, file.Destination)
						if file.Content != "" {
							fmt.Printf("    %s\n", style.DimStyle.Render("(with inline content)"))
						}
					}
					fmt.Println()
				}

				// Volume mappings
				if len(integration.WithVolumes) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Volume Mappings:"))
					for _, vol := range integration.WithVolumes {
						readOnly := ""
						if vol.ReadOnly {
							readOnly = " (read-only)"
						}
						fmt.Printf("  • %s → %s%s\n", vol.Source, vol.Destination, readOnly)
					}
					fmt.Println()
				}

				// Service dependencies
				if len(integration.WithServices) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Service Dependencies:"))
					for _, service := range integration.WithServices {
						fmt.Printf("  • %s\n", service)
					}
					fmt.Println()
				}

				// Usage example
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Usage:"))
				fmt.Printf("  kubiya tool exec --name my-tool --content '<your script>' --integration %s\n\n", name)

				// Show specific example based on integration type
				switch integration.Type {
				case "kubernetes":
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Example:"))
					fmt.Printf("  kubiya tool exec --name k8s-pods --content 'kubectl get pods -A' --integration %s\n", name)
				case "aws":
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Example:"))
					fmt.Printf("  kubiya tool exec --name aws-s3-list --content 'aws s3 ls' --integration %s\n", name)
				case "docker":
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Example:"))
					fmt.Printf("  kubiya tool exec --name docker-ps --content 'docker ps' --integration %s\n", name)
				}

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}
