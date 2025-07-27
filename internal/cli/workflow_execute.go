package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// URLInfo represents parsed URL information
type URLInfo struct {
	Type        string // "github_repo", "raw_url", "local_file"
	Original    string
	RepoURL     string
	Branch      string
	FilePath    string
	IsDirectory bool
	TempDir     string
}

// parseWorkflowURL parses a workflow input to determine if it's a URL, GitHub repo, or local file
func parseWorkflowURL(input string) (*URLInfo, error) {
	// Check if it's a local file first
	if !strings.Contains(input, "://") && !strings.HasPrefix(input, "github.com") {
		if _, err := os.Stat(input); err == nil {
			return &URLInfo{
				Type:     "local_file",
				Original: input,
				FilePath: input,
			}, nil
		}
	}

	// Parse as URL
	parsedURL, err := url.Parse(input)
	if err != nil || parsedURL.Scheme == "" {
		// Try to parse as GitHub shorthand (e.g., "user/repo" or "github.com/user/repo")
		if githubInfo := parseGitHubShorthand(input); githubInfo != nil {
			return githubInfo, nil
		}
		return nil, fmt.Errorf("invalid URL or file path: %s", input)
	}

	// Handle GitHub URLs
	if strings.Contains(parsedURL.Host, "github.com") {
		return parseGitHubURL(parsedURL)
	}

	// Handle raw URLs (direct download)
	return &URLInfo{
		Type:     "raw_url",
		Original: input,
		FilePath: filepath.Base(parsedURL.Path),
	}, nil
}

// parseGitHubShorthand parses GitHub shorthand notation like "user/repo" or "user/repo/path/to/file"
func parseGitHubShorthand(input string) *URLInfo {
	// Remove github.com prefix if present
	input = strings.TrimPrefix(input, "github.com/")
	input = strings.TrimPrefix(input, "www.github.com/")

	// Basic validation - should have at least owner/repo
	parts := strings.Split(input, "/")
	if len(parts) < 2 {
		return nil
	}

	owner, repo := parts[0], parts[1]
	
	// Extract file path if present (parts[2:])
	var filePath string
	if len(parts) > 2 {
		filePath = strings.Join(parts[2:], "/")
	}

	return &URLInfo{
		Type:     "github_repo",
		Original: input,
		RepoURL:  fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		Branch:   "main", // default branch
		FilePath: filePath,
	}
}

// parseGitHubURL parses a full GitHub URL
func parseGitHubURL(parsedURL *url.URL) (*URLInfo, error) {
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URL: must contain owner/repo")
	}

	owner, repo := pathParts[0], pathParts[1]
	
	// Remove .git suffix if present
	repo = strings.TrimSuffix(repo, ".git")

	var branch, filePath string
	
	// Handle different GitHub URL formats
	if len(pathParts) > 2 {
		if pathParts[2] == "blob" || pathParts[2] == "tree" {
			// Format: github.com/owner/repo/blob/branch/path/to/file
			if len(pathParts) > 3 {
				branch = pathParts[3]
				if len(pathParts) > 4 {
					filePath = strings.Join(pathParts[4:], "/")
				}
			}
		} else {
			// Direct path: github.com/owner/repo/path/to/file
			filePath = strings.Join(pathParts[2:], "/")
		}
	}

	if branch == "" {
		branch = "main" // default branch
	}

	return &URLInfo{
		Type:     "github_repo",
		Original: parsedURL.String(),
		RepoURL:  fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		Branch:   branch,
		FilePath: filePath,
	}, nil
}

// fetchWorkflowFromURL downloads or clones the workflow based on URL type
func fetchWorkflowFromURL(urlInfo *URLInfo) (string, error) {
	switch urlInfo.Type {
	case "local_file":
		return urlInfo.FilePath, nil
		
	case "raw_url":
		return downloadFromRawURL(urlInfo)
		
	case "github_repo":
		return fetchFromGitHubRepo(urlInfo)
		
	default:
		return "", fmt.Errorf("unsupported URL type: %s", urlInfo.Type)
	}
}

// downloadFromRawURL downloads a file from a raw URL
func downloadFromRawURL(urlInfo *URLInfo) (string, error) {
	fmt.Printf("%s Downloading workflow from URL...\n", style.InfoStyle.Render("📥"))
	
	resp, err := http.Get(urlInfo.Original)
	if err != nil {
		return "", fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download: HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "workflow-*."+getFileExtension(urlInfo.FilePath))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tempFile.Close()

	// Copy content
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write downloaded content: %w", err)
	}

	fmt.Printf("%s Downloaded to: %s\n", style.SuccessStyle.Render("✅"), tempFile.Name())
	return tempFile.Name(), nil
}

// cloneRepositoryWithAuth attempts to clone repository with GitHub authentication
func cloneRepositoryWithAuth(urlInfo *URLInfo, tempDir string) error {
	// First, try to get GitHub token from Kubiya integrations
	var authRepoURL string
	hasAuth := false
	
	// Create a dummy config to access the client
	if cfg := getCurrentConfig(); cfg != nil {
		client := kubiya.NewClient(cfg)
		ctx := context.Background()
		
		if token, err := client.GetGitHubToken(ctx); err == nil && token != "" {
			// Construct authenticated URL
			// Convert https://github.com/owner/repo.git to https://token@github.com/owner/repo.git
			if strings.HasPrefix(urlInfo.RepoURL, "https://github.com/") {
				authRepoURL = strings.Replace(urlInfo.RepoURL, "https://github.com/", 
					fmt.Sprintf("https://%s@github.com/", token), 1)
				hasAuth = true
				fmt.Printf("%s Using GitHub authentication from integrations\n", 
					style.InfoStyle.Render("🔐"))
			}
		} else {
			// Authentication failed, but continue with public access
			if cfg.Debug {
				fmt.Printf("%s GitHub authentication not available: %v\n", 
					style.DimStyle.Render("ℹ️"), err)
			}
			// Show helpful guidance for setting up GitHub integration
			fmt.Printf("%s For private repositories, set up GitHub integration at:\n", 
				style.DimStyle.Render("💡"))
			fmt.Printf("  • Composer App: %s\n", 
				style.HighlightStyle.Render("https://compose.kubiya.ai"))
			fmt.Printf("  • API: %s\n", 
				style.DimStyle.Render("Use the integrations API"))
			fmt.Printf("  • CLI: %s\n", 
				style.DimStyle.Render("kubiya integrations --help"))
		}
	}

	// Try authenticated clone first if we have auth
	if hasAuth {
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", urlInfo.Branch, authRepoURL, tempDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // Disable interactive prompts
		if err := cmd.Run(); err == nil {
			fmt.Printf("%s Repository cloned with authentication\n", 
				style.SuccessStyle.Render("✅"))
			return nil
		}
		
		// Authenticated clone failed, show helpful message and fallback
		fmt.Printf("%s Authenticated clone failed, trying public access...\n", 
			style.WarningStyle.Render("⚠️"))
	}

	// Fallback to public clone
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", urlInfo.Branch, urlInfo.RepoURL, tempDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // Disable interactive prompts
	return cmd.Run()
}

