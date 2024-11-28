package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
)

// Add these constants for sync modes
const (
	SyncModeInteractive    = "interactive"
	SyncModeNonInteractive = "non-interactive"
	SyncModeCI             = "ci"
)

// Add this type to hold sync options
type SyncOptions struct {
	Mode       string
	Branch     string
	Path       string
	Name       string
	Force      bool
	AutoCommit bool
	NoDiff     bool
	RepoURL    string
}

// Add this type to track sync context
type SyncContext struct {
	RequiresBranchSwitch  bool
	CurrentBranch         string
	TargetBranch          string
	HasUncommittedChanges bool
	IsPathBased           bool
	SourceName            string
	Path                  string
}

func newSourcesCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Short:   "📦 Manage sources",
		Long: `Work with Kubiya sources - list, describe, and manage your tool sources.
Sources contain the tools and capabilities that your teammates can use.`,
	}

	cmd.AddCommand(
		newTeammateCommand(cfg),
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
				fmt.Fprintln(w, style.TitleStyle.Render("  SOURCES"))
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

// Add this helper type to store repository information
type RepoInfo struct {
	BaseURL string // Base repository URL
	Branch  string // Branch name
	Path    string // Path within repository
	FullURL string // Complete URL including branch and path
}

// Update the sync command to handle different scenarios
func newSyncSourceCommand(cfg *config.Config) *cobra.Command {
	var opts SyncOptions

	cmd := &cobra.Command{
		Use:   "sync [path-or-source-id]",
		Short: "🔄 Sync source",
		Example: `  # Interactive sync from current directory
  kubiya source sync

  # Non-interactive sync with auto-commit
  kubiya source sync mermaid --branch main --auto-commit --non-interactive

  # CI mode (no prompts, strict validation)
  kubiya source sync --ci --path mermaid --branch main

  # Sync by name
  kubiya source sync --name "Mermaid Tools"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine mode
			if opts.Force {
				opts.Mode = SyncModeCI
			} else if cmd.Flags().Changed("non-interactive") {
				opts.Mode = SyncModeNonInteractive
			} else {
				opts.Mode = SyncModeInteractive
			}

			// Handle path argument
			if len(args) > 0 {
				opts.Path = args[0]
			}

			return handleSourceSync(cmd.Context(), kubiya.NewClient(cfg), opts)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", "Specify branch to sync from")
	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Source name")
	cmd.Flags().StringVar(&opts.RepoURL, "repo", "", "Repository URL (HTTPS or SSH)")
	cmd.Flags().BoolVar(&opts.Force, "ci", false, "CI mode (strict, no prompts)")
	cmd.Flags().BoolVar(&opts.AutoCommit, "auto-commit", false, "Automatically commit changes")
	cmd.Flags().Bool("non-interactive", false, "Run without prompts")
	cmd.Flags().BoolVar(&opts.NoDiff, "no-diff", false, "Skip showing diff")

	return cmd
}

// Add this function to handle the sync logic
func handleSourceSync(ctx context.Context, client *kubiya.Client, opts SyncOptions) error {
	syncCtx := &SyncContext{
		IsPathBased: opts.Path != "" && opts.Name == "",
	}

	// Get current git info
	gitInfo, err := getGitCommitInfo(".")
	if err == nil {
		syncCtx.CurrentBranch = gitInfo.Branch
	} else {
		return fmt.Errorf("failed to get current git info: %w", err)
	}

	// Determine target branch
	if opts.Branch != "" {
		syncCtx.TargetBranch = opts.Branch
	} else {
		syncCtx.TargetBranch = syncCtx.CurrentBranch
	}

	// Determine if we need to switch branches
	if syncCtx.TargetBranch != syncCtx.CurrentBranch && syncCtx.IsPathBased {
		syncCtx.RequiresBranchSwitch = true
	}

	// Handle branch switching if needed
	if syncCtx.RequiresBranchSwitch {
		fmt.Printf("\n🔄 Switching to branch %s...\n", style.HighlightStyle.Render(syncCtx.TargetBranch))
		if err := switchBranch(syncCtx.TargetBranch); err != nil {
			return fmt.Errorf("failed to switch to branch %s: %w", syncCtx.TargetBranch, err)
		}

		// Ensure we switch back after sync
		defer func() {
			fmt.Printf("\n🔄 Switching back to %s...\n", style.HighlightStyle.Render(syncCtx.CurrentBranch))
			switchBranch(syncCtx.CurrentBranch)
		}()
	}

	// Perform the sync
	err = performSourceSync(ctx, client, opts, syncCtx)
	if err != nil {
		// If no tools are found, suggest switching branches or creating a source
		if err == ErrNoToolsFound {
			fmt.Println(style.WarningStyle.Render("\n⚠️  No tools found in the current branch."))
			// Find matching sources in other branches
			suggestions, err := suggestBranchesWithTools(client, syncCtx, opts)
			if err != nil {
				return err
			}
			if len(suggestions) > 0 {
				fmt.Println("\n💡 The following branches have tools in the specified path:")
				for _, branch := range suggestions {
					fmt.Printf(" - %s\n", style.HighlightStyle.Render(branch))
				}
				fmt.Println("\nYou can switch to one of these branches and run:")
				fmt.Println(style.CommandStyle.Render(fmt.Sprintf("  kubiya source sync %s --branch [branch]", opts.Path)))
			} else {
				fmt.Println("\n💡 You can create a new source from the current branch.")
				fmt.Println("Run the following command to add the source:")
				fmt.Println(style.CommandStyle.Render(fmt.Sprintf("  kubiya source add %s", opts.Path)))
				// Offer to stage and commit changes
				err = handleGitChanges(".", false)
				if err != nil {
					return err
				}
			}
			return nil
		}
		return err
	}
	return nil
}

// Add this function to handle stashing workflow
func handleStashWorkflow(syncCtx *SyncContext) error {
	fmt.Println("\n📦 Stashing changes...")
	stashMsg := fmt.Sprintf("Auto-stash before switching to %s", syncCtx.TargetBranch)
	if err := exec.Command("git", "stash", "push", "-m", stashMsg).Run(); err != nil {
		return fmt.Errorf("failed to stash changes: %w", err)
	}

	// Setup deferred restore
	defer func() {
		fmt.Println("\n📦 Restoring stashed changes...")
		exec.Command("git", "stash", "pop").Run()
	}()

	return nil
}

// Update performSourceSync to handle preview mode
func performSourceSync(ctx context.Context, client *kubiya.Client, opts SyncOptions, syncCtx *SyncContext) error {
	// Load source preview
	preview, err := client.LoadSource(ctx, opts.Path)
	if err != nil {
		return fmt.Errorf("failed to load source: %w", err)
	}

	// Show preview
	fmt.Println(style.SubtitleStyle.Render("\n📦 Source Preview"))
	fmt.Printf("Branch: %s\n", style.HighlightStyle.Render(syncCtx.TargetBranch))
	fmt.Printf("Path:   %s\n", style.HighlightStyle.Render(opts.Path))
	fmt.Printf("Tools:  %d\n", len(preview.Tools))

	// Check if there are no tools in the preview
	if len(preview.Tools) == 0 {
		return ErrNoToolsFound
	}

	// If in preview mode, show what would happen
	if !syncCtx.RequiresBranchSwitch {
		fmt.Println(style.DimStyle.Render("\nℹ️  Preview mode - no changes will be made"))
		fmt.Println("To sync with branch switch, run:")
		fmt.Println(style.CommandStyle.Render(fmt.Sprintf("  kubiya source sync %s --branch %s", opts.Path, syncCtx.TargetBranch)))
		return nil
	}

	// Perform actual sync
	return performSync(ctx, client, []kubiya.Source{*preview}, opts)
}

// Add helper functions for each step
func validateSyncInputs(opts SyncOptions) error {
	switch opts.Mode {
	case SyncModeCI:
		// Strict validation for CI mode
		if opts.Branch == "" {
			return fmt.Errorf("--branch is required in CI mode")
		}
		if opts.Path == "" && opts.Name == "" {
			return fmt.Errorf("either --path or --name is required in CI mode")
		}
	case SyncModeNonInteractive:
		// Less strict but still need minimum info
		if opts.Path == "" && opts.Name == "" {
			return fmt.Errorf("either path or --name is required in non-interactive mode")
		}
	}
	return nil
}

func handleGitOperations(opts SyncOptions, repoInfo RepoInfo) error {
	if opts.Mode == SyncModeCI {
		// In CI mode, fail if there are uncommitted changes
		if status, err := getGitStatus("."); err != nil {
			return err
		} else if status.HasUnstaged || status.HasUncommitted {
			return fmt.Errorf("uncommitted changes detected in CI mode")
		}
	} else if opts.AutoCommit {
		// Auto-commit changes if requested
		if err := handleAutoCommit("."); err != nil {
			return err
		}
	}
	return nil
}

func handleAutoCommit(path string) error {
	status, err := getGitStatus(path)
	if err != nil {
		return err
	}

	if !status.HasUnstaged && !status.HasUncommitted {
		return nil
	}

	// Stage and commit all changes
	if err := exec.Command("git", "-C", path, "add", ".").Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	commitMsg := fmt.Sprintf("Auto-commit: Update source files [kubiya-cli]")
	if err := exec.Command("git", "-C", path, "commit", "-m", commitMsg).Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

// Helper function to check if a string looks like a UUID
func isUUID(s string) bool {
	// Simple UUID format check (not comprehensive)
	return len(s) == 36 && strings.Count(s, "-") == 4
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

// Add these helper functions for git operations
func gitCheckBranchExists(path, branch string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--verify", branch)
	return cmd.Run() == nil
}

func gitCreateAndPushBranch(path, branch string) error {
	// Create new branch
	createCmd := exec.Command("git", "-C", path, "checkout", "-b", branch)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branch, err)
	}

	// Check if there are any changes to commit
	statusCmd := exec.Command("git", "-C", path, "status", "--porcelain")
	status, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if len(status) > 0 {
		// Stage all changes
		addCmd := exec.Command("git", "-C", path, "add", ".")
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Commit changes
		commitCmd := exec.Command("git", "-C", path, "commit", "-m", fmt.Sprintf("Initialize Kubiya source in branch %s", branch))
		if err := commitCmd.Run(); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	// Push the branch
	pushCmd := exec.Command("git", "-C", path, "push", "--set-upstream", "origin", branch)
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branch, err)
	}

	return nil
}

// Helper function to parse GitHub URL components
type GitHubURLComponents struct {
	BaseURL string
	Branch  string
	Path    string
}

func parseGitHubURL(url string) GitHubURLComponents {
	components := GitHubURLComponents{
		BaseURL: url,
	}

	if strings.Contains(url, "/tree/") {
		parts := strings.Split(url, "/tree/")
		components.BaseURL = parts[0]

		// Handle the path part after /tree/
		if len(parts) > 1 {
			pathParts := strings.SplitN(parts[1], "/", 2)

			// The branch might contain forward slashes (e.g., feature/branch-name)
			components.Branch = pathParts[0]

			// If there are additional path components after the branch
			if len(pathParts) > 1 {
				components.Path = pathParts[1]
			}
		}
	}

	return components
}

// Add these helper functions for repository checks and creation
type RepoCreationOptions struct {
	Name        string
	Description string
	Private     bool
	Path        string
}

func checkGitHubCLI() bool {
	cmd := exec.Command("gh", "version")
	return cmd.Run() == nil
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

func createGitHubRepo(opts RepoCreationOptions) error {
	// Check if gh CLI is available
	if !checkGitHubCLI() {
		return fmt.Errorf("GitHub CLI not found. Please install it from https://cli.github.com/")
	}

	visibility := "public"
	if opts.Private {
		visibility = "private"
	}

	args := []string{
		"repo", "create",
		opts.Name,
		"--" + visibility,
	}
	if opts.Description != "" {
		args = append(args, "--description", opts.Description)
	}

	cmd := exec.Command("gh", args...)
	cmd.Dir = opts.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create repository: %s", string(output))
	}

	// Initialize git repository
	if err := exec.Command("git", "init").Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Add remote
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", opts.Name, filepath.Base(opts.Path))
	if err := exec.Command("git", "remote", "add", "origin", repoURL).Run(); err != nil {
		return fmt.Errorf("failed to add remote: %w", err)
	}

	return nil
}

// Update the add command to handle repository creation
func newAddSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		sourceURL    string
		localPath    string
		yes          bool
		branch       string
		createBranch bool
		sourceName   string
		bindTeammate string
		testSource   bool
		createRepo   bool
		private      bool
		repoName     string
		repoDesc     string
	)

	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "➕ Add a new source",
		Example: `  # Add source from current directory
  kubiya source add .

  # Add source with custom name and bind to teammate
  kubiya source add ./my-tools --name "My Custom Tools" --bind-to-teammate "DevOps Bot"

  # Create new GitHub repository and add as source
  kubiya source add . --create-repo --name "my-tools" --description "My DevOps Tools"

  # Add and test the source
  kubiya source add ./my-tools --test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := cmd.Context()

			// Handle path argument
			if len(args) > 0 {
				localPath = args[0]
			}
			if localPath == "" {
				localPath = "."
			}

			// Check if path is a git repository
			if !isGitRepo(localPath) {
				var sb strings.Builder
				sb.WriteString(style.ErrorStyle.Render("\n❌ Not a git repository: ") + style.DimStyle.Render(localPath) + "\n\n")
				sb.WriteString(style.SubtitleStyle.Render("💡 A source must be part of a GitHub repository.\n\n"))

				// Show options
				sb.WriteString("Options:\n\n")

				if checkGitHubCLI() {
					sb.WriteString("1. Create a new GitHub repository:\n")
					sb.WriteString(style.CommandStyle.Render("   kubiya source add . --create-repo --name my-tools\n\n"))
				} else {
					sb.WriteString("1. Install GitHub CLI to create repositories:\n")
					sb.WriteString(style.CommandStyle.Render("   brew install gh  # macOS\n"))
					sb.WriteString(style.CommandStyle.Render("   apt install gh   # Ubuntu/Debian\n\n"))
				}

				sb.WriteString("2. Initialize git repository manually:\n")
				sb.WriteString(style.CommandStyle.Render("   git init\n"))
				sb.WriteString(style.CommandStyle.Render("   git remote add origin https://github.com/username/repo.git\n\n"))

				sb.WriteString("3. Use an existing repository:\n")
				sb.WriteString(style.CommandStyle.Render("   kubiya source add --url https://github.com/org/repo\n"))

				// If --create-repo flag is set, proceed with creation
				if createRepo {
					fmt.Println(style.SubtitleStyle.Render("\n🚀 Creating new GitHub repository...\n"))

					// Interactive mode if needed
					if !yes {
						if repoName == "" {
							fmt.Print("Repository name: ")
							fmt.Scanln(&repoName)
						}
						if repoDesc == "" {
							fmt.Print("Description (optional): ")
							fmt.Scanln(&repoDesc)
						}
						fmt.Print("Make repository private? [y/N] ")
						var privResp string
						fmt.Scanln(&privResp)
						private = strings.ToLower(privResp) == "y"
					}

					opts := RepoCreationOptions{
						Name:        repoName,
						Description: repoDesc,
						Private:     private,
						Path:        localPath,
					}

					if err := createGitHubRepo(opts); err != nil {
						return fmt.Errorf("failed to create repository: %w", err)
					}

					fmt.Printf("✅ Created repository: %s\n", repoName)
				} else {
					return fmt.Errorf(sb.String())
				}
			}

			// Create the source using the client
			source, err := client.CreateSource(ctx, sourceURL)
			if err != nil {
				return fmt.Errorf("failed to create source: %w", err)
			}

			fmt.Printf("✅ Successfully created source: %s\n", source.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceURL, "url", "", "URL to the source repository or directory")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation")
	cmd.Flags().StringVar(&branch, "branch", "", "Specific branch to use")
	cmd.Flags().StringVar(&sourceName, "name", "", "Custom name for the source")
	cmd.Flags().StringVar(&bindTeammate, "bind-to-teammate", "", "Teammate name or ID to bind the source to")
	cmd.Flags().BoolVar(&createBranch, "create-branch", false, "Create and push the branch if it doesn't exist")
	cmd.Flags().BoolVar(&testSource, "test", false, "Test the source after adding")
	cmd.Flags().BoolVar(&createRepo, "create-repo", false, "Create a new GitHub repository")
	cmd.Flags().BoolVar(&private, "private", false, "Make the new repository private")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "Name for the new GitHub repository")
	cmd.Flags().StringVar(&repoDesc, "description", "", "Description for the new GitHub repository")

	return cmd
}

// Update getRepositoryURL to use the new parsing
func getRepositoryURL(path string) (string, error) {
	if strings.HasPrefix(path, "http") {
		return normalizeGitHubURL(path, true)
	}

	// Get the repository URL from local git repository
	remoteCmd := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url")
	remoteURL, err := remoteCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	// Get current branch
	branchCmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	branch, err := branchCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get relative path within repository
	repoRootCmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	repoRoot, err := repoRootCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}

	// Calculate relative path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	relPath, err := filepath.Rel(strings.TrimSpace(string(repoRoot)), absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	// Normalize the base URL
	baseURL, err := normalizeGitHubURL(strings.TrimSpace(string(remoteURL)), false)
	if err != nil {
		return "", err
	}

	// Construct full URL with branch and path
	fullURL := baseURL + "/tree/" + strings.TrimSpace(string(branch))
	if relPath != "." {
		fullURL += "/" + relPath
	}

	return fullURL, nil
}

