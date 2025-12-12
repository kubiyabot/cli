package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newProjectCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects"},
		Short:   "🏗️  Manage projects",
		Long:    `Manage projects to organize agents and teams.`,
	}

	cmd.AddCommand(
		newListProjectsCommand(cfg),
		newGetProjectCommand(cfg),
		newCreateProjectCommand(cfg),
		newUpdateProjectCommand(cfg),
		newDeleteProjectCommand(cfg),
	)

	return cmd
}

func newListProjectsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List all projects",
		Example: `  # List all projects
  kubiya project list

  # Output in JSON format
  kubiya project list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			fmt.Println("🔍 Fetching projects...")
			projects, err := client.ListProjects()
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(projects)
			}

			if len(projects) == 0 {
				fmt.Println("\n╭─────────────────────╮")
				fmt.Println("│                     │")
				fmt.Println("│  No projects found  │")
				fmt.Println("│                     │")
				fmt.Println("╰─────────────────────╯")
				fmt.Println("\nTo create a new project:")
				fmt.Println("  kubiya project create --name \"My Project\"")
				return nil
			}

			// Display projects in a table
			fmt.Printf("\n╔════════════════════════════╗\n")
			fmt.Printf("║                            ║\n")
			fmt.Printf("║   🏗️  Projects (%d)   ║\n", len(projects))
			fmt.Printf("║                            ║\n")
			fmt.Printf("╚════════════════════════════╝\n")
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, " ID\tNAME\tDESCRIPTION\tCREATED")
			fmt.Fprintln(w, strings.Repeat("─", 100))

			for _, p := range projects {
				desc := "-"
				if p.Description != nil && *p.Description != "" {
					desc = *p.Description
					if len(desc) > 40 {
						desc = desc[:37] + "..."
					}
				}

				created := "-"
				if p.CreatedAt != nil {
					created = p.CreatedAt.Format("2006-01-02 15:04")
				}

				// Shorten ID for display
				displayID := p.ID
				if len(displayID) > 12 {
					displayID = displayID[:8] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					style.DimStyle.Render(displayID),
					style.HighlightStyle.Render(p.Name),
					desc,
					created,
				)
			}
			w.Flush()

			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newGetProjectCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get [project-id]",
		Short: "📝 Get project details",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get project details
  kubiya project get abc123

  # Output in JSON format
  kubiya project get abc123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			project, err := client.GetProject(projectID)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(project)
			}

			// Display project details
			fmt.Printf("\n╔═══════════════════════╗\n")
			fmt.Printf("║                       ║\n")
			fmt.Printf("║   🏗️  Project Details ║\n")
			fmt.Printf("║                       ║\n")
			fmt.Printf("╚═══════════════════════╝\n")
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", project.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(project.Name))

			if project.Description != nil && *project.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *project.Description)
			}

			if project.CreatedAt != nil {
				fmt.Fprintf(w, "Created:\t%s\n", project.CreatedAt.Format("2006-01-02 15:04:05"))
			}

			if project.UpdatedAt != nil {
				fmt.Fprintf(w, "Updated:\t%s\n", project.UpdatedAt.Format("2006-01-02 15:04:05"))
			}

			w.Flush()
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreateProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "➕ Create a new project",
		Example: `  # Create a project
  kubiya project create --name "My Project"

  # Create with description
  kubiya project create --name "My Project" --description "A test project"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			var desc *string
			if description != "" {
				desc = &description
			}

			req := &entities.ProjectCreateRequest{
				Name:        name,
				Description: desc,
			}

			fmt.Println("🛠️  Creating project...")
			project, err := client.CreateProject(req)
			if err != nil {
				return fmt.Errorf("failed to create project: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(project)
			}

			fmt.Printf("\n%s\n\n", style.TitleStyle.Render("✅ Project created successfully!"))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", project.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(project.Name))
			if project.Description != nil && *project.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *project.Description)
			}
			w.Flush()

			fmt.Println()
			fmt.Printf("To view project details:\n  %s\n\n",
				style.DimStyle.Render(fmt.Sprintf("kubiya project get %s", project.ID)))

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Project description")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newUpdateProjectCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "update [project-id]",
		Short: "🔄 Update a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update project name
  kubiya project update abc123 --name "New Name"

  # Update description
  kubiya project update abc123 --description "New description"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.ProjectUpdateRequest{}

			if name != "" {
				req.Name = &name
			}
			if description != "" {
				req.Description = &description
			}

			fmt.Println("🔄 Updating project...")
			project, err := client.UpdateProject(projectID, req)
			if err != nil {
				return fmt.Errorf("failed to update project: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(project)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Project updated successfully!"))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", project.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(project.Name))
			if project.Description != nil && *project.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *project.Description)
			}
			w.Flush()

			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "New project name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New project description")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

func newDeleteProjectCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [project-id]",
		Short: "🗑️ Delete a project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Delete a project
  kubiya project delete abc123

  # Force delete without confirmation
  kubiya project delete abc123 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get project details first
			project, err := client.GetProject(projectID)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}

			// Confirm deletion unless forced
			if !force {
				fmt.Printf("Are you sure you want to delete project '%s' (%s)? [y/N] ", project.Name, projectID)
				var confirm string
				fmt.Scanln(&confirm)

				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			// Delete the project
			if err := client.DeleteProject(projectID); err != nil {
				return fmt.Errorf("failed to delete project: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Project deleted successfully!"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	return cmd
}