// getCurrentConfig gets the current configuration (helper function)
var getCurrentConfig = func() *config.Config {
	// This will be set by the main command execution context
	// For now, return nil - we'll update this when we have access to the config
	return nil
}

// cloneRepositoryWithBranchFallback attempts to clone with auth and branch fallback
func cloneRepositoryWithBranchFallback(urlInfo *URLInfo, tempDir string) error {
	// First attempt with specified branch
	if err := cloneRepositoryWithAuth(urlInfo, tempDir); err == nil {
		return nil
	}

	// If the specified branch failed and it's not 'main', try with 'main'
	if urlInfo.Branch != "main" {
		fmt.Printf("%s Branch '%s' not found, trying 'main'...\n", 
			style.WarningStyle.Render("⚠️"), urlInfo.Branch)
		
		// Clean up failed attempt
		os.RemoveAll(tempDir)
		
		// Create new temp directory
		newTempDir, err := os.MkdirTemp("", "kubiya-repo-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory for main branch: %w", err)
		}
		urlInfo.TempDir = newTempDir
		
		// Update branch to main and try again
		originalBranch := urlInfo.Branch
		urlInfo.Branch = "main"
		
		if err := cloneRepositoryWithAuth(urlInfo, newTempDir); err == nil {
			return nil
		}
		
		// Restore original branch for error reporting
		urlInfo.Branch = originalBranch
	}

	// Final attempt without specifying branch (use default)
	fmt.Printf("%s Trying default branch...\n", style.InfoStyle.Render("📥"))
	
	// Clean up previous attempt
	os.RemoveAll(urlInfo.TempDir)
	
	// Create new temp directory
	finalTempDir, err := os.MkdirTemp("", "kubiya-repo-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory for default branch: %w", err)
	}
	urlInfo.TempDir = finalTempDir
	
	// Try without branch specification
	return cloneRepositoryWithoutBranch(urlInfo, finalTempDir)
}

// cloneRepositoryWithoutBranch clones without specifying a branch
func cloneRepositoryWithoutBranch(urlInfo *URLInfo, tempDir string) error {
	// Try to get GitHub token for authentication
	var authRepoURL string
	hasAuth := false
	
	if cfg := getCurrentConfig(); cfg != nil {
		client := kubiya.NewClient(cfg)
		ctx := context.Background()
		
		if token, err := client.GetGitHubToken(ctx); err == nil && token != "" {
			if strings.HasPrefix(urlInfo.RepoURL, "https://github.com/") {
				authRepoURL = strings.Replace(urlInfo.RepoURL, "https://github.com/", 
					fmt.Sprintf("https://%s@github.com/", token), 1)
				hasAuth = true
			}
		}
	}

	// Try authenticated clone first if we have auth
	if hasAuth {
		cmd := exec.Command("git", "clone", "--depth", "1", authRepoURL, tempDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Fallback to public clone
	cmd := exec.Command("git", "clone", "--depth", "1", urlInfo.RepoURL, tempDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.Run()
}

// fetchFromGitHubRepo clones repository and finds the workflow file
func fetchFromGitHubRepo(urlInfo *URLInfo) (string, error) {
	fmt.Printf("%s Cloning repository...\n", style.InfoStyle.Render("📥"))
	
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "kubiya-repo-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	urlInfo.TempDir = tempDir

	// Clone the repository with authentication attempt and branch fallback
	if err := cloneRepositoryWithBranchFallback(urlInfo, tempDir); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("%s Repository cloned to: %s\n", style.SuccessStyle.Render("✅"), tempDir)

	// Find the workflow file
	workflowPath, err := findWorkflowFile(tempDir, urlInfo.FilePath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return workflowPath, nil
}

// findWorkflowFile finds a workflow file in the repository
func findWorkflowFile(repoDir, filePath string) (string, error) {
	if filePath != "" {
		// Specific file path provided
		fullPath := filepath.Join(repoDir, filePath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
		return "", fmt.Errorf("specified file not found: %s", filePath)
	}

	// Search for common workflow files
	commonWorkflowFiles := []string{
		"workflow.yaml", "workflow.yml", "workflow.json",
		".github/workflows/*.yaml", ".github/workflows/*.yml",
		"*.workflow.yaml", "*.workflow.yml", "*.workflow.json",
		"workflows/*.yaml", "workflows/*.yml", "workflows/*.json",
	}

	for _, pattern := range commonWorkflowFiles {
		matches, err := filepath.Glob(filepath.Join(repoDir, pattern))
		if err == nil && len(matches) > 0 {
			fmt.Printf("%s Found workflow file: %s\n", style.InfoStyle.Render("🔍"), 
				strings.TrimPrefix(matches[0], repoDir+"/"))
			return matches[0], nil
		}
	}

	// If no common patterns found, prompt user or list available files
	return promptUserForWorkflowFile(repoDir)
}

// promptUserForWorkflowFile helps user select a workflow file from available options
func promptUserForWorkflowFile(repoDir string) (string, error) {
	// Find all YAML and JSON files
	var candidates []string
	
	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}
		
		if info.IsDir() {
			return nil
		}
		
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			relPath, _ := filepath.Rel(repoDir, path)
			candidates = append(candidates, relPath)
		}
		
		return nil
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to search for workflow files: %w", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no YAML or JSON files found in repository")
	}

	// For now, return the first candidate (in a real implementation, you'd prompt the user)
	fmt.Printf("%s Multiple workflow candidates found, using: %s\n", 
		style.InfoStyle.Render("📄"), candidates[0])
	fmt.Printf("%s Other candidates: %s\n", 
		style.DimStyle.Render("ℹ️"), strings.Join(candidates[1:], ", "))
	
	return filepath.Join(repoDir, candidates[0]), nil
}

// getFileExtension returns the file extension for a given filename
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		// Try to guess from content type or default to yaml
		return "yaml"
	}
	return strings.TrimPrefix(ext, ".")
}