// Update normalizeGitHubURL to handle branch and folder information
func normalizeGitHubURL(url string, preservePath bool) (string, error) {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Convert SSH URLs to HTTPS
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}

	if !preservePath {
		// Remove specific branch/file paths
		if strings.Contains(url, "/blob/") {
			parts := strings.Split(url, "/blob/")
			url = parts[0]
		} else if strings.Contains(url, "/tree/") {
			parts := strings.Split(url, "/tree/")
			url = parts[0]
		}
	}

	return url, nil
}

// Fix the isSourceRelatedToRepo function to use the correct number of arguments
func isSourceRelatedToRepo(source kubiya.Source, repoURL string) bool {
	normalizedSourceURL, err := normalizeGitHubURL(source.URL, false)
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

// Add these helper types and functions

type GitCommitInfo struct {
	SHA     string
	Message string
	Author  string
	Branch  string
}

func getGitCommitInfo(path string) (GitCommitInfo, error) {
	var info GitCommitInfo

	// Get current branch
	branchCmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	branch, err := branchCmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to get branch: %w", err)
	}
	info.Branch = strings.TrimSpace(string(branch))

	// Get commit info
	commitCmd := exec.Command("git", "-C", path, "log", "-1", "--pretty=format:%H%n%s%n%an")
	commit, err := commitCmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to get commit info: %w", err)
	}

	parts := strings.Split(string(commit), "\n")
	if len(parts) >= 3 {
		info.SHA = parts[0]
		info.Message = parts[1]
		info.Author = parts[2]
	}

	return info, nil
}

