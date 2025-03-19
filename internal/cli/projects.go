package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

var (
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

func newProjectCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects"},
		Short:   "🏗️  Manage projects",
		Long:    `Create, list, and manage projects to orchestrate complex workflows.`,
	}

	cmd.AddCommand(
		newListProjectsCommand(cfg),
		newCreateProjectCommand(cfg),
		newUpdateProjectCommand(cfg),
		newDeleteProjectCommand(cfg),
		newDescribeProjectCommand(cfg),
		newListTemplatesCommand(cfg),
		newTemplateInfoCommand(cfg),
		newPlanProjectCommand(cfg),
		newApproveProjectCommand(cfg),
		newGetProjectLogsCommand(cfg),
	)

	return cmd
}

func newListProjectsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List projects",
		Example: `  # List all projects
  kubiya project list

  # Output in JSON format
  kubiya project list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProjects(cmd.Context(), cfg, outputFormat == "json")
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func listProjects(ctx context.Context, cfg *config.Config, plainOutput bool) error {
	// Store original debug setting
	oldDebug := cfg.Debug

	// Disable debug temporarily to avoid printing raw API responses
	cfg.Debug = false

	client := kubiya.NewClient(cfg)

	fmt.Println("🔍 Fetching projects...")
	projects, err := client.ListProjects(ctx)

	// Restore original debug setting
	cfg.Debug = oldDebug

	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if plainOutput {
		return json.NewEncoder(os.Stdout).Encode(projects)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found")
		fmt.Println("\nTo create a new project, run: kubiya project create --template <UUID> --name \"My Project\"")
		fmt.Println("Use: kubiya project templates to list available templates")
		return nil
	}

	// Since API doesn't return explicit status, treat all projects as available projects
	availableProjects := projects

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, style.TitleStyle.Render("🏗️ PROJECTS"))
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tDESCRIPTION\tSOURCE")

	// Function to print projects in a section
	printProjects := func(projects []kubiya.Project, sectionTitle string) {
		if len(projects) > 0 {
			fmt.Fprintln(w, style.SubtitleStyle.Render(sectionTitle))

			// Store project IDs for tips
			var projectIds []string

			for _, p := range projects {
				status := formatStatus(p.Status)
				if p.Status == "" {
					status = formatStatus("available") // Default status for available projects
				}

				// Use ID field instead of UUID
				id := p.ID
				if id == "" {
					id = style.DimStyle.Render("-")
				} else {
					// Don't shorten ID to make it copyable
					projectIds = append(projectIds, id)
				}

				// Get URL (repository) for display
				url := p.URL
				if url == "" {
					url = p.Repository
				}
				if url == "" {
					url = style.DimStyle.Render("-")
				} else {
					// Extract repo name from URL for cleaner display
					parts := strings.Split(url, "/")
					if len(parts) > 2 {
						url = parts[len(parts)-2] + "/" + parts[len(parts)-1]
					}
				}

				// Get description, limit size
				desc := p.Description
				if desc == "" && len(p.Readme) > 0 {
					// Extract first line of readme if available
					lines := strings.Split(p.Readme, "\n")
					if len(lines) > 0 {
						desc = strings.TrimPrefix(lines[0], "# ")
						// Remove markdown formatting if present
						desc = strings.TrimPrefix(desc, "**")
						desc = strings.TrimSuffix(desc, "**")
						desc = strings.TrimSpace(desc)
					}
				}
				if len(desc) > 30 {
					desc = desc[:27] + "..."
				}
				if desc == "" {
					desc = style.DimStyle.Render("-")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					style.DimStyle.Render(id),
					style.HighlightStyle.Render(p.Name),
					status,
					desc,
					url,
				)
			}
			fmt.Fprintln(w)
		}
	}

	// Print available projects
	printProjects(availableProjects, "Available Projects")

	// Show summary footer
	fmt.Fprintf(w, style.DimStyle.Render("Total: %d projects\n"),
		len(projects))

	// Flush the tabwriter to ensure table is displayed before the tips
	w.Flush()

	// Add tips section for common operations
	fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("📋 Quick Reference"))

	// Show example of a project ID from the list if available
	var exampleId string
	if len(projects) > 0 && projects[0].ID != "" {
		exampleId = projects[0].ID
	} else {
		exampleId = "project-id"
	}

	// Show common command examples
	fmt.Printf("• To view project details:\n  %s\n\n",
		style.DimStyle.Render(fmt.Sprintf("kubiya project describe %s", exampleId)))

	fmt.Printf("• To create a new project:\n  %s\n\n",
		style.DimStyle.Render("kubiya project create --template <template-id> --name \"My Project\""))

	fmt.Printf("• To list available templates:\n  %s\n\n",
		style.DimStyle.Render("kubiya project templates"))

	fmt.Printf("• To update a project:\n  %s\n\n",
		style.DimStyle.Render(fmt.Sprintf("kubiya project update %s --name \"New Name\"", exampleId)))

	fmt.Printf("• To create a project plan:\n  %s\n\n",
		style.DimStyle.Render(fmt.Sprintf("kubiya project plan %s", exampleId)))

	return nil
}

func newCreateProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		projectName       string
		projectDesc       string
		templateId        string
		varFlags          []string
		variablesFile     string
		outputFormat      string
		sensitiveVarFlags []string
		skipVarValidation bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "🏗️  Create a new project",
		Long:  `Create a new project from a template with variables and secrets.`,
		Example: `  # Create a project from a template
  kubiya project create --template abc123 --name "My Project"

  # Create a project with variables
  kubiya project create --template abc123 --name "My Project" --var key1=value1 --var key2=value2

  # Create a project with variables from a file
  kubiya project create --template abc123 --name "My Project" --variables-file ./variables.json

  # Create a project with sensitive variables
  kubiya project create --template abc123 --name "My Project" --sensitive-var api_key=ABC123
  
  # Using with environment variables (for secrets)
  export AWS_SECRET_ACCESS_KEY="your-secret"
  kubiya project create --template abc123 --name "My Project"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable debug output for cleaner UX
			oldDebug := cfg.Debug
			cfg.Debug = false

			// Create the client
			client := kubiya.NewClient(cfg)

			// Parse variables
			variables := make(map[string]string)
			for _, v := range varFlags {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("variable flag %q is not in key=value format", v)
				}
				variables[parts[0]] = parts[1]
			}

			// Parse sensitive variables
			sensitiveVariables := make(map[string]string)
			for _, v := range sensitiveVarFlags {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("sensitive variable flag %q is not in key=value format", v)
				}
				sensitiveVariables[parts[0]] = parts[1]
			}

			// Parse variables file if provided
			if variablesFile != "" {
				data, err := os.ReadFile(variablesFile)
				if err != nil {
					return fmt.Errorf("failed to read variables file: %w", err)
				}
				var fileVars map[string]string
				if err := json.Unmarshal(data, &fileVars); err != nil {
					return fmt.Errorf("failed to parse variables file: %w", err)
				}
				for k, v := range fileVars {
					variables[k] = v
				}
			}

			// Combine variables and sensitive variables for validation
			allVariables := make(map[string]string)
			for k, v := range variables {
				allVariables[k] = v
			}
			for k, v := range sensitiveVariables {
				allVariables[k] = v
			}

			// Show project setup information
			fmt.Printf("\n🛠️  Setting up your project")
			if templateId != "" {
				fmt.Printf(" using template %s", style.HighlightStyle.Render(templateId))
			}
			fmt.Println("...")

			// If using a template and not skipping validation, fetch template details and validate variables
			var template *kubiya.ProjectTemplate
			var missingRequired []string
			var extraVars []string
			var typeErrors []string
			requiredVars := make(map[string]bool)
			var missingSecrets []string

			if templateId != "" && !skipVarValidation {
				// Fetch the template to validate variables
				var err error
				template, err = client.GetProjectTemplate(cmd.Context(), templateId)
				if err != nil {
					cfg.Debug = oldDebug
					return fmt.Errorf("failed to fetch template %s: %w", templateId, err)
				}

				// Check for required secrets
				if len(template.Secrets) > 0 {
					for _, secret := range template.Secrets {
						if secret.ToEnv != "" {
							// Check if the environment variable is set
							if _, exists := os.LookupEnv(secret.ToEnv); !exists {
								missingSecrets = append(missingSecrets, secret.ToEnv)
							}
						} else if secret.Name != "" {
							// Fallback to check with the secret name
							if _, exists := os.LookupEnv(secret.Name); !exists {
								missingSecrets = append(missingSecrets, secret.Name)
							}
						}
					}
				}

				// Build list of all expected variables from template and resources
				expectedVars := make(map[string]kubiya.TemplateVariable)

				// Process template variables
				for _, v := range template.Variables {
					expectedVars[v.Name] = v
					if v.Required && v.Default == nil {
						requiredVars[v.Name] = true
					}
				}

				// Process resource variables
				for _, resource := range template.Resources {
					for _, v := range resource.Variables {
						expectedVars[v.Name] = kubiya.TemplateVariable{
							Name:        v.Name,
							Type:        v.Type,
							Default:     v.Default,
							Description: v.Description,
							Required:    true, // Assume resource variables are required
						}
						requiredVars[v.Name] = true
					}
				}

				// Check for missing required variables
				for varName := range requiredVars {
					_, providedInVars := allVariables[varName]
					if !providedInVars {
						missingRequired = append(missingRequired, varName)
					}
				}

				// Check for extra variables not expected by the template
				for varName := range allVariables {
					if _, exists := expectedVars[varName]; !exists {
						extraVars = append(extraVars, varName)
					}
				}

				// Validate variable types
				for varName, varValue := range allVariables {
					if templateVar, exists := expectedVars[varName]; exists {
						valid, err := validateVariableType(varName, varValue, templateVar.Type)
						if !valid {
							typeErrors = append(typeErrors, fmt.Sprintf("%s: %s", varName, err))
						}
					}
				}
			}

			// Display validation results if using a template
			if templateId != "" && !skipVarValidation {
				if len(missingRequired) > 0 || len(extraVars) > 0 || len(typeErrors) > 0 || len(missingSecrets) > 0 {
					fmt.Println()
				}

				// Show variable validation issues if any
				if len(missingRequired) > 0 {
					fmt.Println(style.ErrorStyle.Render("❌ Missing required variables:"))
					for _, v := range missingRequired {
						fmt.Printf("   - %s\n", style.HighlightStyle.Render(v))
					}
					fmt.Println()
				}

				// Show missing secrets if any
				if len(missingSecrets) > 0 {
					fmt.Println(style.ErrorStyle.Render("❌ Missing required environment variables for secrets:"))
					for _, s := range missingSecrets {
						fmt.Printf("   - %s\n", style.HighlightStyle.Render(s))
					}
					fmt.Println()
				}

				if len(extraVars) > 0 {
					fmt.Println(style.WarningStyle.Render("⚠️  Variables not defined in template (will be passed through):"))
					for _, v := range extraVars {
						fmt.Printf("   - %s\n", style.HighlightStyle.Render(v))
					}
					fmt.Println()
				}

				if len(typeErrors) > 0 {
					fmt.Println(style.ErrorStyle.Render("❌ Variable type validation errors:"))
					for _, err := range typeErrors {
						fmt.Printf("   - %s\n", err)
					}
					fmt.Println()
				}

				// If missing required variables or secrets, show available variables and exit
				if len(missingRequired) > 0 || len(missingSecrets) > 0 {
					if len(template.Variables) > 0 {
						fmt.Println(infoStyle.Render("ℹ️  Required variables for this template:"))
						w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
						fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION")

						for _, v := range template.Variables {
							if v.Required && v.Default == nil {
								desc := v.Description
								if desc == "" {
									desc = style.DimStyle.Render("No description")
								}
								fmt.Fprintf(w, "%s\t%s\t%s\n",
									style.HighlightStyle.Render(v.Name),
									v.Type,
									desc)
							}
						}

						for _, resource := range template.Resources {
							for _, v := range resource.Variables {
								desc := v.Description
								if desc == "" {
									desc = style.DimStyle.Render("No description")
								}
								fmt.Fprintf(w, "%s\t%s\t%s\n",
									style.HighlightStyle.Render(v.Name),
									v.Type,
									desc)
							}
						}

						w.Flush()
						fmt.Println()
					}

					if len(template.Secrets) > 0 {
						fmt.Println(infoStyle.Render("ℹ️  Required environment variables for secrets:"))
						for _, s := range template.Secrets {
							envVar := s.ToEnv
							if envVar == "" {
								envVar = s.Name
							}
							desc := s.Description
							if desc == "" {
								desc = "No description"
							}
							fmt.Printf("   %s: %s\n",
								style.HighlightStyle.Render(envVar),
								style.DimStyle.Render(desc))
						}
						fmt.Println()

						// Show example exports
						fmt.Println(style.DimStyle.Render("# Example export commands:"))
						for _, s := range template.Secrets {
							envVar := s.ToEnv
							if envVar == "" {
								envVar = s.Name
							}
							fmt.Printf("%s\n",
								style.DimStyle.Render(fmt.Sprintf("export %s=\"your-secret-value\"", envVar)))
						}
						fmt.Println()
					}

					fmt.Println(infoStyle.Render("To see all template details:"))
					fmt.Printf("   %s\n\n", style.DimStyle.Render(fmt.Sprintf("kubiya project template-info %s", templateId)))

					return fmt.Errorf("missing required variables or environment variables for template %s", templateId)
				}

				// If there are type errors, exit
				if len(typeErrors) > 0 {
					return fmt.Errorf("variable type validation failed")
				}
			}

			// Create the project
			result, err := client.CreateProject(
				cmd.Context(),
				templateId,
				projectName,
				projectDesc,
				variables,
			)

			// Restore debug setting
			cfg.Debug = oldDebug

			if err != nil {
				return fmt.Errorf("failed to create project: %w", err)
			}

			// Handle output formats
			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(result)
			case "text":
				// ID is the uuid unless ID is populated
				id := result.UUID
				if result.ID != "" {
					id = result.ID
				}

				title := "🎉 Project Created Successfully!"
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(title))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				printDetailRow(w, "Project ID", id)
				printDetailRow(w, "Name", result.Name)

				if result.Description != "" {
					printDetailRow(w, "Description", result.Description)
				}

				status := result.Status
				if status == "" {
					status = style.DimStyle.Render("Pending")
				}
				printDetailRow(w, "Status", status)

				// Print template info if available
				if templateId != "" {
					printDetailRow(w, "Template", templateId)
					printDetailRow(w, "Provisioner", style.HighlightStyle.Render("Terraform"))
				}

				w.Flush()

				// Print variables if any
				if len(result.Variables) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Variables"))
					w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "NAME\tVALUE")

					for _, v := range result.Variables {
						if v.Name != "" {
							value := v.Value
							if v.Sensitive {
								value = "********"
							}
							fmt.Fprintf(w, "%s\t%s\n",
								style.HighlightStyle.Render(v.Name),
								value)
						}
					}

					w.Flush()
				}

				// Show next steps and helpful commands
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render("📋 Next Steps"))

				fmt.Printf("• %s\n  %s\n\n",
					style.HighlightStyle.Render("View your project details:"),
					style.DimStyle.Render(fmt.Sprintf("kubiya project describe %s", id)))

				fmt.Printf("• %s\n  %s\n\n",
					style.HighlightStyle.Render("Create and run a plan:"),
					style.DimStyle.Render(fmt.Sprintf("kubiya project plan %s", id)))

				fmt.Printf("• %s\n  %s\n\n",
					style.HighlightStyle.Render("Add or update a variable:"),
					style.DimStyle.Render(fmt.Sprintf("kubiya project update %s --var key=value", id)))

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&projectName, "name", "n", "", "Project name")
	cmd.Flags().StringVarP(&projectDesc, "description", "d", "", "Project description")
	cmd.Flags().StringVarP(&templateId, "template", "t", "", "Template ID to use for the project")
	cmd.Flags().StringVarP(&variablesFile, "variables-file", "f", "", "Path to a JSON file containing variables")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringSliceVarP(&varFlags, "var", "v", []string{}, "Variable in key=value format (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&sensitiveVarFlags, "sensitive-var", []string{}, "Sensitive variable in key=value format (can be specified multiple times)")
	cmd.Flags().BoolVar(&skipVarValidation, "skip-var-validation", false, "Skip validation of variables against template (not recommended)")

	// Make name required
	cmd.MarkFlagRequired("name")

	return cmd
}

