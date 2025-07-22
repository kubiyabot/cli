package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Helper function for min values
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	errorStr := strings.ToLower(err.Error())

	// Don't retry these errors - they're permanent
	nonRetryableErrors := []string{
		"authentication failed",
		"access forbidden",
		"unauthorized",
		"permission denied",
		"invalid api key",
		"rate limit exceeded",
		"quota exceeded",
		"payment required",
		"agent not found",
		"bad request",
		"invalid request",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if strings.Contains(errorStr, nonRetryable) {
			return false
		}
	}

	// Retry these errors - they're typically transient
	retryableErrors := []string{
		"stream_error",
		"connection",
		"timeout",
		"network",
		"socket",
		"eof",
		"reset",
		"refused",
		"unavailable",
		"internal server error",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"temporary",
		"transient",
		"dial",
		"host",
		"dns",
		"tls",
		"ssl",
		"handshake",
		"broken pipe",
		"i/o timeout",
		"no route to host",
		"connection reset by peer",
		"connection aborted",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errorStr, retryable) {
			return true
		}
	}

	// Default to retrying unknown errors (conservative approach)
	return true
}

// isAgentErrorMessage detects if the agent's response indicates an internal error that should trigger session recovery
func isAgentErrorMessage(content string) bool {
	if content == "" {
		return false
	}

	lowerContent := strings.ToLower(content)

	// Common agent apology/error patterns that indicate the agent had an internal issue
	agentErrorPatterns := []string{
		"sorry, i had an issue",
		"sorry, i encountered an issue",
		"sorry, there was an issue",
		"sorry, something went wrong",
		"apologize, i had a problem",
		"apologize, there was a problem",
		"i'm having trouble",
		"i'm experiencing difficulties",
		"internal error occurred",
		"something went wrong on my end",
		"i encountered an unexpected error",
		"sorry for the inconvenience",
		"unable to process your request at this time",
		"experiencing technical difficulties",
		"temporary issue preventing me",
		"let me try that again",
		"please try your request again",
		"i need to restart",
		"let me reset and try again",
		"i'm having connectivity issues",
		"stream interrupted",
		"connection lost",
		"processing error",
	}

	for _, pattern := range agentErrorPatterns {
		if strings.Contains(lowerContent, pattern) {
			return true
		}
	}

	// Also check for generic "sorry" combined with error indicators
	if strings.Contains(lowerContent, "sorry") && 
		(strings.Contains(lowerContent, "error") || 
		 strings.Contains(lowerContent, "problem") || 
		 strings.Contains(lowerContent, "issue") || 
		 strings.Contains(lowerContent, "failed") ||
		 strings.Contains(lowerContent, "unable") ||
		 strings.Contains(lowerContent, "couldn't") ||
		 strings.Contains(lowerContent, "can't")) {
		return true
	}

	return false
}

// calculateBackoffDelay calculates exponential backoff with jitter
func calculateBackoffDelay(attempt int) time.Duration {
	// Base delay starts at 1 second
	baseDelay := time.Second

	// Exponential backoff: min(baseDelay * 2^attempt, maxDelay)
	maxDelay := 30 * time.Second
	delay := time.Duration(1<<uint(attempt)) * baseDelay

	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter (±25% random variation)
	jitter := time.Duration(float64(delay) * 0.25 * (2*rand.Float64() - 1))
	return delay + jitter
}

// formatToolParameters intelligently formats tool parameters for display
func formatToolParameters(toolArgs string) string {
	if !json.Valid([]byte(toolArgs)) {
		return "" // Skip invalid JSON
	}

	var argMap map[string]interface{}
	if json.Unmarshal([]byte(toolArgs), &argMap) != nil {
		return "" // Skip if can't parse
	}

	if len(argMap) == 0 {
		return "" // Skip empty parameters
	}

	var formatted []string
	
	for k, v := range argMap {
		valueStr := fmt.Sprintf("%v", v)

		// Clean up encoding
		valueStr = strings.ReplaceAll(valueStr, "\\n", "\n")
		valueStr = strings.ReplaceAll(valueStr, "\\t", "\t")
		valueStr = strings.ReplaceAll(valueStr, "\\\"", "\"")
		valueStr = strings.ReplaceAll(valueStr, "\\u003c", "<")
		valueStr = strings.ReplaceAll(valueStr, "\\u003e", ">")

		// Smart formatting based on content type
		if isCode(valueStr) {
			// Code content - show first line with context
			lines := strings.Split(valueStr, "\n")
			firstLine := strings.TrimSpace(lines[0])
			if len(firstLine) > 50 {
				firstLine = firstLine[:50] + "..."
			}
			if len(lines) > 3 {
				formatted = append(formatted, fmt.Sprintf("%s: %s ... (%d lines)",
					style.KeyStyle.Render(k),
					style.ValueStyle.Render(firstLine),
					len(lines)))
			} else if len(lines) > 1 {
				formatted = append(formatted, fmt.Sprintf("%s: %s ... (%d lines)",
					style.KeyStyle.Render(k),
					style.ValueStyle.Render(firstLine),
					len(lines)))
			} else {
				formatted = append(formatted, fmt.Sprintf("%s: %s",
					style.KeyStyle.Render(k),
					style.ValueStyle.Render(firstLine)))
			}
		} else if len(valueStr) > 60 {
			// Long content - truncate smartly with better preview
			truncated := valueStr[:60] + "..."
			formatted = append(formatted, fmt.Sprintf("%s: %s",
				style.KeyStyle.Render(k),
				style.ValueStyle.Render(truncated)))
		} else {
			// Short content - show as-is
			formatted = append(formatted, fmt.Sprintf("%s: %s",
				style.KeyStyle.Render(k),
				style.ValueStyle.Render(valueStr)))
		}
	}

	if len(formatted) == 0 {
		return ""
	}

	return strings.Join(formatted, ", ")
}

// formatLiveJSON formats JSON as it's being built for live display
func formatLiveJSON(jsonStr string) string {
	if jsonStr == "" {
		return ""
	}
	
	// Clean up common escape sequences for better display
	cleaned := strings.ReplaceAll(jsonStr, "\\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\\\"", `"`)
	cleaned = strings.ReplaceAll(cleaned, "\\t", " ")
	
	// Remove extra whitespace but preserve structure while making it more compact
	cleaned = strings.TrimSpace(cleaned)
	
	// For very long content, show a more intelligent truncation
	if len(cleaned) > 120 {
		// Try to find a good break point (after comma, before closing brace)
		if commaIdx := strings.LastIndex(cleaned[:100], ","); commaIdx > 50 {
			cleaned = cleaned[:commaIdx] + ", ..."
		} else {
			cleaned = cleaned[:117] + "..."
		}
	}
	
	return cleaned
}

// Global map to track working animations by tool name
var workingAnimations = make(map[string]chan bool)
var workingAnimationsMu sync.Mutex

// showWorkingAnimation displays rotating work messages while AI is thinking
func showWorkingAnimation(toolName string) {
	// Create stop channel for this animation
	workingAnimationsMu.Lock()
	stopChan := make(chan bool, 1)
	workingAnimations[toolName] = stopChan
	workingAnimationsMu.Unlock()
	
	defer func() {
		workingAnimationsMu.Lock()
		delete(workingAnimations, toolName)
		workingAnimationsMu.Unlock()
	}()
	
	workingMessages := []string{
		"Analyzing request...",
		"Planning approach...", 
		"Generating parameters...",
		"Optimizing strategy...",
		"Building context...",
		"Processing input...",
		"Thinking...",
		"Computing...",
		"Preparing...",
	}
	
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	messageIndex := 0
	spinnerIndex := 0
	
	// Show initial state immediately
	fmt.Printf("\r⚡ %s %s %s",
		style.ToolExecutingStyle.Render(toolName),
		style.SpinnerStyle.Render(spinners[spinnerIndex]),
		style.DimStyle.Render(workingMessages[messageIndex]))
	os.Stdout.Sync()
	
	ticker := time.NewTicker(500 * time.Millisecond) // Change message every 500ms
	spinnerTicker := time.NewTicker(80 * time.Millisecond) // Spin every 80ms (faster for more dynamic feel)
	
	defer ticker.Stop()
	defer spinnerTicker.Stop()
	
	for {
		select {
		case <-stopChan:
			return // Stop the animation
		case <-ticker.C:
			// Rotate through working messages
			messageIndex = (messageIndex + 1) % len(workingMessages)
			fmt.Printf("\r⚡ %s %s %s                    ", // Add spaces to clear any leftover text
				style.ToolExecutingStyle.Render(toolName),
				style.SpinnerStyle.Render(spinners[spinnerIndex]),
				style.DimStyle.Render(workingMessages[messageIndex]))
			os.Stdout.Sync()
			
		case <-spinnerTicker.C:
			// Rotate spinner
			spinnerIndex = (spinnerIndex + 1) % len(spinners)
			fmt.Printf("\r⚡ %s %s %s                    ", // Add spaces to clear any leftover text
				style.ToolExecutingStyle.Render(toolName),
				style.SpinnerStyle.Render(spinners[spinnerIndex]),
				style.DimStyle.Render(workingMessages[messageIndex]))
			os.Stdout.Sync()
		}
	}
}

// stopWorkingAnimation stops the working animation for a specific tool
func stopWorkingAnimation(toolName string) {
	workingAnimationsMu.Lock()
	defer workingAnimationsMu.Unlock()
	
	if stopChan, exists := workingAnimations[toolName]; exists {
		select {
		case stopChan <- true:
		default:
		}
	}
}

// isAnimationRunning checks if a working animation is currently running for a tool
func isAnimationRunning(toolName string) bool {
	workingAnimationsMu.Lock()
	defer workingAnimationsMu.Unlock()
	
	_, exists := workingAnimations[toolName]
	return exists
}

// isCode detects if content looks like code
func isCode(content string) bool {
	// Basic heuristics for code detection
	codeIndicators := []string{
		"import ", "from ", "def ", "class ", "function", "var ", "const ",
		"#!/", "<?php", "<script", "SELECT ", "INSERT ", "UPDATE ",
		".py", ".js", ".sql", ".sh", ".rb", ".go", ".java",
		"console.log", "print(", "fmt.Printf", "System.out",
	}

	lowerContent := strings.ToLower(content)
	for _, indicator := range codeIndicators {
		if strings.Contains(lowerContent, strings.ToLower(indicator)) {
			return true
		}
	}

	// Check for common code patterns
	if strings.Count(content, "\n") > 2 && (strings.Contains(content, "{") || strings.Contains(content, ":")) {
		return true
	}

	return false
}

// Clean, minimal loading animation for tool execution
func startToolAnimation(te *toolExecution) {
	if te.status != "running" {
		return
	}

	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := 0

	for te.status == "running" && !te.isComplete {
		// Simple, clean spinner without extra text
		fmt.Printf("\r   %s %s",
			style.SpinnerStyle.Render(spinners[spinnerIndex]),
			style.DimStyle.Render("Running..."))
		os.Stdout.Sync()

		spinnerIndex = (spinnerIndex + 1) % len(spinners)
		time.Sleep(150 * time.Millisecond)
	}

	// Clean up the animation line completely
	fmt.Printf("\r%s\r", strings.Repeat(" ", 50))
}

// Add this type definition at the package level
type toolExecution struct {
	name             string
	args             string
	output           strings.Builder
	errorMsg         string // Store actual error message
	hasOutput        bool
	hasShownOutput   bool // Track if we've shown initial output message
	hasShownError    bool // Track if we've shown error message
	animationStarted bool // Track if animation is running
	isComplete       bool
	msgID            string
	failed           bool
	status           string // "waiting", "running", "done", "failed"
	startTime        time.Time
	runner           string
	toolCallId       string
	outputTruncated  bool
}

// Add connection status tracking
type connectionStatus struct {
	runner      string
	runnerType  string // "k8s", "docker", "local", etc.
	connected   bool
	connectTime time.Time
	lastPing    time.Time
	latency     time.Duration
}

// Add tool call statistics
type toolCallStats struct {
	totalCalls     int
	activeCalls    int
	completedCalls int
	failedCalls    int
	toolTypes      map[string]int
	mu             sync.RWMutex
}

// Add a buffer for chat messages
type chatBuffer struct {
	content     string
	sentence    strings.Builder
	inCodeBlock bool
	codeBlock   strings.Builder
}

// Add status emojis
const (
	statusWaiting = "⏳" // Tool is queued
	statusRunning = "🔄" // Tool is running
	statusDone    = "✅" // Tool completed successfully
	statusFailed  = "❌" // Tool failed
)