// cleanupTempFiles removes temporary files created during URL fetching
func cleanupTempFiles(urlInfo *URLInfo, workflowPath string) {
	if urlInfo.Type == "raw_url" && workflowPath != urlInfo.Original {
		os.Remove(workflowPath)
	}
	if urlInfo.TempDir != "" {
		os.RemoveAll(urlInfo.TempDir)
	}
}

func newWorkflowExecuteCommand(cfg *config.Config) *cobra.Command {
	var (
		runner          string
		variables       []string
		watch           bool
		skipPolicyCheck bool
		verbose         bool
		saveTrace       bool
	)

	cmd := &cobra.Command{
		Use:   "execute [workflow-file-or-url]",
		Short: "Execute a workflow from a file, URL, or GitHub repository",
		Long: `Execute a workflow defined in a YAML or JSON file.

This command loads a workflow from various sources and executes it using the Kubiya API:
• Local files in YAML (.yaml, .yml) or JSON (.json) format
• Raw URLs pointing to workflow files 
• GitHub repositories (with automatic cloning and workflow detection)
• GitHub shorthand notation (owner/repo or owner/repo/path/to/file)

The format will be auto-detected and you can provide variables and choose the runner for execution.`,
		Example: `  # Execute a local YAML workflow
  kubiya workflow execute deploy.yaml

  # Execute from GitHub repository (shorthand)
  kubiya workflow execute myorg/deploy-workflows
  
  # Execute specific file from GitHub repo
  kubiya workflow execute myorg/deploy-workflows/production/deploy.yaml
  
  # Execute from full GitHub URL
  kubiya workflow execute https://github.com/myorg/workflows/blob/main/deploy.yaml

  # Execute from raw URL
  kubiya workflow execute https://raw.githubusercontent.com/myorg/workflows/main/deploy.yaml

  # Execute with variables
  kubiya workflow execute myorg/deploy-workflows --var env=production --var retention=30

  # Execute with specific runner
  kubiya workflow execute https://example.com/workflow.json --runner prod-runner

  # Execute with verbose SSE logging
  kubiya workflow execute myorg/workflows --verbose`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Check for automation mode (either --silent flag or KUBIYA_AUTOMATION env var)
			automationMode := os.Getenv("KUBIYA_AUTOMATION") != ""

			// Set the config for helper functions
			getCurrentConfig = func() *config.Config { return cfg }

			// Parse input to determine if it's a URL, GitHub repo, or local file
			workflowInput := args[0]
			
			// Parse variables first
			vars := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Parse workflow URL/file
			urlInfo, err := parseWorkflowURL(workflowInput)
			if err != nil {
				return fmt.Errorf("failed to parse workflow input: %w", err)
			}

			// Fetch workflow from URL/repo/file
			workflowFile, err := fetchWorkflowFromURL(urlInfo)
			if err != nil {
				return fmt.Errorf("failed to fetch workflow: %w", err)
			}

			// Ensure cleanup of temporary files
			defer cleanupTempFiles(urlInfo, workflowFile)

			// Parse workflow file (supports both JSON and YAML with auto-detection)
			workflow, workflowReq, format, err := parseWorkflowFile(workflowFile, vars)
			if err != nil {
				return err
			}

			var req kubiya.WorkflowExecutionRequest
			if workflowReq != nil {
				// Already in WorkflowExecutionRequest format
				req = *workflowReq
			} else if workflow != nil {
				// Convert from Workflow struct
				req = buildExecutionRequest(*workflow, vars, runner)
			} else {
				return fmt.Errorf("failed to parse workflow file")
			}
			
			// Debug logging for workflow parsing
			if cfg.Debug || verbose {
				fmt.Printf("[DEBUG] Parsed workflow: Name=%s, Steps=%d\n", req.Name, len(req.Steps))
				for i, step := range req.Steps {
					if stepMap, ok := step.(map[string]interface{}); ok {
						if name, ok := stepMap["name"].(string); ok {
							fmt.Printf("[DEBUG] Step %d: %s\n", i+1, name)
						}
					}
				}
			}

			if !automationMode {
				// Show format detection info if helpful
				if format != "" {
					fmt.Printf("%s Detected format: %s\n", style.DimStyle.Render("📄"), format)
				}

				fmt.Printf("%s Executing workflow: %s\n", style.StatusStyle.Render("🚀"), style.HighlightStyle.Render(req.Name))
				
				// Show source information
				switch urlInfo.Type {
				case "local_file":
					fmt.Printf("%s %s\n", style.DimStyle.Render("Source:"), "Local file")
					fmt.Printf("%s %s\n", style.DimStyle.Render("File:"), workflowFile)
				case "raw_url":
					fmt.Printf("%s %s\n", style.DimStyle.Render("Source:"), "Raw URL")
					fmt.Printf("%s %s\n", style.DimStyle.Render("URL:"), urlInfo.Original)
				case "github_repo":
					fmt.Printf("%s %s\n", style.DimStyle.Render("Source:"), "GitHub Repository")
					fmt.Printf("%s %s\n", style.DimStyle.Render("Repository:"), urlInfo.RepoURL)
					if urlInfo.Branch != "main" {
						fmt.Printf("%s %s\n", style.DimStyle.Render("Branch:"), urlInfo.Branch)
					}
					if urlInfo.FilePath != "" {
						fmt.Printf("%s %s\n", style.DimStyle.Render("File:"), urlInfo.FilePath)
					}
				}
			}
			if !automationMode {
				if runner != "" {
					fmt.Printf("%s %s\n", style.DimStyle.Render("Runner:"), runner)
				}
				if len(req.Params) > 0 {
					fmt.Printf("%s\n", style.DimStyle.Render("Params:"))
					for k, v := range req.Params {
						fmt.Printf("  %s = %v\n", style.KeyStyle.Render(k), v)
					}
				}
				fmt.Println()
			}

			// Policy validation (if enabled)
			if !skipPolicyCheck {
				opaEnforce := os.Getenv("KUBIYA_OPA_ENFORCE")
				if opaEnforce == "true" || opaEnforce == "1" {
					fmt.Printf("%s Validating workflow execution permissions...\n", style.InfoStyle.Render("🛡️"))
					
					// Convert workflow to map for validation
					workflowDef := map[string]interface{}{
						"name":        req.Name,
						"description": req.Description,
						"steps":       req.Steps,
					}
					
					allowed, issues, err := client.ValidateWorkflowExecution(ctx, workflowDef, req.Params, runner)
					if err != nil {
						return fmt.Errorf("workflow permission validation failed: %w", err)
					}
					
					if !allowed {
						fmt.Printf("%s Workflow execution denied by policy:\n", style.ErrorStyle.Render("❌"))
						for _, issue := range issues {
							fmt.Printf("  • %s\n", issue)
						}
						return fmt.Errorf("workflow execution denied by policy")
					}
					
					if len(issues) > 0 {
						fmt.Printf("%s Workflow execution permitted with warnings:\n", style.WarningStyle.Render("⚠️"))
						for _, issue := range issues {
							fmt.Printf("  • %s\n", issue)
						}
					} else {
						fmt.Printf("%s Workflow execution permissions validated\n", style.SuccessStyle.Render("✅"))
					}
					fmt.Println()
				}
			}

			// Set the command field as required by the API
			req.Command = "execute_workflow"
			
			// Inject Kubiya API key into workflow environment
			if req.Env == nil {
				req.Env = make(map[string]interface{})
			}
			req.Env["KUBIYA_API_KEY"] = cfg.APIKey
			
			if cfg.Debug || verbose {
				fmt.Printf("[DEBUG] Injected KUBIYA_API_KEY into workflow environment\n")
			}

			// Show connection status
			runnerDisplayName := runner
			if runner == "kubiya-hosted" {
				runnerDisplayName = "Kubiya Hosted Runner"
			}
			if !automationMode {
				fmt.Printf("%s Connecting to %s...", 
					style.InfoStyle.Render("🔌"), runnerDisplayName)
			}
			
			// Enhanced workflow execution with better error handling and reconnection
			if cfg.Debug || verbose {
				fmt.Printf("[DEBUG] Using enhanced workflow execution\n")
			}
			
			// Try enhanced client first, fallback to regular if not available
			enhancedClient := client.WorkflowEnhanced()
			
			var enhancedEvents <-chan kubiya.EnhancedWorkflowEvent
			err = sentryutil.WithWorkflowExecution(ctx, req.Name, req.Name, func(ctx context.Context) error {
				var execErr error
				enhancedEvents, execErr = enhancedClient.ExecuteWorkflowEnhanced(ctx, req, runner)
				return execErr
			})
			
			// Add breadcrumb for workflow execution start
			sentryutil.AddBreadcrumb("kubiya.workflow", "Workflow execution started", map[string]interface{}{
				"workflow_name": req.Name,
				"runner":        runner,
				"step_count":    len(req.Steps),
				"enhanced":      true,
			})
			
			if err != nil {
				if !automationMode {
					fmt.Printf(" %s\n", style.WarningStyle.Render("enhanced failed, trying regular..."))
				}
				
				// Fallback to regular workflow client
				workflowClient := client.Workflow()
				
				var events <-chan kubiya.WorkflowSSEEvent
				err = sentryutil.WithWorkflowExecution(ctx, req.Name, req.Name, func(ctx context.Context) error {
					var execErr error
					events, execErr = workflowClient.ExecuteWorkflow(ctx, req, runner)
					return execErr
				})
				
				// Update breadcrumb for fallback
				sentryutil.AddBreadcrumb("kubiya.workflow", "Fallback to regular workflow execution", map[string]interface{}{
					"workflow_name": req.Name,
					"runner":        runner,
					"enhanced":      false,
				})
				
				if err != nil {
					if !automationMode {
						fmt.Printf(" %s\n", style.ErrorStyle.Render("failed!"))
					}
					return fmt.Errorf("failed to execute workflow: %w", err)
				}
				
				if !automationMode {
					fmt.Printf(" %s\n", style.SuccessStyle.Render("connected!"))
				}
				
				// Process regular events
				var stepStartTimes = make(map[string]time.Time)
				var workflowTrace = NewWorkflowTrace(req.Name, len(req.Steps))
				return processRegularWorkflowEvents(ctx, events, automationMode, verbose, saveTrace, workflowTrace, workflowFile, req, stepStartTimes, 0, len(req.Steps))
			}
			
			if !automationMode {
				fmt.Printf(" %s\n", style.SuccessStyle.Render("connected with enhanced features!"))
			}

			// Process enhanced workflow events with better error handling
			var hasError bool
			var stepCount int
			var completedSteps int
			var workflowID string
			
			// Count total steps for progress tracking
			stepCount = len(req.Steps)
			
			if !automationMode {
				fmt.Printf("\n%s Starting enhanced workflow execution...\n\n", 
					style.InfoStyle.Render("🚀"))
				
				// Show initial progress
				progressBar := generateProgressBar(completedSteps, stepCount)
				fmt.Printf("%s %s %s\n\n", 
					style.InfoStyle.Render("📊"),
					progressBar,
					style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps, stepCount)))
			}
			
			// Initialize workflow execution tracking
			var stepStartTimes = make(map[string]time.Time)
			var workflowTrace = NewWorkflowTrace(req.Name, stepCount)
			
			for enhancedEvent := range enhancedEvents {
				// Extract workflow ID if available
				if enhancedEvent.WorkflowID != "" && workflowID == "" {
					workflowID = enhancedEvent.WorkflowID
					if !automationMode && verbose {
						fmt.Printf("[INFO] Workflow ID: %s\n", workflowID)
					}
				}
				
				switch enhancedEvent.Type {
				case "reconnecting":
					if !automationMode {
						if attempt, ok := enhancedEvent.Data["attempt"].(int); ok {
							if maxAttempts, ok := enhancedEvent.Data["max"].(int); ok {
								fmt.Printf("%s Reconnecting... (attempt %d/%d)\n", 
									style.WarningStyle.Render("🔄"), attempt, maxAttempts)
							}
						}
					}
					
				case "workflow_start":
					if !automationMode {
						fmt.Printf("%s Enhanced workflow started\n", style.InfoStyle.Render("🚀"))
					}
					
				case "step_running":
					if enhancedEvent.Step != nil {
						stepName := enhancedEvent.Step.Name
						stepStartTimes[stepName] = time.Now()
						workflowTrace.StartStep(stepName)
						
						if !automationMode {
							progress := fmt.Sprintf("[%d/%d]", completedSteps+1, stepCount)
							fmt.Printf("%s %s %s\n", 
								style.BulletStyle.Render("▶️"), 
								style.InfoStyle.Render(progress),
								style.ToolNameStyle.Render(stepName))
							fmt.Printf("  %s Running...\n", style.StatusStyle.Render("⏳"))
							
							// Show live logs if available
							if verbose && len(enhancedEvent.Step.Logs) > 0 {
								for _, log := range enhancedEvent.Step.Logs {
									fmt.Printf("  %s %s\n", style.DimStyle.Render("📝"), log)
								}
							}
						}
					}
					
				case "step_complete", "step_update":
					if enhancedEvent.Step != nil {
						stepName := enhancedEvent.Step.Name
						
						// Calculate duration
						var duration time.Duration
						if startTime, ok := stepStartTimes[stepName]; ok {
							duration = time.Since(startTime)
							delete(stepStartTimes, stepName)
						}
						
						if !automationMode {
							durationStr := ""
							if duration > 0 {
								durationStr = fmt.Sprintf(" in %v", duration.Round(time.Millisecond))
							}
							
							fmt.Printf("  %s Step %s%s\n", 
								style.SuccessStyle.Render("✅"), 
								enhancedEvent.Step.Status, durationStr)
							
							// Show step output if available
							if enhancedEvent.Step.Output != "" {
								fmt.Printf("  %s %s\n", 
									style.DimStyle.Render("📤 Output:"),
									style.ToolOutputStyle.Render(formatStepOutput(enhancedEvent.Step.Output)))
							}
							
							// Show variables if verbose and available
							if verbose && enhancedEvent.Step.Variables != nil && len(enhancedEvent.Step.Variables) > 0 {
								fmt.Printf("  %s Variables:\n", style.DimStyle.Render("🔧"))
								for k, v := range enhancedEvent.Step.Variables {
									fmt.Printf("    %s = %v\n", style.KeyStyle.Render(k), v)
								}
							}
							
							// Update progress
							if enhancedEvent.Type == "step_complete" {
								completedSteps++
								progressBar := generateProgressBar(completedSteps, stepCount)
								fmt.Printf("  %s %s %s\n\n", 
									style.SuccessStyle.Render("📊"),
									progressBar,
									style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps, stepCount)))
							}
						}
						
						// Update workflow trace
						workflowTrace.CompleteStep(stepName, enhancedEvent.Step.Status, enhancedEvent.Step.Output)
						if enhancedEvent.Type == "step_complete" {
							completedSteps++
						}
					}
					
				case "step_failed":
					hasError = true
					if enhancedEvent.Step != nil {
						stepName := enhancedEvent.Step.Name
						
						if !automationMode {
							fmt.Printf("  %s Step failed: %s\n", 
								style.ErrorStyle.Render("❌"), 
								enhancedEvent.Step.Error)
							
							if enhancedEvent.Step.CanRetry && workflowID != "" {
								fmt.Printf("  %s This step can be retried with: kubiya workflow retry %s --from-step %s\n",
									style.InfoStyle.Render("💡"), workflowID, stepName)
							}
							
							// Show error logs if verbose
							if verbose && len(enhancedEvent.Step.Logs) > 0 {
								fmt.Printf("  %s Error logs:\n", style.DimStyle.Render("📋"))
								for _, log := range enhancedEvent.Step.Logs {
									fmt.Printf("    %s\n", style.ErrorStyle.Render(log))
								}
							}
						}
						
						// Update workflow trace with error
						workflowTrace.CompleteStep(stepName, "failed", enhancedEvent.Step.Error)
					}
					
				case "workflow_complete":
					success := enhancedEvent.Data != nil && enhancedEvent.Error == nil
					if success {
						workflowTrace.Complete("completed")
						if !automationMode {
							fmt.Printf("%s Enhanced workflow completed successfully!\n", 
								style.SuccessStyle.Render("🎉"))
						}
					} else {
						workflowTrace.Complete("failed")
						if !automationMode {
							fmt.Printf("%s Enhanced workflow execution failed\n", 
								style.ErrorStyle.Render("💥"))
						}
						hasError = true
					}
					
					if !automationMode {
						// Show workflow execution graph
						fmt.Print(workflowTrace.GenerateGraph())
					}
					
					// Save trace to file if requested
					if saveTrace {
						if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
						}
					}
					
					// Show retry information if failed and we have workflow ID
					if hasError && workflowID != "" && !automationMode {
						fmt.Printf("\n%s To retry this workflow, use: kubiya workflow retry %s\n",
							style.InfoStyle.Render("💡"), workflowID)
						fmt.Printf("%s To check status: kubiya workflow status %s\n",
							style.InfoStyle.Render("ℹ️"), workflowID)
					}
					
					return nil
					
				case "error":
					hasError = true
					if enhancedEvent.Error != nil {
						if !automationMode {
							fmt.Printf("%s %s\n", 
								style.ErrorStyle.Render("💥 Error:"), 
								enhancedEvent.Error.Message)
							
							if enhancedEvent.Error.Details != "" && verbose {
								fmt.Printf("  %s %s\n", 
									style.DimStyle.Render("Details:"), 
									enhancedEvent.Error.Details)
							}
							
							// Show retry suggestion if applicable
							if enhancedEvent.Error.Retry && workflowID != "" {
								fmt.Printf("  %s This error can be retried with: kubiya workflow retry %s\n",
									style.InfoStyle.Render("💡"), workflowID)
							}
						}
					}
					
				case "done":
					if verbose && !automationMode {
						fmt.Printf("[VERBOSE] Enhanced stream completed\n")
					}
					goto streamEnd
				}
				
				// Show verbose event details if requested  
				if verbose {
					if enhancedEvent.Error != nil {
						fmt.Printf("[VERBOSE] Enhanced Event: %s, Error: %s\n", enhancedEvent.Type, enhancedEvent.Error.Message)
					} else {
						fmt.Printf("[VERBOSE] Enhanced Event: %s\n", enhancedEvent.Type)
					}
				}
			}
			
		streamEnd:

			if hasError {
				workflowTrace.Complete("failed")
				if !automationMode {
					fmt.Printf("\n%s Workflow execution failed. Check the logs above for details.\n", 
						style.ErrorStyle.Render("💥"))
					fmt.Print(workflowTrace.GenerateGraph())
				}
				
				// Save trace to file if requested
				if saveTrace {
					if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
					}
				}
				
				return fmt.Errorf("workflow execution failed")
			}

			// If we reach here, the stream ended without explicit completion
			// This could mean the workflow completed successfully but didn't send the completion event
			if !automationMode {
				fmt.Printf("\n%s Stream ended - checking workflow status...\n", 
					style.InfoStyle.Render("ℹ️"))
			}
			
			if completedSteps >= stepCount && stepCount > 0 {
				workflowTrace.Complete("completed")
				if !automationMode {
					fmt.Printf("%s Workflow appears to have completed successfully (%d/%d steps)\n", 
						style.SuccessStyle.Render("✅"), completedSteps, stepCount)
				}
			} else {
				workflowTrace.Complete("incomplete")
				if !automationMode {
					fmt.Printf("%s Workflow may be incomplete (%d/%d steps completed)\n", 
						style.WarningStyle.Render("⚠️"), completedSteps, stepCount)
					fmt.Print(workflowTrace.GenerateGraph())
				}
				
				// Save trace to file if requested
				if saveTrace {
					if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
					}
				}
				
				return fmt.Errorf("workflow stream ended unexpectedly")
			}
			
			if !automationMode {
				// Show final workflow execution graph
				fmt.Print(workflowTrace.GenerateGraph())
			}
			
			// Save trace to file if requested
			if saveTrace {
				if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Params in key=value format")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")
	cmd.Flags().BoolVar(&skipPolicyCheck, "skip-policy-check", false, "Skip policy validation before execution")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with detailed SSE logs")
	cmd.Flags().BoolVar(&saveTrace, "save-trace", false, "Save workflow execution trace to JSON file")

	return cmd
}