// Helper function to validate variable types
func validateVariableType(name, value, expectedType string) (bool, string) {
	switch expectedType {
	case "string":
		// All values can be treated as strings
		return true, ""

	case "number":
		// Try to parse as a number
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return false, fmt.Sprintf("expected number but got '%s'", value)
		}

	case "bool", "boolean":
		// Try to parse as a boolean
		lowerValue := strings.ToLower(value)
		if lowerValue != "true" && lowerValue != "false" &&
			lowerValue != "0" && lowerValue != "1" &&
			lowerValue != "yes" && lowerValue != "no" {
			return false, fmt.Sprintf("expected boolean but got '%s'", value)
		}

	case "list", "array":
		// Try to parse as a JSON array
		var list []interface{}
		if err := json.Unmarshal([]byte(value), &list); err != nil {
			return false, fmt.Sprintf("expected JSON array but got '%s'", value)
		}

	case "map", "object":
		// Try to parse as a JSON object
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(value), &m); err != nil {
			return false, fmt.Sprintf("expected JSON object but got '%s'", value)
		}
	}

	return true, ""
}

func newUpdateProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		projectName    string
		projectDesc    string
		variableValues []string
		outputFormat   string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "update [project-uuid]",
		Short: "🔄 Update a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update a project
  kubiya project update abc123 --name "New Name" --description "New description"

  # Update project variables
  kubiya project update abc123 --var key1=value1 --var key2=value2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			projectUUID := args[0]

			// Get current project to preserve values not being updated
			project, err := client.GetProject(cmd.Context(), projectUUID)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}

			// Update name and description if provided
			if projectName != "" {
				project.Name = projectName
			}

			if projectDesc != "" {
				project.Description = projectDesc
			}

			// Parse variables
			variables := make(map[string]string)
			// Extract existing variables
			if project.Variables != nil {
				for _, varObj := range project.Variables {
					variables[varObj.Name] = fmt.Sprintf("%v", varObj.Value)
				}
			}

			// Add new variables
			for _, vv := range variableValues {
				parts := strings.SplitN(vv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid variable format: %s (expected key=value)", vv)
				}
				variables[parts[0]] = parts[1]
			}

			// If we have a template ID, validate variables against it
			if project.UsecaseID != "" {
				// Disable debug temporarily
				oldDebug := cfg.Debug
				cfg.Debug = false

				// Get template details
				template, tErr := client.GetProjectTemplate(cmd.Context(), project.UsecaseID)

				// Restore debug setting
				cfg.Debug = oldDebug

				if tErr != nil {
					fmt.Printf("⚠️  Warning: Could not validate variables against template: %v\n", tErr)
				} else {
					// Display template information
					fmt.Printf("Template: %s\n", style.HighlightStyle.Render(template.Name))

					// Check for missing required variables
					missingRequired := []string{}
					for _, v := range template.Variables {
						if v.Required && v.Default == "" {
							if _, exists := variables[v.Name]; !exists {
								missingRequired = append(missingRequired, v.Name)
							}
						}
					}

					if len(missingRequired) > 0 {
						fmt.Println("\n❌ Missing required variables:")
						for _, name := range missingRequired {
							// Find the variable to get its description
							for _, v := range template.Variables {
								if v.Name == name {
									fmt.Printf("  • %s (%s): %s\n",
										style.ErrorStyle.Render(name),
										v.Type,
										v.Description,
									)
									break
								}
							}
						}
						return fmt.Errorf("please provide all required variables")
					}

					// Display the variables we're updating
					if len(variableValues) > 0 {
						fmt.Println("\nUpdating variables:")
						w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
						fmt.Fprintln(w, "NAME\tTYPE\tNEW VALUE\tDESCRIPTION")

						for _, vv := range variableValues {
							parts := strings.SplitN(vv, "=", 2)
							varName := parts[0]
							varValue := parts[1]

							// Find variable in template
							varType := "string" // default type
							varDesc := ""

							for _, v := range template.Variables {
								if v.Name == varName {
									varType = v.Type
									varDesc = v.Description
									break
								}
							}

							// Mark variables not in template
							if varDesc == "" {
								varDesc = style.DimStyle.Render("(custom variable)")
							}

							// Hide sensitive values
							displayValue := varValue
							for _, v := range template.Variables {
								if v.Name == varName && v.Sensitive {
									displayValue = "********"
									break
								}
							}

							fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
								style.HighlightStyle.Render(varName),
								varType,
								displayValue,
								varDesc,
							)
						}
						w.Flush()
						fmt.Println()
					}
				}
			}

			fmt.Println("🔄 Updating project...")
			// Update the project
			updatedProject, err := client.UpdateProject(cmd.Context(), projectUUID, project.Name, project.Description, variables)
			if err != nil {
				return fmt.Errorf("failed to update project: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(updatedProject)
			case "text":
				fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Project updated successfully!"))
				projectID := updatedProject.ID
				if projectID == "" {
					projectID = updatedProject.UUID
				}
				fmt.Printf("Project ID: %s\n", style.HighlightStyle.Render(projectID))
				fmt.Printf("Name: %s\n", updatedProject.Name)
				fmt.Printf("Status: %s\n", formatStatus(updatedProject.Status))

				// Show next steps
				fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("Next Steps"))
				fmt.Printf("• To view project details:\n  %s\n\n",
					style.DimStyle.Render(fmt.Sprintf("kubiya project describe %s", projectID)))

				fmt.Printf("• To create a plan for this project:\n  %s\n\n",
					style.DimStyle.Render(fmt.Sprintf("kubiya project plan %s", projectID)))

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&projectName, "name", "n", "", "Project name")
	cmd.Flags().StringVarP(&projectDesc, "description", "d", "", "Project description")
	cmd.Flags().StringArrayVarP(&variableValues, "var", "v", []string{}, "Variable values in key=value format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&nonInteractive, "non-interactive", "", false, "Run in non-interactive mode")

	return cmd
}

func newDeleteProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		force bool
	)

	cmd := &cobra.Command{
		Use:   "delete [project-uuid]",
		Short: "🗑️ Delete a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Delete a project
  kubiya project delete abc123

  # Force delete without confirmation
  kubiya project delete abc123 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			projectUUID := args[0]

			// Get project details
			project, err := client.GetProject(cmd.Context(), projectUUID)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}

			// Confirm deletion unless forced
			if !force {
				fmt.Printf("Are you sure you want to delete project '%s' (%s)? [y/N] ", project.Name, projectUUID)
				var confirm string
				fmt.Scanln(&confirm)

				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			// Delete the project
			if err := client.DeleteProject(cmd.Context(), projectUUID); err != nil {
				return fmt.Errorf("failed to delete project: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Project deleted successfully!"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	return cmd
}

func newDescribeProjectCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "describe [project-uuid]",
		Short: "📝 Describe a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Describe a project
  kubiya project describe abc123

  # Output in JSON format
  kubiya project describe abc123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable debug output for cleaner UX
			oldDebug := cfg.Debug
			cfg.Debug = false

			client := kubiya.NewClient(cfg)
			projectUUID := args[0]

			fmt.Printf("\n🔍 Fetching project details for %s...\n", style.HighlightStyle.Render(projectUUID))

			project, err := client.GetProject(cmd.Context(), projectUUID)

			// Restore debug setting
			cfg.Debug = oldDebug

			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(project)
			case "text":
				// Create a title with the project name
				title := fmt.Sprintf("🏗️  Project: %s", project.Name)
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(title))

				// Create a tabwriter for aligned output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Print basic project details
				printDetailRow(w, "ID", project.ID)
				printDetailRow(w, "UUID", project.UUID)

				// Handle description - extract from README if empty
				desc := project.Description
				if desc == "" && len(project.Readme) > 0 {
					lines := strings.Split(project.Readme, "\n")
					if len(lines) > 0 {
						desc = strings.TrimPrefix(lines[0], "# ")
						desc = strings.TrimPrefix(desc, "**")
						desc = strings.TrimSuffix(desc, "**")
						desc = strings.TrimSpace(desc)
					}
				}
				printDetailRow(w, "Description", desc)

				// Print status with formatting
				status := formatStatus(project.Status)
				if project.Status == "" {
					status = formatStatus("available")
				}
				fmt.Fprintf(w, "Status:\t%s\n", status)

				// Print other details
				printDetailRow(w, "Usecase ID", project.UsecaseID)

				// Format timestamps nicely
				printDetailRow(w, "Created At", formatTimestamp(project.CreatedAt))
				printDetailRow(w, "Updated At", formatTimestamp(project.UpdatedAt))

				// Print URL/Repository
				url := project.URL
				if url == "" {
					url = project.Repository
				}
				printDetailRow(w, "Source URL", url)

				w.Flush()

				// Print variables section if available
				if len(project.Variables) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Variables"))
					w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "KEY\tVALUE")
					for _, varObj := range project.Variables {
						// Skip empty variables
						if varObj.Name == "" {
							continue
						}

						// Convert value to string for display
						valueStr := fmt.Sprintf("%v", varObj.Value)

						// Check if this is a sensitive variable
						if varObj.Sensitive {
							valueStr = "********"
						}

						fmt.Fprintf(w, "%s\t%s\n", varObj.Name, valueStr)
					}
					w.Flush()
				}

				// Print README section if available
				if len(project.Readme) > 0 {
					readme := project.Readme
					// Limit readme length for display
					maxReadmeLen := 500
					if len(readme) > maxReadmeLen {
						readme = readme[:maxReadmeLen] + "...\n\n(Readme truncated, use API to get full content)"
					}

					fmt.Printf("\n%s\n\n", style.SubtitleStyle.Render("README"))
					fmt.Println(style.DimStyle.Render(readme))
				}

				// Add tips section for next steps with this project
				fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("📋 Next Steps"))

				projectId := project.ID
				if projectId == "" {
					projectId = project.UUID
				}

				// Show common command examples specific to this project
				fmt.Printf("• To update this project:\n  %s\n\n",
					style.DimStyle.Render(fmt.Sprintf("kubiya project update %s --name \"New Name\" --description \"New description\"", projectId)))

				fmt.Printf("• To delete this project:\n  %s\n\n",
					style.DimStyle.Render(fmt.Sprintf("kubiya project delete %s", projectId)))

				fmt.Printf("• To create a plan for this project:\n  %s\n\n",
					style.DimStyle.Render(fmt.Sprintf("kubiya project plan %s", projectId)))

				// Show example of how to handle variables if there are any
				if len(project.Variables) > 0 {
					// Get first variable name as an example
					var exampleVarName string
					for _, varObj := range project.Variables {
						if varObj.Name != "" {
							exampleVarName = varObj.Name
							break
						}
					}

					if exampleVarName != "" {
						fmt.Printf("• To update a variable value:\n  %s\n\n",
							style.DimStyle.Render(fmt.Sprintf("kubiya project update %s --var %s=new-value", projectId, exampleVarName)))
					}
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

// Helper function to print a detail row with proper formatting for missing values
func printDetailRow(w *tabwriter.Writer, label string, value string) {
	if value == "" {
		value = style.DimStyle.Render("N/A")
	}
	fmt.Fprintf(w, "%s:\t%s\n", label, value)
}

// Helper function to format timestamps
func formatTimestamp(timestamp string) string {
	if timestamp == "" {
		return style.DimStyle.Render("N/A")
	}

	// Try to parse the timestamp and format it nicely
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t.Format("Jan 02, 2006 15:04:05 MST")
	}

	// If parsing fails, return the original timestamp
	return timestamp
}

func newListTemplatesCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string
	var repository string
	var showVariables bool

	cmd := &cobra.Command{
		Use:   "templates",
		Short: "🔖 List available project templates",
		Long: `List available project templates that can be used to create projects.

You can use a custom repository by specifying the --repository flag.
The repository URL should be a GitHub repository URL.`,
		Example: `  # List all templates in the default repository
  kubiya project templates

  # List all templates in a custom repository
  kubiya project templates --repository https://github.com/yourusername/yourrepo

  # Show template details including variables and secrets
  kubiya project templates --show-variables`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable debug output for cleaner UX
			oldDebug := cfg.Debug
			cfg.Debug = false

			client := kubiya.NewClient(cfg)
			templates, err := client.ListProjectTemplates(cmd.Context(), repository)

			// Restore debug setting
			cfg.Debug = oldDebug

			if err != nil {
				return fmt.Errorf("failed to list templates: %w", err)
			}

			if len(templates) == 0 {
				fmt.Println("No templates found.")
				if repository != "" {
					fmt.Printf("You specified a custom repository: %s\n", repository)
					fmt.Println("Make sure this repository exists and contains Kubiya templates.")
				} else {
					fmt.Println("Try using the default repository or specify a custom one with --repository.")
				}
				return nil
			}

			// Store a template ID to use in the tips
			// Get the first template ID if available
			var exampleTemplateID string
			if len(templates) > 0 {
				exampleTemplateID = templates[0].ID
				if exampleTemplateID == "" {
					exampleTemplateID = templates[0].UUID
				}
			}

			// Base repository URL to show in output
			repoURL := "default repository"
			if repository != "" {
				repoURL = repository
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(templates)
			case "text":
				fmt.Printf("\n📋 Templates from %s\n\n", style.HighlightStyle.Render(repoURL))

				// Use tabwriter for clean alignment
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Define headers based on whether to show variables
				if showVariables {
					fmt.Fprintln(w, "ID\tNAME\tPROVISIONER\tDESCRIPTION\tVARIABLES\tSECRETS")
				} else {
					fmt.Fprintln(w, "ID\tNAME\tPROVISIONER\tDESCRIPTION")
				}

				for _, template := range templates {
					// Get the ID to display (prefer ID, fallback to UUID)
					displayID := template.ID
					if displayID == "" {
						displayID = template.UUID
					}

					// Prepare description
					description := template.Description
					if description == "" {
						description = style.DimStyle.Render("No description")
					} else if len(description) > 40 {
						description = description[:37] + "..."
					}

					// Count unique variables and secrets
					uniqueVars := make(map[string]bool)
					requiredVars := 0

					// Count template variables
					for _, v := range template.Variables {
						if v.Name != "" {
							uniqueVars[v.Name] = true
							if v.Required && v.Default == nil {
								requiredVars++
							}
						}
					}

					// Count resource variables
					for _, resource := range template.Resources {
						for _, v := range resource.Variables {
							if v.Name != "" {
								uniqueVars[v.Name] = true
								if v.Default == nil {
									requiredVars++
								}
							}
						}
					}

					// Get unique secrets
					uniqueSecrets := make(map[string]kubiya.TemplateSecret)
					for _, s := range template.Secrets {
						if s.Name != "" {
							uniqueSecrets[s.Name] = s
						}
					}

					if showVariables {
						// Build variable and secret info strings
						varInfo := fmt.Sprintf("%d", len(uniqueVars))
						if requiredVars > 0 {
							varInfo = fmt.Sprintf("%d (%d required)", len(uniqueVars), requiredVars)
						}

						secretInfo := fmt.Sprintf("%d", len(uniqueSecrets))
						if len(uniqueSecrets) > 0 {
							// Create a list of secret names with env vars
							secretList := ""
							for _, secret := range uniqueSecrets {
								envVar := secret.ToEnv
								if envVar == "" {
									envVar = secret.Name
								}
								if secretList != "" {
									secretList += ", "
								}
								secretList += fmt.Sprintf("%s (%s)",
									style.ErrorStyle.Render(secret.Name),
									style.HighlightStyle.Render(envVar))
							}
							secretInfo = secretList
						}

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
							style.HighlightStyle.Render(displayID),
							template.Name,
							style.HighlightStyle.Render("Terraform"),
							description,
							varInfo,
							secretInfo,
						)
					} else {
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
							style.HighlightStyle.Render(displayID),
							template.Name,
							style.HighlightStyle.Render("Terraform"),
							description,
						)
					}
				}

				w.Flush()

				// Add tips at the bottom for better UX
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render("💡 Quick Reference"))

				// Only provide tips if we have at least one template
				if exampleTemplateID != "" {
					fmt.Printf("• %s\n  %s\n\n",
						style.HighlightStyle.Render("Create a project from a template:"),
						style.DimStyle.Render(fmt.Sprintf("kubiya project create --template %s --name \"My Project\"", exampleTemplateID)))

					fmt.Printf("• %s\n  %s\n\n",
						style.HighlightStyle.Render("Create a project with variables:"),
						style.DimStyle.Render(fmt.Sprintf("kubiya project create --template %s --name \"My Project\" --var region=us-east-1", exampleTemplateID)))

					fmt.Printf("• %s\n  %s\n\n",
						style.HighlightStyle.Render("View existing projects:"),
						style.DimStyle.Render("kubiya project list"))

					// If a custom repository was used, show how to fetch from different repository
					if repository == "" {
						fmt.Printf("• %s\n  %s\n\n",
							style.HighlightStyle.Render("List templates from a custom repository:"),
							style.DimStyle.Render("kubiya project templates --repository https://github.com/your-org/terraform-templates"))
					} else {
						// Show how to view default templates
						fmt.Printf("• %s\n  %s\n\n",
							style.HighlightStyle.Render("List templates from default repository:"),
							style.DimStyle.Render("kubiya project templates"))
					}

					// Show how to view detailed template info
					fmt.Printf("• %s\n  %s\n\n",
						style.HighlightStyle.Render("Get detailed information about a template:"),
						style.DimStyle.Render(fmt.Sprintf("kubiya project template-info %s", exampleTemplateID)))
				}

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository URL to fetch templates from")
	cmd.Flags().BoolVar(&showVariables, "show-variables", false, "Show variables and secrets information")

	return cmd
}

