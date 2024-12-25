package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/util"
	"github.com/spf13/cobra"
)

// Add these constants for sync modes
const (
	SyncModeInteractive    = "interactive"
	SyncModeNonInteractive = "non-interactive"
	SyncModeCI             = "ci"
)

// Keep the sync context type
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
		Long: `Work with Kubiya sources - list, scan, add and manage your tool sources.
Sources contain the tools and capabilities that your teammates can use.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newListSourcesCommand(cfg),
		newScanSourceCommand(cfg),
		newAddSourceCommand(cfg),
		newDescribeSourceCommand(cfg),
		newDeleteSourceCommand(cfg),
		newSyncSourceCommand(cfg),
	)

	return cmd
}

func newListSourcesCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "📋 List all sources",
		Example:      "  kubiya source list\n  kubiya source list --output json",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(sources)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.TitleStyle.Render("📦 SOURCES"))
				fmt.Fprintln(w, "UUID\tNAME\tTOOLS\tSTATUS")
				for _, s := range sources {
					status := style.SuccessStyle.Render("✅")
					if len(s.Tools) == 0 {
						status = style.WarningStyle.Render("⚠️")
					}

					fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
						style.DimStyle.Render(s.UUID),
						style.HighlightStyle.Render(s.Name),
						len(s.Tools),
						status,
					)
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newScanSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		interactive   bool
		outputFormat  string
		dynamicConfig string
		local         bool
		repo          string
		branch        string
		path          string
		remote        string
		force         bool
		push          bool
		commitMsg     string
		addAll        bool
		runnerName    string
	)

	cmd := &cobra.Command{
		Use:   "scan [url|path]",
		Short: "🔍 Scan a source URL or local directory for available tools",
		Example: `  # Scan current directory (uses current branch)
  kubiya source scan .

  # Scan with specific runner
  kubiya source scan . --runner enforcer

  # Scan and automatically stage, commit and push changes
  kubiya source scan . --add --push --commit-msg "feat: update tools"

  # Stage specific files and push
  kubiya source scan . --add "tools/*,README.md" --push`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sourceURL := ""

			// Handle different input methods
			if len(args) > 0 {
				sourceURL = args[0]
			} else if repo != "" {
				sourceURL = fmt.Sprintf("https://github.com/%s", repo)
				if branch != "" {
					sourceURL = fmt.Sprintf("%s/tree/%s", sourceURL, branch)
				}
				if path != "" {
					sourceURL = fmt.Sprintf("%s/%s", sourceURL, path)
				}
			}

			// Handle local directory scanning
			if sourceURL == "." || strings.HasPrefix(sourceURL, "./") || strings.HasPrefix(sourceURL, "/") {
				local = true
			}

			if local {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📂 Local Directory Scan"))

				// Check Git status before proceeding
				status, err := getGitStatus(sourceURL)
				if err != nil {
					return err
				}

				if status.HasChanges() {
					fmt.Printf("\n%s\n", style.WarningStyle.Render("⚠️ Uncommitted Changes"))
					if len(status.Unstaged) > 0 {
						fmt.Printf("\nUnstaged changes:\n")
						for _, file := range status.Unstaged {
							fmt.Printf("  • %s\n", file)
						}
					}
					if len(status.Staged) > 0 {
						fmt.Printf("\nStaged changes:\n")
						for _, file := range status.Staged {
							fmt.Printf("  • %s\n", file)
						}
					}

					// Handle changes based on flags
					if addAll || push {
						if err := handleGitChanges(sourceURL, status, addAll, push, commitMsg); err != nil {
							return err
						}
					} else {
						fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Available Actions:"))
						fmt.Println("• Stage and commit changes: --add --commit-msg \"your message\"")
						fmt.Println("• Push to remote: --push")
						fmt.Println("• Continue without committing: --force")
						return fmt.Errorf("uncommitted changes found")
					}
				}

				// Get git info with enhanced options
				gitInfo, err := getGitInfo(sourceURL, remote, branch, force)
				if err != nil {
					fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Common Solutions:"))
					fmt.Println("• Ensure you're in a git repository")
					fmt.Println("• Set up a remote with: git remote add origin <url>")
					fmt.Println("• Push your changes to the remote repository")
					return nil
				}

				// Show git info
				fmt.Printf("Repository: %s\n", style.HighlightStyle.Render(gitInfo.RepoURL))
				if gitInfo.Remote != "origin" {
					fmt.Printf("Remote: %s\n", gitInfo.Remote)
				}
				fmt.Printf("Branch: %s", gitInfo.Branch)
				if gitInfo.IsCurrentBranch {
					fmt.Printf(" (current)")
				}
				fmt.Println()
				if gitInfo.RelativePath != "" {
					fmt.Printf("Path: %s\n", gitInfo.RelativePath)
				}
				fmt.Println()

				sourceURL = gitInfo.FullURL
			}

			// Handle dynamic configuration
			var dynConfig map[string]interface{}
			if dynamicConfig != "" {
				data, err := os.ReadFile(dynamicConfig)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
				if err := json.Unmarshal(data, &dynConfig); err != nil {
					return fmt.Errorf("invalid config JSON: %w", err)
				}
			}

			fmt.Printf("\n%s %s\n\n",
				style.TitleStyle.Render("🔍 Scanning Source:"),
				style.HighlightStyle.Render(sourceURL))

			// Show runner info or warning
			if runnerName != "" {
				fmt.Printf("Runner: %s\n\n", style.HighlightStyle.Render(runnerName))
			} else {
				fmt.Printf("%s No runner specified - using default runner\n\n",
					style.WarningStyle.Render("⚠️"))
			}

			// Use the discovery API endpoint instead of LoadSource
			discovered, err := client.DiscoverSource(cmd.Context(), sourceURL, dynConfig, runnerName)
			if err != nil {
				if discovery, ok := err.(*kubiya.SourceDiscoveryResponse); ok && len(discovery.Errors) > 0 {
					// Show a clean error output with source context
					fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Scan failed"))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Source Information:"))
					fmt.Printf("Name: %s\n", discovery.Name)
					fmt.Printf("Branch: %s\n", discovery.Source.Branch)
					fmt.Printf("Commit: %s\n", discovery.Source.Commit)
					fmt.Printf("Committer: %s\n\n", discovery.Source.Committer)

					fmt.Printf("%s\n", style.SubtitleStyle.Render("Error Details:"))
					for _, e := range discovery.Errors {
						fmt.Printf("• %s: %s\n", e.Type, e.Error)
						if e.Details != "" {
							fmt.Printf("  Details: %s\n", e.Details)
						}
					}

					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Common Solutions:"))
					fmt.Println("• Check if the source code is valid and can be imported")
					fmt.Println("• Ensure all required dependencies are available")
					fmt.Println("• Check for syntax errors in tool definitions")
					fmt.Println("• Verify the branch and file paths are correct")
				} else {
					// Simple error output for non-discovery errors
					fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Scan failed"))
					fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
				}
				return nil // Don't propagate the error since we handled it
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(discovered)
			default:
				fmt.Printf("%s\n\n", style.SuccessStyle.Render("✅ Scan completed"))
				fmt.Printf("URL: %s\n", discovered.Source.URL)
				if discovered.Name != "" {
					fmt.Printf("Name: %s\n", discovered.Name)
				}

				if len(discovered.Tools) > 0 {
					fmt.Printf("\n%s Found %s tools\n",
						style.SuccessStyle.Render("✅"),
						style.HighlightStyle.Render(fmt.Sprintf("%d", len(discovered.Tools))))

					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Available Tools:"))
					for _, tool := range discovered.Tools {
						fmt.Printf("• %s\n", style.HighlightStyle.Render(tool.Name))
						if tool.Description != "" {
							fmt.Printf("  %s\n", tool.Description)
						}
						if len(tool.Args) > 0 {
							fmt.Printf("  Arguments: %d required, %d optional\n",
								util.CountRequiredArgs(tool.Args),
								len(tool.Args)-util.CountRequiredArgs(tool.Args))
						}
					}

					// Show add command
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Add this source:"))
					addCmd := fmt.Sprintf("kubiya source add %s", sourceURL)
					if dynamicConfig != "" {
						addCmd += fmt.Sprintf(" --config %s", dynamicConfig)
					}
					if runnerName != "" {
						addCmd += fmt.Sprintf(" --runner %s", runnerName)
					}
					fmt.Printf("%s\n", style.CommandStyle.Render(addCmd))
				} else {
					fmt.Printf("\n%s No tools found in source\n",
						style.WarningStyle.Render("⚠️"))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Possible reasons:"))
					fmt.Println("• The source doesn't contain any tool definitions")
					fmt.Println("• Tools are in a different branch or directory")
					fmt.Println("• Tool definitions might be invalid")
					fmt.Println("• Runner might not support the tool format")

					// Show directory contents if local
					if local {
						fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Directory contents:"))
						if files, err := os.ReadDir(args[0]); err == nil {
							for _, file := range files {
								fmt.Printf("• %s\n", file.Name())
							}
						}
					}
				}
				return nil
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&repo, "repo", "r", "", "Repository name (org/repo format)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Path within repository")
	cmd.Flags().StringVar(&remote, "remote", "origin", "Git remote to use")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force branch switch for local repositories")
	cmd.Flags().StringVarP(&dynamicConfig, "config", "c", "", "Dynamic configuration file (JSON)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().BoolVar(&push, "push", false, "Push changes to remote")
	cmd.Flags().StringVar(&commitMsg, "commit-msg", "", "Commit message for local changes")
	cmd.Flags().BoolVar(&addAll, "add", false, "Stage all changes")
	cmd.Flags().StringVar(&runnerName, "runner", "", "Runner name to use for loading the source")

	return cmd
}

// GitInfo holds information about a Git repository
type GitInfo struct {
	RepoURL         string
	Branch          string
	Remote          string
	RelativePath    string
	FullURL         string
	IsCurrentBranch bool
}

// getGitInfo gets Git repository information with enhanced options
func getGitInfo(path, remote, targetBranch string, force bool) (*GitInfo, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Find git root
	gitRoot, err := findGitRoot(absPath)
	if err != nil {
		return nil, err
	}

	// Get remote URL
	remoteURL, err := getRemoteURL(gitRoot, remote)
	if err != nil {
		return nil, err
	}

	// Get current branch
	currentBranch, err := getCurrentBranch(gitRoot)
	if err != nil {
		return nil, err
	}

	// Determine branch to use
	branch := currentBranch
	isCurrentBranch := true
	if targetBranch != "" {
		if currentBranch != targetBranch {
			if !force {
				return nil, fmt.Errorf("current branch is '%s', use --force to switch to '%s'", currentBranch, targetBranch)
			}
			isCurrentBranch = false
		}
		branch = targetBranch
	}

	// Get relative path if we're in a subdirectory
	var relativePath string
	if gitRoot != absPath {
		relativePath, err = filepath.Rel(gitRoot, absPath)
		if err != nil {
			return nil, err
		}
	}

	// Clean and format the URL
	repoURL := strings.TrimSuffix(remoteURL, ".git")
	fullURL := fmt.Sprintf("%s/tree/%s", repoURL, branch)
	if relativePath != "" {
		fullURL = fmt.Sprintf("%s/%s", fullURL, relativePath)
	}

	return &GitInfo{
		RepoURL:         repoURL,
		Branch:          branch,
		Remote:          remote,
		RelativePath:    relativePath,
		FullURL:         fullURL,
		IsCurrentBranch: isCurrentBranch,
	}, nil
}

// Helper functions for Git operations
func getCurrentBranch(gitDir string) (string, error) {
	cmd := exec.Command("git", "-C", gitDir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getRemoteURL(gitDir, remote string) (string, error) {
	cmd := exec.Command("git", "-C", gitDir, "remote", "get-url", remote)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("remote '%s' not found (try: git remote add %s <url>)", remote, remote)
	}
	return strings.TrimSpace(string(out)), nil
}

func newAddSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		name          string
		dynamicConfig string
		noConfirm     bool
	)

	cmd := &cobra.Command{
		Use:          "add [url]",
		Short:        "➕ Add a new source",
		SilenceUsage: true,
		Example: `  # Add a source
  kubiya source add https://github.com/org/repo
  
  # Add with custom name and config
  kubiya source add https://github.com/org/repo --name "My Source" --config config.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sourceURL := args[0]

			// First scan the source
			var dynConfig map[string]interface{}
			if dynamicConfig != "" {
				data, err := os.ReadFile(dynamicConfig)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
				if err := json.Unmarshal(data, &dynConfig); err != nil {
					return fmt.Errorf("invalid config JSON: %w", err)
				}
			}

			// Load/scan the source first
			scanned, err := client.LoadSource(cmd.Context(), sourceURL)
			if err != nil {
				return err
			}

			if !noConfirm {
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 📦 Source Preview "))
				fmt.Printf("URL: %s\n", scanned.URL)
				fmt.Printf("Tools found: %d\n\n", len(scanned.Tools))

				if len(scanned.Tools) > 0 {
					fmt.Println(style.SubtitleStyle.Render("Tools to be added:"))
					for _, tool := range scanned.Tools {
						fmt.Printf("• %s\n", style.HighlightStyle.Render(tool.Name))
						if tool.Description != "" {
							fmt.Printf("  %s\n", tool.Description)
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
			created, err := client.CreateSource(cmd.Context(), sourceURL)
			if err != nil {
				return err
			}

			fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Source added successfully!"))
			fmt.Printf("UUID: %s\n", created.UUID)
			fmt.Printf("Tools: %d\n", len(created.Tools))

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Source name")
	cmd.Flags().StringVarP(&dynamicConfig, "config", "c", "", "Dynamic configuration file (JSON)")
	cmd.Flags().BoolVar(&noConfirm, "yes", false, "Skip confirmation")

	return cmd
}

func newDescribeSourceCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "describe [uuid]",
		Short: "📖 Show detailed information about a source",
		Example: `  # Show source details
  kubiya source describe abc-123
  
  # Get details in JSON format
  kubiya source describe abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			source, err := client.GetSourceMetadata(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(source)
			default:
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 📦 Source Details "))
				fmt.Printf("UUID: %s\n", style.DimStyle.Render(source.UUID))
				fmt.Printf("Name: %s\n", style.HighlightStyle.Render(source.Name))
				fmt.Printf("URL: %s\n", source.URL)
				fmt.Printf("Tools: %d\n\n", len(source.Tools))

				if len(source.Tools) > 0 {
					fmt.Println(style.SubtitleStyle.Render("Available Tools:"))
					for _, tool := range source.Tools {
						fmt.Printf("• %s\n", style.HighlightStyle.Render(tool.Name))
						if tool.Description != "" {
							fmt.Printf("  %s\n", tool.Description)
						}
						if len(tool.Args) > 0 {
							fmt.Printf("  Arguments: %d required, %d optional\n",
								util.CountRequiredArgs(tool.Args),
								len(tool.Args)-util.CountRequiredArgs(tool.Args))
						}
					}
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newDeleteSourceCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:          "delete [uuid]",
		Short:        "🗑️ Delete a source",
		SilenceUsage: true,
		Example: `  # Delete a source
  kubiya source delete abc-123
  
  # Force delete without confirmation
  kubiya source delete abc-123 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get source details first
			source, err := client.GetSource(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("\n%s\n\n", style.WarningStyle.Render(" ⚠️  Warning "))
				fmt.Printf("About to delete source:\n")
				fmt.Printf("Name: %s\n", style.HighlightStyle.Render(source.Name))
				fmt.Printf("URL: %s\n", source.URL)
				fmt.Printf("Tools: %d\n\n", len(source.Tools))

				fmt.Print("Are you sure you want to delete this source? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			if err := client.DeleteSource(cmd.Context(), args[0]); err != nil {
				fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Failed to delete source:"))
				fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
				return err
			}

			fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Source deleted successfully!"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newSyncSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		mode       string
		branch     string
		force      bool
		autoCommit bool
		noDiff     bool
	)

	cmd := &cobra.Command{
		Use:          "sync [uuid]",
		Short:        "🔄 Sync a source",
		SilenceUsage: true,
		Example: `  # Sync a source interactively
  kubiya source sync abc-123
  
  # Sync with specific branch
  kubiya source sync abc-123 --branch main
  
  # Non-interactive sync
  kubiya source sync abc-123 --mode non-interactive --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get source details first
			source, err := client.GetSource(cmd.Context(), args[0])
			if err != nil {
				fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Failed to get source:"))
				fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
				return nil // Don't propagate error to avoid duplicate messages
			}

			// Store options for future use when client supports them
			opts := kubiya.SyncOptions{
				Mode:       mode,
				Branch:     branch,
				Force:      force,
				AutoCommit: autoCommit,
				NoDiff:     noDiff,
			}

			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔄 Syncing Source "))
			fmt.Printf("Name: %s\n", style.HighlightStyle.Render(source.Name))
			fmt.Printf("URL: %s\n", source.URL)

			// Call sync endpoint with options
			synced, err := client.SyncSource(cmd.Context(), args[0], opts)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Source not found"))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Common Solutions:"))
					fmt.Println("• Verify the source UUID is correct")
					fmt.Println("• Check if the source still exists")
					fmt.Println("• Try listing sources with: kubiya source list")
				} else {
					fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Sync failed:"))
					fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
				}
				return nil // Don't propagate error to avoid duplicate messages
			}

			fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Source synced successfully!"))
			fmt.Printf("Tools: %d\n", len(synced.Tools))

			// Show changes if any
			if len(synced.Tools) != len(source.Tools) {
				fmt.Printf("Changes: %d tools added/removed\n",
					len(synced.Tools)-len(source.Tools))
			}

			return nil
		},
	}

	// Keep the flags for future use
	cmd.Flags().StringVarP(&mode, "mode", "m", SyncModeInteractive,
		"Sync mode (interactive|non-interactive|ci)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to sync")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force sync")
	cmd.Flags().BoolVar(&autoCommit, "auto-commit", false, "Automatically commit changes")
	cmd.Flags().BoolVar(&noDiff, "no-diff", false, "Skip showing diffs")

	return cmd
}

// Helper function to find git root directory
func findGitRoot(start string) (string, error) {
	dir := start
	for {
		if isGitRepo(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found")
		}
		dir = parent
	}
}

// Helper function to check if directory is a git repo
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// GitStatus represents the state of the Git repository
type GitStatus struct {
	Unstaged []string
	Staged   []string
}

func (s *GitStatus) HasChanges() bool {
	return len(s.Unstaged) > 0 || len(s.Staged) > 0
}

// getGitStatus checks the Git status of the repository
func getGitStatus(path string) (*GitStatus, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	gitRoot, err := findGitRoot(absPath)
	if err != nil {
		return nil, err
	}

	// Get unstaged changes
	cmd := exec.Command("git", "-C", gitRoot, "diff", "--name-only")
	unstaged, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get unstaged changes: %w", err)
	}

	// Get staged changes
	cmd = exec.Command("git", "-C", gitRoot, "diff", "--cached", "--name-only")
	staged, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged changes: %w", err)
	}

	status := &GitStatus{
		Unstaged: parseGitFiles(string(unstaged)),
		Staged:   parseGitFiles(string(staged)),
	}

	return status, nil
}

// handleGitChanges manages Git operations based on flags
func handleGitChanges(path string, status *GitStatus, add, push bool, commitMsg string) error {
	gitRoot, err := findGitRoot(path)
	if err != nil {
		return err
	}

	if add {
		fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📝 Staging Changes"))
		cmd := exec.Command("git", "-C", gitRoot, "add", ".")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		fmt.Printf("%s Changes staged\n", style.SuccessStyle.Render("✓"))
	}

	if commitMsg != "" {
		fmt.Printf("\n%s\n", style.SubtitleStyle.Render("💾 Committing Changes"))
		cmd := exec.Command("git", "-C", gitRoot, "commit", "-m", commitMsg)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
		fmt.Printf("%s Changes committed\n", style.SuccessStyle.Render("✓"))
	}

	if push {
		if commitMsg == "" {
			return fmt.Errorf("commit message required when pushing (use --commit-msg)")
		}
		fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🚀 Pushing Changes"))
		cmd := exec.Command("git", "-C", gitRoot, "push")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to push changes: %w", err)
		}
		fmt.Printf("%s Changes pushed to remote\n", style.SuccessStyle.Render("✓"))
	}

	return nil
}

// parseGitFiles converts Git command output to string slice
func parseGitFiles(output string) []string {
	var files []string
	for _, file := range strings.Split(strings.TrimSpace(output), "\n") {
		if file != "" {
			files = append(files, file)
		}
	}
	return files
}