type ToolDiff struct {
	Added    []kubiya.Tool
	Removed  []kubiya.Tool
	Modified []kubiya.Tool
}

func compareTools(current, new []kubiya.Tool) ToolDiff {
	var diff ToolDiff
	currentMap := make(map[string]kubiya.Tool)
	newMap := make(map[string]kubiya.Tool)

	// Create maps for easier comparison
	for _, t := range current {
		currentMap[t.Name] = t
	}
	for _, t := range new {
		newMap[t.Name] = t
	}

	// Find added and modified tools
	for name, tool := range newMap {
		if currentTool, exists := currentMap[name]; !exists {
			diff.Added = append(diff.Added, tool)
		} else if !reflect.DeepEqual(tool, currentTool) {
			diff.Modified = append(diff.Modified, tool)
		}
	}

	// Find removed tools
	for name, tool := range currentMap {
		if _, exists := newMap[name]; !exists {
			diff.Removed = append(diff.Removed, tool)
		}
	}

	// Sort the slices for consistent output
	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].Name < diff.Added[j].Name })
	sort.Slice(diff.Removed, func(i, j int) bool { return diff.Removed[i].Name < diff.Removed[j].Name })
	sort.Slice(diff.Modified, func(i, j int) bool { return diff.Modified[i].Name < diff.Modified[j].Name })

	return diff
}