// Add a new command to display detailed information about a template
func newTemplateInfoCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "template-info [template-id]",
		Short: "📋 Show detailed information about a template",
		Args:  cobra.ExactArgs(1),
		Example: `  # Show detailed information about a template
  kubiya project template-info abc123

  # Output in JSON format
  kubiya project template-info abc123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable debug output for cleaner UX
			oldDebug := cfg.Debug
			cfg.Debug = false

			client := kubiya.NewClient(cfg)
			templateID := args[0]

			fmt.Printf("\n🔍 Fetching template information for %s...\n", style.HighlightStyle.Render(templateID))

			template, err := client.GetProjectTemplate(cmd.Context(), templateID)

			// Restore debug setting
			cfg.Debug = oldDebug

			if err != nil {
				return fmt.Errorf("failed to get template: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(template)
			case "text":
				// Create a title with the template name
				title := fmt.Sprintf("📚 Template: %s", template.Name)
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(title))

				// Create a tabwriter for aligned output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Print basic template details
				printDetailRow(w, "ID", template.ID)
				if template.UUID != "" && template.UUID != template.ID {
					printDetailRow(w, "UUID", template.UUID)
				} else {
					printDetailRow(w, "UUID", style.DimStyle.Render("N/A"))
				}
				printDetailRow(w, "Name", template.Name)

				if template.Description != "" {
					printDetailRow(w, "Description", template.Description)
				} else {
					printDetailRow(w, "Description", style.DimStyle.Render("N/A"))
				}

				printDetailRow(w, "Provisioner", style.HighlightStyle.Render("Terraform"))

				// Print URL/Repository
				url := template.URL
				if url == "" {
					url = template.Repository
				}
				if url == "" {
					url = style.DimStyle.Render("No source URL provided")
				}
				printDetailRow(w, "Source URL", url)

				w.Flush()

				// Collect all variables in a deduplicated map
				allVariables := make(map[string]kubiya.TemplateVariable)
				secretsMap := make(map[string]kubiya.TemplateSecret)

				// Process template variables
				for _, v := range template.Variables {
					if v.Name == "" {
						continue // Skip empty variables
					}

					// Skip variables that are actually secrets
					isSecret := false
					for _, s := range template.Secrets {
						if s.Name == v.Name || s.ToEnv == v.Name {
							isSecret = true
							break
						}
					}
					if isSecret {
						continue
					}

					allVariables[v.Name] = v
				}

				// Process resource variables
				for _, resource := range template.Resources {
					for _, v := range resource.Variables {
						if v.Name == "" {
							continue // Skip empty variables
						}

						// Skip if it's a secret
						isSecret := false
						for _, s := range template.Secrets {
							if s.Name == v.Name || s.ToEnv == v.Name {
								isSecret = true
								break
							}
						}
						if isSecret {
							continue
						}

						// If already exists, only overwrite if not having default
						if existing, ok := allVariables[v.Name]; ok {
							// Keep the existing one if it has a default value and new one doesn't
							if existing.Default != nil && v.Default == nil {
								continue
							}
						}

						allVariables[v.Name] = kubiya.TemplateVariable{
							Name:        v.Name,
							Type:        v.Type,
							Default:     v.Default,
							Description: v.Description,
							Required:    v.Default == nil, // Only required if no default
						}
					}
				}

				// Process secrets
				for _, s := range template.Secrets {
					if s.Name == "" {
						continue // Skip empty secrets
					}
					secretsMap[s.Name] = s
				}

				// Prepare sorted lists of names for later use
				var variableNames []string
				for name := range allVariables {
					variableNames = append(variableNames, name)
				}
				sort.Strings(variableNames)

				var secretNames []string
				for name := range secretsMap {
					secretNames = append(secretNames, name)
				}
				sort.Strings(secretNames)

				// Print infrastructure details
				if len(template.Resources) > 0 || len(template.Providers) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Infrastructure Details"))
					w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

					if len(template.Providers) > 0 {
						providers := []string{}
						for _, p := range template.Providers {
							if p.Name != "" {
								providers = append(providers, p.Name)
							}
						}
						if len(providers) > 0 {
							printDetailRow(w, "Providers", strings.Join(providers, ", "))
						}
					}

					if len(template.Resources) > 0 {
						resourceTypes := []string{}
						resourceNames := []string{}
						for _, r := range template.Resources {
							if r.Type != "" {
								resourceTypes = append(resourceTypes, r.Type)
							}
							if r.Name != "" {
								resourceNames = append(resourceNames, r.Name)
							}
						}
						if len(resourceTypes) > 0 {
							printDetailRow(w, "Resource Types", strings.Join(resourceTypes, ", "))
						}
						if len(resourceNames) > 0 {
							printDetailRow(w, "Resources", strings.Join(resourceNames, ", "))
						}
						printDetailRow(w, "Resource Count", fmt.Sprintf("%d", len(template.Resources)))
					}

					w.Flush()
				}

				// Print variables section if available
				if len(allVariables) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Variables"))
					w = tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
					fmt.Fprintln(w, "NAME\tTYPE\tREQUIRED\tDEFAULT\tDESCRIPTION")

					for _, name := range variableNames {
						v := allVariables[name]

						required := "No"
						if v.Required && v.Default == nil {
							required = style.ErrorStyle.Render("Yes")
						}

						defaultVal := "none"
						if v.Default != nil {
							defaultVal = fmt.Sprintf("%v", v.Default)
						}

						description := v.Description
						if description == "" {
							description = style.DimStyle.Render("No description")
						} else if len(description) > 60 {
							description = description[:57] + "..."
						}

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
							style.HighlightStyle.Render(v.Name),
							v.Type,
							required,
							defaultVal,
							description,
						)
					}

					w.Flush()
				}

				// Print secrets section if available
				if len(secretsMap) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Secrets"))
					w = tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
					fmt.Fprintln(w, "NAME\tENVIRONMENT VARIABLE\tDESCRIPTION")

					for _, name := range secretNames {
						s := secretsMap[name]

						envVar := s.ToEnv
						if envVar == "" {
							envVar = s.Name
						}

						description := s.Description
						if description == "" {
							description = style.DimStyle.Render("No description")
						} else if len(description) > 60 {
							description = description[:57] + "..."
						}

						fmt.Fprintf(w, "%s\t%s\t%s\n",
							style.ErrorStyle.Render(s.Name),
							style.HighlightStyle.Render(envVar),
							description,
						)
					}

					w.Flush()

					// Add a note about how to provide secrets
					fmt.Printf("\n%s\n\n", infoStyle.Render("ℹ️ Secrets must be set as environment variables before creating a project."))

					// Show example export commands
					fmt.Println(style.DimStyle.Render("# Example:"))
					for _, name := range secretNames {
						s := secretsMap[name]
						envVar := s.ToEnv
						if envVar == "" {
							envVar = s.Name
						}
						fmt.Printf("%s\n", style.DimStyle.Render(fmt.Sprintf("export %s=\"your-secret-value\"", envVar)))
					}
					fmt.Println()
				}

				// Print Readme section if available
				if len(template.Readme) > 0 {
					readme := template.Readme
					// Limit readme length for display
					maxReadmeLen := 500
					if len(readme) > maxReadmeLen {
						readme = readme[:maxReadmeLen] + "...\n\n(Readme truncated, use API to get full content)"
					}

					fmt.Printf("\n%s\n\n", style.SubtitleStyle.Render("README"))
					fmt.Println(style.DimStyle.Render(readme))
				}

				// Add next steps section
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render("📋 Next Steps"))

				// Get the template ID for examples
				exampleId := template.ID
				if exampleId == "" {
					exampleId = template.UUID
				}

				// Show common command examples specific to this template
				fmt.Printf("• %s\n  %s\n\n",
					style.HighlightStyle.Render("To create a project from this template:"),
					style.DimStyle.Render(fmt.Sprintf("kubiya project create --template %s --name \"My Project\"", exampleId)))

				// If the template has required variables, show how to provide them
				var requiredVars []string
				for _, name := range variableNames {
					v := allVariables[name]
					if v.Required && v.Default == nil {
						requiredVars = append(requiredVars, v.Name)
					}
				}

				if len(requiredVars) > 0 {
					varArgs := ""
					for _, name := range requiredVars {
						varArgs += fmt.Sprintf(" --var %s=value", name)
					}

					fmt.Printf("• %s\n  %s\n\n",
						style.HighlightStyle.Render("This template requires the following variables:"),
						style.DimStyle.Render(fmt.Sprintf("kubiya project create --template %s --name \"My Project\"%s", exampleId, varArgs)))
				}

				// If the template has secrets, show how to handle them
				if len(secretsMap) > 0 {
					fmt.Printf("• %s\n", style.HighlightStyle.Render("This template requires the following secrets:"))
					for _, name := range secretNames {
						s := secretsMap[name]
						envVar := s.ToEnv
						if envVar == "" {
							envVar = s.Name
						}
						fmt.Printf("  %s\n", style.DimStyle.Render(fmt.Sprintf("- %s (exported as %s)", s.Name, envVar)))
					}
					fmt.Println()

					// Complete example including secrets
					fmt.Printf("• %s\n", style.HighlightStyle.Render("Complete example with secrets:"))

					// First show exports
					for _, name := range secretNames {
						s := secretsMap[name]
						envVar := s.ToEnv
						if envVar == "" {
							envVar = s.Name
						}
						fmt.Printf("  %s\n", style.DimStyle.Render(fmt.Sprintf("export %s=\"your-secret-value\"", envVar)))
					}

					// Then show command with variables
					cmd := fmt.Sprintf("kubiya project create --template %s --name \"My Project\"", exampleId)
					if len(requiredVars) > 0 {
						for _, name := range requiredVars {
							cmd += fmt.Sprintf(" --var %s=value", name)
						}
					}

					fmt.Printf("  %s\n\n", style.DimStyle.Render(cmd))
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

func newPlanProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		autoApprove  bool
		follow       bool
	)

	cmd := &cobra.Command{
		Use:   "plan [project-uuid]",
		Short: "📐 Create a plan for a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Create a plan for a project
  kubiya project plan abc123

  # Create a plan and auto-approve
  kubiya project plan abc123 --auto-approve`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			projectUUID := args[0]

			// Create a plan
			fmt.Println("Creating plan...")
			plan, err := client.CreateProjectPlan(cmd.Context(), projectUUID)
			if err != nil {
				return fmt.Errorf("failed to create plan: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Plan created successfully!"))
			fmt.Printf("Plan ID: %s\n", plan.PlanID)
			fmt.Printf("Status: %s\n", formatStatus(plan.Status))

			// Display changes
			if len(plan.Changes) > 0 {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Planned Changes"))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "RESOURCE\tACTION")
				for _, change := range plan.Changes {
					fmt.Fprintf(w, "%s\t%s\n", change.ResourceID, formatAction(change.Action))
				}
				w.Flush()
			} else {
				fmt.Println("\nNo changes detected")
			}

			// Auto-approve if requested
			if autoApprove && len(plan.Changes) > 0 {
				fmt.Println("\nAuto-approving plan...")
				execution, err := client.ApproveProjectPlan(cmd.Context(), plan.PlanID)
				if err != nil {
					return fmt.Errorf("failed to approve plan: %w", err)
				}

				fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Plan approved!"))
				fmt.Printf("Execution ID: %s\n", execution.ExecutionID)
				fmt.Printf("Status: %s\n", formatStatus(execution.Status))

				// Follow logs if requested
				if follow {
					fmt.Println("\nFollowing execution logs:")
					if err := followProjectExecution(client, execution.ExecutionID); err != nil {
						return fmt.Errorf("failed to follow execution: %w", err)
					}
				}
			} else if len(plan.Changes) > 0 {
				fmt.Printf("\nTo approve this plan, run: kubiya project approve %s\n", plan.PlanID)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&autoApprove, "auto-approve", "y", false, "Auto-approve the plan")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow execution logs after approval")
	return cmd
}

func newApproveProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		follow       bool
	)

	cmd := &cobra.Command{
		Use:   "approve [plan-id]",
		Short: "✅ Approve a project plan",
		Args:  cobra.ExactArgs(1),
		Example: `  # Approve a plan
  kubiya project approve plan123

  # Approve and follow execution logs
  kubiya project approve plan123 --follow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			planID := args[0]

			// Get the plan details
			plan, err := client.GetProjectPlan(cmd.Context(), planID)
			if err != nil {
				return fmt.Errorf("failed to get plan: %w", err)
			}

			// Display changes
			if len(plan.Changes) > 0 {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Planned Changes"))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "RESOURCE\tACTION")
				for _, change := range plan.Changes {
					fmt.Fprintf(w, "%s\t%s\n", change.ResourceID, formatAction(change.Action))
				}
				w.Flush()
			} else {
				fmt.Println("\nNo changes detected")
				return nil
			}

			// Confirm approval
			fmt.Print("\nDo you want to approve this plan? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm)

			if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
				fmt.Println("Approval cancelled")
				return nil
			}

			// Approve the plan
			fmt.Println("Approving plan...")
			execution, err := client.ApproveProjectPlan(cmd.Context(), planID)
			if err != nil {
				return fmt.Errorf("failed to approve plan: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Plan approved!"))
			fmt.Printf("Execution ID: %s\n", execution.ExecutionID)
			fmt.Printf("Status: %s\n", formatStatus(execution.Status))

			// Follow logs if requested
			if follow {
				fmt.Println("\nFollowing execution logs:")
				if err := followProjectExecution(client, execution.ExecutionID); err != nil {
					return fmt.Errorf("failed to follow execution: %w", err)
				}
			} else {
				fmt.Printf("\nTo view logs, run: kubiya project logs %s\n", execution.ExecutionID)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow execution logs")
	return cmd
}

func newGetProjectLogsCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		follow       bool
	)

	cmd := &cobra.Command{
		Use:   "logs [execution-id]",
		Short: "📝 View project execution logs",
		Args:  cobra.ExactArgs(1),
		Example: `  # View execution logs
  kubiya project logs exec123

  # Follow logs in real-time
  kubiya project logs exec123 --follow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			executionID := args[0]

			// Get execution details
			execution, err := client.GetProjectExecution(cmd.Context(), executionID)
			if err != nil {
				return fmt.Errorf("failed to get execution: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 📝 Execution Logs: %s ", executionID)))
			fmt.Printf("Project: %s\n", execution.ProjectID)
			fmt.Printf("Plan: %s\n", execution.PlanID)
			fmt.Printf("Status: %s\n", formatStatus(execution.Status))
			fmt.Printf("Started: %s\n", execution.StartTime.Format(time.RFC3339))
			if execution.EndTime != nil {
				fmt.Printf("Completed: %s\n", execution.EndTime.Format(time.RFC3339))
			}
			fmt.Println()

			if follow {
				return followProjectExecution(client, execution.ExecutionID)
			}

			// Get logs
			logs, err := client.GetProjectExecutionLogs(cmd.Context(), executionID)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}

			for _, log := range logs {
				fmt.Println(log)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs in real-time")
	return cmd
}

