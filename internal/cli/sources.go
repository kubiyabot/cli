package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Add these constants for sync modes
const (
	SyncModeInteractive    = "interactive"
	SyncModeNonInteractive = "non-interactive"
	SyncModeCI             = "ci"
)

var runnerName string

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
		newUpdateSourceCommand(cfg),
		newDebugSourceCommand(cfg),
	)

	cmd.PersistentFlags().StringVarP(&runnerName, "runner", "r", "", "Runner name")
	return cmd
}

func newListSourcesCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat  string
		full          bool
		debug         bool
		fetchMetadata bool
		maxConcurrent int
	)

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "📋 List all sources",
		Example:      "  kubiya source list\n  kubiya source list --output json\n  kubiya source list --full",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Enable debugging if requested
			oldDebug := cfg.Debug
			if debug {
				cfg.Debug = true
				defer func() { cfg.Debug = oldDebug }()
			}

			// Define spinner frames for progress indication
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIdx := 0

			// Start with a loading indicator
			fmt.Printf("%s Fetching sources...\r", spinner[spinnerIdx])

			client := kubiya.NewClient(cfg)
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return err
			}

			if debug {
				fmt.Println("Initial sources list received:")
				for i, source := range sources {
					fmt.Printf("Source %d: %s (UUID: %s)\n", i+1, source.Name, source.UUID)
					fmt.Printf("  Type: %s, URL: %s\n", source.Type, source.URL)
					fmt.Printf("  Tools: %d, InlineTools: %d\n\n", len(source.Tools), len(source.InlineTools))
				}
			}

			// Fetch metadata for each source (either full details or just tools count)
			if full || fetchMetadata {
				// Clear previous line and show message
				fmt.Print("\033[2K")
				fmt.Printf("🔍 Fetching source metadata for %d sources\n", len(sources))

				// Use a wait group to fetch metadata in parallel
				var wg sync.WaitGroup
				metadataLock := sync.Mutex{}

				// Track progress for UI
				progress := struct {
					completed    int
					successful   int
					failed       int
					errorSources []string
					sync.Mutex
				}{}

				// Create a context with cancellation
				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()

				// Optimize concurrency: Default to 10, allow override, adapt to source count
				if maxConcurrent <= 0 {
					maxConcurrent = 10 // Default
				}
				if maxConcurrent > len(sources) {
					maxConcurrent = len(sources)
				}

				// Limit concurrency with a semaphore
				semaphore := make(chan struct{}, maxConcurrent)

				// Set up UI updates
				if !debug {
					ticker := time.NewTicker(100 * time.Millisecond)

					// Start UI updater
					go func() {
						for {
							select {
							case <-ctx.Done():
								return
							case <-ticker.C:
								spinnerIdx = (spinnerIdx + 1) % len(spinner)
								progress.Lock()
								total := len(sources)
								completed := progress.completed
								successful := progress.successful
								failed := progress.failed
								percent := int(float64(completed) / float64(total) * 100)

								// Progress bar calculation
								width := 30 // Width of progress bar
								progressChars := int(float64(completed) / float64(total) * float64(width))
								progressBar := strings.Repeat("█", progressChars) + strings.Repeat("░", width-progressChars)

								// Clear line and show updated progress
								fmt.Printf("\r\033[K%s Progress: [%s] %d/%d (%d%%) | ✅ %d | ❌ %d",
									spinner[spinnerIdx],
									progressBar,
									completed, total, percent,
									successful, failed)
								progress.Unlock()
							}
						}
					}()
					defer ticker.Stop()
				}

				// Start fetching metadata
				for i := range sources {
					wg.Add(1)
					go func(index int) {
						defer wg.Done()

						// Acquire semaphore slot
						select {
						case <-ctx.Done():
							return
						case semaphore <- struct{}{}:
						}
						defer func() { <-semaphore }()

						if debug {
							fmt.Printf("\nFetching metadata for %s (UUID: %s)\n", sources[index].Name, sources[index].UUID)
						}

						metadata, err := client.GetSourceMetadata(ctx, sources[index].UUID)

						// Track progress and results
						progress.Lock()
						progress.completed++
						if err == nil && metadata != nil {
							progress.successful++
							metadataLock.Lock()
							sources[index] = *metadata
							metadataLock.Unlock()

							if debug {
								fmt.Printf("Metadata received: Type=%s, Tools=%d, InlineTools=%d\n",
									metadata.Type, len(metadata.Tools), len(metadata.InlineTools))
							}
						} else {
							progress.failed++
							progress.errorSources = append(progress.errorSources, sources[index].Name)
							if debug {
								fmt.Printf("Failed to get metadata for %s: %v\n", sources[index].Name, err)
							}
						}
						progress.Unlock()
					}(i)
				}

				wg.Wait()

				// Clear progress line and show summary
				fmt.Print("\r\033[K")
				fmt.Printf("✅ Metadata fetched for %d sources\n", len(sources))

				// Show errors if any
				progress.Lock()
				if progress.failed > 0 {
					fmt.Printf("⚠️ Failed to fetch metadata for %d sources\n", progress.failed)
					if progress.failed <= 3 {
						for _, name := range progress.errorSources {
							fmt.Printf("   - %s\n", name)
						}
					}
				}
				progress.Unlock()
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(sources)
			default:
				// Group sources by type for better organization
				gitSources := []kubiya.Source{}
				inlineSources := []kubiya.Source{}
				otherSources := []kubiya.Source{}

				for _, s := range sources {
					sourceType := getSourceType(&s)
					if sourceType == "🔄 git" {
						gitSources = append(gitSources, s)
					} else if sourceType == "📝 inline" {
						inlineSources = append(inlineSources, s)
					} else {
						otherSources = append(otherSources, s)
					}
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.TitleStyle.Render("📦 SOURCES"))
				fmt.Fprintln(w, "UUID\tNAME\tTYPE\tTOOLS\tSTATUS\tRUNNER")

				// Function to print sources list
				printSources := func(sources []kubiya.Source, sectionTitle string) {
					if len(sources) > 0 {
						fmt.Fprintln(w, style.SubtitleStyle.Render(sectionTitle))
						for _, s := range sources {
							toolCount := len(s.Tools) + len(s.InlineTools)

							status := style.SuccessStyle.Render("✅")
							if toolCount == 0 {
								status = style.WarningStyle.Render("⚠️")
							}

							sourceType := getSourceType(&s)
							runner := s.Runner
							if runner == "" {
								runner = style.DimStyle.Render("default")
							}

							fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
								style.DimStyle.Render(s.UUID),
								style.HighlightStyle.Render(s.Name),
								sourceType,
								toolCount,
								status,
								runner,
							)
						}
						fmt.Fprintln(w)
					}
				}

				// Print each section
				if len(gitSources) > 0 {
					printSources(gitSources, "Git Sources")
				}

				if len(inlineSources) > 0 {
					printSources(inlineSources, "Inline Sources")
				}

				if len(otherSources) > 0 {
					printSources(otherSources, "Other Sources")
				}

				// Show summary footer
				fmt.Fprintf(w, style.DimStyle.Render("Total: %d sources (%d git, %d inline, %d other)\n"),
					len(sources), len(gitSources), len(inlineSources), len(otherSources))

				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&full, "full", "f", false, "Fetch detailed information for all sources (slower)")
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Show debug information")
	cmd.Flags().BoolVarP(&fetchMetadata, "metadata", "m", true, "Fetch metadata to get accurate tool counts")
	cmd.Flags().IntVarP(&maxConcurrent, "concurrency", "c", 10, "Maximum number of concurrent metadata requests")
	return cmd
}

