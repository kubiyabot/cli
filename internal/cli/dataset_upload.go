package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newDatasetUploadCommand creates the upload command for adding knowledge to datasets
func newDatasetUploadCommand(cfg *config.Config) *cobra.Command {
	var (
		title        string
		tags         []string
		metadataJSON string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "upload <dataset-id> <file-or-dir>",
		Short: "Upload knowledge from local files to dataset",
		Long: `Upload knowledge from local files or directories to a dataset.

Supports:
  - Single files (text, markdown, json, etc.)
  - Directories (recursively uploads all files)
  - Automatic content type detection
  - Metadata tagging

The uploaded knowledge will be processed and made available for semantic search.`,
		Example: `  # Upload single file
  kubiya memory dataset upload abc-123 ./document.md --title "Project Documentation"

  # Upload with tags
  kubiya memory dataset upload abc-123 ./guide.md \
    --title "Setup Guide" \
    --tags production,setup,documentation

  # Upload directory
  kubiya memory dataset upload abc-123 ./docs/ --title "Documentation"

  # Upload with metadata
  kubiya memory dataset upload abc-123 ./config.json \
    --title "Configuration" \
    --metadata-json '{"env":"prod","version":"1.0"}'`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]
			path := args[1]

			// Check if path exists
			fileInfo, err := os.Stat(path)
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Parse metadata if provided
			var metadata map[string]interface{}
			if metadataJSON != "" {
				if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
					return fmt.Errorf("failed to parse metadata JSON: %w", err)
				}
			}

			if outputFormat != "json" {
				fmt.Printf("📤 Uploading knowledge to dataset\n")
				fmt.Printf("  Dataset: %s\n", style.HighlightStyle.Render(datasetID))
				fmt.Printf("  Path: %s\n", path)
				fmt.Println()
			}

			// Handle directory or single file
			var uploadedCount int
			if fileInfo.IsDir() {
				// Upload all files in directory
				uploadedCount, err = uploadDirectory(client, datasetID, path, title, tags, metadata, outputFormat != "json")
				if err != nil {
					return err
				}
			} else {
				// Upload single file
				if err := uploadSingleFile(client, datasetID, path, title, tags, metadata, outputFormat != "json"); err != nil {
					return err
				}
				uploadedCount = 1
			}

			// Output results
			switch outputFormat {
			case "json":
				result := map[string]interface{}{
					"dataset_id":     datasetID,
					"uploaded_count": uploadedCount,
					"status":         "success",
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				result := map[string]interface{}{
					"dataset_id":     datasetID,
					"uploaded_count": uploadedCount,
					"status":         "success",
				}
				data, _ := yaml.Marshal(result)
				fmt.Println(string(data))
			default:
				fmt.Println()
				fmt.Printf("%s Successfully uploaded %d file(s)\n", style.SuccessStyle.Render("✓"), uploadedCount)
				fmt.Printf("  Dataset ID: %s\n", style.DimStyle.Render(datasetID))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Title for the uploaded content")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for categorization (comma-separated)")
	cmd.Flags().StringVar(&metadataJSON, "metadata-json", "", "Additional metadata as JSON string")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

// uploadSingleFile uploads a single file as knowledge
func uploadSingleFile(client *controlplane.Client, datasetID, filePath, title string, tags []string, metadata map[string]interface{}, showProgress bool) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Use filename as title if not provided
	if title == "" {
		title = filepath.Base(filePath)
	}

	// Add file metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["filename"] = filepath.Base(filePath)
	metadata["file_path"] = filePath
	metadata["file_size"] = len(content)

	// Store memory
	req := &entities.MemoryStoreRequest{
		DatasetID: datasetID,
		Context: entities.MemoryContext{
			Title:   title,
			Content: string(content),
			Tags:    tags,
		},
		Metadata: metadata,
	}

	resp, err := client.StoreMemory(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	if showProgress {
		fmt.Printf("  ✓ Uploaded: %s (Memory ID: %s)\n", filepath.Base(filePath), style.DimStyle.Render(resp.MemoryID))
	}

	return nil
}

// uploadDirectory recursively uploads all files in a directory
func uploadDirectory(client *controlplane.Client, datasetID, dirPath, baseTitle string, tags []string, metadata map[string]interface{}, showProgress bool) (int, error) {
	uploadedCount := 0

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and certain extensions
		if shouldSkipFile(path) {
			return nil
		}

		// Generate title from relative path
		relPath, _ := filepath.Rel(dirPath, path)
		fileTitle := relPath
		if baseTitle != "" {
			fileTitle = baseTitle + " - " + relPath
		}

		// Upload file
		if err := uploadSingleFile(client, datasetID, path, fileTitle, tags, metadata, showProgress); err != nil {
			if showProgress {
				fmt.Printf("  ⚠ Warning: Failed to upload %s: %v\n", relPath, err)
			}
			return nil // Continue with other files
		}

		uploadedCount++
		return nil
	})

	return uploadedCount, err
}

// shouldSkipFile determines if a file should be skipped during upload
func shouldSkipFile(path string) bool {
	base := filepath.Base(path)

	// Skip hidden files
	if len(base) > 0 && base[0] == '.' {
		return true
	}

	// Skip binary and common non-text files
	skipExtensions := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".mp4": true, ".avi": true, ".mov": true,
		".mp3": true, ".wav": true,
		".pdf": true, // PDFs need special handling
		".pyc": true, ".class": true, ".o": true,
	}

	ext := filepath.Ext(path)
	return skipExtensions[ext]
}