// Helper functions

func formatStatus(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return style.DimStyle.Render("⏳ Pending")
	case "running":
		return style.HighlightStyle.Render("🔄 Running")
	case "completed":
		return style.SuccessStyle.Render("✅ Completed")
	case "failed":
		return style.ErrorStyle.Render("❌ Failed")
	case "approved":
		return style.SuccessStyle.Render("✓ Approved")
	case "rejected":
		return style.ErrorStyle.Render("✗ Rejected")
	case "available":
		return style.SuccessStyle.Render("✅ Available")
	case "":
		return style.DimStyle.Render("⏳ Pending")
	default:
		return status
	}
}

func formatAction(action string) string {
	switch strings.ToLower(action) {
	case "create":
		return style.SuccessStyle.Render("+ Create")
	case "update":
		return style.HighlightStyle.Render("~ Update")
	case "delete":
		return style.ErrorStyle.Render("- Delete")
	default:
		return action
	}
}

func followProjectExecution(client *kubiya.Client, executionID string) error {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIdx := 0
	lastLogCount := 0

	ctx := context.Background()

	for {
		time.Sleep(1 * time.Second)

		// Get execution status
		execution, err := client.GetProjectExecution(ctx, executionID)
		if err != nil {
			return fmt.Errorf("failed to get execution status: %w", err)
		}

		// Get logs
		logs, err := client.GetProjectExecutionLogs(ctx, executionID)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		// Print new logs
		for i := lastLogCount; i < len(logs); i++ {
			fmt.Println(logs[i])
		}
		lastLogCount = len(logs)

		// Check if execution is complete
		if execution.Status == "completed" || execution.Status == "failed" {
			if execution.Status == "completed" {
				fmt.Fprintf(os.Stderr, "\n%s\n", style.HighlightStyle.Render("✅ Execution completed"))
			} else {
				fmt.Fprintf(os.Stderr, "\n%s\n", style.HighlightStyle.Render("❌ Execution failed"))
			}
			break
		} else {
			// Show spinner
			fmt.Fprintf(os.Stderr, "\r%s %s", spinner[spinnerIdx], formatStatus(execution.Status))
			spinnerIdx = (spinnerIdx + 1) % len(spinner)
		}
	}

	return nil
}