// getSourceType returns a formatted source type with an appropriate emoji
func getSourceType(source *kubiya.Source) string {
	emoji := "🔗"
	typeStr := source.Type

	switch source.Type {
	case "git":
		emoji = "🔄"
	case "inline":
		emoji = "📝"
	case "remote":
		emoji = "🌐"
	case "local":
		emoji = "📂"
	}

	return fmt.Sprintf("%s %s", emoji, typeStr)
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
			discovered, err := client.DiscoverSource(cmd.Context(), sourceURL, dynConfig, runnerName, nil)
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
		inlineFile    string
		inlineStdin   bool
		runnerName    string
	)

	cmd := &cobra.Command{
		Use:          "add [url]",
		Short:        "➕ Add a new source",
		SilenceUsage: true,
		Example: `  # Add a source from a URL
  kubiya source add https://github.com/org/repo
  
  # Add with custom name and config
  kubiya source add https://github.com/org/repo --name "My Source" --config config.json

  # Add an inline source from a file
  kubiya source add --inline tools.yaml --name "My Inline Tools" --runner my-runner
  
  # Add an inline source from stdin
  kubiya source add --inline-stdin --name "From Stdin" --runner my-runner`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sourceURL := ""

			// Check if we're adding an inline source
			if inlineFile != "" || inlineStdin {
				var tools []kubiya.Tool
				var err error

				// Read tools from file or stdin
				if inlineFile != "" {
					tools, err = loadToolsFromFile(inlineFile)
					if err != nil {
						return fmt.Errorf("failed to load tools from file: %w", err)
					}
				} else if inlineStdin {
					tools, err = loadToolsFromStdin()
					if err != nil {
						return fmt.Errorf("failed to load tools from stdin: %w", err)
					}
				}

				if len(tools) == 0 {
					return fmt.Errorf("no tools found in the provided source")
				}

				// Configure source options
				options := []kubiya.SourceOption{
					kubiya.WithInlineTools(tools),
				}

				if name != "" {
					options = append(options, kubiya.WithName(name))
				}

				if runnerName != "" {
					options = append(options, kubiya.WithRunner(runnerName))
				}

				if dynamicConfig != "" {
					configData, err := loadDynamicConfig(dynamicConfig)
					if err != nil {
						return err
					}
					options = append(options, kubiya.WithDynamicConfig(configData))
				}

				// Preview the tools
				if !noConfirm {
					fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 📦 Inline Source Preview "))
					fmt.Printf("Name: %s\n", name)
					if runnerName != "" {
						fmt.Printf("Runner: %s\n", runnerName)
					}
					fmt.Printf("Tools found: %d\n\n", len(tools))

					fmt.Println(style.SubtitleStyle.Render("Tools to be added:"))
					if len(tools) == 0 {
						fmt.Println("  No tools found or tools could not be parsed correctly.")
					} else {
						for i, tool := range tools {
							fmt.Printf("• %s\n", style.HighlightStyle.Render(tool.Name))
							if tool.Description != "" {
								fmt.Printf("  Description: %s\n", tool.Description)
							}
							if tool.Type != "" {
								fmt.Printf("  Type: %s\n", tool.Type)
							}
							if len(tool.Args) > 0 {
								fmt.Println("  Args:")
								for _, arg := range tool.Args {
									fmt.Printf("    - %s\n", arg.Name)
								}
							}
							if len(tool.Env) > 0 {
								fmt.Println("  Environment Variables:")
								for _, env := range tool.Env {
									fmt.Printf("    - %s\n", env)
								}
							}
							if len(tool.Secrets) > 0 {
								fmt.Println("  Secrets:")
								for _, secret := range tool.Secrets {
									fmt.Printf("    - %s\n", secret)
								}
							}
							if tool.WithFiles != nil {
								fmt.Printf("  Has Files: Yes\n")
								files := tool.GetToolFiles()
								if len(files) > 0 {
									fmt.Println("    Files:")
									for _, file := range files[:Min(len(files), 5)] {
										fmt.Printf("      - %s\n", file)
									}
									if len(files) > 5 {
										fmt.Printf("      ... and %d more files\n", len(files)-5)
									}
								}
							}
							if tool.WithVolumes != nil {
								fmt.Printf("  Has Volumes: Yes\n")
							}
							if tool.LongRunning {
								fmt.Printf("  Long Running: Yes\n")
							}
							if tool.Content != "" && len(tool.Content) > 200 {
								fmt.Printf("  Content: %s...(truncated)\n", tool.Content[:200])
							} else if tool.Content != "" {
								fmt.Printf("  Content: %s\n", tool.Content)
							}

							// Add a separator between tools except for the last one
							if i < len(tools)-1 {
								fmt.Println("  ---")
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

				// Create the source directly without scanning
				created, err := client.CreateSource(cmd.Context(), "", options...)
				if err != nil {
					return err
				}

				fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Inline source added successfully!"))
				fmt.Printf("UUID: %s\n", created.UUID)
				fmt.Printf("Tools: %d\n", len(created.Tools)+len(created.InlineTools))

				return nil
			}

			// Regular URL-based source
			if len(args) < 1 {
				return fmt.Errorf("url argument is required for non-inline sources")
			}
			sourceURL = args[0]

			// First scan the source
			var config map[string]interface{}
			if dynamicConfig != "" {
				var err error
				config, err = loadDynamicConfig(dynamicConfig)
				if err != nil {
					return err
				}
			}

			// Configure source options
			options := []kubiya.SourceOption{}

			if name != "" {
				options = append(options, kubiya.WithName(name))
			}

			if runnerName != "" {
				options = append(options, kubiya.WithRunner(runnerName))
			}

			if config != nil {
				options = append(options, kubiya.WithDynamicConfig(config))
			}

			// Load/scan the source first
			scanned, err := client.LoadSource(cmd.Context(), sourceURL, options...)
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
			created, err := client.CreateSource(cmd.Context(), sourceURL, options...)
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
	cmd.Flags().StringVar(&inlineFile, "inline", "", "File containing inline tool definitions (YAML or JSON)")
	cmd.Flags().BoolVar(&inlineStdin, "inline-stdin", false, "Read inline tool definitions from stdin")
	cmd.Flags().StringVar(&runnerName, "runner", "", "Runner name for the source")

	return cmd
}

// Helper function to load tools from a file
func loadToolsFromFile(filePath string) ([]kubiya.Tool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return parseToolsData(data, filePath)
}

// Helper function to load tools from stdin
func loadToolsFromStdin() ([]kubiya.Tool, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read from stdin: %w", err)
	}

	return parseToolsData(data, "")
}

// Helper function to parse tools data from YAML or JSON
func parseToolsData(data []byte, filename string) ([]kubiya.Tool, error) {
	var tools []kubiya.Tool

	// Determine format based on file extension or content
	isJSON := false
	if filename != "" {
		if strings.HasSuffix(strings.ToLower(filename), ".json") {
			isJSON = true
		}
	} else {
		// Try to determine from content
		trimmed := bytes.TrimSpace(data)
		if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			isJSON = true
		}
	}

	// Add debug output to see the data we're trying to parse
	fmt.Printf("Parsing data format: %s\n", map[bool]string{true: "JSON", false: "YAML"}[isJSON])

	if isJSON {
		if err := json.Unmarshal(data, &tools); err != nil {
			// Try as a single tool
			var tool kubiya.Tool
			if jsonErr := json.Unmarshal(data, &tool); jsonErr == nil {
				return []kubiya.Tool{tool}, nil
			}
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		// Try to parse YAML in multiple ways

		// 1. First try as an array of tools (standard format)
		if err := yaml.Unmarshal(data, &tools); err != nil {
			fmt.Printf("Failed to parse YAML as array, trying other formats: %v\n", err)

			// 2. Try as a structure with a 'tools' key containing an array
			var toolsWrapper struct {
				Tools []kubiya.Tool `yaml:"tools"`
			}
			if err := yaml.Unmarshal(data, &toolsWrapper); err == nil && len(toolsWrapper.Tools) > 0 {
				fmt.Printf("Successfully parsed YAML with 'tools' key containing %d tools\n", len(toolsWrapper.Tools))
				return toolsWrapper.Tools, nil
			}

			// 3. Try as a single tool
			var tool kubiya.Tool
			if yamlErr := yaml.Unmarshal(data, &tool); yamlErr == nil {
				// Ensure the tool has a name
				if tool.Name == "" && filename != "" {
					baseName := filepath.Base(filename)
					ext := filepath.Ext(baseName)
					if ext != "" {
						baseName = baseName[:len(baseName)-len(ext)]
					}
					tool.Name = baseName
				}

				if tool.Name != "" {
					return []kubiya.Tool{tool}, nil
				}
			}

			// 4. Try as a map with tool names as keys
			var toolMap map[string]interface{}
			if mapErr := yaml.Unmarshal(data, &toolMap); mapErr == nil {
				fmt.Printf("Parsed YAML as a map with %d keys\n", len(toolMap))

				// Check for a 'tools' key first - it might be an array of tools
				if toolsData, ok := toolMap["tools"]; ok {
					if toolList, ok := toolsData.([]interface{}); ok {
						fmt.Printf("Found 'tools' key with array of %d items\n", len(toolList))
						for _, item := range toolList {
							// Convert each item to a Tool
							itemBytes, _ := yaml.Marshal(item)
							var parsedTool kubiya.Tool
							if err := yaml.Unmarshal(itemBytes, &parsedTool); err == nil {
								tools = append(tools, parsedTool)
								fmt.Printf("Added tool from tools array: %s\n", parsedTool.Name)
							}
						}
						if len(tools) > 0 {
							return tools, nil
						}
					}
				}

				// Try to convert each key-value pair to a Tool
				for name, rawValue := range toolMap {
					// Skip the 'tools' key if we already processed it
					if name == "tools" {
						continue
					}

					// Convert the value to YAML for remarshaling
					valueBytes, mErr := yaml.Marshal(rawValue)
					if mErr != nil {
						fmt.Printf("Failed to marshal value for key %s: %v\n", name, mErr)
						continue
					}

					var parsedTool kubiya.Tool
					if tErr := yaml.Unmarshal(valueBytes, &parsedTool); tErr == nil {
						if parsedTool.Name == "" {
							parsedTool.Name = name
						}
						tools = append(tools, parsedTool)
						fmt.Printf("Added tool with name: %s\n", parsedTool.Name)
					} else {
						fmt.Printf("Failed to parse tool from key %s: %v\n", name, tErr)

						// If the value is a string, it might be a simple command
						if strValue, ok := rawValue.(string); ok {
							tools = append(tools, kubiya.Tool{
								Name:    name,
								Content: strValue,
							})
							fmt.Printf("Added simple command tool: %s\n", name)
						}
					}
				}

				if len(tools) > 0 {
					return tools, nil
				}
			}

			// 5. Try a more relaxed approach for nested YAML structures
			var rawYaml interface{}
			if rawErr := yaml.Unmarshal(data, &rawYaml); rawErr == nil {
				fmt.Println("Attempting to parse with relaxed YAML structure...")

				// Handle case where tool(s) might be nested under a top-level key
				if m, ok := rawYaml.(map[string]interface{}); ok {
					for key, value := range m {
						// Handle the 'tools' array specifically
						if key == "tools" {
							if toolsList, ok := value.([]interface{}); ok {
								for i, toolData := range toolsList {
									toolBytes, _ := yaml.Marshal(toolData)
									var tool kubiya.Tool
									if err := yaml.Unmarshal(toolBytes, &tool); err == nil {
										tools = append(tools, tool)
										fmt.Printf("Added tool %d from 'tools' array: %s\n", i+1, tool.Name)
									} else {
										fmt.Printf("Failed to parse tool %d from 'tools' array: %v\n", i+1, err)
									}
								}
								if len(tools) > 0 {
									return tools, nil
								}
							}
							continue
						}

						// If we find a map of maps, each inner map might be a tool
						if innerMap, ok := value.(map[string]interface{}); ok {
							for innerKey, innerValue := range innerMap {
								// Convert to YAML and try to parse as Tool
								innerBytes, _ := yaml.Marshal(innerValue)
								var innerTool kubiya.Tool
								if innerErr := yaml.Unmarshal(innerBytes, &innerTool); innerErr == nil {
									if innerTool.Name == "" {
										innerTool.Name = innerKey
									}
									tools = append(tools, innerTool)
									fmt.Printf("Added nested tool: %s\n", innerTool.Name)
								}
							}
						} else {
							// Try to parse this item directly as a Tool
							itemBytes, _ := yaml.Marshal(value)
							var tool kubiya.Tool
							if itemErr := yaml.Unmarshal(itemBytes, &tool); itemErr == nil {
								if tool.Name == "" {
									tool.Name = key
								}
								tools = append(tools, tool)
								fmt.Printf("Added top-level tool: %s\n", tool.Name)
							}
						}
					}
				}

				if len(tools) > 0 {
					return tools, nil
				}
			}

			// If we made it here, we couldn't parse the YAML in any of our supported formats
			if len(data) < 100 {
				return nil, fmt.Errorf("failed to parse data as tools: %s", string(data))
			} else {
				return nil, fmt.Errorf("failed to parse YAML data as tools: %w", err)
			}
		}
	}

	// If we made it here with an empty tools slice, try a last-resort approach
	if len(tools) == 0 && !isJSON {
		// The data might be a single tool file, but YAML unmarshaler is strict about types
		// Let's create a single tool with the content from the file
		var toolName string
		if filename != "" {
			baseName := filepath.Base(filename)
			ext := filepath.Ext(baseName)
			if ext != "" {
				baseName = baseName[:len(baseName)-len(ext)]
			}
			toolName = baseName
		} else {
			toolName = "tool-from-content"
		}

		// Create a tool with the content as is
		tool := kubiya.Tool{
			Name:    toolName,
			Content: string(data),
		}

		// Try to extract description from the first line if it looks like a comment
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 && (strings.HasPrefix(lines[0], "#") || strings.HasPrefix(lines[0], "//")) {
			tool.Description = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(lines[0], "#"), "//"))
		}

		return []kubiya.Tool{tool}, nil
	}

	return tools, nil
}

// Helper function to load dynamic configuration
func loadDynamicConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w", err)
	}

	return config, nil
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

				// Show type if available
				sourceType := "git"
				if source.Type == "inline" {
					sourceType = "inline"
				} else if source.URL == "" || len(source.InlineTools) > 0 {
					sourceType = "inline"
				}

				if sourceType == "inline" {
					fmt.Printf("Type: %s\n", style.SubtitleStyle.Render("inline"))
				} else if source.Type != "" {
					fmt.Printf("Type: %s\n", style.SubtitleStyle.Render(source.Type))
				} else {
					fmt.Printf("Type: %s\n", style.SubtitleStyle.Render("git"))
				}

				fmt.Printf("URL: %s\n", source.URL)

				// Show runner if available
				if source.Runner != "" {
					fmt.Printf("Runner: %s\n", source.Runner)
				}

				// Count and display tools
				totalTools := len(source.Tools) + len(source.InlineTools)
				fmt.Printf("Tools: %d\n\n", totalTools)

				// Display tools
				if len(source.Tools) > 0 || len(source.InlineTools) > 0 {
					fmt.Println(style.SubtitleStyle.Render("Available Tools:"))

					// Show git-based tools
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

					// Show inline tools if any
					for _, tool := range source.InlineTools {
						// Mark inline tools with a different indicator
						fmt.Printf("• %s %s\n",
							style.HighlightStyle.Render(tool.Name),
							style.WarningStyle.Render("[inline]"))
						if tool.Description != "" {
							fmt.Printf("  %s\n", tool.Description)
						}
						if tool.Type != "" {
							fmt.Printf("  Type: %s\n", tool.Type)
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

			if err := client.DeleteSource(cmd.Context(), args[0], runnerName); err != nil {
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
			synced, err := client.SyncSource(cmd.Context(), args[0], opts, runnerName)
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

func newUpdateSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		name          string
		dynamicConfig string
		noConfirm     bool
		inlineFile    string
		inlineStdin   bool
		runnerName    string
	)

	cmd := &cobra.Command{
		Use:          "update [uuid]",
		Short:        "🔄 Update an existing source",
		SilenceUsage: true,
		Example: `  # Update an inline source from a file
  kubiya source update abc-123 --inline tools.yaml --name "Updated Tools" --runner my-runner
  
  # Update an inline source from stdin
  kubiya source update abc-123 --inline-stdin --name "Updated From Stdin"
  
  # Update source name or runner
  kubiya source update abc-123 --name "New Name" --runner "new-runner"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			uuid := args[0]

			// First get the existing source
			source, err := client.GetSourceMetadata(cmd.Context(), uuid)
			if err != nil {
				return fmt.Errorf("failed to get source: %w", err)
			}

			// Configure options
			options := []kubiya.SourceOption{}

			// Update tools if inline file or stdin specified
			if inlineFile != "" || inlineStdin {
				var tools []kubiya.Tool

				// Read tools from file or stdin
				if inlineFile != "" {
					tools, err = loadToolsFromFile(inlineFile)
					if err != nil {
						return fmt.Errorf("failed to load tools from file: %w", err)
					}
				} else if inlineStdin {
					tools, err = loadToolsFromStdin()
					if err != nil {
						return fmt.Errorf("failed to read tools from stdin: %w", err)
					}
				}

				if len(tools) == 0 {
					return fmt.Errorf("no tools found in the provided source")
				}

				options = append(options, kubiya.WithInlineTools(tools))
			}

			// Add other options if specified
			if name != "" {
				options = append(options, kubiya.WithName(name))
			}

			if runnerName != "" {
				options = append(options, kubiya.WithRunner(runnerName))
			}

			if dynamicConfig != "" {
				config, err := loadDynamicConfig(dynamicConfig)
				if err != nil {
					return err
				}
				options = append(options, kubiya.WithDynamicConfig(config))
			}

			// Nothing to update
			if len(options) == 0 {
				fmt.Println("No updates specified")
				return nil
			}

			// Show preview of the update
			if !noConfirm {
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔄 Source Update Preview "))
				fmt.Printf("UUID: %s\n", uuid)

				if name != "" {
					fmt.Printf("Name: %s -> %s\n", source.Name, name)
				}

				if runnerName != "" {
					prevRunner := "default"
					if source.Runner != "" {
						prevRunner = source.Runner
					}
					fmt.Printf("Runner: %s -> %s\n", prevRunner, runnerName)
				}

				// If updating inline tools, show what will change
				var tools []kubiya.Tool
				for _, opt := range options {
					var temp kubiya.Source
					opt(&temp)
					if len(temp.InlineTools) > 0 {
						tools = temp.InlineTools
					}
				}

				if len(tools) > 0 {
					currentToolCount := len(source.Tools) + len(source.InlineTools)
					fmt.Printf("Tools: %d -> %d\n\n", currentToolCount, len(tools))

					fmt.Println(style.SubtitleStyle.Render("New tools:"))
					for i, tool := range tools {
						fmt.Printf("• %s\n", style.HighlightStyle.Render(tool.Name))
						if tool.Description != "" {
							fmt.Printf("  Description: %s\n", tool.Description)
						}
						if tool.Type != "" {
							fmt.Printf("  Type: %s\n", tool.Type)
						}
						if len(tool.Args) > 0 {
							fmt.Println("  Args:")
							for _, arg := range tool.Args {
								fmt.Printf("    - %s\n", arg.Name)
							}
						}
						if len(tool.Env) > 0 {
							fmt.Println("  Environment Variables:")
							for _, env := range tool.Env {
								fmt.Printf("    - %s\n", env)
							}
						}
						if len(tool.Secrets) > 0 {
							fmt.Println("  Secrets:")
							for _, secret := range tool.Secrets {
								fmt.Printf("    - %s\n", secret)
							}
						}
						if tool.WithFiles != nil {
							fmt.Printf("  Has Files: Yes\n")
							files := tool.GetToolFiles()
							if len(files) > 0 {
								fmt.Println("    Files:")
								for _, file := range files[:Min(len(files), 5)] {
									fmt.Printf("      - %s\n", file)
								}
								if len(files) > 5 {
									fmt.Printf("      ... and %d more files\n", len(files)-5)
								}
							}
						}
						if tool.WithVolumes != nil {
							fmt.Printf("  Has Volumes: Yes\n")
						}
						if tool.LongRunning {
							fmt.Printf("  Long Running: Yes\n")
						}
						if tool.Content != "" && len(tool.Content) > 200 {
							fmt.Printf("  Content: %s...(truncated)\n", tool.Content[:200])
						} else if tool.Content != "" {
							fmt.Printf("  Content: %s\n", tool.Content)
						}

						// Add a separator between tools except for the last one
						if i < len(tools)-1 {
							fmt.Println("  ---")
						}
					}
				}

				fmt.Print("\nDo you want to update this source? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			// Update the source
			updated, err := client.UpdateSource(cmd.Context(), uuid, options...)
			if err != nil {
				return fmt.Errorf("failed to update source: %w", err)
			}

			fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Source updated successfully!"))
			fmt.Printf("UUID: %s\n", updated.UUID)
			totalTools := len(updated.Tools) + len(updated.InlineTools)
			fmt.Printf("Tools: %d\n", totalTools)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "New source name")
	cmd.Flags().StringVarP(&dynamicConfig, "config", "c", "", "Dynamic configuration file (JSON)")
	cmd.Flags().BoolVar(&noConfirm, "yes", false, "Skip confirmation")
	cmd.Flags().StringVar(&inlineFile, "inline", "", "File containing inline tool definitions (YAML or JSON)")
	cmd.Flags().BoolVar(&inlineStdin, "inline-stdin", false, "Read inline tool definitions from stdin")
	cmd.Flags().StringVar(&runnerName, "runner", "", "Runner name for the source")

	return cmd
}

func newDebugSourceCommand(cfg *config.Config) *cobra.Command {
	var (
		fullDebug     bool
		outputFormat  string
		showRawOutput bool
	)

	cmd := &cobra.Command{
		Use:          "debug [uuid]",
		Short:        "🔍 Debug source metadata",
		Hidden:       false, // Make visible in help
		SilenceUsage: true,
		Example: `  # Basic debug info
  kubiya source debug abc-123
  
  # Full debug details
  kubiya source debug abc-123 --full
  
  # JSON output
  kubiya source debug abc-123 --output json
  
  # Show raw API response
  kubiya source debug abc-123 --raw`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Enable config debug if full debug requested
			oldDebug := cfg.Debug
			if fullDebug {
				cfg.Debug = true
				defer func() { cfg.Debug = oldDebug }()
			}

			// Define spinner frames for progress indication
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIdx := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			// Start spinner in background
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						spinnerIdx = (spinnerIdx + 1) % len(spinner)
						fmt.Printf("\r%s Loading source information...", spinner[spinnerIdx])
					}
				}
			}()

			client := kubiya.NewClient(cfg)
			uuid := args[0]

			// Get basic source info
			source, err := client.GetSource(ctx, uuid)
			if err != nil {
				cancel()              // Stop spinner
				fmt.Print("\r\033[K") // Clear line
				return fmt.Errorf("failed to get source: %w", err)
			}

			// Get detailed metadata
			var metadata *kubiya.Source
			metadata, err = client.GetSourceMetadata(ctx, uuid)

			// Stop spinner
			cancel()
			fmt.Print("\r\033[K") // Clear line

			// Output in JSON format if requested
			if outputFormat == "json" {
				result := map[string]interface{}{
					"basic_info": source,
				}
				if err == nil {
					result["detailed_info"] = metadata
				} else {
					result["metadata_error"] = err.Error()
				}
				return json.NewEncoder(os.Stdout).Encode(result)
			}

			// Show raw output if requested
			if showRawOutput {
				rawResp, rawErr := client.GetRawSourceMetadata(ctx, uuid)
				if rawErr != nil {
					return fmt.Errorf("failed to get raw source metadata: %w", rawErr)
				}

				// Pretty print JSON
				var prettyJSON bytes.Buffer
				rawJSON := []byte(rawResp)
				if err := json.Indent(&prettyJSON, rawJSON, "", "  "); err != nil {
					fmt.Println(rawResp) // If can't pretty print, show raw
				} else {
					fmt.Println(prettyJSON.String())
				}
				return nil
			}

			// Show source details in an attractive format
			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔍 Source Debug Information "))

			// Basic Information Section
			fmt.Printf("%s\n", style.SubtitleStyle.Render("📋 Basic Source Info"))
			fmt.Printf("UUID: %s\n", style.HighlightStyle.Render(source.UUID))
			fmt.Printf("Name: %s\n", style.HighlightStyle.Render(source.Name))
			fmt.Printf("Type: %s\n", getSourceType(source))
			fmt.Printf("URL: %s\n", source.URL)
			if source.Runner != "" {
				fmt.Printf("Runner: %s\n", style.HighlightStyle.Render(source.Runner))
			}

			// Show creation and update times if available
			if !source.CreatedAt.IsZero() {
				fmt.Printf("Created: %s\n", source.CreatedAt.Format(time.RFC3339))
			}
			if !source.UpdatedAt.IsZero() {
				fmt.Printf("Updated: %s\n", source.UpdatedAt.Format(time.RFC3339))
			}

			// Stats section
			fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📊 Stats"))
			totalTools := len(source.Tools) + len(source.InlineTools)
			fmt.Printf("Tools count: %d (%d regular, %d inline)\n",
				totalTools, len(source.Tools), len(source.InlineTools))

			if source.ConnectedAgentsCount > 0 {
				fmt.Printf("Connected Agents: %d\n", source.ConnectedAgentsCount)
			}
			if source.ConnectedToolsCount > 0 {
				fmt.Printf("Connected Tools: %d\n", source.ConnectedToolsCount)
			}
			if source.ConnectedWorkflowsCount > 0 {
				fmt.Printf("Connected Workflows: %d\n", source.ConnectedWorkflowsCount)
			}
			if source.ErrorsCount > 0 {
				fmt.Printf("Errors: %s%d\n", style.ErrorStyle.Render("⚠️ "), source.ErrorsCount)
			}

			// If metadata fetch failed, show error and offer debugging help
			if err != nil {
				fmt.Printf("\n%s %s\n",
					style.ErrorStyle.Render("❌"),
					style.ErrorStyle.Render("Failed to fetch detailed metadata"))
				fmt.Printf("Error: %s\n", err)

				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🛠️ Troubleshooting Options"))
				fmt.Println("• Verify API key and permissions")
				fmt.Println("• Check network connectivity")
				fmt.Println("• Try with --raw to see raw API response")
				fmt.Println("• Try syncing the source: kubiya source sync " + uuid)

				// Try to get raw response to diagnose
				fmt.Printf("\nAttempting to get raw metadata for diagnostics...\n")
				rawResp, rawErr := client.GetRawSourceMetadata(ctx, uuid)
				if rawErr != nil {
					fmt.Printf("Raw metadata fetch also failed: %v\n", rawErr)
					return nil
				}

				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🔍 Raw Metadata Sample (first 500 chars)"))
				sample := rawResp
				if len(rawResp) > 500 {
					sample = rawResp[:500] + "..."
				}
				fmt.Println(sample)

				return nil
			}

			// Detailed tool information
			if len(metadata.Tools) > 0 || len(metadata.InlineTools) > 0 {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🛠️ Tools"))

				// Regular tools
				if len(metadata.Tools) > 0 {
					fmt.Printf("%s (%d)\n", style.SubtitleStyle.Render("Regular Tools"), len(metadata.Tools))
					for i, tool := range metadata.Tools {
						toolTypeEmoji := "🧰"
						if tool.Type == "function" {
							toolTypeEmoji = "🔧"
						} else if tool.Type == "workflow" {
							toolTypeEmoji = "⚙️"
						}

						fmt.Printf("%d. %s %s\n", i+1, toolTypeEmoji, style.HighlightStyle.Render(tool.Name))

						if tool.Description != "" {
							fmt.Printf("   %s\n", tool.Description)
						}

						if fullDebug {
							fmt.Printf("   - Type: %s\n", tool.Type)
							if tool.WithFiles != nil {
								fmt.Printf("   - WithFiles: %v\n", tool.WithFiles)
								files := tool.GetToolFiles()
								if len(files) > 0 {
									fmt.Printf("   - Files: %v\n", files)
								}
							}
							if tool.WithVolumes != nil {
								fmt.Printf("   - WithVolumes: %v\n", tool.WithVolumes)
								volumes := tool.GetVolumes()
								if len(volumes) > 0 {
									fmt.Printf("   - Volumes: %v\n", volumes)
								}
							}
							if tool.Metadata != nil {
								fmt.Printf("   - Has Metadata: yes\n")
								metadataItems := tool.GetMetadata()
								if len(metadataItems) > 0 {
									fmt.Printf("   - Metadata Items: %v\n", metadataItems)
								}
							}
							fmt.Printf("   - Args: %d\n", len(tool.Args))
							if tool.LongRunning {
								fmt.Printf("   - Long Running: %s\n", style.WarningStyle.Render("yes"))
							}
						} else {
							// Simplified view for regular mode
							fmt.Printf("   - Args: %d (%d required)\n",
								len(tool.Args), util.CountRequiredArgs(tool.Args))
						}
						fmt.Println()
					}
				}

				// Inline tools
				if len(metadata.InlineTools) > 0 {
					fmt.Printf("%s (%d)\n", style.SubtitleStyle.Render("Inline Tools"), len(metadata.InlineTools))
					for i, tool := range metadata.InlineTools {
						fmt.Printf("%d. %s %s\n", i+1, "📝", style.HighlightStyle.Render(tool.Name))

						if tool.Description != "" {
							fmt.Printf("   %s\n", tool.Description)
						}

						if fullDebug {
							fmt.Printf("   - Type: %s\n", tool.Type)
							if tool.WithFiles != nil {
								fmt.Printf("   - WithFiles: %v\n", tool.WithFiles)
								files := tool.GetToolFiles()
								if len(files) > 0 {
									fmt.Printf("   - Files: %v\n", files)
								}
							}
							if tool.WithVolumes != nil {
								fmt.Printf("   - WithVolumes: %v\n", tool.WithVolumes)
								volumes := tool.GetVolumes()
								if len(volumes) > 0 {
									fmt.Printf("   - Volumes: %v\n", volumes)
								}
							}
							if tool.Metadata != nil {
								fmt.Printf("   - Has Metadata: yes\n")
								metadataItems := tool.GetMetadata()
								if len(metadataItems) > 0 {
									fmt.Printf("   - Metadata Items: %v\n", metadataItems)
								}
							}
							fmt.Printf("   - Args: %d\n", len(tool.Args))
							if tool.LongRunning {
								fmt.Printf("   - Long Running: %s\n", style.WarningStyle.Render("yes"))
							}
						} else {
							// Simplified view for regular mode
							fmt.Printf("   - Args: %d (%d required)\n",
								len(tool.Args), util.CountRequiredArgs(tool.Args))
						}
						fmt.Println()
					}
				}
			}

			// Dynamic config section if available
			if metadata.DynamicConfig != nil && len(metadata.DynamicConfig) > 0 {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("⚙️ Dynamic Configuration"))
				configJSON, err := json.MarshalIndent(metadata.DynamicConfig, "", "  ")
				if err != nil {
					fmt.Printf("Error formatting config: %v\n", err)
				} else {
					fmt.Println(string(configJSON))
				}
			}

			// Helpful commands section
			fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📚 Helpful Commands"))
			fmt.Printf("• View tools: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya tool list --source %s", uuid)))
			fmt.Printf("• Sync source: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya source sync %s", uuid)))
			fmt.Printf("• Update source: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya source update %s --name \"New Name\"", uuid)))
			fmt.Printf("• Delete source: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya source delete %s", uuid)))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&fullDebug, "full", "f", false, "Enable full debugging with detailed information")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVar(&showRawOutput, "raw", false, "Show raw API response")
	return cmd
}

// Min returns the smaller of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
