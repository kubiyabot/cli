package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newDatasetCodeCommand creates the code command group
func newDatasetCodeCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Code ingestion operations",
		Long: `Ingest code repositories into datasets for semantic search and analysis.

Supports local repositories with automatic language detection, dependency extraction,
and streaming upload for efficient processing of large codebases.`,
		Example: `  # Ingest local code repository
  kubiya memory dataset code ingest <dataset-id> /path/to/repo

  # Ingest with specific patterns
  kubiya memory dataset code ingest <dataset-id> /path/to/repo \
    --patterns "**/*.py,**/*.go" \
    --exclude "**/node_modules/**,**/__pycache__/**"

  # Check ingestion job status
  kubiya memory dataset code status <dataset-id> <job-id>`,
	}

	cmd.AddCommand(
		newDatasetCodeIngestCommand(cfg),
		newDatasetCodeStatusCommand(cfg),
	)

	return cmd
}

// newDatasetCodeIngestCommand creates the code ingest command
func newDatasetCodeIngestCommand(cfg *config.Config) *cobra.Command {
	var (
		patterns         []string
		excludePatterns  []string
		batchSize        int
		sessionDuration  int
		outputFormat     string
		showProgress     bool
	)

	cmd := &cobra.Command{
		Use:   "ingest <dataset-id> <path>",
		Short: "Ingest code repository into dataset",
		Long: `Ingest a local code repository into a dataset for semantic search.

The command will:
1. Traverse the repository and collect matching files
2. Extract metadata (dependencies, exports, functions, classes)
3. Upload files in batches with streaming
4. Trigger knowledge graph processing (cognify)

Supported languages: Python, JavaScript, TypeScript, Go, Java, Rust, C/C++, Ruby, PHP`,
		Example: `  # Ingest Python repository
  kubiya memory dataset code ingest abc-123 /path/to/python-repo

  # Ingest with custom patterns
  kubiya memory dataset code ingest abc-123 /path/to/repo \
    --patterns "**/*.py,**/*.js,**/*.ts" \
    --exclude "**/tests/**,**/__pycache__/**"

  # Ingest large repository with larger batches
  kubiya memory dataset code ingest abc-123 /path/to/large-repo \
    --batch-size 100 \
    --session-duration 240

  # JSON output
  kubiya memory dataset code ingest abc-123 /path/to/repo --output json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]
			repoPath := args[1]

			// Validate path exists
			if _, err := os.Stat(repoPath); os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", repoPath)
			}

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Default patterns if not specified
			if len(patterns) == 0 {
				patterns = []string{
					"**/*.py", "**/*.js", "**/*.jsx", "**/*.ts", "**/*.tsx",
					"**/*.go", "**/*.java", "**/*.rs", "**/*.rb", "**/*.php",
					"**/*.c", "**/*.cpp", "**/*.h", "**/*.hpp",
				}
			}

			// Default exclusions
			if len(excludePatterns) == 0 {
				excludePatterns = []string{
					"**/__pycache__/**", "**/*.pyc", "**/node_modules/**",
					"**/dist/**", "**/build/**", "**/.git/**", "**/.venv/**",
					"**/venv/**", "**/target/**", "**/vendor/**", "**/.next/**",
				}
			}

			if outputFormat != "json" && showProgress {
				fmt.Printf("🚀 Starting code ingestion\n")
				fmt.Printf("  Dataset: %s\n", style.HighlightStyle.Render(datasetID))
				fmt.Printf("  Path: %s\n", repoPath)
				fmt.Println()
			}

			// 1. Start session
			sessionReq := &entities.CodeStreamSessionCreate{
				SessionDurationMinutes: sessionDuration,
				Config: entities.CodeIngestionConfig{
					SourceType:         "local",
					BasePath:           repoPath,
					IncludedPatterns:   patterns,
					ExcludedPatterns:   excludePatterns,
					ExtractDependencies: true,
				},
			}

			session, err := client.StartCodeSession(datasetID, sessionReq)
			if err != nil {
				return fmt.Errorf("failed to start code session: %w", err)
			}

			if outputFormat != "json" && showProgress {
				fmt.Printf("✓ Session started: %s\n", style.DimStyle.Render(session.ID))
			}

			// 2. Collect and upload files
			files, err := collectCodeFiles(repoPath, patterns, excludePatterns)
			if err != nil {
				return fmt.Errorf("failed to collect files: %w", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("no files found matching patterns")
			}

			if outputFormat != "json" && showProgress {
				fmt.Printf("✓ Found %d files\n", len(files))
				fmt.Println()
			}

			// Upload in batches
			batchNum := 0
			processedFiles := 0

			for i := 0; i < len(files); i += batchSize {
				end := i + batchSize
				if end > len(files) {
					end = len(files)
				}

				batch := files[i:end]
				batchFiles := []entities.CodeFileUpload{}

				for _, filePath := range batch {
					content, err := os.ReadFile(filePath)
					if err != nil {
						if outputFormat != "json" && showProgress {
							fmt.Printf("  ⚠ Warning: Failed to read %s: %v\n", filePath, err)
						}
						continue
					}

					relPath, _ := filepath.Rel(repoPath, filePath)
					metadata := extractCodeMetadata(relPath, content)

					batchFiles = append(batchFiles, entities.CodeFileUpload{
						Content:  string(content),
						Metadata: metadata,
					})
				}

				if len(batchFiles) == 0 {
					continue
				}

				// Upload batch
				batchReq := &entities.CodeStreamBatchRequest{
					SessionID: session.ID,
					BatchID:   fmt.Sprintf("batch_%d", batchNum),
					Files:     batchFiles,
				}

				batchResp, err := client.UploadCodeBatch(datasetID, batchReq)
				if err != nil {
					return fmt.Errorf("failed to upload batch %d: %w", batchNum, err)
				}

				processedFiles += batchResp.Summary.Processed

				if outputFormat != "json" && showProgress {
					fmt.Printf("  Batch %d: Processed %d/%d files\n",
						batchNum+1,
						batchResp.Summary.Processed,
						batchResp.Summary.Total)
				}

				batchNum++
			}

			if outputFormat != "json" && showProgress {
				fmt.Println()
				fmt.Printf("✓ Uploaded %d files in %d batches\n", processedFiles, batchNum)
				fmt.Println()
			}

			// 3. Commit session
			commitResp, err := client.CommitCodeSession(datasetID, session.ID)
			if err != nil {
				return fmt.Errorf("failed to commit session: %w", err)
			}

			// Output results
			switch outputFormat {
			case "json":
				result := map[string]interface{}{
					"job_id":            commitResp.JobID,
					"status":            commitResp.Status,
					"processed_files":   processedFiles,
					"total_files":       len(files),
					"session_id":        session.ID,
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				result := map[string]interface{}{
					"job_id":          commitResp.JobID,
					"status":          commitResp.Status,
					"processed_files": processedFiles,
					"total_files":     len(files),
					"session_id":      session.ID,
				}
				data, _ := yaml.Marshal(result)
				fmt.Println(string(data))
			default:
				fmt.Printf("%s Code ingestion completed\n", style.SuccessStyle.Render("✓"))
				fmt.Printf("  Job ID: %s\n", style.HighlightStyle.Render(commitResp.JobID))
				fmt.Printf("  Status: %s\n", commitResp.Status)
				fmt.Printf("  Files Processed: %d/%d\n", processedFiles, len(files))
				fmt.Println()
				fmt.Printf("  %s Check status: %s\n",
					style.DimStyle.Render("💡"),
					style.DimStyle.Render(fmt.Sprintf("kubiya memory dataset code status %s %s", datasetID, commitResp.JobID)))
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&patterns, "patterns", nil, "File patterns to include (comma-separated)")
	cmd.Flags().StringSliceVar(&excludePatterns, "exclude", nil, "Patterns to exclude (comma-separated)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 50, "Number of files per batch (1-100)")
	cmd.Flags().IntVar(&sessionDuration, "session-duration", 120, "Session timeout in minutes")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")
	cmd.Flags().BoolVar(&showProgress, "progress", true, "Show progress updates")

	return cmd
}

// newDatasetCodeStatusCommand creates the code status command
func newDatasetCodeStatusCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		follow       bool
	)

	cmd := &cobra.Command{
		Use:   "status <dataset-id> <job-id>",
		Short: "Check code ingestion job status",
		Long:  `Check the status of a code ingestion job with detailed metrics.`,
		Example: `  # Check job status
  kubiya memory dataset code status abc-123 job_xyz

  # Follow status until completion
  kubiya memory dataset code status abc-123 job_xyz --follow

  # JSON output
  kubiya memory dataset code status abc-123 job_xyz --output json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]
			jobID := args[1]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Poll for status if following
			for {
				status, err := client.GetCodeJobStatus(datasetID, jobID)
				if err != nil {
					return fmt.Errorf("failed to get job status: %w", err)
				}

				// Output
				switch outputFormat {
				case "json":
					data, _ := json.MarshalIndent(status, "", "  ")
					fmt.Println(string(data))
				case "yaml":
					data, _ := yaml.Marshal(status)
					fmt.Println(string(data))
				default:
					fmt.Printf("📊 Code Ingestion Job Status\n\n")
					fmt.Printf("  Job ID: %s\n", style.HighlightStyle.Render(status.JobID))
					fmt.Printf("  Status: %s\n", status.Status)

					if status.TotalFiles > 0 {
						fmt.Printf("  Progress: %d/%d files (%.1f%%)\n",
							status.ProcessedFiles,
							status.TotalFiles,
							float64(status.ProcessedFiles)/float64(status.TotalFiles)*100)
					}

					if status.FailedFiles > 0 {
						fmt.Printf("  Failed: %d files\n", status.FailedFiles)
					}

					if len(status.FilesByLanguage) > 0 {
						fmt.Printf("\n  Files by Language:\n")
						for lang, count := range status.FilesByLanguage {
							fmt.Printf("    %s: %d\n", lang, count)
						}
					}

					if status.CognifyStatus != "" {
						fmt.Printf("\n  Cognify Status: %s\n", status.CognifyStatus)
					}

					if status.StartedAt != nil {
						fmt.Printf("\n  Started: %s\n", status.StartedAt.Format("2006-01-02 15:04:05"))
					}

					if status.CompletedAt != nil {
						fmt.Printf("  Completed: %s\n", status.CompletedAt.Format("2006-01-02 15:04:05"))
					}

					if len(status.Errors) > 0 {
						fmt.Printf("\n  Errors (%d):\n", len(status.Errors))
						for i, err := range status.Errors {
							if i >= 5 {
								fmt.Printf("    ... and %d more\n", len(status.Errors)-5)
								break
							}
							fmt.Printf("    - %v\n", err)
						}
					}
				}

				// Check if we should continue following
				if !follow || status.Status == "completed" || status.Status == "failed" || status.Status == "partial" {
					break
				}

				// Wait before next poll
				time.Sleep(2 * time.Second)
				if outputFormat == "" {
					fmt.Print("\033[2K\r") // Clear line
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow status until completion")

	return cmd
}

// collectCodeFiles collects files matching patterns
func collectCodeFiles(basePath string, patterns, excludePatterns []string) ([]string, error) {
	var files []string

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(basePath, path)

		// Check exclusions first
		for _, excl := range excludePatterns {
			matched, _ := filepath.Match(strings.ReplaceAll(excl, "**/", ""), relPath)
			if matched || strings.Contains(relPath, strings.TrimPrefix(strings.TrimSuffix(excl, "**"), "**/")) {
				return nil
			}
		}

		// Check inclusions
		for _, pattern := range patterns {
			matched, _ := filepath.Match(strings.TrimPrefix(pattern, "**/"), filepath.Base(path))
			if matched {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	return files, err
}

// extractCodeMetadata extracts metadata from code file
func extractCodeMetadata(filePath string, content []byte) entities.CodeFileMetadata {
	ext := filepath.Ext(filePath)
	language := detectLanguage(ext)

	// Calculate hash
	hash := sha256.Sum256(content)
	fileHash := hex.EncodeToString(hash[:])

	// Count lines of code
	lines := strings.Split(string(content), "\n")
	loc := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			loc++
		}
	}

	metadata := entities.CodeFileMetadata{
		FilePath:     filePath,
		Language:     language,
		SizeBytes:    len(content),
		LinesOfCode:  loc,
		Dependencies: []string{},
		Exports:      []string{},
		FileHash:     fileHash,
	}

	// Basic dependency/export extraction (simplified)
	contentStr := string(content)

	if language == "python" {
		// Extract Python imports
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "import ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					metadata.Dependencies = append(metadata.Dependencies, parts[1])
				}
			} else if strings.HasPrefix(line, "from ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					metadata.Dependencies = append(metadata.Dependencies, parts[1])
				}
			}
		}
	} else if language == "javascript" || language == "typescript" {
		// Extract JS/TS imports
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "import ") && strings.Contains(line, "from") {
				// Simple import extraction
				if idx := strings.Index(line, "from "); idx != -1 {
					rest := line[idx+5:]
					rest = strings.Trim(rest, " \"';")
					metadata.Dependencies = append(metadata.Dependencies, rest)
				}
			} else if strings.Contains(line, "require(") {
				// CommonJS require
				start := strings.Index(line, "require(") + 8
				end := strings.Index(line[start:], ")")
				if end > 0 {
					dep := strings.Trim(line[start:start+end], " \"'")
					metadata.Dependencies = append(metadata.Dependencies, dep)
				}
			}
		}
	}

	// Note: For production, this should use proper AST parsing like in the backend
	// This is a simplified version for the CLI
	_ = contentStr // Keep linter happy

	return metadata
}

// detectLanguage detects programming language from file extension
func detectLanguage(ext string) string {
	extMap := map[string]string{
		".py":   "python",
		".js":   "javascript",
		".jsx":  "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".go":   "go",
		".java": "java",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
	}

	if lang, ok := extMap[ext]; ok {
		return lang
	}
	return "unknown"
}