// parseWorkflowFile parses a workflow file that can be in JSON or YAML format
// It returns either a Workflow struct or a WorkflowExecutionRequest, along with format info
func parseWorkflowFile(filePath string, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read workflow file: %w", err)
	}

	// First, try to determine format by file extension
	isJSON := strings.HasSuffix(strings.ToLower(filePath), ".json")
	isYAML := strings.HasSuffix(strings.ToLower(filePath), ".yaml") || strings.HasSuffix(strings.ToLower(filePath), ".yml")

	// If no clear extension, try to auto-detect format
	if !isJSON && !isYAML {
		// Try JSON first by looking for typical JSON markers
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			isJSON = true
		} else {
			isYAML = true // Default to YAML
		}
	}

	if isJSON {
		return parseJSONWorkflow(data, vars)
	}
	return parseYAMLWorkflow(data, vars)
}

// parseJSONWorkflow parses JSON workflow data
func parseJSONWorkflow(data []byte, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	// Try to parse as Workflow struct first
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err == nil {
		// Successfully parsed as Workflow struct
		return &workflow, nil, "json-workflow", nil
	}

	// Try to parse as WorkflowExecutionRequest
	var req kubiya.WorkflowExecutionRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse JSON as either Workflow or WorkflowExecutionRequest: %w", err)
	}

	// Merge variables into the request
	if req.Params == nil {
		req.Params = vars
	} else {
		for k, v := range vars {
			req.Params[k] = v
		}
	}

	return nil, &req, "json-request", nil
}

