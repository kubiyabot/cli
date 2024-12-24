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
	)

	// Helper function to count required arguments
	countRequiredArgs := func(args []kubiya.ToolArg) int {
		count := 0
		for _, arg := range args {
			if arg.Required {
				count++
			}
		}
		return count
	}

	cmd := &cobra.Command{
		Use:   "scan [url|path]",
		Short: "🔍 Scan a source URL or local directory for available tools",
		Example: `  # Scan a source URL
  kubiya source scan https://github.com/org/repo

  # Scan current directory
  kubiya source scan . --local

  # Scan with dynamic configuration
  kubiya source scan https://github.com/org/repo --config config.json

  # Interactive scan
  kubiya source scan --interactive`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			if interactive {
				fmt.Printf("\n%s\n", style.TitleStyle.Render("🔍 Interactive Source Scan"))
				fmt.Printf("\nEnter source URL or path (. for current directory): ")
				var input string
				if _, err := fmt.Scanln(&input); err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				args = []string{strings.TrimSpace(input)}
			}

			if len(args) == 0 {
				return fmt.Errorf("source URL or path is required")
			}

			sourceURL := args[0]

			// Handle local paths
			if sourceURL == "." || strings.HasPrefix(sourceURL, "./") || strings.HasPrefix(sourceURL, "/") {
				local = true
			}

			if local {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📂 Local Directory Scan"))

				// Get git remote URL
				var err error
				sourceURL, err = getGitRemoteURL(sourceURL)
				if err != nil {
					fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Common Solutions:"))
					fmt.Println("• Ensure you're in a git repository")
					fmt.Println("• Set up a remote with: git remote add origin <url>")
					fmt.Println("• Push your changes to the remote repository")
					return nil
				}
				fmt.Printf("Found repository: %s\n\n", style.HighlightStyle.Render(sourceURL))
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

			// Use the discovery API endpoint instead of LoadSource
			discovered, err := client.DiscoverSource(cmd.Context(), sourceURL, dynConfig)
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
								countRequiredArgs(tool.Args),
								len(tool.Args)-countRequiredArgs(tool.Args))
						}
					}

					// Show add command
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Add this source:"))
					addCmd := fmt.Sprintf("kubiya source add %s", sourceURL)
					if dynamicConfig != "" {
						addCmd += fmt.Sprintf(" --config %s", dynamicConfig)
					}
					fmt.Printf("%s\n", style.CommandStyle.Render(addCmd))
				} else {
					fmt.Printf("\n%s No tools found in source\n",
						style.WarningStyle.Render("⚠️"))
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Possible reasons:"))
					fmt.Println("• The source doesn't contain any tool definitions")
					fmt.Println("• Tools are in a different branch or directory")
					fmt.Println("• Tool definitions might be invalid")

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

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVarP(&dynamicConfig, "config", "c", "", "Dynamic configuration file (JSON)")
	cmd.Flags().BoolVarP(&local, "local", "l", false, "Scan local directory")

	return cmd
}

// Enhanced getGitRemoteURL function
func getGitRemoteURL(path string) (string, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", absPath)
	}

	// Run git commands to verify repo and get remote URL
	gitDir := absPath
	if !isGitRepo(gitDir) {
		// Try to find git root directory
		gitDir, err = findGitRoot(absPath)
		if err != nil {
			return "", fmt.Errorf("not a git repository: %s", absPath)
		}
	}

	// Get remote URL
	cmd := exec.Command("git", "-C", gitDir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no remote URL found (try: git remote add origin <url>)")
	}

	// Clean and format the URL
	url := strings.TrimSpace(string(out))
	url = strings.TrimSuffix(url, ".git")

	// If path is different from git root, append the relative path
	if gitDir != absPath {
		relPath, err := filepath.Rel(gitDir, absPath)
		if err == nil && relPath != "." {
			url = fmt.Sprintf("%s/tree/master/%s", url, relPath)
		}
	}

	return url, nil
}

// Helper function to check if directory is a git repo
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
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

	// Helper function to count required arguments
	countRequiredArgs := func(args []kubiya.ToolArg) int {
		count := 0
		for _, arg := range args {
			if arg.Required {
				count++
			}
		}
		return count
	}

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
								countRequiredArgs(tool.Args),
								len(tool.Args)-countRequiredArgs(tool.Args))
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
				return err
			}

			// Store options for future use when client supports them
			_ = SyncOptions{
				Mode:       mode,
				Branch:     branch,
				Force:      force,
				AutoCommit: autoCommit,
				NoDiff:     noDiff,
			}

			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔄 Syncing Source "))
			fmt.Printf("Name: %s\n", style.HighlightStyle.Render(source.Name))
			fmt.Printf("URL: %s\n", source.URL)

			// Call sync endpoint (without options for now)
			synced, err := client.SyncSource(cmd.Context(), args[0])
			if err != nil {
				fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Sync failed:"))
				fmt.Printf("%s\n", style.ErrorStyle.Render(err.Error()))
				return err
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
