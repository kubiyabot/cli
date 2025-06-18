package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Tool represents a Kubiya tool definition
type Tool struct {
	Name        string    `yaml:"name"`
	Image       string    `yaml:"image,omitempty"`
	Description string    `yaml:"description"`
	Alias       string    `yaml:"alias,omitempty"`
	LongRunning bool      `yaml:"long_running,omitempty"`
	Content     string    `yaml:"content,omitempty"`
	Args        []ToolArg `yaml:"args,omitempty"`
	Env         []string  `yaml:"env,omitempty"`
	WithVolumes []Volume  `yaml:"with_volumes,omitempty"`
	WithFiles   []File    `yaml:"with_files,omitempty"`
}

// ToolArg represents a tool argument
type ToolArg struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Type        string `yaml:"type,omitempty"`
}

// Volume represents a volume mount
type Volume struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// File represents a file mount
type File struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

// Workflow represents a Kubiya workflow definition
type Workflow struct {
	Name  string         `yaml:"name"`
	Steps []WorkflowStep `yaml:"steps"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description,omitempty"`
	Command     string           `yaml:"command,omitempty"`
	Executor    WorkflowExecutor `yaml:"executor,omitempty"`
	Output      string           `yaml:"output,omitempty"`
	Depends     []string         `yaml:"depends,omitempty"`
}

// WorkflowExecutor represents the executor configuration for a workflow step
type WorkflowExecutor struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

func newInitCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "🎯 Initialize Kubiya resources",
		Long:  `Create new Kubiya tools and workflows with ready-to-use templates.`,
	}

	cmd.AddCommand(
		newInitToolCommand(cfg),
		newInitWorkflowCommand(cfg),
	)

	return cmd
}