func newChatCommand(cfg *config.Config) *cobra.Command {
	var (
		agentID      string
		agentName    string
		message      string
		promptFile   string   // New flag for prompt file
		templateVars []string // New flag for Go template variables
		noClassify   bool

		interactive     bool
		debug           bool
		stream          bool
		clearSession    bool
		sessionID       string
		contextFiles    []string
		stdinInput      bool
		sourceTest      bool
		sourceUUID      string
		sourceName      string
		suggestTool     string
		permissionLevel string

		showToolCalls bool
		retries       int
		silent        bool // New flag for automation mode

		// Inline agent flags
		inline         bool
		agentSpec      string // New flag for agent specification file/URL
		toolsFile      string
		toolsJSON      string
		aiInstructions string
		description    string
		runners        []string
		integrations   []string
		secrets        []string
		envVars        []string
		llmModel       string
		isDebugMode    bool
	)

	// Helper function to validate and normalize URLs
	validateURL := func(rawURL string) (string, error) {
		// Handle common GitHub URL patterns and convert to raw URLs
		if strings.Contains(rawURL, "github.com") {
			// Convert github.com/user/repo/blob/branch/path to raw.githubusercontent.com/user/repo/branch/path
			githubBlobRegex := regexp.MustCompile(`https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)`)
			if matches := githubBlobRegex.FindStringSubmatch(rawURL); len(matches) == 5 {
				user, repo, branch, path := matches[1], matches[2], matches[3], matches[4]
				rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", user, repo, branch, path)
				if debug {
					fmt.Printf("🔗 Converted GitHub blob URL to raw URL: %s\n", rawURL)
				}
			}

			// Convert github.com/user/repo/tree/branch/path to raw format for directory listing
			githubTreeRegex := regexp.MustCompile(`https?://github\.com/([^/]+)/([^/]+)/tree/([^/]+)/?(.*)`)
			if matches := githubTreeRegex.FindStringSubmatch(rawURL); len(matches) >= 4 {
				return "", fmt.Errorf("cannot fetch directory URLs directly. Please specify a file URL or use a raw file URL")
			}
		}

		// Validate URL format
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return "", fmt.Errorf("invalid URL format: %w", err)
		}

		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return "", fmt.Errorf("only HTTP and HTTPS URLs are supported")
		}

		return rawURL, nil
	}

	// Helper function to create cache directory and get cache file path
	getCacheFilePath := func(rawURL string) (string, error) {
		// Create cache directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		cacheDir := filepath.Join(homeDir, ".kubiya", "cache", "prompt-files")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create cache directory: %w", err)
		}

		// Create cache file name from URL hash
		hasher := sha256.New()
		hasher.Write([]byte(rawURL))
		hash := fmt.Sprintf("%x", hasher.Sum(nil))[:16] // Use first 16 chars of hash

		// Extract file extension from URL
		parsedURL, _ := url.Parse(rawURL)
		ext := filepath.Ext(parsedURL.Path)
		if ext == "" {
			ext = ".txt" // Default extension
		}

		cacheFile := filepath.Join(cacheDir, hash+ext)
		return cacheFile, nil
	}

	// Enhanced helper function to fetch content from URL with caching and validation
	fetchURL := func(rawURL string) (string, error) {
		// Validate and normalize URL
		validURL, err := validateURL(rawURL)
		if err != nil {
			return "", err
		}

		// Check cache first
		cacheFile, err := getCacheFilePath(validURL)
		if err != nil {
			if debug {
				fmt.Printf("⚠️ Cache setup failed: %v\n", err)
			}
		} else {
			// Check if cache file exists and is recent (less than 1 hour old)
			if info, err := os.Stat(cacheFile); err == nil {
				if time.Since(info.ModTime()) < time.Hour {
					if debug {
						fmt.Printf("📁 Using cached content from: %s\n", cacheFile)
					}
					content, err := os.ReadFile(cacheFile)
					if err == nil {
						return string(content), nil
					}
				}
			}
		}

		if debug {
			fmt.Printf("🌐 Fetching content from URL: %s\n", validURL)
		}

		// Create HTTP client with timeout and headers
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		req, err := http.NewRequest("GET", validURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		// Set user agent
		req.Header.Set("User-Agent", "Kubiya-CLI/1.0")

		// Add GitHub token if available for private repos
		if strings.Contains(validURL, "githubusercontent.com") {
			if token := os.Getenv("GITHUB_TOKEN"); token != "" {
				req.Header.Set("Authorization", "token "+token)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to fetch URL %s: %w", validURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return "", fmt.Errorf("file not found at URL %s (404). Please verify the URL and ensure it points to a raw file", validURL)
		} else if resp.StatusCode == 403 {
			return "", fmt.Errorf("access denied to URL %s (403). For private GitHub repos, set GITHUB_TOKEN environment variable", validURL)
		} else if resp.StatusCode != 200 {
			return "", fmt.Errorf("failed to fetch URL %s: HTTP %d", validURL, resp.StatusCode)
		}

		// Check content type
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && !strings.Contains(contentType, "text/") && !strings.Contains(contentType, "application/json") {
			if debug {
				fmt.Printf("⚠️ Warning: Content-Type is %s, expected text content\n", contentType)
			}
		}

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}

		contentStr := string(content)

		// Validate content is not empty
		if strings.TrimSpace(contentStr) == "" {
			return "", fmt.Errorf("fetched content is empty from URL: %s", validURL)
		}

		// Cache the content if cache is available
		if cacheFile != "" {
			if err := os.WriteFile(cacheFile, content, 0644); err != nil {
				if debug {
					fmt.Printf("⚠️ Failed to cache content: %v\n", err)
				}
			} else if debug {
				fmt.Printf("📁 Cached content to: %s\n", cacheFile)
			}
		}

		if debug {
			fmt.Printf("✅ Successfully fetched %d bytes from URL\n", len(content))
		}

		return contentStr, nil
	}

	// Helper function to expand wildcards and read files
	expandAndReadFiles := func(patterns []string) (map[string]string, error) {
		context := make(map[string]string)
		for _, pattern := range patterns {
			// Handle URLs
			if strings.HasPrefix(pattern, "http://") || strings.HasPrefix(pattern, "https://") {
				content, err := fetchURL(pattern)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch URL %s: %w", pattern, err)
				}
				context[pattern] = content
				continue
			}

			// Handle file patterns
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
			}

			if len(matches) == 0 {
				return nil, fmt.Errorf("no files match pattern: %s", pattern)
			}

			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil {
					return nil, fmt.Errorf("failed to stat file %s: %w", match, err)
				}

				// Skip directories
				if info.IsDir() {
					continue
				}

				content, err := os.ReadFile(match)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", match, err)
				}
				context[match] = string(content)
			}
		}
		return context, nil
	}

	// Helper function to parse tools from JSON
	parseTools := func(toolsJSON string) ([]kubiya.Tool, error) {
		var tools []kubiya.Tool
		if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
			return nil, fmt.Errorf("failed to parse tools JSON: %w", err)
		}
		return tools, nil
	}

	// Helper function to validate tool definition
	validateTool := func(tool kubiya.Tool) error {
		if tool.Name == "" {
			return fmt.Errorf("tool name is required")
		}
		if tool.Description == "" {
			return fmt.Errorf("tool description is required")
		}
		if tool.Content == "" {
			return fmt.Errorf("tool content is required")
		}
		// Validate required args
		for _, arg := range tool.Args {
			if arg.Name == "" {
				return fmt.Errorf("tool arg name is required")
			}
			if arg.Description == "" {
				return fmt.Errorf("tool arg description is required for arg: %s", arg.Name)
			}
		}
		return nil
	}

	// Helper function to load tools from file or URL

	// Helper function to discover tools.json file in the same directory as agent spec
	discoverToolsFile := func(agentSpecURL string) (string, error) {
		if !strings.HasPrefix(agentSpecURL, "http://") && !strings.HasPrefix(agentSpecURL, "https://") {
			// For local files, look in same directory
			dir := filepath.Dir(agentSpecURL)
			toolsPath := filepath.Join(dir, "tools.json")
			if _, err := os.Stat(toolsPath); err == nil {
				if debug {
					fmt.Printf("🔍 Discovered local tools file: %s\n", toolsPath)
				}
				return toolsPath, nil
			}
			return "", nil
		}

		// For URLs, construct tools.json URL in same directory
		parsedURL, err := url.Parse(agentSpecURL)
		if err != nil {
			return "", err
		}

		// Get directory path from URL
		dir := filepath.Dir(parsedURL.Path)
		if dir == "." {
			dir = ""
		}
		toolsPath := filepath.Join(dir, "tools.json")

		// Construct tools.json URL
		toolsURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, toolsPath)

		if debug {
			fmt.Printf("🔍 Checking for tools file at: %s\n", toolsURL)
		}

		// Try to fetch it with a quick HEAD request to avoid downloading if it doesn't exist
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("HEAD", toolsURL, nil)
		if err != nil {
			return "", nil // Not critical, just return no tools file found
		}

		// Add GitHub token if available
		if strings.Contains(toolsURL, "githubusercontent.com") {
			if token := os.Getenv("GITHUB_TOKEN"); token != "" {
				req.Header.Set("Authorization", "token "+token)
			}
		}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return "", nil // Tools file doesn't exist, that's okay
		}
		resp.Body.Close()

		if debug {
			fmt.Printf("✅ Found tools file at: %s\n", toolsURL)
		}

		return toolsURL, nil
	}

	// Helper function to escape shell strings
	shellescape := func(s string) string {
		// Replace single quotes with '\'' and wrap in single quotes
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	// Helper function to parse template variables from --var flags
	parseTemplateVars := func(vars []string) (map[string]interface{}, error) {
		templateData := make(map[string]interface{})
		for _, v := range vars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid template variable format: %s (expected KEY=VALUE)", v)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Try to parse value as different types
			if value == "true" || value == "false" {
				templateData[key] = value == "true"
			} else if num, err := strconv.Atoi(value); err == nil {
				templateData[key] = num
			} else if num, err := strconv.ParseFloat(value, 64); err == nil {
				templateData[key] = num
			} else {
				// Remove quotes if present
				if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
					(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
					value = value[1 : len(value)-1]
				}
				templateData[key] = value
			}
		}
		return templateData, nil
	}

	// Helper function to create template function map with useful functions
	createTemplateFuncMap := func() template.FuncMap {
		return template.FuncMap{
			// String functions
			"upper":     strings.ToUpper,
			"lower":     strings.ToLower,
			"title":     strings.Title,
			"trim":      strings.TrimSpace,
			"replace":   strings.ReplaceAll,
			"contains":  strings.Contains,
			"hasPrefix": strings.HasPrefix,
			"hasSuffix": strings.HasSuffix,
			"split":     strings.Split,
			"join":      strings.Join,

			// Utility functions
			"default": func(defaultVal interface{}, val interface{}) interface{} {
				if val == nil || val == "" {
					return defaultVal
				}
				return val
			},
			"env": os.Getenv,
			"now": time.Now,
			"date": func(format string) string {
				return time.Now().Format(format)
			},
			"formatTime": func(format string, t time.Time) string {
				return t.Format(format)
			},

			// Math functions
			"add": func(a, b int) int { return a + b },
			"sub": func(a, b int) int { return a - b },
			"mul": func(a, b int) int { return a * b },
			"div": func(a, b int) int {
				if b == 0 {
					return 0
				}
				return a / b
			},
			"mod": func(a, b int) int {
				if b == 0 {
					return 0
				}
				return a % b
			},

			// Conditional functions
			"eq": func(a, b interface{}) bool { return a == b },
			"ne": func(a, b interface{}) bool { return a != b },
			"lt": func(a, b int) bool { return a < b },
			"le": func(a, b int) bool { return a <= b },
			"gt": func(a, b int) bool { return a > b },
			"ge": func(a, b int) bool { return a >= b },

			// List functions
			"list": func(items ...interface{}) []interface{} { return items },
			"dict": func(pairs ...interface{}) map[string]interface{} {
				dict := make(map[string]interface{})
				for i := 0; i < len(pairs); i += 2 {
					if i+1 < len(pairs) {
						if key, ok := pairs[i].(string); ok {
							dict[key] = pairs[i+1]
						}
					}
				}
				return dict
			},
		}
	}

	// Helper function to handle command substitution
	processCommandSubstitution := func(content string, debug bool, errors *[]string) string {
		result := content
		for strings.Contains(result, "$(") {
			start := strings.Index(result, "$(")
			if start == -1 {
				break
			}

			parenCount := 1
			end := start + 2
			for end < len(result) && parenCount > 0 {
				if result[end] == '(' {
					parenCount++
				} else if result[end] == ')' {
					parenCount--
				}
				end++
			}

			if parenCount == 0 {
				command := result[start+2 : end-1]
				cmdExec := exec.Command("bash", "-c", command)
				output, cmdErr := cmdExec.Output()
				if cmdErr == nil {
					result = result[:start] + strings.TrimSpace(string(output)) + result[end:]
					if debug {
						fmt.Printf("✅ Command substitution successful: %s\n", command)
					}
				} else {
					*errors = append(*errors, fmt.Sprintf("command substitution '%s': %v", command, cmdErr))
					break
				}
			} else {
				*errors = append(*errors, "unmatched parentheses in command substitution")
				break
			}
		}
		return result
	}

	// Helper function for manual environment variable substitution
	manualEnvSubstitution := func(content string, debug bool) string {
		result := content
		substitutionCount := 0

		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				// Replace ${VAR:-default} patterns
				for {
					defaultPattern := "${" + key + ":-"
					start := strings.Index(result, defaultPattern)
					if start == -1 {
						break
					}

					// Find matching closing brace
					braceCount := 1
					end := start + len(defaultPattern)
					for end < len(result) && braceCount > 0 {
						if result[end] == '{' {
							braceCount++
						} else if result[end] == '}' {
							braceCount--
						}
						end++
					}

					if braceCount == 0 {
						result = result[:start] + value + result[end:]
						substitutionCount++
					} else {
						break
					}
				}

				// Replace ${VAR} patterns
				oldResult := result
				result = strings.ReplaceAll(result, "${"+key+"}", value)
				if result != oldResult {
					substitutionCount++
				}
			}
		}

		if debug && substitutionCount > 0 {
			fmt.Printf("✅ Manual substitution: %d replacements\n", substitutionCount)
		}

		return result
	}

	// Helper function to process content with templating support (both Go templates and shell)
	processTemplateContent := func(content string, templateVars []string, contentType string) (string, error) {
		if debug {
			fmt.Printf("🔍 Processing %s with templating (%d characters, %d variables)\n", contentType, len(content), len(templateVars))
		}

		// Detect if content contains Go template syntax
		hasGoTemplate := strings.Contains(content, "{{") && strings.Contains(content, "}}")
		hasShellVars := strings.Contains(content, "$") || strings.Contains(content, "$(")

		result := content
		var processingErrors []string

		// Phase 1: Process Go templates if present and variables provided
		if hasGoTemplate && len(templateVars) > 0 {
			if debug {
				fmt.Printf("🔍 Processing Go template in %s with %d variables\n", contentType, len(templateVars))
			}

			templateData, err := parseTemplateVars(templateVars)
			if err != nil {
				processingErrors = append(processingErrors, fmt.Sprintf("template variables: %v", err))
			} else {
				// Add environment variables to template data
				for _, env := range os.Environ() {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						// Only add if not already defined by --var
						if _, exists := templateData[parts[0]]; !exists {
							templateData[parts[0]] = parts[1]
						}
					}
				}

				// Create and parse template
				tmpl := template.New(contentType).Funcs(createTemplateFuncMap())
				tmpl, err = tmpl.Parse(content)
				if err != nil {
					processingErrors = append(processingErrors, fmt.Sprintf("template parsing in %s: %v", contentType, err))
				} else {
					var buf bytes.Buffer
					err = tmpl.Execute(&buf, templateData)
					if err != nil {
						processingErrors = append(processingErrors, fmt.Sprintf("template execution in %s: %v", contentType, err))
					} else {
						result = buf.String()
						if debug {
							fmt.Printf("✅ Go template processing successful for %s\n", contentType)
						}
					}
				}
			}
		} else if hasGoTemplate && len(templateVars) == 0 {
			if debug {
				fmt.Printf("⚠️ Go template syntax detected in %s but no --var provided, skipping template processing\n", contentType)
			}
		}

		// Phase 2: Process shell substitution (always attempt if shell variables detected)
		if hasShellVars {
			if debug {
				fmt.Printf("🔍 Processing shell substitution in %s\n", contentType)
			}

			// Try envsubst first, then bash, then manual as fallbacks
			func() {
				defer func() {
					if r := recover(); r != nil {
						processingErrors = append(processingErrors, fmt.Sprintf("envsubst panic in %s: %v", contentType, r))
					}
				}()

				cmd := exec.Command("envsubst")
				cmd.Stdin = strings.NewReader(result)
				cmd.Env = os.Environ()

				if processedContent, err := cmd.Output(); err == nil {
					result = string(processedContent)
					if debug {
						fmt.Printf("✅ envsubst processing successful for %s\n", contentType)
					}

					// Handle command substitution after envsubst
					result = processCommandSubstitution(result, debug, &processingErrors)
				} else {
					processingErrors = append(processingErrors, fmt.Sprintf("envsubst in %s: %v", contentType, err))
				}
			}()

			// Fallback to bash if envsubst failed
			if len(processingErrors) > 0 {
				if debug {
					fmt.Printf("⚠️ envsubst failed for %s, trying bash expansion\n", contentType)
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							processingErrors = append(processingErrors, fmt.Sprintf("bash expansion panic in %s: %v", contentType, r))
						}
					}()

					bashCmd := exec.Command("bash", "-c", "echo "+shellescape(result))
					bashCmd.Env = os.Environ()

					if bashOutput, bashErr := bashCmd.Output(); bashErr == nil {
						result = strings.TrimSpace(string(bashOutput))
						if debug {
							fmt.Printf("✅ bash expansion successful for %s\n", contentType)
						}
						// Clear envsubst errors since bash worked
						processingErrors = []string{}
					} else {
						processingErrors = append(processingErrors, fmt.Sprintf("bash expansion in %s: %v", contentType, bashErr))
					}
				}()
			}

			// Final fallback to manual substitution
			if len(processingErrors) > 0 {
				if debug {
					fmt.Printf("⚠️ Both envsubst and bash failed for %s, using manual substitution\n", contentType)
				}

				result = manualEnvSubstitution(result, debug)
				if debug {
					fmt.Printf("✅ Manual substitution completed for %s\n", contentType)
				}
			}
		}

		// Report any processing warnings
		if len(processingErrors) > 0 && debug {
			fmt.Printf("⚠️ Processing warnings for %s: %s\n", contentType, strings.Join(processingErrors, "; "))
		}

		finalResult := strings.TrimSpace(result)
		if debug {
			fmt.Printf("🔍 Final processed %s content: %d characters\n", contentType, len(finalResult))
		}

		return finalResult, nil
	}

	// Helper function to load and process agent specification from file or URL
	loadAgentSpecFromSource := func(source string, templateVars []string) (map[string]interface{}, error) {
		var specData string
		var err error

		// Load agent spec content (file or URL)
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			if debug {
				fmt.Printf("🔍 Loading agent specification from URL: %s\n", source)
			}

			specData, err = fetchURL(source)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch agent spec from URL %s: %w", source, err)
			}
		} else {
			if debug {
				fmt.Printf("🔍 Loading agent specification from file: %s\n", source)
			}

			rawData, err := os.ReadFile(source)
			if err != nil {
				return nil, fmt.Errorf("failed to read agent spec file %s: %w", source, err)
			}
			specData = string(rawData)
		}

		// Process templating in agent specification
		processedSpecData, err := processTemplateContent(specData, templateVars, "agent specification")
		if err != nil {
			return nil, fmt.Errorf("failed to process agent spec templating: %w", err)
		}

		// Parse agent specification JSON
		var agentSpec map[string]interface{}
		if err := json.Unmarshal([]byte(processedSpecData), &agentSpec); err != nil {
			return nil, fmt.Errorf("failed to parse agent specification JSON from %s: %w", source, err)
		}

		if debug {
			fmt.Printf("✅ Successfully loaded and parsed agent specification from %s\n", source)
		}

		return agentSpec, nil
	}

	// Helper function to load tools with templating support
	loadToolsFromSourceWithTemplating := func(source string, templateVars []string) ([]kubiya.Tool, error) {
		var toolsData string
		var err error

		// Load tools content (file or URL)
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			if debug {
				fmt.Printf("🔍 Loading tools from URL: %s\n", source)
			}

			toolsData, err = fetchURL(source)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch tools file from URL %s: %w", source, err)
			}
		} else {
			if debug {
				fmt.Printf("🔍 Loading tools from file: %s\n", source)
			}

			rawData, err := os.ReadFile(source)
			if err != nil {
				return nil, fmt.Errorf("failed to read tools file %s: %w", source, err)
			}
			toolsData = string(rawData)
		}

		// Process templating in tools specification
		processedToolsData, err := processTemplateContent(toolsData, templateVars, "tools specification")
		if err != nil {
			return nil, fmt.Errorf("failed to process tools spec templating: %w", err)
		}

		// Validate JSON format
		if !json.Valid([]byte(processedToolsData)) {
			return nil, fmt.Errorf("tools specification from %s does not contain valid JSON after templating", source)
		}

		// Parse tools JSON
		var tools []kubiya.Tool
		if err := json.Unmarshal([]byte(processedToolsData), &tools); err != nil {
			return nil, fmt.Errorf("failed to parse tools JSON from %s: %w", source, err)
		}

		// Validate all tools
		for i, tool := range tools {
			if err := validateTool(tool); err != nil {
				return nil, fmt.Errorf("tool validation failed for tool %d (%s) from %s: %w", i, tool.Name, source, err)
			}
		}

		if debug {
			fmt.Printf("✅ Successfully loaded and validated %d tools from %s\n", len(tools), source)
		}

		return tools, nil
	}

	// Helper function to parse environment variables
	parseEnvVars := func(envVars []string) (map[string]string, error) {
		envMap := make(map[string]string)
		for _, env := range envVars {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", env)
			}
			envMap[parts[0]] = parts[1]
		}
		return envMap, nil
	}

	// Helper function to escape shell strings

	// Enhanced helper function to read and process prompt file with both shell substitution and Go templating
	readPromptFile := func(filePath string, templateVars []string) (string, error) {
		var content string
		var err error

		// Check if filePath is a URL
		if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
			// Handle URL
			content, err = fetchURL(filePath)
			if err != nil {
				return "", fmt.Errorf("failed to fetch prompt from URL %s: %w", filePath, err)
			}
			if debug {
				fmt.Printf("🔍 Fetched prompt from URL: %s (%d characters)\n", filePath, len(content))
			}
		} else {
			// Handle local file
			// Validate file exists and is readable
			if _, err := os.Stat(filePath); err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("prompt file does not exist: %s", filePath)
				}
				return "", fmt.Errorf("cannot access prompt file: %s (%w)", filePath, err)
			}

			// Read the file content
			rawContent, err := os.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("failed to read prompt file %s: %w", filePath, err)
			}

			content = string(rawContent)
			if debug {
				fmt.Printf("🔍 Read prompt file: %s (%d characters)\n", filePath, len(content))
			}
		}

		// Detect if file contains Go template syntax ({{ }})
		hasGoTemplate := strings.Contains(content, "{{") && strings.Contains(content, "}}")
		hasShellVars := strings.Contains(content, "$") || strings.Contains(content, "$(")

		var result string = content
		var processingErrors []string

		// Phase 1: Process Go templates if present and variables provided
		if hasGoTemplate && len(templateVars) > 0 {
			if debug {
				fmt.Printf("🔍 Processing Go template with %d variables\n", len(templateVars))
			}

			templateData, err := parseTemplateVars(templateVars)
			if err != nil {
				processingErrors = append(processingErrors, fmt.Sprintf("template variables: %v", err))
			} else {
				// Add environment variables to template data
				for _, env := range os.Environ() {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						// Only add if not already defined by --var
						if _, exists := templateData[parts[0]]; !exists {
							templateData[parts[0]] = parts[1]
						}
					}
				}

				// Create and parse template
				tmpl := template.New("prompt").Funcs(createTemplateFuncMap())
				tmpl, err = tmpl.Parse(content)
				if err != nil {
					processingErrors = append(processingErrors, fmt.Sprintf("template parsing: %v", err))
				} else {
					var buf bytes.Buffer
					err = tmpl.Execute(&buf, templateData)
					if err != nil {
						processingErrors = append(processingErrors, fmt.Sprintf("template execution: %v", err))
					} else {
						result = buf.String()
						if debug {
							fmt.Printf("✅ Go template processing successful\n")
						}
					}
				}
			}
		} else if hasGoTemplate && len(templateVars) == 0 {
			if debug {
				fmt.Printf("⚠️ Go template syntax detected but no --var provided, skipping template processing\n")
			}
		}

		// Phase 2: Process shell substitution (always attempt if shell variables detected)
		if hasShellVars {
			if debug {
				fmt.Printf("🔍 Processing shell substitution\n")
			}

			// Method 1: Try envsubst (GNU gettext) - most reliable for env vars
			func() {
				defer func() {
					if r := recover(); r != nil {
						processingErrors = append(processingErrors, fmt.Sprintf("envsubst panic: %v", r))
					}
				}()

				cmd := exec.Command("envsubst")
				cmd.Stdin = strings.NewReader(result)
				cmd.Env = os.Environ()

				if processedContent, err := cmd.Output(); err == nil {
					result = string(processedContent)
					if debug {
						fmt.Printf("✅ envsubst processing successful\n")
					}

					// Handle command substitution after envsubst
					result = processCommandSubstitution(result, debug, &processingErrors)
				} else {
					processingErrors = append(processingErrors, fmt.Sprintf("envsubst: %v", err))
				}
			}()

			// Method 2: Fallback to bash expansion if envsubst failed
			if len(processingErrors) > 0 {
				if debug {
					fmt.Printf("⚠️ envsubst failed, trying bash expansion\n")
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							processingErrors = append(processingErrors, fmt.Sprintf("bash expansion panic: %v", r))
						}
					}()

					bashCmd := exec.Command("bash", "-c", "echo "+shellescape(result))
					bashCmd.Env = os.Environ()

					if bashOutput, bashErr := bashCmd.Output(); bashErr == nil {
						result = strings.TrimSpace(string(bashOutput))
						if debug {
							fmt.Printf("✅ bash expansion successful\n")
						}
						// Clear envsubst errors since bash worked
						processingErrors = []string{}
					} else {
						processingErrors = append(processingErrors, fmt.Sprintf("bash expansion: %v", bashErr))
					}
				}()
			}

			// Method 3: Manual environment variable substitution as final fallback
			if len(processingErrors) > 0 {
				if debug {
					fmt.Printf("⚠️ Both envsubst and bash failed, using manual substitution\n")
				}

				result = manualEnvSubstitution(result, debug)
				if debug {
					fmt.Printf("✅ Manual substitution completed\n")
				}
			}
		}

		// Phase 3: Final validation and error reporting
		if len(processingErrors) > 0 && debug {
			fmt.Printf("⚠️ Processing warnings: %s\n", strings.Join(processingErrors, "; "))
		}

		finalResult := strings.TrimSpace(result)
		if debug {
			fmt.Printf("🔍 Final processed content: %d characters\n", len(finalResult))
		}

		return finalResult, nil
	}

	// Helper function to encode files to base64 and create with_files entries
	encodeFilesToBase64 := func(context map[string]string) ([]map[string]interface{}, error) {
		var withFiles []map[string]interface{}

		for filename, content := range context {
			// Skip URLs as they can't be encoded as files
			if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") {
				continue
			}

			// Encode content to base64
			encodedContent := base64.StdEncoding.EncodeToString([]byte(content))

			// Create with_files entry - place all files in /tmp/<filename>
			fileEntry := map[string]interface{}{
				"destination": "/tmp/" + filepath.Base(filename), // Place in /tmp/<filename>
				"content":     encodedContent,
			}

			withFiles = append(withFiles, fileEntry)
		}

		return withFiles, nil
	}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a agent",
		Long: `Start a chat session with a Kubiya agent.
You can either use enhanced interactive mode, specify a message directly, use a prompt file, or pipe input from stdin.
Use --context to include additional files for context (supports wildcards and URLs).
The command will automatically select the most appropriate agent unless one is specified.

Enhanced Interactive Mode Features:
• Beautiful terminal UI with colors and formatting
• Session persistence and history management
• Automatic retry and error recovery (15 retries by default)
• Tool execution tracking with real-time status
• Keyboard shortcuts for improved productivity
• Auto-save functionality
• Message history navigation

Automatic Retry Features:
• Connection errors are automatically retried with exponential backoff
• Stream errors and timeouts trigger automatic reconnection
• Agent errors trigger session recovery with original prompt
• Comprehensive retry patterns for network, TLS, DNS, and connection issues

Permission Levels:
• read: Execute read-only operations (kubectl get, describe, logs, etc.)
• readwrite: Execute all operations including read-write (kubectl apply, delete, etc.)
• ask: Ask for confirmation before executing operations

Automation Mode:
Use --silent flag or set KUBIYA_AUTOMATION environment variable to suppress progress updates,
connection status messages, and tool execution details for clean output in automation scripts.

Prompt Files:
Use --prompt-file to load complex, multiline prompts from files or URLs with dual processing support:

1. Shell Substitution: Environment variables ($USER, ${HOME}), command substitution $(date), etc.
2. Go Templates: Use {{.Variable}} syntax with --var flags for advanced templating including:
   - Variables: {{.ProjectName}}, {{.Environment}}
   - Conditionals: {{if .Debug}}debug mode{{end}}
   - Loops: {{range .Items}}{{.}}{{end}}
   - Functions: {{upper .Name}}, {{date "2006-01-02"}}, {{env "HOME"}}

URL Support:
- Raw URLs: https://raw.githubusercontent.com/user/repo/branch/file.txt
- GitHub blob URLs (auto-converted): https://github.com/user/repo/blob/branch/file.txt
- Automatic caching (1 hour) with GITHUB_TOKEN support for private repos

Both methods can be used together. Go templates are processed first, then shell substitution.

For inline agents, use --inline with --tools-file or --tools-json to provide custom tools.`,
		Example: `  # Enhanced interactive chat mode
  kubiya chat --interactive

  # Using context files with wildcards
  kubiya chat -n "security" -m "Review this code" --context "src/*.go" --context "tests/**/*_test.go"

  # Using URLs as context
  kubiya chat -n "security" -m "Check this" --context https://raw.githubusercontent.com/org/repo/main/config.yaml

  # Multiple context sources
  kubiya chat -n "devops" \
    --context "k8s/*.yaml" \
    --context "https://example.com/deployment.yaml" \
    --context "Dockerfile" \
    -m "Review deployment"

  # Pipe from stdin with context
  cat error.log | kubiya chat -n "debug" --stdin --context "config/*.yaml"

  # Auto-classify the most appropriate agent
  kubiya chat -m "Help me with Kubernetes deployment issues"

  # Different permission levels
  kubiya chat -n "devops" -m "Show me the pods" --permission-level read
  kubiya chat -n "devops" -m "Deploy the application" --permission-level readwrite
  kubiya chat -n "devops" -m "Check system status" --permission-level ask

  # Automation mode with clean output
  kubiya chat -n "devops" -m "kubectl get pods" --silent
  export KUBIYA_AUTOMATION=1 && kubiya chat -n "devops" -m "kubectl get nodes"

  # Using prompt files with shell substitution
  kubiya chat -n "devops" --prompt-file deployment-prompt.txt
  kubiya chat -n "security" -f analysis-prompt.md --context "src/**/*.go"

  # Using prompt files with Go templates
  kubiya chat -f template-prompt.txt --var "ProjectName=MyApp" --var "Environment=production"
  kubiya chat -f report-template.md --var "Debug=true" --var "Items=pod1,pod2,pod3"

  # Using prompt files from URLs (GitHub raw URLs)
  kubiya chat -f https://raw.githubusercontent.com/user/repo/main/prompts/deploy.txt
  kubiya chat -f https://github.com/user/repo/blob/main/prompts/analysis.md --var "Project=MyApp"

  # Continue a previous conversation
  kubiya chat --session abc123-def456-ghi789 -m "What about the logs?"

  # Inline agent with tools from file
  kubiya chat --inline --tools-file tools.json --ai-instructions "You are a helpful assistant" \
    --description "Custom inline agent" --runners "kubiyamanaged" -m "kubectl get pods"

  # Inline agent with tools from JSON string
  kubiya chat --inline --tools-json '[{"name":"echo","description":"Echo tool","content":"echo hello"}]' \
    --llm-model "azure/gpt-4-32k" --debug-mode -m "Run echo command"

  # Inline agent with environment variables and secrets
  kubiya chat --inline --tools-file tools.json --env-vars "ENV1=value1" --env-vars "ENV2=value2" \
    --secrets "SECRET1" --integrations "jira" -m "Use the tools"

  # Inline agent with tools from GitHub URL
  kubiya chat --inline --tools-file https://raw.githubusercontent.com/user/tools/main/k8s-tools.json \
    --ai-instructions "You are a Kubernetes expert" -m "Check cluster status"
  kubiya chat --inline --tools-file https://github.com/user/tools/blob/main/devops-tools.json \
    --description "DevOps Assistant" -m "Deploy the application"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence usage on errors for clean error messages
			cmd.SilenceUsage = true

			cfg.Debug = cfg.Debug || debug

			// Check for automation mode (either --silent flag or KUBIYA_AUTOMATION env var)
			automationMode := silent || os.Getenv("KUBIYA_AUTOMATION") != ""

			if interactive {
				chatUI := tui.NewEnhancedChatUI(cfg)
				return chatUI.Run()
			}

			// Handle inline agent validation
			if inline {
				if agentID != "" || agentName != "" {
					return fmt.Errorf("cannot use --inline with --agent or --name")
				}

				// Validate agent spec vs tools specification
				if agentSpec != "" {
					// Agent spec provided - tools may be included or separate
					if toolsJSON != "" {
						return fmt.Errorf("cannot use both --agent-spec and --tools-json (tools should be in agent spec or separate --tools-file)")
					}
				} else {
					// No agent spec - must provide tools
					if toolsFile == "" && toolsJSON == "" {
						return fmt.Errorf("--inline requires either --agent-spec or tools specification (--tools-file or --tools-json)")
					}
					if toolsFile != "" && toolsJSON != "" {
						return fmt.Errorf("cannot use both --tools-file and --tools-json")
					}
				}
				if description == "" {
					description = "Inline agent"
				}
				if len(runners) == 0 {
					runners = []string{"kubiyamanaged"}
				}
				if llmModel == "" {
					llmModel = "azure/gpt-4-32k"
				}
				if len(integrations) == 0 {
					integrations = []string{}
				}
			}

			// Session storage file path
			sessionFile := filepath.Join(os.TempDir(), "kubiya_last_session")

			// Handle clear session flag
			if clearSession {
				if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to clear session: %w", err)
				}
				fmt.Println("Session cleared.")
				return nil
			}

			// Load last session ID if autoSession is enabled
			if sessionID == "" && cfg.AutoSession {
				if data, err := os.ReadFile(sessionFile); err == nil {
					sessionID = string(data)
					if !automationMode {
						fmt.Printf("Resuming session ID: %s\n", sessionID)
					}
				}
			}

			// Handle stdin input
			if stdinInput {
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) != 0 {
					return fmt.Errorf("no stdin data provided")
				}

				reader := bufio.NewReader(os.Stdin)
				var sb strings.Builder
				for {
					line, err := reader.ReadString('\n')
					if err != nil && err != io.EOF {
						return fmt.Errorf("error reading stdin: %w", err)
					}
					sb.WriteString(line)
					if err == io.EOF {
						break
					}
				}
				message = sb.String()
			}

			// Handle prompt file input
			if promptFile != "" {
				if message != "" {
					return fmt.Errorf("cannot use both --message and --prompt-file")
				}
				if stdinInput {
					return fmt.Errorf("cannot use both --stdin and --prompt-file")
				}

				promptContent, err := readPromptFile(promptFile, templateVars)
				if err != nil {
					return err
				}
				message = promptContent

				if debug {
					fmt.Printf("🔍 Processed prompt from file: %s (%d characters)\n", promptFile, len(message))
				}
			}

			// Validate input
			if message == "" && !stdinInput {
				return fmt.Errorf("message is required (use -m, --prompt-file, --stdin, or pipe input)")
			}

			// Validate permission level
			if permissionLevel != "" && permissionLevel != "read" && permissionLevel != "readwrite" && permissionLevel != "ask" {
				return fmt.Errorf("invalid permission level: %s (must be 'read', 'readwrite', or 'ask')", permissionLevel)
			}
			if permissionLevel == "" {
				permissionLevel = "read" // Default to read-only
			}

			// Load context from all sources
			context, err := expandAndReadFiles(contextFiles)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}

			// Enhance message with permission level context
			enhancedMessage := message
			if !interactive {
				permissionMsg := ""
				switch permissionLevel {
				case "read":
					permissionMsg = "\n\n[SYSTEM] You have permission to execute READ-ONLY operations (kubectl get, describe, logs, etc.). You should execute these operations directly without asking for confirmation."
				case "readwrite":
					permissionMsg = "\n\n[SYSTEM] You have FULL PERMISSION to execute any operations including read-write operations (kubectl apply, delete, create, etc.). Execute operations directly without asking for confirmation."
				case "ask":
					permissionMsg = "\n\n[SYSTEM] You should ASK for confirmation before executing any operations. Present the commands you want to run and wait for user approval."
				}
				enhancedMessage = message + permissionMsg
			}

			// Setup client
			client := kubiya.NewClient(cfg)

			// Add these variables
			var (
				toolExecutions map[string]*toolExecution = make(map[string]*toolExecution)
				messageBuffer  map[string]*chatBuffer    = make(map[string]*chatBuffer)
				noColor        bool                      = !isatty.IsTerminal(os.Stdout.Fd())
				connStatus     *connectionStatus
				toolStats      = &toolCallStats{
					toolTypes: make(map[string]int),
				}
				rawEventLogging = os.Getenv("KUBIYA_RAW_EVENTS") == "1" || debug
				msgChan         <-chan kubiya.ChatMessage
				inlineAgent     map[string]interface{}
			)

			// Handle inline agent
			if inline {
				var tools []kubiya.Tool

				// Load agent specification if provided
				if agentSpec != "" {
					if debug {
						fmt.Printf("🔍 Loading agent specification from: %s\n", agentSpec)
					}

					// Load and process agent spec with templating
					agentSpecData, err := loadAgentSpecFromSource(agentSpec, templateVars)
					if err != nil {
						return err
					}

					// Use the loaded agent specification
					inlineAgent = agentSpecData

					// Check if tools are included in agent spec
					if toolsData, exists := agentSpecData["tools"]; exists {
						if debug {
							fmt.Printf("🔍 Found tools in agent specification\n")
						}

						// Convert tools data to proper format
						switch toolsArray := toolsData.(type) {
						case []interface{}:
							// Tools are already in the spec - convert to kubiya.Tool format for validation
							toolsJSON, err := json.Marshal(toolsArray)
							if err != nil {
								return fmt.Errorf("failed to marshal tools from agent spec: %w", err)
							}

							if err := json.Unmarshal(toolsJSON, &tools); err != nil {
								return fmt.Errorf("failed to parse tools from agent spec: %w", err)
							}

							// Validate tools
							for i, tool := range tools {
								if err := validateTool(tool); err != nil {
									return fmt.Errorf("tool validation failed for tool %d (%s) from agent spec: %w", i, tool.Name, err)
								}
							}

						default:
							return fmt.Errorf("tools in agent specification must be an array")
						}
					} else {
						// No tools in agent spec, check for separate tools file
						if toolsFile != "" {
							tools, err = loadToolsFromSourceWithTemplating(toolsFile, templateVars)
							if err != nil {
								return err
							}
						} else {
							// Try to discover tools.json in same directory
							discoveredToolsPath, err := discoverToolsFile(agentSpec)
							if err != nil {
								return fmt.Errorf("failed to discover tools file: %w", err)
							}

							if discoveredToolsPath != "" {
								if debug {
									fmt.Printf("🔍 Auto-discovered tools file: %s\n", discoveredToolsPath)
								}
								tools, err = loadToolsFromSourceWithTemplating(discoveredToolsPath, templateVars)
								if err != nil {
									return err
								}
							} else if debug {
								fmt.Printf("⚠️ No tools found in agent spec and no tools.json discovered\n")
							}
						}
					}

				} else {
					// Traditional inline agent mode - load tools separately
					if toolsFile != "" {
						tools, err = loadToolsFromSourceWithTemplating(toolsFile, templateVars)
						if err != nil {
							return err
						}
					} else if toolsJSON != "" {
						if debug {
							fmt.Printf("🔍 Parsing tools from JSON string\n")
						}

						// Process templating on tools JSON string
						processedToolsJSON, err := processTemplateContent(toolsJSON, templateVars, "tools JSON string")
						if err != nil {
							return fmt.Errorf("failed to process tools JSON templating: %w", err)
						}

						tools, err = parseTools(processedToolsJSON)
						if err != nil {
							return fmt.Errorf("failed to parse tools from JSON: %w", err)
						}

						// Validate tools from JSON string
						for i, tool := range tools {
							if err := validateTool(tool); err != nil {
								return fmt.Errorf("tool validation failed for tool %d (%s): %w", i, tool.Name, err)
							}
						}
					}
				}

				// Parse environment variables
				envVarsMap, err := parseEnvVars(envVars)
				if err != nil {
					return fmt.Errorf("failed to parse environment variables: %w", err)
				}

				// Add KUBIYA_RUNNER if runners are specified
				if len(runners) > 0 {
					envVarsMap["KUBIYA_RUNNER"] = runners[0]
				}

				// Encode context files to base64 if provided
				var contextFiles []map[string]interface{}
				if len(context) > 0 {
					contextFiles, err = encodeFilesToBase64(context)
					if err != nil {
						return fmt.Errorf("failed to encode context files: %w", err)
					}
					if debug && len(contextFiles) > 0 {
						fmt.Printf("📁 Encoded %d context files for inline agent\n", len(contextFiles))
					}
				}

				if debug {
					fmt.Printf("🤖 Creating inline agent with %d tools\n", len(tools))
					fmt.Printf("📋 Tools: %v\n", func() []string {
						names := make([]string, len(tools))
						for i, t := range tools {
							names[i] = t.Name
						}
						return names
					}())
				}

				// Convert tools to the correct format for inline agent
				inlineTools := make([]map[string]interface{}, len(tools))
				for i, tool := range tools {
					// Ensure args is an empty slice if nil
					args := tool.Args
					if args == nil {
						args = []kubiya.ToolArg{}
					}

					// Ensure env is an empty slice if nil
					env := tool.Env
					if env == nil {
						env = []string{}
					}

					// Start with existing with_files from the tool
					withFiles := []interface{}{}
					if tool.WithFiles != nil {
						switch v := tool.WithFiles.(type) {
						case []interface{}:
							withFiles = v
						case []string:
							for _, f := range v {
								withFiles = append(withFiles, f)
							}
						case map[string]interface{}:
							withFiles = append(withFiles, v)
						default:
							withFiles = []interface{}{v}
						}
					}

					// Add context files to with_files for each tool
					for _, contextFile := range contextFiles {
						withFiles = append(withFiles, contextFile)
					}

					// Start with existing with_volumes from the tool
					withVolumes := []interface{}{}
					if tool.WithVolumes != nil {
						switch v := tool.WithVolumes.(type) {
						case []interface{}:
							withVolumes = v
						case []string:
							for _, vol := range v {
								withVolumes = append(withVolumes, vol)
							}
						case map[string]interface{}:
							withVolumes = append(withVolumes, v)
						default:
							withVolumes = []interface{}{v}
						}
					}

					// Add shared volume by default for inline agents
					sharedVolume := map[string]interface{}{
						"path": "/shared",
						"name": "shared-data",
					}
					withVolumes = append(withVolumes, sharedVolume)

					inlineTools[i] = map[string]interface{}{
						"name":         tool.Name,
						"alias":        tool.Alias,
						"description":  tool.Description,
						"type":         tool.Type,
						"content":      tool.Content,
						"args":         args,
						"env":          env,
						"image":        tool.Image,
						"with_files":   withFiles,
						"with_volumes": withVolumes,
					}
				}

				// Create or update inline agent request
				if inlineAgent == nil {
					// Traditional inline agent mode - create from flags
					inlineAgent = map[string]interface{}{
						"uuid":                  nil,
						"name":                  "inline",
						"description":           description,
						"ai_instructions":       aiInstructions,
						"tools":                 inlineTools,
						"runners":               runners,
						"integrations":          integrations,
						"secrets":               secrets,
						"environment_variables": envVarsMap,
						"llm_model":             llmModel,
						"is_debug_mode":         true, // Always true for inline agents to capture tool output
						"owners":                []string{},
						"allowed_users":         []string{},
						"allowed_groups":        []string{},
						"starters":              []interface{}{},
						"tasks":                 []string{},
						"sources":               []string{},
						"links":                 []string{},
						"image":                 "",
						"additional_data":       map[string]interface{}{},
					}
				} else {
					// Agent spec mode - merge with flag overrides and set tools
					// Always use inlineTools format which includes proper volumes and structure
					inlineAgent["tools"] = inlineTools

					// Override with command line flags if provided
					if description != "" && description != "Inline agent" {
						inlineAgent["description"] = description
					}
					if aiInstructions != "" {
						inlineAgent["ai_instructions"] = aiInstructions
					}
					if len(runners) > 0 {
						inlineAgent["runners"] = runners
					}
					if len(integrations) > 0 {
						inlineAgent["integrations"] = integrations
					}
					if len(secrets) > 0 {
						inlineAgent["secrets"] = secrets
					}
					if len(envVarsMap) > 0 {
						// Merge with existing environment variables
						if existingEnv, ok := inlineAgent["environment_variables"].(map[string]interface{}); ok {
							for k, v := range envVarsMap {
								existingEnv[k] = v
							}
						} else {
							inlineAgent["environment_variables"] = envVarsMap
						}
					}
					if llmModel != "" {
						inlineAgent["llm_model"] = llmModel
					}

					// Ensure debug mode is enabled
					inlineAgent["is_debug_mode"] = true

					// Set defaults for missing fields to match traditional inline agent structure
					if inlineAgent["name"] == nil {
						inlineAgent["name"] = "inline"
					}
					if inlineAgent["owners"] == nil {
						inlineAgent["owners"] = []string{}
					}
					if inlineAgent["allowed_users"] == nil {
						inlineAgent["allowed_users"] = []string{}
					}
					if inlineAgent["allowed_groups"] == nil {
						inlineAgent["allowed_groups"] = []string{}
					}
					if inlineAgent["starters"] == nil {
						inlineAgent["starters"] = []interface{}{}
					}
					if inlineAgent["tasks"] == nil {
						inlineAgent["tasks"] = []string{}
					}
					if inlineAgent["sources"] == nil {
						inlineAgent["sources"] = []string{}
					}
					if inlineAgent["links"] == nil {
						inlineAgent["links"] = []string{}
					}
					if inlineAgent["image"] == nil {
						inlineAgent["image"] = ""
					}
					if inlineAgent["additional_data"] == nil {
						inlineAgent["additional_data"] = map[string]interface{}{}
					}
				}

				if debug {
					fmt.Printf("🔍 CLI: Creating inline agent with payload:\n")
					if jsonPayload, err := json.MarshalIndent(inlineAgent, "", "  "); err == nil {
						fmt.Printf("Agent Definition: %s\n", string(jsonPayload))
					}
					fmt.Printf("Message: %s\n", message)
					fmt.Printf("SessionID: %s\n", sessionID)
					fmt.Printf("Context files: %d\n", len(context))
					fmt.Printf("User Email: %s\n", os.Getenv("KUBIYA_USER_EMAIL"))
					fmt.Printf("Organization: %s\n", os.Getenv("KUBIYA_ORG"))
					apiKey := os.Getenv("KUBIYA_API_KEY")
					if len(apiKey) > 20 {
						fmt.Printf("API Key: %s\n", apiKey[:20]+"...")
					} else {
						fmt.Printf("API Key: %s\n", apiKey)
					}
					fmt.Printf("Base URL: %s\n", cfg.BaseURL)
					fmt.Printf("Debug mode: %v\n", debug)
				}

				msgChan, err = client.SendInlineAgentMessage(cmd.Context(), message, sessionID, context, inlineAgent)
				if err != nil {
					// If inline agent connection fails, retry for retryable errors
					if isRetryableError(err) {
						if !automationMode {
							fmt.Printf("\r%s\n", style.WarningStyle.Render("⚠️  Inline agent connection failed, retrying..."))
						}
						// For inline agents, we retry by sending the message again with exponential backoff
						for retryAttempt := 1; retryAttempt <= retries; retryAttempt++ {
							backoffDelay := calculateBackoffDelay(retryAttempt - 1)
							if !automationMode {
								fmt.Printf("%s\n", style.SpinnerStyle.Render(
									fmt.Sprintf("🔄 Retrying inline agent connection (attempt %d/%d) in %.1fs...", 
										retryAttempt, retries, backoffDelay.Seconds())))
							}
							time.Sleep(backoffDelay)
							
							msgChan, err = client.SendInlineAgentMessage(cmd.Context(), message, sessionID, context, inlineAgent)
							if err == nil {
								break // Success!
							}
							if !isRetryableError(err) {
								break // Non-retryable error
							}
						}
						if err != nil {
							return fmt.Errorf("failed to connect to inline agent after %d retries: %w", retries, err)
						}
					} else {
						return err
					}
				}

				// Set agentID for inline agent to use the same processing logic
				agentID = "inline"
			}

			// Auto-classify by default unless agent is explicitly specified or --no-classify is set
			shouldClassify := agentID == "" && agentName == "" && !noClassify && !inline

			// If auto-classify is enabled (default), use the classification endpoint
			if shouldClassify {
				if debug {
					fmt.Printf("🔍 Classification prompt: %s\n", message)
				}

				// Create classification request
				reqBody := map[string]string{
					"message": message,
				}
				reqJSON, err := json.Marshal(reqBody)
				if err != nil {
					return fmt.Errorf("failed to marshal classification request: %w", err)
				}

				// Create HTTP request
				baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
				baseURL = strings.TrimSuffix(baseURL, "/api/v1")

				classifyURL := fmt.Sprintf("%s/http-bridge/v1/classify/agent", baseURL)
				req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, classifyURL, bytes.NewBuffer(reqJSON))
				if err != nil {
					return fmt.Errorf("failed to create classification request: %w", err)
				}

				// Set headers
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "UserKey "+cfg.APIKey)

				if debug {
					fmt.Printf("🌐 Sending classification request to: %s\n", classifyURL)
				}

				// Send request
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("failed to send classification request: %w", err)
				}
				defer resp.Body.Close()

				// Read response body
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("failed to read classification response: %w", err)
				}

				if debug {
					fmt.Printf("📥 Classification response status: %d\n", resp.StatusCode)
					fmt.Printf("📄 Classification response body: %s\n", string(body))
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("classification failed with status %d: %s", resp.StatusCode, string(body))
				}

				// Parse response
				var agents []struct {
					UUID        string `json:"uuid"`
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if err := json.Unmarshal(body, &agents); err != nil {
					return fmt.Errorf("failed to parse classification response: %w", err)
				}

				if len(agents) == 0 {
					if debug {
						fmt.Println("❌ No suitable agent found in the classification response")
					}
					return fmt.Errorf("no suitable agent found for the task")
				}

				// Use the first (best) agent
				agentID = agents[0].UUID
				if !automationMode {
					fmt.Printf("🤖 Auto-selected agent: %s (%s)\n", agents[0].Name, agents[0].Description)
				}
			}

			// If agent name is provided, look up the ID
			if agentName != "" && agentID == "" {
				if debug {
					fmt.Printf("🔍 Looking up agent by name: %s\n", agentName)
				}

				agents, err := client.GetAgents(cmd.Context())
				if err != nil {
					return fmt.Errorf("failed to list agents: %w", err)
				}

				if debug {
					fmt.Printf("📋 Found %d agents\n", len(agents))
				}

				found := false
				for _, t := range agents {
					if strings.EqualFold(t.Name, agentName) {
						agentID = t.UUID
						found = true
						if debug {
							fmt.Printf("✅ Found matching agent: %s (UUID: %s)\n", t.Name, t.UUID)
						}
						break
					}
				}

				if !found {
					if debug {
						fmt.Printf("❌ No agent found with name: %s\n", agentName)
					}
					return fmt.Errorf("agent with name '%s' not found", agentName)
				}
			}

			// Ensure we have a agent ID by this point
			if agentID == "" {
				return fmt.Errorf("no agent selected - please specify a agent or allow auto-classification")
			}

			// Before the message handling loop, add style configuration for non-TTY:
			if noColor {
				// Disable all styling for non-TTY environments
				style.DisableColors()
			}

			// Initialize connection status - fetch actual runner from agent
			var agentRunner string
			var agentInfo *kubiya.Agent

			// Fetch agent information to get the actual runner (skip for inline agents)
			if !inline {
				if agentInfo, err = client.GetAgent(cmd.Context(), agentID); err != nil {
					if debug {
						fmt.Printf("⚠️ Failed to get agent info: %v, using default runner\n", err)
					}
					agentRunner = "kubiyamanaged"
				} else if len(agentInfo.Runners) > 0 {
					agentRunner = agentInfo.Runners[0] // Use first runner
				} else {
					agentRunner = "kubiyamanaged" // fallback
				}
			} else {
				// For inline agents, use the provided runners or default
				if len(runners) > 0 {
					agentRunner = runners[0]
				} else {
					agentRunner = "kubiyamanaged"
				}
			}

			// Override with environment variable if set
			if os.Getenv("KUBIYA_RUNNER") != "" {
				agentRunner = os.Getenv("KUBIYA_RUNNER")
			}

			connStatus = &connectionStatus{
				runner:      agentRunner,
				runnerType:  "k8s",
				connected:   false,
				connectTime: time.Now(),
			}

			// Show connection flow (only if not in automation mode)
			if !automationMode {
				fmt.Printf("🔗 Connecting to agent server...\n")
				fmt.Printf("⏳ Initializing %s runner...\n", connStatus.runner)
				os.Stdout.Sync() // Force immediate display
			}

			// Send message with context, with retry mechanism for robustness (skip for inline agents)
			if !inline {
				msgChan, err = client.SendMessageWithContext(cmd.Context(), agentID, enhancedMessage, sessionID, context)
				if err != nil {
					// If context method fails, try with retry mechanism for retryable errors
					if isRetryableError(err) {
						if !automationMode {
							fmt.Printf("\r%s\n", style.WarningStyle.Render("⚠️  Initial connection failed, retrying with enhanced resilience..."))
						}
						msgChan, err = client.SendMessageWithRetry(cmd.Context(), agentID, enhancedMessage, sessionID, retries)
						if err != nil {
							return fmt.Errorf("failed to send message after %d retries: %w", retries, err)
						}
					} else {
						return err
					}
				}
			}

			// Connection established
			connStatus.connected = true
			connStatus.lastPing = time.Now()
			connStatus.latency = time.Since(connStatus.connectTime)

			// Show compact connection status
			if !automationMode {
				fmt.Printf("\r\033[K")      // Clear current line
				fmt.Printf("\033[2A\033[K") // Clear both lines
				fmt.Printf("✅ Connected \u2022 🚀 Processing...\n")
				os.Stdout.Sync() // Force immediate display
			}

			// Read messages and handle session ID with session recovery support
			var finalResponse strings.Builder
			var completionReason string
			var hasError bool
			var actualSessionID string
			var toolsExecuted bool
			var streamRetryCount int
			var anyOutputTruncated bool
			var sessionRetryCount int

			// Add these message type constants
			const (
				systemMsg  = "system"
				chatMsg    = "chat"
				toolMsg    = "tool"
				toolOutput = "tool_output"
			)

			// Main session retry loop for agent error recovery
			for sessionRetryCount <= retries {
				// Reset per-session variables
				finalResponse.Reset()
				completionReason = ""
				hasError = false
				toolsExecuted = false
				anyOutputTruncated = false
				
				// Update the message handling loop with stream error handling:
				sessionRecoveryNeeded := false
				for msg := range msgChan {
				if msg.Error != "" {
					// Smart retry logic with proper error classification
					errorObj := fmt.Errorf(msg.Error)

					if isRetryableError(errorObj) && streamRetryCount < retries {
						streamRetryCount++

						// Calculate proper exponential backoff with jitter
						backoffDelay := calculateBackoffDelay(streamRetryCount - 1)

						if !automationMode {
							fmt.Printf("\r%s\n", style.WarningStyle.Render(
								fmt.Sprintf("⚠️  Stream error (attempt %d/%d): %s",
									streamRetryCount, retries, msg.Error)))
							fmt.Printf("%s\n", style.SpinnerStyle.Render(
								fmt.Sprintf("🔄 Retrying in %.1fs...", backoffDelay.Seconds())))
						}

						// Wait with proper backoff
						select {
						case <-cmd.Context().Done():
							return cmd.Context().Err()
						case <-time.After(backoffDelay):
							// Continue with retry
						}

						// Attempt to reconnect with remaining retries
						if !inline {
							remainingRetries := retries - streamRetryCount
							if remainingRetries < 1 {
								remainingRetries = 1
							}

							msgChan, err = client.SendMessageWithRetry(cmd.Context(), agentID, enhancedMessage, actualSessionID, remainingRetries)
							if err != nil {
								if !automationMode {
									fmt.Printf("%s\n", style.ErrorStyle.Render(fmt.Sprintf("❌ Reconnection failed: %v", err)))
								}
								// Don't continue here - let it fall through to retry logic
								continue
							}
							if !automationMode {
								fmt.Printf("%s\n", style.SuccessStyle.Render("✅ Reconnected successfully, continuing..."))
							}
							// Reset retry count on successful reconnection
							streamRetryCount = 0
							continue
						}
					} else {
						// Either non-retryable error or exhausted retries
						if streamRetryCount >= retries {
							fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render(
								fmt.Sprintf("❌ Stream failed after %d retries: %s", retries, msg.Error)))
						} else {
							fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render(
								fmt.Sprintf("❌ Non-retryable error: %s", msg.Error)))
						}
						hasError = true
						return fmt.Errorf("stream error: %s", msg.Error)
					}
				}

				// Raw event logging for debugging
				if rawEventLogging {
					fmt.Printf("[RAW EVENT] Type: %s, Content: %s, MessageID: %s, Final: %v, SessionID: %s\n",
						msg.Type, msg.Content, msg.MessageID, msg.Final, msg.SessionID)
				}

				// Capture completion reason and session ID from final messages
				if msg.Final && msg.FinishReason != "" {
					completionReason = msg.FinishReason
				}
				if msg.SessionID != "" {
					actualSessionID = msg.SessionID
				}

				// Handle system messages (only show if not in automation mode)
				if msg.Type == systemMsg {
					if !automationMode {
						fmt.Fprintf(os.Stderr, "%s\n", style.SystemStyle.Render("🔄 "+msg.Content))
					}
					continue
				}

				switch msg.Type {
				case toolMsg:
					toolInfo := strings.TrimSpace(msg.Content)
					if strings.HasPrefix(toolInfo, "Tool:") {
						parts := strings.SplitN(toolInfo, "Arguments:", 2)
						toolName := strings.TrimSpace(strings.TrimPrefix(parts[0], "Tool:"))
						toolArgs := ""
						if len(parts) > 1 {
							toolArgs = strings.TrimSpace(parts[1])
						}

						// LIVE streaming of tool calls and arguments
						if showToolCalls && !automationMode {
							// Check if this is a new tool call (first time we see this tool)
							isNewTool := true
							for _, te := range toolExecutions {
								if te.name == toolName && !te.isComplete {
									isNewTool = false
									break
								}
							}

							if isNewTool {
								// Compact tool header - start the live streaming line
								// No sync here - we'll update this line as args build
							}

							// LIVE streaming - keep working animation until tool execution starts
							if toolArgs == "" {
								// Initial tool detection - start dynamic working animation
								if !isAnimationRunning(toolName) {
									go showWorkingAnimation(toolName)
								}
							} else if !strings.HasSuffix(toolArgs, "}") {
								// Parameters are building but not complete - ensure animation is running
								if !isAnimationRunning(toolName) {
									go showWorkingAnimation(toolName)
								}
								// This gives users continuous feedback that the AI is actively working
							} else {
								// Parameters are complete - show final result and prepare for execution
								stopWorkingAnimation(toolName)
								
								// Parameters complete - show formatted final args and move to next line
								displayArgs := formatLiveJSON(toolArgs)
								fmt.Printf("\r⚡ %s %s %s\n",
									style.ToolExecutingStyle.Render(toolName),
									style.ValueStyle.Render(displayArgs),
									style.DimStyle.Render("✓"))
								os.Stdout.Sync()
							}
						}

						// Only process final tool execution if we have complete arguments
						if strings.HasSuffix(toolArgs, "}") {
							// Check for duplicate tool execution
							isDuplicate := false
							for _, te := range toolExecutions {
								if te.name == toolName && te.args == toolArgs && !te.isComplete {
									isDuplicate = true
									break
								}
							}

							if !isDuplicate {
								// Update tool statistics
								toolStats.mu.Lock()
								toolStats.totalCalls++
								toolStats.activeCalls++
								toolStats.toolTypes[toolName]++
								toolStats.mu.Unlock()

								// Create new tool execution
								te := &toolExecution{
									name:       toolName,
									args:       toolArgs,
									msgID:      msg.MessageID,
									status:     "waiting",
									startTime:  time.Now(),
									runner:     connStatus.runner,
									toolCallId: msg.MessageID,
								}
								toolExecutions[msg.MessageID] = te
								toolsExecuted = true

								// Show smart parameter summary if available
								if showToolCalls && !automationMode {
									paramSummary := formatToolParameters(toolArgs)
									if paramSummary != "" {
										fmt.Printf("   %s\n", style.DimStyle.Render(paramSummary))
									}
									os.Stdout.Sync()
								}
							}
						}
					}

				case toolOutput:
					te := toolExecutions[msg.MessageID]
					if te != nil && !te.isComplete {
						// Mark that we received output
						te.hasOutput = true

						// Update status to running when we start receiving output
						if te.status == "waiting" {
							te.status = "running"
							// Clear the previous line and show running state
							if showToolCalls && !automationMode {
								fmt.Printf("\r⚡ %s %s\n",
									style.ToolExecutingStyle.Render(te.name),
									style.SpinnerStyle.Render("🔄 executing..."))
								os.Stdout.Sync()
							}
							// Show clean animation only during execution
							if showToolCalls && !automationMode && !te.animationStarted {
								te.animationStarted = true
								go startToolAnimation(te)
							}
						}

						// Also check if we need to mark as complete based on content
						lowerContent := strings.ToLower(msg.Content)
						if strings.Contains(lowerContent, "completed") ||
							strings.Contains(lowerContent, "finished") ||
							strings.Contains(lowerContent, "done") ||
							strings.Contains(lowerContent, "success") ||
							strings.Contains(lowerContent, "created") ||
							strings.Contains(lowerContent, "saved") {
							te.isComplete = true
							// Ensure it's not marked as failed if we detect success
							if !te.failed {
								te.status = "completed"
							}
						}

						// Store the full content
						te.output.WriteString(msg.Content)

						// Get the current content buffer for this message
						storedContent := messageBuffer[msg.MessageID]
						if storedContent == nil {
							storedContent = &chatBuffer{}
							messageBuffer[msg.MessageID] = storedContent
						}

						// Only process new content since last update
						newContent := msg.Content
						if len(storedContent.content) > 0 && len(msg.Content) > len(storedContent.content) {
							newContent = msg.Content[len(storedContent.content):]
						} else if len(storedContent.content) > 0 {
							newContent = ""
						}

						// Process new content if any
						trimmedContent := strings.TrimSpace(newContent)

						// Check for empty output scenarios that should show "output truncated"
						if trimmedContent == "" || trimmedContent == "\"\"" || trimmedContent == "{}" {
							te.outputTruncated = true
							anyOutputTruncated = true
							if showToolCalls && !automationMode {
								// Just track that output was truncated, don't spam the user
								// We'll show a single message at the end
							}
						} else if trimmedContent != "" {
							// Try to parse as JSON first for structured output
							var outputData struct {
								State   string `json:"state,omitempty"`
								Status  string `json:"status,omitempty"`
								Output  string `json:"output,omitempty"`
								Error   string `json:"error,omitempty"`
								Message string `json:"message,omitempty"`
							}

							if err := json.Unmarshal([]byte(trimmedContent), &outputData); err == nil {
								// Handle structured output
								if outputData.State != "" {
									te.status = outputData.State
								}
								if outputData.Error != "" {
									te.failed = true
									te.errorMsg = outputData.Error // Store the actual error message
									te.status = "failed"
									hasError = true
									
									// Stop working animation if it's still running
									stopWorkingAnimation(te.name)
									
									if showToolCalls && !automationMode {
										// Show clear "Tool call failed" message
										errorMsg := outputData.Error
										if len(errorMsg) > 100 {
											errorMsg = errorMsg[:97] + "..."
										}
										fmt.Printf("\r⚡ %s %s\n",
											style.ToolExecutingStyle.Render(te.name),
											style.ErrorStyle.Render(fmt.Sprintf("Tool call failed: %s", errorMsg)))
									}
								}
								if outputData.Output != "" || outputData.Message != "" {
									output := outputData.Output
									if output == "" {
										output = outputData.Message
									}
									// Don't show every line - just track that we got output
									// This prevents spam during streaming
									if showToolCalls && !automationMode && !te.hasShownOutput {
										fmt.Printf("   %s\n", style.LiveStatusStyle.Render("Receiving output..."))
										te.hasShownOutput = true
										os.Stdout.Sync()
									}
								}
							} else {
								// Handle plain text output - analyze for key messages only
								if showToolCalls && !automationMode {
									// Only show important messages, not every line
									lowerContent := strings.ToLower(trimmedContent)
									
									// More sophisticated error detection - only flag actual failures, not mentions of errors
									isActualError := false
									// Look for actual error patterns, excluding common success patterns
									if (strings.Contains(lowerContent, "error:") || strings.Contains(lowerContent, "failed:") ||
										strings.Contains(lowerContent, "error occurred") || strings.Contains(lowerContent, "execution failed") ||
										strings.Contains(lowerContent, "command failed") || strings.Contains(lowerContent, "operation failed") ||
										(strings.Contains(lowerContent, "exit code") && (strings.Contains(lowerContent, " 1") || strings.Contains(lowerContent, " 2")))) &&
										// Exclude success patterns that might contain "error" in context
										!strings.Contains(lowerContent, "successfully") &&
										!strings.Contains(lowerContent, "completed") &&
										!strings.Contains(lowerContent, "created") &&
										!strings.Contains(lowerContent, "saved") &&
										!strings.Contains(lowerContent, "finished") {
										isActualError = true
									}
									
									if isActualError {
										// Show friendly warning only for actual errors
										if !te.hasShownError {
											// Try to extract meaningful error info
											errorMsg := "encountered an issue"
											if strings.Contains(lowerContent, "not found") {
												errorMsg = "resource not found"
											} else if strings.Contains(lowerContent, "permission") || strings.Contains(lowerContent, "credentials") {
												errorMsg = "permission issue"
											} else if strings.Contains(lowerContent, "timeout") {
												errorMsg = "timed out"
											} else if strings.Contains(lowerContent, "connection") {
												errorMsg = "connection issue"
											}
											
											// Stop working animation
											stopWorkingAnimation(te.name)
											
											fmt.Printf("\r⚡ %s %s\n",
												style.ToolExecutingStyle.Render(te.name),
												style.ErrorStyle.Render(fmt.Sprintf("Tool call failed: %s", errorMsg)))
											te.hasShownError = true
											te.failed = true
											te.status = "failed"
											te.errorMsg = errorMsg // Store the inferred error message
											hasError = true
											os.Stdout.Sync()
										}
									} else if strings.Contains(lowerContent, "success") || strings.Contains(lowerContent, "completed") || 
										strings.Contains(lowerContent, "created") || strings.Contains(lowerContent, "saved") ||
										strings.Contains(lowerContent, "finished") || strings.Contains(lowerContent, "done") {
										// Stop working animation on success
										stopWorkingAnimation(te.name)
										
										fmt.Printf("\r⚡ %s %s\n",
											style.ToolExecutingStyle.Render(te.name),
											style.SuccessStyle.Render("✅ completed successfully"))
										os.Stdout.Sync()
									} else if !te.hasShownOutput {
										fmt.Printf("   %s\n", style.LiveStatusStyle.Render("Processing..."))
										te.hasShownOutput = true
										os.Stdout.Sync()
									}
								}
							}
						}

						// Update stored content
						storedContent.content = msg.Content
					}

				default:
					// Handle tool completion - check on any non-tool message type if we have pending tool executions
					for msgID, te := range toolExecutions {
						if te.hasOutput && !te.isComplete {
							// Mark as complete if we receive a final message or if enough time has passed
							if msg.Final || time.Since(te.startTime) > 2*time.Minute {
								te.isComplete = true
							} else if te.status == "running" && time.Since(te.startTime) > 30*time.Second {
								// If running for more than 30 seconds without completion, mark as complete
								te.isComplete = true
							}

							if te.isComplete {
								// Stop any running animation cleanly
								te.status = "complete"

								// Update tool statistics
								toolStats.mu.Lock()
								toolStats.activeCalls--
								if te.failed {
									toolStats.failedCalls++
								} else {
									toolStats.completedCalls++
								}
								toolStats.mu.Unlock()

								duration := time.Since(te.startTime).Seconds()

								// Show clean completion status with actual error messages
								if showToolCalls && !automationMode {
									if te.failed {
										// Show compact error message
										errorDisplay := "error"
										if te.errorMsg != "" {
											errorDisplay = strings.TrimSpace(te.errorMsg)
											if len(errorDisplay) > 50 {
												errorDisplay = errorDisplay[:50] + "..."
											}
											// Clean for single line display
											errorDisplay = strings.ReplaceAll(errorDisplay, "\n", " ")
											errorDisplay = strings.ReplaceAll(errorDisplay, "\r", " ")
										}
										fmt.Printf("   ⚠️  %s: %s (%.1fs)\n",
											style.WarningStyle.Render(te.name),
											style.DimStyle.Render(errorDisplay),
											duration)
									} else {
										fmt.Printf("   ✓ %s (%.1fs)\n",
											style.SuccessStyle.Render("Completed"),
											duration)
									}
									os.Stdout.Sync()
									continue // Skip the old completion display
								}

								// Legacy variables for any remaining code
								var statusEmoji string
								var completionStatus string
								if te.failed {
									statusEmoji = statusFailed
									completionStatus = "failed"
								} else {
									statusEmoji = statusDone
									completionStatus = "completed"
								}

								// Show updated statistics only if tool calls are enabled and not in automation mode
								if showToolCalls && !automationMode {
									toolStats.mu.RLock()
									updatedStatsStr := fmt.Sprintf("[%d active, %d completed, %d failed]",
										toolStats.activeCalls, toolStats.completedCalls, toolStats.failedCalls)
									toolStats.mu.RUnlock()

									// Print completion status with enhanced summary
									fmt.Printf("\n%s\n",
										style.InfoBoxStyle.Render(fmt.Sprintf("%s %s %s (%0.1fs) %s",
											statusEmoji,
											style.ToolNameStyle.Render(te.name),
											style.ToolCompleteStyle.Render(strings.ToUpper(completionStatus)),
											duration,
											style.ToolStatsStyle.Render(updatedStatsStr))))

									// Print error summary if failed
									if te.failed {
										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("⚠️"),
											style.ErrorStyle.Render("Tool encountered errors during execution"))
									}

									// Print output summary with better formatting
									if te.output.Len() > 0 {
										outputSize := te.output.Len()
										var sizeStr string
										if outputSize < 1024 {
											sizeStr = fmt.Sprintf("%d bytes", outputSize)
										} else if outputSize < 1024*1024 {
											sizeStr = fmt.Sprintf("%.1f KB", float64(outputSize)/1024)
										} else {
											sizeStr = fmt.Sprintf("%.1f MB", float64(outputSize)/(1024*1024))
										}

										outputSummary := fmt.Sprintf("Output: %s on runner://%s", sizeStr, te.runner)
										if te.outputTruncated {
											outputSummary += " (output was truncated)"
										}

										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("📊"),
											style.ToolSummaryStyle.Render(outputSummary))
									} else if te.outputTruncated {
										fmt.Printf("%s\n", style.DimStyle.Render(
											"💡 Set KUBIYA_DEBUG=1 to see full output"))
									}

									// Remove redundant divider line for cleaner output
								}
								delete(toolExecutions, msgID)
							}
						}
					}

					// Regular chat message
					if msg.SenderName != "You" {
						buf, exists := messageBuffer[msg.MessageID]
						if !exists {
							buf = &chatBuffer{}
							messageBuffer[msg.MessageID] = buf
						}

						if len(msg.Content) > len(buf.content) {
							newContent := msg.Content[len(buf.content):]

							// Accumulate content and handle code blocks
							for _, char := range newContent {
								if char == '`' {
									buf.inCodeBlock = !buf.inCodeBlock
									if buf.inCodeBlock {
										// Print accumulated sentence before code block
										if buf.sentence.Len() > 0 {
											sentence := strings.TrimSpace(buf.sentence.String())
											if sentence != "" {
												// Print without [Bot] prefix
												fmt.Printf("%s\n",
													style.AgentStyle.Render(sentence))
												buf.sentence.Reset()
											}
										}
									} else {
										// Print accumulated code block
										if buf.codeBlock.Len() > 0 {
											fmt.Printf("%s\n%s\n%s\n",
												style.CodeBlockStyle.Render("```"),
												style.CodeBlockStyle.Render(buf.codeBlock.String()),
												style.CodeBlockStyle.Render("```"))
											buf.codeBlock.Reset()
										}
									}
									continue
								}

								if buf.inCodeBlock {
									buf.codeBlock.WriteRune(char)
								} else {
									buf.sentence.WriteRune(char)
									if char == '.' || char == '!' || char == '?' || char == '\n' {
										sentence := strings.TrimSpace(buf.sentence.String())
										if sentence != "" {
											// Print without [Bot] prefix
											fmt.Printf("%s\n",
												style.AgentStyle.Render(sentence))
											buf.sentence.Reset()
										}
									}
								}
							}

							buf.content = msg.Content
						}
					}

					if msg.Final {
						// Print any remaining content in the sentence buffer
						if buf, exists := messageBuffer[msg.MessageID]; exists {
							remaining := strings.TrimSpace(buf.sentence.String())
							if remaining != "" {
								fmt.Printf("%s\n",
									style.AgentStyle.Render(remaining))
							}
							// Also handle any remaining code block
							if buf.codeBlock.Len() > 0 {
								fmt.Printf("%s\n%s\n%s\n",
									style.CodeBlockStyle.Render("```"),
									style.CodeBlockStyle.Render(buf.codeBlock.String()),
									style.CodeBlockStyle.Render("```"))
							}

							// Check for agent error messages and trigger session recovery if needed
							fullContent := buf.content
							if isAgentErrorMessage(fullContent) {
								// Agent indicated an internal error - trigger session recovery
								if streamRetryCount < retries {
									streamRetryCount++
									backoffDelay := calculateBackoffDelay(streamRetryCount - 1)
									
									if !automationMode {
										fmt.Printf("\n%s\n", style.WarningStyle.Render(
											fmt.Sprintf("⚠️  Agent error detected (attempt %d/%d): %s",
												streamRetryCount, retries, "Agent indicated internal issue")))
										fmt.Printf("%s\n", style.SpinnerStyle.Render(
											fmt.Sprintf("🔄 Starting new session in %.1fs...", backoffDelay.Seconds())))
									}

									// Wait before retry
									time.Sleep(backoffDelay)

									// Start new session with original prompt by triggering session recovery
									sessionRecoveryNeeded = true
									hasError = true
									break // Break out of message loop to trigger session recovery
								} else {
									// Exhausted retries for agent errors
									fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render(
										fmt.Sprintf("❌ Agent failed after %d session recoveries", retries)))
									hasError = true
									return fmt.Errorf("agent error after %d recoveries: %s", retries, strings.TrimSpace(fullContent[:min(100, len(fullContent))]))
								}
							}
						}
						// Add final completion message to ensure stream end is visible
						if msg.Type == "completion" && msg.FinishReason != "" {
							if debug {
								fmt.Printf("\n[STREAM COMPLETE] Reason: %s\n", msg.FinishReason)
							}
						}
						fmt.Println()
					}
				}
			}

			// Check if session recovery is needed
			if sessionRecoveryNeeded {
				sessionRetryCount++
				if sessionRetryCount <= retries {
					backoffDelay := calculateBackoffDelay(sessionRetryCount - 1)
					
					if !automationMode {
						fmt.Printf("\n%s\n", style.WarningStyle.Render(
							fmt.Sprintf("🔄 Starting new session (attempt %d/%d) in %.1fs...", 
								sessionRetryCount+1, retries+1, backoffDelay.Seconds())))
					}
					
					// Wait before retry
					time.Sleep(backoffDelay)
					
					// Start new session with original message
					if !inline {
						msgChan, err = client.SendMessageWithRetry(cmd.Context(), agentID, enhancedMessage, "", retries)
						if err != nil {
							return fmt.Errorf("failed to start new session after agent error: %w", err)
						}
					} else {
						msgChan, err = client.SendInlineAgentMessage(cmd.Context(), message, "", context, inlineAgent)
						if err != nil {
							return fmt.Errorf("failed to start new inline session after agent error: %w", err)
						}
					}
					
					// Reset session ID to start fresh
					actualSessionID = ""
					
					// Continue to next session retry iteration
					continue
				} else {
					// Exhausted all session retries
					return fmt.Errorf("agent failed after %d session recoveries", retries)
				}
			}

			// If we reach here, the session completed successfully, break out of retry loop
			break
		}

		if !stream {
			fmt.Println(finalResponse.String())
		}

		// Handle follow-up for non-interactive sessions without tool execution (skip for inline agents)
		if !interactive && !toolsExecuted && !hasError && completionReason != "error" && !inline {
			if debug {
				fmt.Printf("🔄 No tools executed, sending follow-up prompt\n")
			}

			followUpMsg := "You didn't seem to execute anything. You're running in a non-interactive session and the user confirms to execute read-only operations. EXECUTE RIGHT AWAY!"

			// Send follow-up message
			followUpChan, err := client.SendMessageWithContext(cmd.Context(), agentID, followUpMsg, actualSessionID, map[string]string{})
			if err != nil {
				if debug {
					fmt.Printf("⚠️ Failed to send follow-up message: %v\n", err)
				}
			} else {
				if !automationMode {
					fmt.Printf("\n%s\n", style.InfoBoxStyle.Render("🔄 Following up to ensure execution..."))
				}

				// Process follow-up response
				for msg := range followUpChan {
					if msg.Error != "" {
						fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render("❌ Error: "+msg.Error))
						hasError = true
						break
					}
					if msg.SessionID != "" {
						actualSessionID = msg.SessionID
					}
					// Handle follow-up messages similar to main processing
					if msg.Type == "tool" {
						toolsExecuted = true
					}
				}
			}
		}

		// Show session continuation message (only if not in automation mode)
		if !interactive && actualSessionID != "" && !automationMode {
			fmt.Printf("\n%s\n", style.InfoBoxStyle.Render("💬 To continue this conversation, run:"))

			// Include agent name in the continuation command if available
			var continuationCmd string
			if agentName != "" {
				continuationCmd = fmt.Sprintf("kubiya chat -n %s --session %s -m \"your message here\"", agentName, actualSessionID)
			} else if agentID != "" {
				continuationCmd = fmt.Sprintf("kubiya chat -t %s --session %s -m \"your message here\"", agentID, actualSessionID)
			} else {
				continuationCmd = fmt.Sprintf("kubiya chat --session %s -m \"your message here\"", actualSessionID)
			}

			fmt.Printf("%s\n", style.HighlightStyle.Render(continuationCmd))
			fmt.Println()
		}

		// Show simple debug tip if any output was truncated
		if anyOutputTruncated && !interactive && !automationMode {
			fmt.Printf("\n%s\n", style.DimStyle.Render("💡 Use KUBIYA_DEBUG=1 to see full tool outputs"))
		}

		// Handle completion status and exit codes
		if debug {
			fmt.Printf("🔍 Completion reason: %s, hasError: %v, toolsExecuted: %v\n", completionReason, hasError, toolsExecuted)
			// Debug tool execution states
			for msgID, te := range toolExecutions {
				fmt.Printf("🔍 Tool %s (ID: %s): failed=%v, isComplete=%v, status=%s, errorMsg='%s'\n",
					te.name, msgID[:8], te.failed, te.isComplete, te.status, te.errorMsg)
			}
		}

		// Return proper exit code based on completion status
		// Check if we actually have any failed tools before exiting with error
		actuallyHasFailures := false
		for _, te := range toolExecutions {
			if te.failed && te.isComplete {
				actuallyHasFailures = true
				break
			}
		}

		if actuallyHasFailures {
			if debug {
				fmt.Printf("🚨 Exiting with error code 1 due to actual tool failures\n")
			}
			return fmt.Errorf("Some errors occurred during execution")
		} else if hasError && debug {
			// Debug: hasError was set but no actual failures
			fmt.Printf("⚠️  Warning: hasError=true but no actual tool failures detected - ignoring\n")
		}

		// Check for non-successful completion reasons
		switch completionReason {
		case "error":
			if debug {
				fmt.Printf("🚨 Exiting with error code 1 due to completion reason: %s\n", completionReason)
			}
			return fmt.Errorf("agent execution failed with reason: %s", completionReason)
		case "stop", "length", "":
			// Normal successful completion
			if debug {
				fmt.Printf("✅ Exiting with success code 0\n")
			}
			return nil
		default:
			// Unknown completion reason - log but don't fail
			if debug {
				fmt.Printf("⚠️ Unknown completion reason: %s, treating as success\n", completionReason)
			}
			return nil
		}
		},
	}

	cmd.Flags().StringVarP(&agentID, "agent", "t", "", "Agent ID (optional)")
	cmd.Flags().StringVarP(&agentName, "name", "n", "", "Agent name (optional)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().StringVarP(&promptFile, "prompt-file", "f", "", "File or URL containing the prompt (supports shell substitution, Go templates, and GitHub raw URLs)")
	cmd.Flags().StringArrayVar(&templateVars, "var", []string{}, "Template variables for Go templates in prompt files (KEY=VALUE format)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive chat mode")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	cmd.Flags().BoolVar(&stream, "stream", true, "Stream the response")
	cmd.Flags().BoolVar(&clearSession, "clear-session", false, "Clear the current session")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to resume")
	cmd.Flags().StringArrayVar(&contextFiles, "context", []string{}, "Files to include as context (supports wildcards and URLs)")
	cmd.Flags().BoolVar(&stdinInput, "stdin", false, "Read message from stdin")
	cmd.Flags().BoolVar(&sourceTest, "source-test", false, "Test source connection")
	cmd.Flags().StringVar(&sourceUUID, "source-uuid", "", "Source UUID")
	cmd.Flags().StringVar(&sourceName, "source-name", "", "Source name")
	cmd.Flags().StringVar(&suggestTool, "suggest-tool", "", "Suggest a tool to use")
	cmd.Flags().BoolVar(&noClassify, "no-classify", false, "Disable automatic agent classification")
	cmd.Flags().StringVar(&permissionLevel, "permission-level", "read", "Permission level for tool execution (read, readwrite, ask)")
	cmd.Flags().BoolVar(&showToolCalls, "show-tool-calls", true, "Show tool call execution details")
	cmd.Flags().IntVar(&retries, "retries", 15, "Number of automatic retries for connection/stream/agent errors (default: 15)")
	cmd.Flags().BoolVar(&silent, "silent", false, "Suppress progress updates for automation (can also use KUBIYA_AUTOMATION env var)")

	// Inline agent flags
	cmd.Flags().BoolVar(&inline, "inline", false, "Use inline agent mode")
	cmd.Flags().StringVar(&agentSpec, "agent-spec", "", "JSON file or URL containing complete agent specification (supports templating and GitHub raw URLs)")
	cmd.Flags().StringVar(&toolsFile, "tools-file", "", "JSON file or URL containing tools definition (supports templating and GitHub raw URLs)")
	cmd.Flags().StringVar(&toolsJSON, "tools-json", "", "JSON string containing tools definition (supports templating)")
	cmd.Flags().StringVar(&aiInstructions, "ai-instructions", "", "AI instructions for the inline agent")
	cmd.Flags().StringVar(&description, "description", "", "Description for the inline agent")
	cmd.Flags().StringArrayVar(&runners, "runners", []string{}, "Runners for the inline agent")
	cmd.Flags().StringArrayVar(&integrations, "integrations", []string{}, "Integrations for the inline agent")
	cmd.Flags().StringArrayVar(&secrets, "secrets", []string{}, "Secrets for the inline agent")
	cmd.Flags().StringArrayVar(&envVars, "env-vars", []string{}, "Environment variables for the inline agent (KEY=VALUE format)")
	cmd.Flags().StringVar(&llmModel, "llm-model", "", "LLM model for the inline agent")
	cmd.Flags().BoolVar(&isDebugMode, "debug-mode", false, "Enable debug mode for the inline agent")

	return cmd
}

// Add this helper function at package level:
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