func selectSourceInteractively(sources []kubiya.Source) kubiya.Source {
	if len(sources) == 0 {
		return kubiya.Source{}
	}

	fmt.Println("\n📦 Available Sources:")
	for i, s := range sources {
		fmt.Printf("%d. %s (UUID: %s)\n", i+1, s.Name, s.UUID)
		if s.Description != "" {
			fmt.Printf("   %s\n", s.Description)
		}
	}

	for {
		fmt.Print("\nEnter number or search term: ")
		var input string
		fmt.Scanln(&input)

		// Try to parse as number first
		if num, err := strconv.Atoi(input); err == nil {
			if num > 0 && num <= len(sources) {
				return sources[num-1]
			}
			fmt.Println("Invalid number, please try again")
			continue
		}

		// If not a number, treat as search term
		var matches []kubiya.Source
		searchTerm := strings.ToLower(input)
		for _, s := range sources {
			if strings.Contains(strings.ToLower(s.Name), searchTerm) ||
				strings.Contains(strings.ToLower(s.Description), searchTerm) ||
				strings.Contains(strings.ToLower(s.UUID), searchTerm) {
				matches = append(matches, s)
			}
		}

		if len(matches) == 1 {
			return matches[0]
		} else if len(matches) > 1 {
			fmt.Printf("\nFound %d matches:\n", len(matches))
			for i, s := range matches {
				fmt.Printf("%d. %s (UUID: %s)\n", i+1, s.Name, s.UUID)
			}
			continue
		}

		fmt.Println("No matches found, please try again")
	}
}