func newInitToolCommand(cfg *config.Config) *cobra.Command {
	var (
		name           string
		description    string
		image          string
		outputFile     string
		longRunning    bool
		nonInteractive bool
		envVars        []string
	)

	// Common environment variables that are frequently used
	commonEnvVars := []string{
		"KUBIYA_API_KEY",
		"KUBIYA_USER_EMAIL",
		"KUBIYA_USER_ORG",
		"KUBIYA_AGENT_UUID",
		"KUBIYA_BASE_URL",
		"KUBIYA_DEBUG",
		"KUBIYA_ENVIRONMENT",
		"KUBIYA_WORKSPACE",
		"KUBIYA_RUNNER",
		"KUBIYA_SOURCE_ID",
	}

	// Additional environment variables that might be needed
	additionalEnvVars := []string{
		"AWS_PROFILE",
		"OPENAI_API_KEY",
		"OPENAI_API_BASE",
		"SLACK_API_TOKEN",
		"GH_TOKEN",
		"INFRACOST_API_KEY",
	}

	cmd := &cobra.Command{
		Use:   "tool",
		Short: "🛠️ Initialize a new tool",
		Example: `  # Create a tool interactively (default)
  kubiya init tool

  # Create a tool non-interactively
  kubiya init tool --non-interactive --name my-tool --desc "My awesome tool"

  # Create a Python tool
  kubiya init tool --name deploy-app --desc "Deploy application" --image python:3.11

  # Create a long-running tool
  kubiya init tool --name monitor --desc "Monitor resources" --long-running`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if !nonInteractive {
				// Interactive mode
				if name == "" {
					prompt := promptui.Prompt{
						Label:   "Tool Name",
						Default: "my-tool",
						Validate: func(s string) error {
							if s == "" {
								return fmt.Errorf("name is required")
							}
							return nil
						},
					}
					name, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}

				if description == "" {
					prompt := promptui.Prompt{
						Label:   "Description",
						Default: "A Kubiya tool",
					}
					description, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}

				// Image selection with common options
				if image == "" {
					imageSelect := promptui.Select{
						Label: "Container Image",
						Items: []string{
							"python:3.11",
							"python:3.10",
							"alpine:latest",
							"ubuntu:latest",
							"node:latest",
							"custom",
							"none",
						},
					}
					_, image, err = imageSelect.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}

					if image == "custom" {
						prompt := promptui.Prompt{
							Label:   "Enter custom image",
							Default: "python:3.11",
						}
						image, err = prompt.Run()
						if err != nil {
							return fmt.Errorf("prompt failed: %w", err)
						}
					} else if image == "none" {
						image = ""
					}
				}

				if !cmd.Flags().Changed("long-running") {
					prompt := promptui.Prompt{
						Label:     "Long Running",
						IsConfirm: true,
					}
					result, err := prompt.Run()
					if err == nil && strings.ToLower(result) == "y" {
						longRunning = true
					}
				}

				// Environment variables selection
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Environment Variables"))

				// First, select Kubiya environment variables
				fmt.Printf("\n%s Select Kubiya environment variables:\n",
					style.SubtitleStyle.Render("Kubiya Variables"))
				fmt.Printf("Use ↑/↓ to navigate, Enter to select, q to finish\n\n")

				envSelect := promptui.Select{
					Label: "Add Kubiya environment variables",
					Items: append(commonEnvVars, "done"),
				}

				for {
					_, result, err := envSelect.Run()
					if err != nil || result == "done" {
						break
					}

					// Check if already selected
					exists := false
					for _, env := range envVars {
						if env == result {
							exists = true
							break
						}
					}
					if !exists {
						envVars = append(envVars, result)
						fmt.Printf("%s Added: %s\n",
							style.SuccessStyle.Render("✓"),
							style.HighlightStyle.Render(result))
					}
				}

				// Then, optionally select additional environment variables
				fmt.Printf("\n%s Add additional environment variables?\n",
					style.SubtitleStyle.Render("Additional Variables"))

				addMorePrompt := promptui.Prompt{
					Label:     "Add additional environment variables",
					IsConfirm: true,
				}

				if result, err := addMorePrompt.Run(); err == nil && strings.ToLower(result) == "y" {
					additionalSelect := promptui.Select{
						Label: "Add additional environment variables",
						Items: append(additionalEnvVars, "custom", "done"),
					}

					for {
						_, result, err := additionalSelect.Run()
						if err != nil || result == "done" {
							break
						}

						if result == "custom" {
							prompt := promptui.Prompt{
								Label: "Enter environment variable name",
							}
							customEnv, err := prompt.Run()
							if err != nil {
								break
							}
							if customEnv != "" {
								envVars = append(envVars, customEnv)
								fmt.Printf("%s Added: %s\n",
									style.SuccessStyle.Render("✓"),
									style.HighlightStyle.Render(customEnv))
							}
						} else {
							// Check if already selected
							exists := false
							for _, env := range envVars {
								if env == result {
									exists = true
									break
								}
							}
							if !exists {
								envVars = append(envVars, result)
								fmt.Printf("%s Added: %s\n",
									style.SuccessStyle.Render("✓"),
									style.HighlightStyle.Render(result))
							}
						}
					}
				}

				// Ask for output file
				if outputFile == "" {
					prompt := promptui.Prompt{
						Label:   "Output File",
						Default: fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(name, " ", "-"))),
					}
					outputFile, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}
			} else if name == "" {
				return fmt.Errorf("tool name is required in non-interactive mode")
			}

			// Generate alias from name
			alias := strings.ToLower(strings.ReplaceAll(name, "_", "-"))

			// Create tool structure
			tool := Tool{
				Name:        name,
				Image:       image,
				Description: description,
				Alias:       alias,
				LongRunning: longRunning,
				Content: `# Example tool script
# Set default values for environment variables
KUBIYA_EXAMPLE="${KUBIYA_EXAMPLE:-default_value}"

# Your tool logic here
echo "Running {{ .example_arg }}"`,
				Args: []ToolArg{
					{
						Name:        "example_arg",
						Description: "An example argument",
						Required:    true,
						Type:        "string",
					},
				},
				Env: envVars,
				WithVolumes: []Volume{
					{
						Name: "data",
						Path: "/data",
					},
				},
				WithFiles: []File{
					{
						Source:      "$HOME/.config/example",
						Destination: "/root/.config/example",
					},
				},
			}

			// If no output file specified, use the tool name
			if outputFile == "" {
				outputFile = fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(name, " ", "-")))
			}

			// Create tools array for YAML output
			tools := struct {
				Tools []Tool `yaml:"tools"`
			}{
				Tools: []Tool{tool},
			}

			// Marshal to YAML
			data, err := yaml.Marshal(tools)
			if err != nil {
				return fmt.Errorf("failed to generate YAML: %w", err)
			}

			// Write to file
			if err := os.WriteFile(outputFile, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			// Success message
			fmt.Printf("\n%s Created tool template: %s\n\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(outputFile))

			// Show the content
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Tool Definition"))
			fmt.Printf("```yaml\n%s```\n\n", string(data))

			// Show next steps
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Next Steps"))
			fmt.Printf("1. Edit %s to customize your tool\n", outputFile)
			fmt.Printf("2. Add the tool to a source:\n")
			fmt.Printf("   %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya source add --inline %s", outputFile)))
			fmt.Printf("3. Test your tool:\n")
			fmt.Printf("   %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya tool execute %s", name)))

			return nil
		},
	}

	// Add flags but make them optional since interactive is default
	cmd.Flags().StringVarP(&name, "name", "n", "", "Tool name")
	cmd.Flags().StringVarP(&description, "desc", "d", "", "Tool description")
	cmd.Flags().StringVarP(&image, "image", "i", "", "Container image (e.g., python:3.11)")
	cmd.Flags().BoolVarP(&longRunning, "long-running", "l", false, "Mark tool as long-running")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: <tool-name>.yaml)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Run in non-interactive mode")

	return cmd
}