// parseYAMLWorkflow parses YAML workflow data
func parseYAMLWorkflow(data []byte, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	// Try to parse as Workflow struct first
	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err == nil && workflow.Name != "" {
		// Successfully parsed as Workflow struct
		return &workflow, nil, "yaml-workflow", nil
	}

	// Try to parse as WorkflowExecutionRequest
	var req kubiya.WorkflowExecutionRequest
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse YAML as either Workflow or WorkflowExecutionRequest: %w", err)
	}

	// Merge variables into the request
	if req.Params == nil {
		req.Params = vars
	} else {
		for k, v := range vars {
			req.Params[k] = v
		}
	}

	return nil, &req, "yaml-request", nil
}

// buildExecutionRequest converts a Workflow to WorkflowExecutionRequest
func buildExecutionRequest(workflow Workflow, vars map[string]interface{}, runner string) kubiya.WorkflowExecutionRequest {
	// Convert WorkflowStep to interface{} for the API
	steps := make([]interface{}, len(workflow.Steps))
	for i, step := range workflow.Steps {
		stepMap := map[string]interface{}{
			"name": step.Name,
		}
		if step.Description != "" {
			stepMap["description"] = step.Description
		}
		if step.Command != "" {
			stepMap["command"] = step.Command
		}
		if step.Executor.Type != "" {
			stepMap["executor"] = map[string]interface{}{
				"type":   step.Executor.Type,
				"config": step.Executor.Config,
			}
		}
		if step.Output != "" {
			stepMap["output"] = step.Output
		}
		if len(step.Depends) > 0 {
			stepMap["depends"] = step.Depends
		}
		steps[i] = stepMap
	}

	return kubiya.WorkflowExecutionRequest{
		Command:     "execute_workflow",
		Name:        workflow.Name,
		Description: fmt.Sprintf("Execution of %s", workflow.Name),
		Steps:       steps,
		Params:   vars,
	}
}