// Add this helper function to format source details with better UX
func displaySourceDetails(source kubiya.Source, commitInfo GitCommitInfo, preview *kubiya.Source, repoInfo RepoInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(os.Stdout, style.TitleStyle.Render("\n📦 Source Details"))

	// Basic Information
	fmt.Fprintln(os.Stdout, style.SubtitleStyle.Render("\n📋 Basic Information"))
	fmt.Fprintf(w, "  Name:\t%s\n", style.HighlightStyle.Render(source.Name))
	fmt.Fprintf(w, "  UUID:\t%s\n", source.UUID)
	fmt.Fprintf(w, "  Description:\t%s\n", source.Description)
	fmt.Fprintf(w, "  Managed By:\t%s\n", source.ManagedBy)
	w.Flush()

	// Repository Information
	fmt.Fprintln(os.Stdout, style.SubtitleStyle.Render("\n📚 Repository Information"))
	fmt.Fprintf(w, "  Repository:\t%s\n", style.HighlightStyle.Render(repoInfo.BaseURL))
	if repoInfo.Path != "" {
		fmt.Fprintf(w, "  Path:\t%s\n", style.HighlightStyle.Render(repoInfo.Path))
	}
	fmt.Fprintf(w, "  Branch:\t%s\n", style.HighlightStyle.Render(repoInfo.Branch))
	if commitInfo.SHA != "" {
		fmt.Fprintf(w, "  Commit:\t%s\n", style.HighlightStyle.Render(fmt.Sprintf("%s (%s)", commitInfo.SHA[:8], commitInfo.Message)))
		fmt.Fprintf(w, "  Author:\t%s\n", commitInfo.Author)
	}
	w.Flush()

	// Statistics
	fmt.Fprintln(os.Stdout, style.SubtitleStyle.Render("\n📊 Statistics"))
	stats := []struct {
		label string
		value interface{}
		icon  string
	}{
		{"Connected Tools", source.ConnectedToolsCount, "🛠️"},
		{"Connected Teammates", source.ConnectedAgentsCount, "👥"},
		{"Connected Workflows", source.ConnectedWorkflowsCount, "📋"},
		{"Errors", source.ErrorsCount, "⚠️"},
		{"New Tools (after sync)", len(preview.Tools) - source.ConnectedToolsCount, "✨"},
	}

	for _, stat := range stats {
		var valueStr string
		switch v := stat.value.(type) {
		case int:
			if v > 0 {
				valueStr = style.HighlightStyle.Render(fmt.Sprintf("%d", v))
			} else {
				valueStr = style.DimStyle.Render("0")
			}
		default:
			valueStr = fmt.Sprintf("%v", stat.value)
		}
		fmt.Fprintf(w, "  %s %s:\t%s\n", stat.icon, stat.label, valueStr)
	}
	w.Flush()

	// Tools Preview
	if len(preview.Tools) > 0 {
		fmt.Fprintln(os.Stdout, style.SubtitleStyle.Render("\n🛠️  Available Tools"))
		for i, tool := range preview.Tools {
			fmt.Printf("\n  %d. %s\n", i+1, style.HighlightStyle.Render(tool.Name))
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
					fmt.Printf("       • %s\n", env)
				}
			}
		}
	}

	// Timestamps
	fmt.Fprintln(os.Stdout, style.SubtitleStyle.Render("\n⏰ Timestamps"))
	fmt.Fprintf(w, "  Created:\t%s\n", source.CreatedAt.Format(time.RFC822))
	fmt.Fprintf(w, "  Updated:\t%s\n", source.UpdatedAt.Format(time.RFC822))
	fmt.Fprintf(w, "  Age:\t%s\n", formatDuration(time.Since(source.CreatedAt)))
	w.Flush()

	fmt.Println() // Add final newline for spacing
}

// Add this helper type for git status
type GitStatus struct {
	HasUnstaged    bool
	HasUncommitted bool
	UnstagedFiles  []string
	StagedFiles    []string
}

// Add this helper function to check git status
func getGitStatus(path string) (GitStatus, error) {
	var status GitStatus

	// Check for unstaged changes
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return status, fmt.Errorf("failed to get git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		for _, line := range lines {
			if len(line) < 2 {
				continue
			}
			switch line[0] {
			case '?': // Untracked files
				status.HasUnstaged = true
				status.UnstagedFiles = append(status.UnstagedFiles, strings.TrimSpace(line[2:]))
			case 'M': // Modified files
				status.HasUncommitted = true
				status.StagedFiles = append(status.StagedFiles, strings.TrimSpace(line[2:]))
			case 'A': // Added files
				status.HasUncommitted = true
				status.StagedFiles = append(status.StagedFiles, strings.TrimSpace(line[2:]))
			case ' ':
				if line[1] == 'M' { // Modified but not staged
					status.HasUnstaged = true
					status.UnstagedFiles = append(status.UnstagedFiles, strings.TrimSpace(line[2:]))
				}
			}
		}
	}

	return status, nil
}

// Add this function to handle git changes
func handleGitChanges(path string, yes bool) error {
	status, err := getGitStatus(path)
	if err != nil {
		return err
	}

	if !status.HasUnstaged && !status.HasUncommitted {
		return nil
	}

	fmt.Println(style.WarningStyle.Render("\n⚠️  Uncommitted changes detected"))

	if status.HasUnstaged {
		fmt.Println(style.SubtitleStyle.Render("\nUnstaged changes to be committed:"))
		for _, file := range status.UnstagedFiles {
			fmt.Printf("  • %s\n", file)
		}
	}

	if status.HasUncommitted {
		fmt.Println(style.SubtitleStyle.Render("\nStaged changes to be committed:"))
		for _, file := range status.StagedFiles {
			fmt.Printf("  • %s\n", file)
		}
	}

	// Stage all changes
	fmt.Println("\n📦 Staging changes...")
	stageCmd := exec.Command("git", "-C", path, "add", ".")
	if err := stageCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Get commit message
	var message string
	fmt.Print("\nEnter commit message: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		message = scanner.Text()
	}
	if message == "" {
		message = "Update source files before branch switch"
	}

	// Commit changes
	fmt.Println("\n📝 Committing changes...")
	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Ask about pushing changes
	fmt.Print("\nPush changes to remote? [Y/n] ")
	var pushResponse string
	fmt.Scanln(&pushResponse)
	if pushResponse == "" || strings.ToLower(pushResponse) == "y" {
		fmt.Println("\n🚀 Pushing changes to remote...")
		pushCmd := exec.Command("git", "-C", path, "push")
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("failed to push changes: %w", err)
		}
		fmt.Println(style.SuccessStyle.Render("✅ Changes pushed successfully"))
	}

	return nil
}