func newInitWorkflowCommand(cfg *config.Config) *cobra.Command {
	var (
		name           string
		teammate       string
		outputFile     string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "📋 Initialize a new workflow",
		Example: `  # Create a workflow interactively (default)
  kubiya init workflow

  # Create a workflow non-interactively
  kubiya init workflow --non-interactive --name my-workflow --teammate demo_teammate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if !nonInteractive {
				// Interactive mode
				if name == "" {
					prompt := promptui.Prompt{
						Label:   "Workflow Name",
						Default: "my-workflow",
						Validate: func(s string) error {
							if s == "" {
								return fmt.Errorf("name is required")
							}
							return nil
						},
					}
					name, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}

				if teammate == "" {
					prompt := promptui.Prompt{
						Label:   "Teammate Name",
						Default: "demo_teammate",
					}
					teammate, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}

				// Ask for output file
				if outputFile == "" {
					prompt := promptui.Prompt{
						Label:   "Output File",
						Default: fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(name, " ", "-"))),
					}
					outputFile, err = prompt.Run()
					if err != nil {
						return fmt.Errorf("prompt failed: %w", err)
					}
				}
			} else if name == "" {
				return fmt.Errorf("workflow name is required in non-interactive mode")
			}

			if teammate == "" {
				teammate = "demo_teammate" // Default teammate
			}

			// Create workflow structure
			workflow := Workflow{
				Name: name,
				Steps: []WorkflowStep{
					{
						Name: "ask-kubiya",
						Executor: WorkflowExecutor{
							Type: "agent",
							Config: map[string]interface{}{
								"teammate_name": teammate,
								"message":       "run cluster health",
							},
						},
						Output: "AGENT_RESPONSE",
					},
					{
						Name: "send-to-slack",
						Executor: WorkflowExecutor{
							Type: "agent",
							Config: map[string]interface{}{
								"teammate_name": teammate,
								"message":       "Send a Slack msg to channel #tf-test saying $AGENT_RESPONSE , before sending make ths msg nice and readable ",
							},
						},
						Output:  "SLACK_RESPONSE",
						Depends: []string{"ask-kubiya"},
					},
				},
			}

			// If no output file specified, use the workflow name
			if outputFile == "" {
				outputFile = fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(name, " ", "-")))
			}

			// Marshal to YAML
			data, err := yaml.Marshal(workflow)
			if err != nil {
				return fmt.Errorf("failed to generate YAML: %w", err)
			}

			// Create directory if it doesn't exist
			dir := filepath.Dir(outputFile)
			if dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			// Write to file
			if err := os.WriteFile(outputFile, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			// Success message
			fmt.Printf("\n%s Created workflow template: %s\n\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(outputFile))

			// Show the content
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Workflow Definition"))
			fmt.Printf("```yaml\n%s```\n\n", string(data))

			// Show next steps
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Next Steps"))
			fmt.Printf("1. Edit %s to customize your workflow\n", outputFile)
			fmt.Printf("2. Add the workflow to a source:\n")
			fmt.Printf("   %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya source add --inline %s", outputFile)))
			fmt.Printf("3. Test your workflow:\n")
			fmt.Printf("   %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya workflow execute %s", name)))

			return nil
		},
	}

	// Add flags but make them optional since interactive is default
	cmd.Flags().StringVarP(&name, "name", "n", "", "Workflow name")
	cmd.Flags().StringVarP(&teammate, "teammate", "t", "", "Teammate name (default: demo_teammate)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: <workflow-name>.yaml)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Run in non-interactive mode")

	return cmd
}
