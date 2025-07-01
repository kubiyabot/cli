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

// ToolIntegrationTemplate defines a reusable integration configuration for tools
type ToolIntegrationTemplate struct {
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description" yaml:"description"`
	Type         string                 `json:"type" yaml:"type"` // k8s, aws, gcp, etc.
	EnvVars      map[string]string      `json:"env_vars,omitempty" yaml:"env_vars,omitempty"`
	WithFiles    []FileMapping          `json:"with_files,omitempty" yaml:"with_files,omitempty"`
	WithVolumes  []VolumeMapping        `json:"with_volumes,omitempty" yaml:"with_volumes,omitempty"`
	WithServices []string               `json:"with_services,omitempty" yaml:"with_services,omitempty"`
	Secrets      []string               `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// FileMapping represents a file to be mounted in the container
type FileMapping struct {
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	Content     string `json:"content,omitempty" yaml:"content,omitempty"` // Optional inline content
}

// VolumeMapping represents a volume to be mounted
type VolumeMapping struct {
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	ReadOnly    bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

// PredefinedToolIntegrations contains built-in integration templates
var PredefinedToolIntegrations = map[string]ToolIntegrationTemplate{
	"k8s/incluster": {
		Name:        "k8s/incluster",
		Description: "Kubernetes in-cluster authentication",
		Type:        "kubernetes",
		WithFiles: []FileMapping{
			{
				Source:      "/var/run/secrets/kubernetes.io/serviceaccount/token",
				Destination: "/tmp/kubernetes_context_token",
			},
			{
				Source:      "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
				Destination: "/tmp/kubernetes_context_cert",
			},
		},
		EnvVars: map[string]string{
			"KUBERNETES_SERVICE_HOST": "kubernetes.default.svc",
			"KUBERNETES_SERVICE_PORT": "443",
		},
	},
	"k8s/kubeconfig": {
		Name:        "k8s/kubeconfig",
		Description: "Kubernetes authentication using kubeconfig file",
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
	},
	"aws/creds": {
		Name:        "aws/creds",
		Description: "AWS credentials from environment",
		Type:        "aws",
		EnvVars: map[string]string{
			"AWS_ACCESS_KEY_ID":     "${AWS_ACCESS_KEY_ID}",
			"AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}",
			"AWS_SESSION_TOKEN":     "${AWS_SESSION_TOKEN}",
			"AWS_REGION":            "${AWS_REGION:-us-east-1}",
		},
	},
	"aws/profile": {
		Name:        "aws/profile",
		Description: "AWS credentials from profile",
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
			"AWS_PROFILE": "${AWS_PROFILE:-default}",
		},
	},
	"gcp/adc": {
		Name:        "gcp/adc",
		Description: "Google Cloud Application Default Credentials",
		Type:        "gcp",
		WithFiles: []FileMapping{
			{
				Source:      "~/.config/gcloud/application_default_credentials.json",
				Destination: "/root/.config/gcloud/application_default_credentials.json",
			},
		},
		EnvVars: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json",
		},
	},
	"docker/socket": {
		Name:        "docker/socket",
		Description: "Docker socket access",
		Type:        "docker",
		WithVolumes: []VolumeMapping{
			{
				Source:      "/var/run/docker.sock",
				Destination: "/var/run/docker.sock",
			},
		},
	},
	"ssh/keys": {
		Name:        "ssh/keys",
		Description: "SSH keys from host",
		Type:        "ssh",
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
	},
	"database/postgres": {
		Name:        "database/postgres",
		Description: "PostgreSQL database with pgAdmin",
		Type:        "database",
		WithServices: []string{
			"postgres:15",
			"dpage/pgadmin4:latest",
		},
		EnvVars: map[string]string{
			"POSTGRES_DB":              "${POSTGRES_DB:-mydb}",
			"POSTGRES_USER":            "${POSTGRES_USER:-admin}",
			"POSTGRES_PASSWORD":        "${POSTGRES_PASSWORD:-secret}",
			"PGADMIN_DEFAULT_EMAIL":    "admin@example.com",
			"PGADMIN_DEFAULT_PASSWORD": "admin",
		},
	},
	"cache/redis": {
		Name:        "cache/redis",
		Description: "Redis cache service",
		Type:        "cache",
		WithServices: []string{
			"redis:7-alpine",
		},
		EnvVars: map[string]string{
			"REDIS_HOST": "redis",
			"REDIS_PORT": "6379",
		},
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
			// Expand home directory
			source := expandPath(file.Source)
			dest := expandPath(file.Destination)

			fileMap := map[string]interface{}{
				"source":      source,
				"destination": dest,
			}

			// Add inline content if specified
			if file.Content != "" {
				fileMap["content"] = file.Content
			} else if isLocalPath(source) {
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
				"source":      expandPath(vol.Source),
				"destination": expandPath(vol.Destination),
				"read_only":   vol.ReadOnly,
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
