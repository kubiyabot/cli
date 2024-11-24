package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newSourcesCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Short:   "📦 Manage sources",
		Long: `Work with Kubiya sources - list, describe, and manage your tool sources.
Sources contain the tools and capabilities that your teammates can use.`,
	}

	cmd.AddCommand(
		newListSourcesCommand(cfg),
		newDescribeSourceCommand(cfg),
		newDeleteSourceCommand(cfg),
		newSyncSourceCommand(cfg),
		newSyncBatchCommand(cfg),
		newAddSourceCommand(cfg),
		newInteractiveSourceCommand(cfg),
	)

	return cmd
}

func newListSourcesCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all sources",
		Example: "  kubiya source list\n  kubiya source list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(sources)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.TitleStyle.Render(" 📦 SOURCES\n"))
				fmt.Fprintln(w, "UUID\tNAME\t🛠️ TOOLS\t👥 TEAMMATES\t⚠️ ERRORS\tMANAGED BY")
				for _, s := range sources {
					status := "✅"
					if s.ErrorsCount > 0 {
						status = fmt.Sprintf("⚠️ %d", s.ErrorsCount)
					}

					fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\n",
						s.UUID,
						s.Name,
						s.ConnectedToolsCount,
						s.ConnectedAgentsCount,
						status,
						s.ManagedBy,
					)
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newDescribeSourceCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "describe [source-uuid]",
		Short: "📖 Show detailed information about a source",
		Example: `  # Describe a source
  kubiya source describe abc-123

  # Output in JSON format
  kubiya source describe abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sourceUUID := args[0]

			source, err := client.GetSourceMetadata(cmd.Context(), sourceUUID)
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(source)
			case "text":
				// Header section
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 📦 Source Details "))

				// Basic information section
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Basic Information"))
				fmt.Printf("  Name:        %s\n", style.HighlightStyle.Render(source.Name))
				fmt.Printf("  UUID:        %s\n", source.UUID)
				fmt.Printf("  Description: %s\n", source.Description)
				fmt.Printf("  URL:         %s\n", source.URL)
				fmt.Printf("  Managed By:  %s\n", source.ManagedBy)
				fmt.Println()

				// Statistics section
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Statistics"))
				stats := []struct {
					name  string
					count int
					icon  string
				}{
					{"Tools", source.ConnectedToolsCount, "🛠️"},
					{"Agents", source.ConnectedAgentsCount, "🤖"},
					{"Workflows", source.ConnectedWorkflowsCount, "📋"},
					{"Errors", source.ErrorsCount, "⚠️"},
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, stat := range stats {
					fmt.Fprintf(w, "  %s %s:\t%s\n",
						stat.icon,
						stat.name,
						style.HighlightStyle.Render(fmt.Sprintf("%d", stat.count)))
				}
				w.Flush()
				fmt.Println()

				// Metadata section if available
				if source.KubiyaMetadata.LastUpdated != "" || source.KubiyaMetadata.UserLastUpdated != "" {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Kubiya Metadata"))
					if source.KubiyaMetadata.LastUpdated != "" {
						fmt.Printf("  Last Updated: %s\n", source.KubiyaMetadata.LastUpdated)
					}
					if source.KubiyaMetadata.UserLastUpdated != "" {
						fmt.Printf("  Updated By:   %s\n", source.KubiyaMetadata.UserLastUpdated)
					}
					fmt.Println()
				}

				// Tools section
				if len(source.Tools) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Available Tools"))
					for i, tool := range source.Tools {
						fmt.Printf("  %d. %s\n", i+1, style.HighlightStyle.Render(tool.Name))
						if tool.Description != "" {
							fmt.Printf("     %s\n", tool.Description)
						}

						// Show arguments if any
						if len(tool.Args) > 0 {
							fmt.Printf("     %s:\n", style.DimStyle.Render("Arguments"))
							for _, arg := range tool.Args {
								required := style.DimStyle.Render("optional")
								if arg.Required {
									required = style.HighlightStyle.Render("required")
								}
								fmt.Printf("       • %s: %s (%s)\n",
									style.HighlightStyle.Render(arg.Name),
									arg.Description,
									required)
							}
						}

						// Show environment variables if any
						if len(tool.Env) > 0 {
							fmt.Printf("     %s:\n", style.DimStyle.Render("Environment"))
							for _, env := range tool.Env {
								if icon, label, ok := getEnvIntegration(env); ok {
									fmt.Printf("       • %s %s %s\n",
										env,
										icon,
										style.DimStyle.Render(fmt.Sprintf("(Inherited from %s)", label)))
								} else {
									fmt.Printf("       • %s\n", env)
								}
							}
						}
						fmt.Println()
					}
				}

				// Timestamps section
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Timestamps"))
				fmt.Printf("  Created: %s\n", source.CreatedAt.Format(time.RFC822))
				fmt.Printf("  Updated: %s\n", source.UpdatedAt.Format(time.RFC822))
				fmt.Printf("  Age:     %s\n", formatDuration(time.Since(source.CreatedAt)))
				fmt.Println()

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

// Helper function to format duration in a human-readable way
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	parts := []string{}
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 && days == 0 { // Only show minutes if less than a day old
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}

	if len(parts) == 0 {
		return "less than a minute"
	}

	return strings.Join(parts, ", ")
}

func newDeleteSourceCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete [source-uuid]",
		Short:   "🗑️  Delete a source",
		Example: "  kubiya source delete abc-123\n  kubiya source delete abc-123 --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("⚠️  Are you sure you want to delete source %s? [y/N] ", args[0])
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			client := kubiya.NewClient(cfg)
			if err := client.DeleteSource(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("✅ Source %s deleted successfully\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")
	return cmd
}

func newSyncSourceCommand(cfg *config.Config) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "sync [uuid]",
		Short:   "🔄 Sync source",
		Example: "  kubiya source sync abc-123\n  kubiya source sync abc-123 --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get source details first for better feedback
			source, err := client.GetSourceMetadata(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get source details: %w", err)
			}

			if !yes {
				fmt.Printf("About to sync source:\n")
				fmt.Printf("  Name: %s\n", source.Name)
				if source.URL != "" {
					fmt.Printf("  URL: %s\n", source.URL)
				}
				fmt.Printf("  Connected Tools: %d\n", source.ConnectedToolsCount)
				fmt.Printf("  Connected Agents: %d\n", source.ConnectedAgentsCount)
				fmt.Print("\nDo you want to proceed? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			fmt.Printf("🔄 Syncing source %s...\n", source.Name)
			updated, err := client.SyncSource(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("✅ Successfully synced source: %s\n", updated.Name)
			fmt.Printf("Last updated: %s\n", updated.KubiyaMetadata.LastUpdated)
			fmt.Printf("Updated by: %s\n", updated.KubiyaMetadata.UserLastUpdated)

			if updated.ErrorsCount > 0 {
				return fmt.Errorf("source has %d error(s)", updated.ErrorsCount)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newSyncBatchCommand(cfg *config.Config) *cobra.Command {
	var (
		repoPath string
		yes      bool
	)

	cmd := &cobra.Command{
		Use:   "sync-batch",
		Short: "🔄 Batch sync sources related to a repository",
		Example: `  # Sync sources from a GitHub repository URL
  kubiya source sync-batch --repo https://github.com/kubiyabot/community-tools

  # Sync sources from a local directory
  kubiya source sync-batch --repo ./my-repo

  # Skip confirmation
  kubiya source sync-batch --repo ./my-repo --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get repository URL
			repoURL, err := getRepositoryURL(repoPath)
			if err != nil {
				return fmt.Errorf("failed to get repository URL: %w", err)
			}

			// List all sources
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list sources: %w", err)
			}

			// Find related sources
			var relatedSources []kubiya.Source
			for _, source := range sources {
				if isSourceRelatedToRepo(source, repoURL) {
					relatedSources = append(relatedSources, source)
				}
			}

			if len(relatedSources) == 0 {
				return fmt.Errorf("no sources found related to repository: %s", repoURL)
			}

			// Show preview and get confirmation
			if !yes {
				fmt.Printf("Found %d related sources:\n\n", len(relatedSources))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tURL\tTOOLS\tAGENTS")
				for _, s := range relatedSources {
					fmt.Fprintf(w, "%s\t%s\t%d\t%d\n",
						s.Name,
						s.URL,
						s.ConnectedToolsCount,
						s.ConnectedAgentsCount,
					)
				}
				w.Flush()

				fmt.Print("\nDo you want to sync these sources? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			// Sync each source
			fmt.Println("\n🔄 Starting batch sync...")
			var syncErrors []string
			for _, source := range relatedSources {
				fmt.Printf("Syncing %s... ", source.Name)
				updated, err := client.SyncSource(cmd.Context(), source.UUID)
				if err != nil {
					syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", source.Name, err))
					fmt.Println("❌")
					continue
				}
				fmt.Println("✅")

				if updated.ErrorsCount > 0 {
					syncErrors = append(syncErrors, fmt.Sprintf("%s: %d errors found", source.Name, updated.ErrorsCount))
				}
			}

			// Show summary
			fmt.Printf("\nSync completed for %d sources\n", len(relatedSources))
			if len(syncErrors) > 0 {
				var errMsg strings.Builder
				errMsg.WriteString(fmt.Sprintf("%d sources had errors:\n", len(syncErrors)))
				for _, err := range syncErrors {
					errMsg.WriteString(fmt.Sprintf("  • %s\n", err))
				}
				return fmt.Errorf(errMsg.String())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repoPath, "repo", "r", "", "Repository URL or local path")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func newAddSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		sourceURL string
		localPath string
		yes       bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "➕ Add a new source",
		Example: `  # Add source from GitHub URL
  kubiya source add --url https://github.com/kubiyabot/community-tools/tree/main/just_in_time_access

  # Add source from local directory (must be a git repository)
  kubiya source add --path ./my-tools

  # Skip confirmation
  kubiya source add --url https://github.com/org/repo --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get the URL to use
			var url string
			if sourceURL != "" {
				url = sourceURL
			} else if localPath != "" {
				// Get repository URL from local git repository
				repoURL, err := getRepositoryURL(localPath)
				if err != nil {
					return fmt.Errorf("failed to get repository URL from local path: %w", err)
				}
				url = repoURL
			} else {
				return fmt.Errorf("either --url or --path must be specified")
			}

			// Preview the source
			fmt.Printf("🔍 Loading source from: %s\n", url)
			metadata, err := client.LoadSource(cmd.Context(), url)
			if err != nil {
				return fmt.Errorf("failed to load source: %w", err)
			}

			// Show preview
			if !yes {
				fmt.Println("\n📦 Source Preview:")
				fmt.Printf("Name: %s\n", metadata.Name)
				if metadata.Description != "" {
					fmt.Printf("Description: %s\n", metadata.Description)
				}
				fmt.Printf("\n🛠️  Tools (%d):\n", len(metadata.Tools))
				for i, tool := range metadata.Tools {
					fmt.Printf("\n%d. %s\n", i+1, tool.Name)
					if tool.Description != "" {
						fmt.Printf("   Description: %s\n", tool.Description)
					}
					if len(tool.Args) > 0 {
						fmt.Printf("   Arguments:\n")
						for _, arg := range tool.Args {
							required := ""
							if arg.Required {
								required = " (required)"
							}
							fmt.Printf("   • %s%s\n", arg.Name, required)
						}
					}
				}

				fmt.Print("\nDo you want to add this source? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			// Create the source
			source, err := client.CreateSource(cmd.Context(), url)
			if err != nil {
				return fmt.Errorf("failed to create source: %w", err)
			}

			fmt.Printf("\n✅ Successfully added source: %s\n", source.Name)
			fmt.Printf("UUID: %s\n", source.UUID)
			fmt.Printf("Connected Tools: %d\n", source.ConnectedToolsCount)
			if source.ErrorsCount > 0 {
				fmt.Printf("⚠️  Source has %d error(s)\n", source.ErrorsCount)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sourceURL, "url", "", "URL to the source repository or directory")
	cmd.Flags().StringVar(&localPath, "path", "", "Path to local directory containing the source")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.MarkFlagsMutuallyExclusive("url", "path")

	return cmd
}

// Helper function to get repository URL from path or URL
func getRepositoryURL(path string) (string, error) {
	// If it's already a URL, normalize it
	if strings.HasPrefix(path, "http") {
		return normalizeGitHubURL(path)
	}

	// Try to get URL from local git repository
	cmd := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	return normalizeGitHubURL(strings.TrimSpace(string(output)))
}

// Helper function to normalize GitHub URLs
func normalizeGitHubURL(url string) (string, error) {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Convert SSH URLs to HTTPS
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}

	// Remove specific branch/file paths
	if strings.Contains(url, "/blob/") {
		parts := strings.Split(url, "/blob/")
		url = parts[0]
	} else if strings.Contains(url, "/tree/") {
		parts := strings.Split(url, "/tree/")
		url = parts[0]
	}

	return url, nil
}

// Helper function to check if a source is related to a repository
func isSourceRelatedToRepo(source kubiya.Source, repoURL string) bool {
	normalizedSourceURL, err := normalizeGitHubURL(source.URL)
	if err != nil {
		return false
	}

	// Check if the source URL is part of the repository
	return strings.HasPrefix(normalizedSourceURL, repoURL)
}

func newInteractiveSourceCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "interactive",
		Aliases: []string{"i"},
		Short:   "🎮 Start interactive source browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := tui.NewSourceBrowser(cfg)
			return app.Run()
		},
	}
}