// generateProgressBar creates a visual progress bar for workflow execution
func generateProgressBar(completed, total int) string {
	if total == 0 {
		return "[-]"
	}
	
	barLength := 20
	completedLength := (completed * barLength) / total
	
	bar := "["
	for i := 0; i < barLength; i++ {
		if i < completedLength {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	
	return bar
}

// formatStepOutput formats step output for better display
func formatStepOutput(output string) string {
	// Limit output length for readability
	maxLength := 500
	if len(output) > maxLength {
		return output[:maxLength] + "... (truncated)"
	}
	
	// Clean up common escape sequences and whitespace
	formatted := strings.TrimSpace(output)
	formatted = strings.ReplaceAll(formatted, "\\n", "\n")
	formatted = strings.ReplaceAll(formatted, "\\t", "\t")
	
	// If it looks like JSON, try to format it
	if strings.HasPrefix(formatted, "{") && strings.HasSuffix(formatted, "}") {
		var jsonObj interface{}
		if err := json.Unmarshal([]byte(formatted), &jsonObj); err == nil {
			if prettyBytes, err := json.MarshalIndent(jsonObj, "    ", "  "); err == nil {
				return string(prettyBytes)
			}
		}
	}
	
	return formatted
}

// WorkflowTrace tracks the execution of a workflow for visualization
type WorkflowTrace struct {
	Name       string        `json:"name"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	Duration   time.Duration `json:"duration"`
	TotalSteps int           `json:"total_steps"`
	Steps      []StepTrace   `json:"steps"`
	Status     string        `json:"status"` // "running", "completed", "failed"
}

// StepTrace tracks the execution of a single step
type StepTrace struct {
	Name        string        `json:"name"`
	StartTime   *time.Time    `json:"start_time,omitempty"`
	EndTime     *time.Time    `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration"`
	Status      string        `json:"status"` // "pending", "running", "completed", "failed"
	Output      string        `json:"output,omitempty"`
	OutputVars  map[string]interface{} `json:"output_vars,omitempty"`
	Description string        `json:"description,omitempty"`
}

// NewWorkflowTrace creates a new workflow trace
func NewWorkflowTrace(name string, totalSteps int) *WorkflowTrace {
	return &WorkflowTrace{
		Name:       name,
		StartTime:  time.Now(),
		TotalSteps: totalSteps,
		Steps:      make([]StepTrace, 0, totalSteps),
		Status:     "running",
	}
}

// AddStep adds a step to the workflow trace
func (wt *WorkflowTrace) AddStep(name, description string) {
	step := StepTrace{
		Name:        name,
		Status:      "pending", 
		Description: description,
		OutputVars:  make(map[string]interface{}),
	}
	wt.Steps = append(wt.Steps, step)
}

// StartStep marks a step as started
func (wt *WorkflowTrace) StartStep(name string) {
	for i := range wt.Steps {
		if wt.Steps[i].Name == name {
			now := time.Now()
			wt.Steps[i].StartTime = &now
			wt.Steps[i].Status = "running"
			return
		}
	}
	// If step doesn't exist, add it
	now := time.Now()
	step := StepTrace{
		Name:       name,
		StartTime:  &now,
		Status:     "running",
		OutputVars: make(map[string]interface{}),
	}
	wt.Steps = append(wt.Steps, step)
}

// CompleteStep marks a step as completed
func (wt *WorkflowTrace) CompleteStep(name, status, output string) {
	for i := range wt.Steps {
		if wt.Steps[i].Name == name {
			now := time.Now()
			wt.Steps[i].EndTime = &now
			wt.Steps[i].Status = status
			wt.Steps[i].Output = output
			
			if wt.Steps[i].StartTime != nil {
				wt.Steps[i].Duration = now.Sub(*wt.Steps[i].StartTime)
			}
			return
		}
	}
}

// Complete marks the workflow as completed
func (wt *WorkflowTrace) Complete(status string) {
	now := time.Now()
	wt.EndTime = &now
	wt.Duration = now.Sub(wt.StartTime)
	wt.Status = status
}

// GenerateGraph creates a visual representation of the workflow execution
func (wt *WorkflowTrace) GenerateGraph() string {
	var graph strings.Builder
	
	graph.WriteString(fmt.Sprintf("\n%s Workflow Execution Graph\n", 
		style.HeaderStyle.Render("📊")))
	graph.WriteString(fmt.Sprintf("%s %s\n", 
		style.DimStyle.Render("Name:"), wt.Name))
	graph.WriteString(fmt.Sprintf("%s %s\n", 
		style.DimStyle.Render("Status:"), getStatusEmoji(wt.Status)))
	
	if wt.EndTime != nil {
		graph.WriteString(fmt.Sprintf("%s %v\n", 
			style.DimStyle.Render("Duration:"), wt.Duration.Round(time.Millisecond)))
	}
	
	graph.WriteString("\n")
	
	// Generate step graph
	for i, step := range wt.Steps {
		// Step connector
		if i == 0 {
			graph.WriteString("┌─")
		} else {
			graph.WriteString("├─")
		}
		
		// Step info
		statusEmoji := getStatusEmoji(step.Status)
		graph.WriteString(fmt.Sprintf(" %s %s", statusEmoji, step.Name))
		
		if step.Duration > 0 {
			graph.WriteString(fmt.Sprintf(" (%v)", step.Duration.Round(time.Millisecond)))
		}
		graph.WriteString("\n")
		
		// Show output if available and not too long
		if step.Output != "" && len(step.Output) < 100 {
			if i == len(wt.Steps)-1 {
				graph.WriteString("  └─ 📤 ")
			} else {
				graph.WriteString("│ └─ 📤 ")
			}
			graph.WriteString(style.DimStyle.Render(step.Output))
			graph.WriteString("\n")
		}
	}
	
	return graph.String()
}

// getStatusEmoji returns an emoji for the given status
func getStatusEmoji(status string) string {
	switch status {
	case "pending":
		return "⏳ Pending"
	case "running":
		return "🔄 Running"
	case "completed", "finished":
		return "✅ Completed"
	case "failed":
		return "❌ Failed"
	default:
		return "❓ " + status
	}
}

// saveWorkflowTrace saves the workflow trace to a JSON file
func saveWorkflowTrace(trace *WorkflowTrace, workflowFile string) error {
	// Generate trace filename based on workflow file and timestamp
	baseFilename := strings.TrimSuffix(workflowFile, filepath.Ext(workflowFile))
	timestamp := trace.StartTime.Format("20060102-150405")
	traceFilename := fmt.Sprintf("%s-trace-%s.json", baseFilename, timestamp)
	
	// Marshal trace to JSON
	traceData, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(traceFilename, traceData, 0644); err != nil {
		return fmt.Errorf("failed to write trace file: %w", err)
	}
	
	fmt.Printf("\n%s Workflow trace saved to: %s\n", 
		style.InfoStyle.Render("💾"), traceFilename)
	
	return nil
}

// processRegularWorkflowEvents handles the original workflow event processing for fallback
func processRegularWorkflowEvents(ctx context.Context, events <-chan kubiya.WorkflowSSEEvent, automationMode, verbose, saveTrace bool, workflowTrace *WorkflowTrace, workflowFile string, req kubiya.WorkflowExecutionRequest, stepStartTimes map[string]time.Time, completedSteps, stepCount int) error {
	var hasError bool
	
	if !automationMode {
		fmt.Printf("\n%s Starting workflow execution...\n\n", 
			style.InfoStyle.Render("🚀"))
		
		// Show initial progress
		progressBar := generateProgressBar(completedSteps, stepCount)
		fmt.Printf("%s %s %s\n\n", 
			style.InfoStyle.Render("📊"),
			progressBar,
			style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps, stepCount)))
	}
	
	for event := range events {
		if event.Type == "data" {
			// Parse JSON data for workflow events
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &jsonData); err == nil {
				if eventType, ok := jsonData["type"].(string); ok {
					switch eventType {
					case "step_running":
						if step, ok := jsonData["step"].(map[string]interface{}); ok {
							if stepName, ok := step["name"].(string); ok {
								stepStartTimes[stepName] = time.Now()
								
								// Update workflow trace
								workflowTrace.StartStep(stepName)
								
								if !automationMode {
									// Show step starting
									progress := fmt.Sprintf("[%d/%d]", completedSteps+1, stepCount)
									fmt.Printf("%s %s %s\n", 
										style.BulletStyle.Render("▶️"), 
										style.InfoStyle.Render(progress),
										style.ToolNameStyle.Render(stepName))
									fmt.Printf("  %s Running...\n", style.StatusStyle.Render("⏳"))
								}
							}
						}
						
					case "step_complete":
						if step, ok := jsonData["step"].(map[string]interface{}); ok {
							if stepName, ok := step["name"].(string); ok {
								// Calculate duration
								var duration time.Duration
								if startTime, ok := stepStartTimes[stepName]; ok {
									duration = time.Since(startTime)
									delete(stepStartTimes, stepName)
								}
								
								// Extract step output if available
								var stepOutput string
								var stepStatus string = "finished"
								if output, ok := step["output"].(string); ok && output != "" {
									stepOutput = output
								}
								if status, ok := step["status"].(string); ok {
									stepStatus = status
								}
								
								if !automationMode {
									// Show step completion with status
									if duration > 0 {
										fmt.Printf("  %s Step %s in %v\n", 
											style.SuccessStyle.Render("✅"), 
											stepStatus,
											duration.Round(time.Millisecond))
									} else {
										fmt.Printf("  %s Step %s\n", 
											style.SuccessStyle.Render("✅"),
											stepStatus)
									}
									
									// Show step output if available
									if stepOutput != "" {
										// Format output nicely
										fmt.Printf("  %s %s\n", 
											style.DimStyle.Render("📤 Output:"),
											style.ToolOutputStyle.Render(formatStepOutput(stepOutput)))
									}
									
									// Update progress
									progressBar := generateProgressBar(completedSteps+1, stepCount)
									fmt.Printf("  %s %s %s\n\n", 
										style.SuccessStyle.Render("📊"),
										progressBar,
										style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps+1, stepCount)))
								}
								
								// Update workflow trace
								workflowTrace.CompleteStep(stepName, stepStatus, stepOutput)
								
								// Update progress
								completedSteps++
							}
						}
						
					case "workflow_complete":
						// Workflow finished
						if success, ok := jsonData["success"].(bool); ok && success {
							workflowTrace.Complete("completed")
							if !automationMode {
								fmt.Printf("%s Workflow completed successfully!\n", 
									style.SuccessStyle.Render("🎉"))
							}
						} else {
							workflowTrace.Complete("failed")
							if !automationMode {
								fmt.Printf("%s Workflow execution failed\n", 
									style.ErrorStyle.Render("💥"))
							}
							hasError = true
						}
						
						if !automationMode {
							// Show workflow execution graph
							fmt.Print(workflowTrace.GenerateGraph())
						}
						
						// Save trace to file if requested
						if saveTrace {
							if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
								fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
							}
						}
						
						return nil
					}
				}
			}
		} else if event.Type == "error" {
			if !automationMode {
				fmt.Printf("%s %s\n", 
					style.ErrorStyle.Render("💀 Error:"), 
					event.Data)
			}
			hasError = true
		} else if event.Type == "done" {
			// Stream ended
			break
		}
		
		// Show verbose SSE details if requested  
		if verbose {
			fmt.Printf("[VERBOSE] Event: %s, Data: %s\n", event.Type, event.Data)
		}
	}

	if hasError {
		workflowTrace.Complete("failed")
		if !automationMode {
			fmt.Printf("\n%s Workflow execution failed. Check the logs above for details.\n", 
				style.ErrorStyle.Render("💥"))
			fmt.Print(workflowTrace.GenerateGraph())
		}
		
		// Save trace to file if requested
		if saveTrace {
			if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
			}
		}
		
		return fmt.Errorf("workflow execution failed")
	}

	// If we reach here, the stream ended without explicit completion
	if !automationMode {
		fmt.Printf("\n%s Stream ended - checking workflow status...\n", 
			style.InfoStyle.Render("ℹ️"))
	}
	
	if completedSteps >= stepCount && stepCount > 0 {
		workflowTrace.Complete("completed")
		if !automationMode {
			fmt.Printf("%s Workflow appears to have completed successfully (%d/%d steps)\n", 
				style.SuccessStyle.Render("✅"), completedSteps, stepCount)
		}
	} else {
		workflowTrace.Complete("incomplete")
		if !automationMode {
			fmt.Printf("%s Workflow may be incomplete (%d/%d steps completed)\n", 
				style.WarningStyle.Render("⚠️"), completedSteps, stepCount)
			fmt.Print(workflowTrace.GenerateGraph())
		}
		
		// Save trace to file if requested
		if saveTrace {
			if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
			}
		}
		
		return fmt.Errorf("workflow stream ended unexpectedly")
	}
	
	if !automationMode {
		// Show final workflow execution graph
		fmt.Print(workflowTrace.GenerateGraph())
	}
	
	// Save trace to file if requested
	if saveTrace {
		if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
		}
	}

	return nil
}