// Update the parseRepoInfo function to better handle path construction
func parseRepoInfo(input string) (RepoInfo, error) {
	var info RepoInfo

	// Check if input is a local path
	if !strings.HasPrefix(input, "http") && !strings.HasPrefix(input, "git@") {
		// Get repository URL from local path
		repoURL, err := getRepositoryURL(input)
		if err != nil {
			return info, err
		}
		input = repoURL
	}

	// Normalize the URL first
	normalizedURL, err := normalizeGitHubURL(input, true)
	if err != nil {
		return info, err
	}

	u, err := url.Parse(normalizedURL)
	if err != nil {
		return info, err
	}

	// Extract BaseURL (without branch and path)
	pathParts := strings.Split(u.Path, "/tree/")
	info.BaseURL = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, pathParts[0])

	// Extract branch and path if present
	if len(pathParts) > 1 {
		branchAndPath := pathParts[1]
		branchParts := strings.SplitN(branchAndPath, "/", 2)
		info.Branch = branchParts[0]
		if len(branchParts) > 1 {
			info.Path = branchParts[1]
		}
	}

	// Construct FullURL
	info.FullURL = normalizedURL

	return info, nil
}

// Update the source matching logic in the sync command
func isPathMatch(sourcePath, targetPath string) bool {
	// Normalize paths by trimming slashes and converting to lowercase
	sourcePath = strings.Trim(strings.ToLower(sourcePath), "/")
	targetPath = strings.Trim(strings.ToLower(targetPath), "/")

	// Direct match
	if sourcePath == targetPath {
		return true
	}

	// Check if one path is a parent of the other
	return strings.HasPrefix(sourcePath, targetPath) || strings.HasPrefix(targetPath, sourcePath)
}

// Add this function to handle source matching
func findMatchingSources(sources []kubiya.Source, repoInfo RepoInfo) []kubiya.Source {
	var matchingSources []kubiya.Source

	// Find sources that match the repository and path
	for _, s := range sources {
		sourceInfo, _ := parseRepoInfo(s.URL)
		if sourceInfo.BaseURL == repoInfo.BaseURL {
			// More flexible path matching
			if repoInfo.Path == "" || isPathMatch(sourceInfo.Path, repoInfo.Path) {
				matchingSources = append(matchingSources, s)
			}
		}
	}

	return matchingSources
}

// Update the error formatting function for better visibility and alignment
func formatNoSourcesError(pathArg string, sources []kubiya.Source, repoInfo RepoInfo) string {
	var sb strings.Builder

	// Get current git info
	gitInfo, err := getGitCommitInfo(".")
	if err == nil {
		// Header with clear separation
		sb.WriteString("\n") // Start with newline for better separation
		sb.WriteString(style.ErrorStyle.Render("❌ No sources found matching path: ") +
			style.HighlightStyle.Render(pathArg))

		// Current context in a clean, aligned format
		sb.WriteString("\n\nCurrent Context:\n")
		if repoInfo.BaseURL != "" {
			sb.WriteString(fmt.Sprintf("  Repository: %s\n", style.DimStyle.Render(repoInfo.BaseURL)))
		}
		sb.WriteString(fmt.Sprintf("  Branch:     %s\n", style.HighlightStyle.Render(gitInfo.Branch)))
		sb.WriteString(fmt.Sprintf("  Commit:     %s %s\n",
			style.DimStyle.Render(gitInfo.SHA[:8]),
			style.DimStyle.Render("("+gitInfo.Message+")")))
		sb.WriteString(fmt.Sprintf("  Author:     %s\n", style.DimStyle.Render(gitInfo.Author)))

		// Find sources with matching path across all branches
		var sourcesWithPath []kubiya.Source
		for _, s := range sources {
			sourceInfo, _ := parseRepoInfo(s.URL)
			if sourceInfo.BaseURL == repoInfo.BaseURL &&
				(strings.Contains(sourceInfo.Path, pathArg) || strings.Contains(pathArg, sourceInfo.Path)) {
				sourcesWithPath = append(sourcesWithPath, s)
			}
		}

		if len(sourcesWithPath) > 0 {
			// Show matching sources with clear formatting
			sb.WriteString(style.SubtitleStyle.Render("\nFound matching sources in other branches:\n"))
			for _, s := range sourcesWithPath {
				sourceInfo, _ := parseRepoInfo(s.URL)
				sb.WriteString(fmt.Sprintf("\n  %s\n", style.HighlightStyle.Render(s.Name)))
				sb.WriteString(fmt.Sprintf("  Branch: %s\n", style.DimStyle.Render(sourceInfo.Branch)))
				sb.WriteString(fmt.Sprintf("  Path:   %s\n", style.DimStyle.Render(sourceInfo.Path)))
				sb.WriteString(fmt.Sprintf("  Tools:  %s\n",
					style.HighlightStyle.Render(fmt.Sprintf("%d", s.ConnectedToolsCount))))

				// Add quick sync command with proper indentation
				sb.WriteString("\n  " + style.CommandStyle.Render(fmt.Sprintf("kubiya source sync %s --branch %s",
					pathArg, sourceInfo.Branch)) + "\n")
			}
		} else {
			// Show options with clear separation and formatting
			sb.WriteString(style.SubtitleStyle.Render("\nNo matching sources found."))
			sb.WriteString("\nYou can:\n")

			// Option 1: Create new source
			sb.WriteString("\n1. Create a new source in the current branch:\n")
			sb.WriteString("   " + style.CommandStyle.Render(fmt.Sprintf("kubiya source add --path %s", pathArg)) + "\n")

			// Option 2: Try different branches
			branches, _ := getAvailableBranches(".")
			if len(branches) > 0 {
				// Filter relevant branches
				var relevantBranches []string
				for _, b := range branches {
					if b == "main" || b == "master" ||
						strings.HasPrefix(b, strings.Split(gitInfo.Branch, "/")[0]) {
						if b != gitInfo.Branch {
							relevantBranches = append(relevantBranches, b)
						}
					}
				}

				if len(relevantBranches) > 0 {
					sb.WriteString("\n2. Try a different branch:\n")
					// Show at most 3 relevant branches with proper indentation
					for _, b := range relevantBranches[:min(3, len(relevantBranches))] {
						sb.WriteString("   " + style.CommandStyle.Render(fmt.Sprintf("kubiya source sync %s --branch %s",
							pathArg, b)) + "\n")
					}

					if len(branches) > 3 {
						sb.WriteString("\n" + style.DimStyle.Render("   Use 'git branch -a' to see all available branches") + "\n")
					}
				}
			}
		}

		// Add final newline for better spacing
		sb.WriteString("\n")
	} else {
		// Simpler message if we can't get git info
		sb.WriteString("\n" + style.ErrorStyle.Render("❌ No sources found matching path: ") +
			style.HighlightStyle.Render(pathArg) + "\n")
		sb.WriteString("\nEnsure you're in a git repository and try again.\n\n")
	}

	return sb.String()
}

// Add this function to get available git branches
func getAvailableBranches(path string) ([]string, error) {
	// Get all branches (both local and remote)
	cmd := exec.Command("git", "-C", path, "branch", "-a", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	// Split output into lines and clean up branch names
	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	var cleanBranches []string
	seenBranches := make(map[string]bool)

	for _, branch := range branches {
		// Clean up remote branch names
		branch = strings.TrimSpace(branch)
		branch = strings.TrimPrefix(branch, "remotes/origin/")

		// Skip HEAD reference and empty branches
		if branch == "" || branch == "HEAD" || seenBranches[branch] {
			continue
		}

		seenBranches[branch] = true
		cleanBranches = append(cleanBranches, branch)
	}

	// Sort branches to put main/master first, then other branches alphabetically
	sort.Slice(cleanBranches, func(i, j int) bool {
		// Main/master branches should come first
		if cleanBranches[i] == "main" || cleanBranches[i] == "master" {
			return true
		}
		if cleanBranches[j] == "main" || cleanBranches[j] == "master" {
			return false
		}
		// Sort other branches alphabetically
		return cleanBranches[i] < cleanBranches[j]
	})

	return cleanBranches, nil
}

// Add this function to check if we need to switch branches
func checkBranchSwitch(currentBranch, targetBranch string) (bool, error) {
	if currentBranch == targetBranch {
		return false, nil
	}

	// Check if target branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", targetBranch)
	if err := checkCmd.Run(); err != nil {
		checkRemoteCmd := exec.Command("git", "rev-parse", "--verify", "origin/"+targetBranch)
		if err := checkRemoteCmd.Run(); err != nil {
			return false, fmt.Errorf("branch %s not found locally or remotely", targetBranch)
		}
	}

	return true, nil
}

// Add this function to get repository info
func getRepoInfo(opts SyncOptions) (RepoInfo, error) {
	var info RepoInfo
	var err error

	// If repo URL is provided, use it
	if opts.RepoURL != "" {
		info, err = parseRepoInfo(opts.RepoURL)
		if err != nil {
			return info, fmt.Errorf("invalid repository URL: %w", err)
		}
	} else if opts.Path != "" {
		// If path is provided, get repo info from path
		info, err = parseRepoInfo(opts.Path)
		if err != nil {
			return info, fmt.Errorf("failed to get repository info from path: %w", err)
		}
	} else {
		// Use current directory
		info, err = parseRepoInfo(".")
		if err != nil {
			return info, fmt.Errorf("not in a git repository: %w", err)
		}
	}

	// Override branch if specified
	if opts.Branch != "" {
		info.Branch = opts.Branch
		info.FullURL = info.BaseURL + "/tree/" + opts.Branch
		if info.Path != "" {
			info.FullURL += "/" + info.Path
		}
	}

	return info, nil
}

// Add this function to show preview and get confirmation
func showPreviewAndConfirm(sources []kubiya.Source, repoInfo RepoInfo) error {
	if len(sources) == 0 {
		return fmt.Errorf("no sources to sync")
	}

	// Get git info for current context
	gitInfo, err := getGitCommitInfo(".")
	if err == nil {
		fmt.Println(style.SubtitleStyle.Render("\n📝 Source Sync Preview"))
		fmt.Printf("\nCurrent Context:\n")
		fmt.Printf("  Branch: %s\n", style.HighlightStyle.Render(gitInfo.Branch))
		fmt.Printf("  Commit: %s %s\n",
			style.DimStyle.Render(gitInfo.SHA[:8]),
			style.DimStyle.Render("("+gitInfo.Message+")"))
	}

	// Show sources to be synced
	fmt.Printf("\nSources to sync:\n")
	for _, s := range sources {
		fmt.Printf("\n• %s\n", style.HighlightStyle.Render(s.Name))
		fmt.Printf("  UUID: %s\n", style.DimStyle.Render(s.UUID))
		fmt.Printf("  Tools: %d\n", s.ConnectedToolsCount)
		if s.Description != "" {
			fmt.Printf("  Description: %s\n", s.Description)
		}
	}

	// Get confirmation
	fmt.Print(style.SubtitleStyle.Render("\n❓ Do you want to proceed? [y/N] "))
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("operation cancelled")
	}

	return nil
}

// Add this function to perform the sync
func performSync(ctx context.Context, client *kubiya.Client, sources []kubiya.Source, opts SyncOptions) error {
	if opts.Mode != SyncModeInteractive {
		fmt.Println("🔄 Starting source sync...")
	}

	var syncErrors []error
	for i, source := range sources {
		if opts.Mode != SyncModeInteractive {
			fmt.Printf("(%d/%d) Syncing %s...\n", i+1, len(sources), source.Name)
		}

		// Perform sync
		updated, err := client.SyncSource(ctx, source.UUID)
		if err != nil {
			if opts.Mode == SyncModeCI {
				return fmt.Errorf("failed to sync %s: %w", source.Name, err)
			}
			syncErrors = append(syncErrors, fmt.Errorf("failed to sync %s: %w", source.Name, err))
			continue
		}

		// Show sync results
		fmt.Printf("\n✅ Successfully synced %s\n", source.Name)
		fmt.Printf("Last updated: %s\n", updated.KubiyaMetadata.LastUpdated)
		fmt.Printf("Updated by: %s\n", updated.KubiyaMetadata.UserLastUpdated)

		if updated.ErrorsCount > 0 {
			syncErrors = append(syncErrors, fmt.Errorf("source %s has %d error(s)", source.Name, updated.ErrorsCount))
		}
	}

	// Report any errors
	if len(syncErrors) > 0 {
		fmt.Println(style.ErrorStyle.Render("\n⚠️  Some errors occurred during sync:"))
		for _, err := range syncErrors {
			fmt.Printf("• %v\n", err)
		}
		if opts.Mode == SyncModeCI {
			return fmt.Errorf("sync completed with errors")
		}
	}

	return nil
}

// Add this function to handle branch switching
func switchBranch(branch string) error {
	// Try to checkout the branch directly first
	checkoutCmd := exec.Command("git", "checkout", branch)
	if err := checkoutCmd.Run(); err != nil {
		// If direct checkout fails, try to fetch and checkout from remote
		fmt.Printf("📥 Fetching branch %s from remote...\n", style.HighlightStyle.Render(branch))
		fetchCmd := exec.Command("git", "fetch", "origin", branch)
		if err := fetchCmd.Run(); err != nil {
			return fmt.Errorf("failed to fetch branch %s: %w", branch, err)
		}

		// Try to checkout again after fetch
		checkoutCmd = exec.Command("git", "checkout", branch)
		if err := checkoutCmd.Run(); err != nil {
			// If it still fails, try to create tracking branch
			checkoutTrackCmd := exec.Command("git", "checkout", "-b", branch, "--track", "origin/"+branch)
			if err := checkoutTrackCmd.Run(); err != nil {
				return fmt.Errorf("failed to switch to branch %s: %w", branch, err)
			}
		}
	}

	// Update branch from remote
	fmt.Printf("📥 Updating branch %s from remote...\n", style.HighlightStyle.Render(branch))
	pullCmd := exec.Command("git", "pull", "--ff-only")
	if err := pullCmd.Run(); err != nil {
		// If pull fails, we should return an error to prevent infinite loops
		return fmt.Errorf("failed to update branch %s: %w", branch, err)
	}

	return nil
}

// Add this function to handle commit changes
func handleCommitChanges(path string) error {
	// Get current status
	status, err := getGitStatus(path)
	if err != nil {
		return err
	}

	if !status.HasUnstaged && !status.HasUncommitted {
		return nil
	}

	// Show changes to be committed
	if status.HasUnstaged {
		fmt.Println(style.SubtitleStyle.Render("\nUnstaged changes to be committed:"))
		for _, file := range status.UnstagedFiles {
			fmt.Printf("  • %s\n", file)
		}
	}
	if status.HasUncommitted {
		fmt.Println(style.SubtitleStyle.Render("\nStaged changes to be committed:"))
		for _, file := range status.StagedFiles {
			fmt.Printf("  • %s\n", file)
		}
	}

	// Stage all changes
	fmt.Println("\n📦 Staging changes...")
	stageCmd := exec.Command("git", "-C", path, "add", ".")
	if err := stageCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Get commit message
	var message string
	fmt.Print("\nEnter commit message: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		message = scanner.Text()
	}
	if message == "" {
		message = "Update source files before branch switch"
	}

	// Commit changes
	fmt.Println("\n📝 Committing changes...")
	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Ask about pushing changes
	fmt.Print("\nPush changes to remote? [Y/n] ")
	var pushResponse string
	fmt.Scanln(&pushResponse)
	if pushResponse == "" || strings.ToLower(pushResponse) == "y" {
		fmt.Println("\n🚀 Pushing changes to remote...")
		pushCmd := exec.Command("git", "-C", path, "push")
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("failed to push changes: %w", err)
		}
		fmt.Println(style.SuccessStyle.Render("✅ Changes pushed successfully"))
	}

	return nil
}

// Update the suggestBranchesWithTools function to find all branches with tools in the specified path
func suggestBranchesWithTools(client *kubiya.Client, syncCtx *SyncContext, opts SyncOptions) ([]string, error) {
	// Get repository info
	repoInfo, err := parseRepoInfo(".")
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Get list of sources from the API
	sources, err := client.ListSources(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list sources from API: %w", err)
	}

	var suggestions []string
	branchSet := make(map[string]struct{})

	for _, source := range sources {
		sourceInfo, err := parseRepoInfo(source.URL)
		if err != nil {
			continue
		}

		// Check if the source matches the repository
		if sourceInfo.BaseURL == repoInfo.BaseURL {
			// Check if the paths overlap
			if pathsOverlap(sourceInfo.Path, opts.Path) {
				// Check if the source has tools
				if source.ConnectedToolsCount > 0 {
					branchName := strings.TrimSpace(sourceInfo.Branch)
					if branchName != "" && !strings.EqualFold(branchName, syncCtx.CurrentBranch) {
						branchSet[branchName] = struct{}{}
					}
				}
			}
		}
	}

	// Convert branchSet to a slice
	for branch := range branchSet {
		suggestions = append(suggestions, branch)
	}

	// Sort the suggestions
	sort.Strings(suggestions)

	return suggestions, nil
}

// Add this helper function to check if paths overlap
func pathsOverlap(path1, path2 string) bool {
	// Normalize paths by trimming slashes and converting to lowercase
	path1 = strings.Trim(strings.ToLower(path1), "/")
	path2 = strings.Trim(strings.ToLower(path2), "/")

	// Split paths into components
	parts1 := strings.Split(path1, "/")
	parts2 := strings.Split(path2, "/")

	minLength := min(len(parts1), len(parts2))

	// Compare path components
	for i := 0; i < minLength; i++ {
		if parts1[i] != parts2[i] {
			return false
		}
	}
	return true
}

// Helper function to get the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Add this error constant
var ErrNoToolsFound = fmt.Errorf("no tools found in the source")
