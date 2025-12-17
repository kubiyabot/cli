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
	"gopkg.in/yaml.v3"
)

func newMemoryCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memory",
		Aliases: []string{"mem"},
		Short:   "🧠 Manage cognitive memory",
		Long: `Store, recall, and manage contextual memories using the context graph.

Cognitive memory allows you to store and recall context using semantic search,
enabling your agents to remember and learn from past interactions.`,
		Example: `  # Store a memory
  kubiya memory store --title "AWS Setup" --content "Region: us-east-1"

  # Recall memories
  kubiya memory recall "AWS configuration"

  # List all memories
  kubiya memory list

  # Manage datasets
  kubiya memory dataset create --name "production-context" --scope org`,
	}

	cmd.AddCommand(
		newMemoryStoreCommand(cfg),
		newMemoryRecallCommand(cfg),
		newMemoryListCommand(cfg),
		newMemoryStatusCommand(cfg),
		newMemoryDatasetCommand(cfg),
	)

	return cmd
}

func newMemoryStoreCommand(cfg *config.Config) *cobra.Command {
	var (
		title        string
		content      string
		contentFile  string
		tags         []string
		metadataJSON string
		datasetID    string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "store",
		Short: "Store a new memory",
		Long: `Store contextual memory in the knowledge graph.

The memory will be processed using semantic embeddings and stored with
your organization context for later recall.`,
		Example: `  # Store a simple memory
  kubiya memory store --title "AWS Config" --content "Region: us-east-1"

  # Store with tags
  kubiya memory store \
    --title "Production DB" \
    --content "Connection string: postgres://..." \
    --tags production,database

  # Store from file
  kubiya memory store \
    --title "Deployment Guide" \
    --content-file ./deploy.md \
    --metadata-json '{"env":"prod","version":"1.2.3"}'

  # Store to specific dataset
  kubiya memory store \
    --title "Team Context" \
    --content "Team uses Slack for notifications" \
    --dataset-id abc-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			if datasetID == "" {
				return fmt.Errorf("--dataset-id is required")
			}

			// Read content from file if specified
			if contentFile != "" {
				data, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("failed to read content file: %w", err)
				}
				content = string(data)
			}

			if content == "" {
				return fmt.Errorf("--content or --content-file is required")
			}

			// Parse metadata JSON if provided
			var metadata map[string]interface{}
			if metadataJSON != "" {
				if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
					return fmt.Errorf("failed to parse metadata JSON: %w", err)
				}
			}

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Build request
			req := &entities.MemoryStoreRequest{
				DatasetID: datasetID,
				Context: entities.MemoryContext{
					Title:   title,
					Content: content,
					Tags:    tags,
				},
				Metadata: metadata,
			}

			// Store memory
			resp, err := client.StoreMemory(req)
			if err != nil {
				return fmt.Errorf("failed to store memory: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(resp)
				fmt.Println(string(data))
			default:
				fmt.Printf("%s Memory stored successfully\n", style.SuccessStyle.Render("✓"))
				fmt.Printf("  Memory ID: %s\n", style.HighlightStyle.Render(resp.MemoryID))
				fmt.Printf("  Status: %s\n", resp.Status)
				if resp.JobID != nil && *resp.JobID != "" {
					fmt.Printf("  Job ID: %s\n", *resp.JobID)
					fmt.Printf("\n  %s Check status: %s\n",
						style.DimStyle.Render("💡"),
						style.DimStyle.Render(fmt.Sprintf("kubiya memory status %s", *resp.JobID)))
				}
				if resp.Message != "" {
					fmt.Printf("  Message: %s\n", resp.Message)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Memory title (required)")
	cmd.Flags().StringVar(&content, "content", "", "Memory content")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "Read content from file")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags (comma-separated)")
	cmd.Flags().StringVar(&metadataJSON, "metadata-json", "", "Metadata as JSON string")
	cmd.Flags().StringVar(&datasetID, "dataset-id", "", "Dataset ID (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newMemoryRecallCommand(cfg *config.Config) *cobra.Command {
	var (
		query        string
		tags         []string
		topK         int
		minScore     float64
		searchType   string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "recall [query]",
		Short: "Recall memories using semantic search",
		Long: `Query stored memories using semantic search.

Returns memories ranked by similarity to your query. You can filter by tags
and set minimum similarity scores.`,
		Example: `  # Recall memories
  kubiya memory recall "AWS configuration"

  # Recall with filters
  kubiya memory recall "database setup" --tags production --top-k 5

  # Recall with minimum score
  kubiya memory recall "deployment process" --min-score 0.7

  # Recall with specific search type
  kubiya memory recall "kubernetes architecture" --search-type GRAPH_COMPLETION
  kubiya memory recall "recent changes" --search-type TEMPORAL
  kubiya memory recall "feedback" --search-type FEEDBACK

  # Output as JSON
  kubiya memory recall "kubernetes config" --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get query from args or flag
			if len(args) > 0 {
				query = args[0]
			}

			if query == "" {
				return fmt.Errorf("query is required (provide as argument or --query flag)")
			}

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Build request
			req := &entities.MemoryRecallRequest{
				Query:      query,
				Tags:       tags,
				TopK:       topK,
				MinScore:   minScore,
				SearchType: searchType,
			}

			// Recall memories
			resp, err := client.RecallMemory(req)
			if err != nil {
				return fmt.Errorf("failed to recall memories: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(resp)
				fmt.Println(string(data))
			default:
				fmt.Printf("🔍 Memory Recall Results (%d matches)\n\n", resp.Count)

				if len(resp.Results) == 0 {
					fmt.Println(style.DimStyle.Render("  No memories found matching your query"))
					return nil
				}

				for i, result := range resp.Results {
					// Display result number and score
					fmt.Printf("%s. Score: %s\n",
						style.BoldStyle.Render(fmt.Sprintf("%d", i+1)),
						style.HighlightStyle.Render(fmt.Sprintf("%.4f", result.SimilarityScore)))

					// Display node ID if available
					if result.NodeID != nil && *result.NodeID != "" {
						fmt.Printf("   Node ID: %s\n", style.DimStyle.Render(*result.NodeID))
					}

					// Display source
					if result.Source != "" {
						fmt.Printf("   Source: %s\n", style.DimStyle.Render(result.Source))
					}

					// Display metadata if present
					if len(result.Metadata) > 0 {
						if title, ok := result.Metadata["title"].(string); ok && title != "" {
							fmt.Printf("   Title: %s\n", style.HighlightStyle.Render(title))
						}
						if tags, ok := result.Metadata["tags"].([]interface{}); ok && len(tags) > 0 {
							tagStrs := make([]string, len(tags))
							for i, tag := range tags {
								tagStrs[i] = fmt.Sprintf("%v", tag)
							}
							fmt.Printf("   Tags: %s\n", strings.Join(tagStrs, ", "))
						}
					}

					// Show content preview (first 300 chars)
					content := result.Content
					if len(content) > 300 {
						content = content[:300] + "..."
					}
					fmt.Printf("   Content: %s\n", style.DimStyle.Render(content))

					if i < len(resp.Results)-1 {
						fmt.Println()
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Filter by tags (comma-separated)")
	cmd.Flags().IntVar(&topK, "top-k", 10, "Number of results to return")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.0, "Minimum similarity score (0.0-1.0)")
	cmd.Flags().StringVar(&searchType, "search-type", "", "Search type: GRAPH_COMPLETION, TEMPORAL, FEEDBACK, RAG_COMPLETION, CHUNKS")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newMemoryListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all stored memories",
		Long:  `List all memories stored in your organization's context graph.`,
		Example: `  # List all memories
  kubiya memory list

  # List as JSON
  kubiya memory list --output json

  # List as YAML
  kubiya memory list --output yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// List memories
			memories, err := client.ListMemories()
			if err != nil {
				return fmt.Errorf("failed to list memories: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(memories, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(memories)
				fmt.Println(string(data))
			default:
				fmt.Printf("🧠 Memories (%d)\n\n", len(memories))

				if len(memories) == 0 {
					fmt.Println(style.DimStyle.Render("  No memories found"))
					return nil
				}

				// Create table
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.BoldStyle.Render("TITLE\tMEMORY ID\tTAGS\tCREATED"))

				for _, memory := range memories {
					title := memory.Context.Title
					if len(title) > 40 {
						title = title[:37] + "..."
					}

					memID := memory.MemoryID
					if len(memID) > 30 {
						memID = memID[:27] + "..."
					}

					tags := strings.Join(memory.Context.Tags, ",")
					if len(tags) > 30 {
						tags = tags[:27] + "..."
					}

					created := ""
					if memory.CreatedAt != nil {
						created = memory.CreatedAt.Format("2006-01-02")
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						title,
						style.DimStyle.Render(memID),
						tags,
						created)
				}

				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml, table)")

	return cmd
}

func newMemoryStatusCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Check memory job status",
		Long:  `Check the status of an async memory processing job.`,
		Example: `  # Check job status
  kubiya memory status job_abc123

  # Output as JSON
  kubiya memory status job_abc123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get job status
			status, err := client.GetMemoryJobStatus(jobID)
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
				fmt.Printf("⚙️  Memory Job Status\n\n")
				fmt.Printf("  Job ID: %s\n", style.HighlightStyle.Render(status.JobID))
				fmt.Printf("  Status: %s\n", status.Status)

				if status.Progress > 0 {
					fmt.Printf("  Progress: %.1f%%\n", status.Progress*100)
				}

				if status.Message != "" {
					fmt.Printf("  Message: %s\n", status.Message)
				}

				if status.Error != "" {
					fmt.Printf("  Error: %s\n", style.ErrorStyle.Render(status.Error))
				}

				if status.CompletedAt != nil {
					fmt.Printf("  Completed: %s\n", status.CompletedAt.Format("2006-01-02 15:04:05"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newMemoryDatasetCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dataset",
		Aliases: []string{"ds"},
		Short:   "Manage cognitive datasets",
		Long: `Create and manage cognitive datasets for organizing memories.

Datasets allow you to organize memories into logical groups with
different access scopes (user, organization, or role-based).`,
		Example: `  # Create a dataset
  kubiya memory dataset create --name "production-context" --scope org

  # List datasets
  kubiya memory dataset list

  # Get dataset details
  kubiya memory dataset get abc-123

  # Delete dataset
  kubiya memory dataset delete abc-123`,
	}

	cmd.AddCommand(
		newDatasetCreateCommand(cfg),
		newDatasetListCommand(cfg),
		newDatasetGetCommand(cfg),
		newDatasetDeleteCommand(cfg),
		newDatasetPurgeCommand(cfg),
		newDatasetGetDataCommand(cfg),
		newDatasetCodeCommand(cfg),
		newDatasetUploadCommand(cfg),
	)

	return cmd
}

func newDatasetCreateCommand(cfg *config.Config) *cobra.Command {
	var (
		name         string
		description  string
		scope        string
		allowedRoles []string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new dataset",
		Long: `Create a new cognitive dataset for organizing memories.

Scopes:
  - user: Private to your user account
  - org: Shared across your organization
  - role: Accessible to specific roles`,
		Example: `  # Create user-scoped dataset
  kubiya memory dataset create --name "my-notes" --scope user

  # Create org-scoped dataset
  kubiya memory dataset create \
    --name "production-context" \
    --scope org \
    --description "Production environment context"

  # Create role-scoped dataset
  kubiya memory dataset create \
    --name "team-shared" \
    --scope role \
    --allowed-roles developer,devops`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			if scope == "" {
				return fmt.Errorf("--scope is required (user, org, or role)")
			}

			// Validate scope
			if scope != "user" && scope != "org" && scope != "role" {
				return fmt.Errorf("invalid scope: must be 'user', 'org', or 'role'")
			}

			// Validate allowed roles for role scope
			if scope == "role" && len(allowedRoles) == 0 {
				return fmt.Errorf("--allowed-roles is required for role scope")
			}

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Build request
			req := &entities.DatasetCreateRequest{
				Name:         name,
				Description:  description,
				Scope:        scope,
				AllowedRoles: allowedRoles,
			}

			// Create dataset
			dataset, err := client.CreateDataset(req)
			if err != nil {
				return fmt.Errorf("failed to create dataset: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(dataset, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(dataset)
				fmt.Println(string(data))
			default:
				fmt.Printf("%s Dataset created successfully\n", style.SuccessStyle.Render("✓"))
				fmt.Printf("  ID: %s\n", style.HighlightStyle.Render(dataset.ID))
				fmt.Printf("  Name: %s\n", dataset.Name)
				fmt.Printf("  Scope: %s\n", dataset.Scope)
				if len(dataset.AllowedRoles) > 0 {
					fmt.Printf("  Allowed Roles: %s\n", strings.Join(dataset.AllowedRoles, ", "))
				}
				if dataset.Description != "" {
					fmt.Printf("  Description: %s\n", dataset.Description)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Dataset name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Dataset description")
	cmd.Flags().StringVar(&scope, "scope", "", "Access scope: user, org, or role (required)")
	cmd.Flags().StringSliceVar(&allowedRoles, "allowed-roles", nil, "Allowed roles (required for role scope)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newDatasetListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all datasets",
		Long:  `List all cognitive datasets accessible to you.`,
		Example: `  # List datasets
  kubiya memory dataset list

  # List as JSON
  kubiya memory dataset list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// List datasets
			datasets, err := client.ListDatasets()
			if err != nil {
				return fmt.Errorf("failed to list datasets: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(datasets, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(datasets)
				fmt.Println(string(data))
			default:
				fmt.Printf("📁 Datasets (%d)\n\n", len(datasets))

				if len(datasets) == 0 {
					fmt.Println(style.DimStyle.Render("  No datasets found"))
					return nil
				}

				// Create table
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.BoldStyle.Render("NAME\tID\tSCOPE\tCREATED"))

				for _, dataset := range datasets {
					name := dataset.Name
					if len(name) > 30 {
						name = name[:27] + "..."
					}

					id := dataset.ID
					if len(id) > 25 {
						id = id[:22] + "..."
					}

					created := ""
					if dataset.CreatedAt != nil {
						created = dataset.CreatedAt.Format("2006-01-02")
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						name,
						style.DimStyle.Render(id),
						dataset.Scope,
						created)
				}

				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml, table)")

	return cmd
}

func newDatasetGetCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get <dataset-id>",
		Short: "Get dataset details",
		Long:  `Retrieve details for a specific dataset.`,
		Example: `  # Get dataset
  kubiya memory dataset get abc-123

  # Get as JSON
  kubiya memory dataset get abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get dataset
			dataset, err := client.GetDataset(datasetID)
			if err != nil {
				return fmt.Errorf("failed to get dataset: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(dataset, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(dataset)
				fmt.Println(string(data))
			default:
				fmt.Printf("📁 Dataset Details\n\n")
				fmt.Printf("  ID: %s\n", style.HighlightStyle.Render(dataset.ID))
				fmt.Printf("  Name: %s\n", dataset.Name)
				fmt.Printf("  Scope: %s\n", dataset.Scope)

				if dataset.Description != "" {
					fmt.Printf("  Description: %s\n", dataset.Description)
				}

				if len(dataset.AllowedRoles) > 0 {
					fmt.Printf("  Allowed Roles: %s\n", strings.Join(dataset.AllowedRoles, ", "))
				}

				if dataset.CreatedBy != "" {
					fmt.Printf("  Created By: %s\n", dataset.CreatedBy)
				}

				if dataset.CreatedAt != nil {
					fmt.Printf("  Created: %s\n", dataset.CreatedAt.Format("2006-01-02 15:04:05"))
				}

				if dataset.UpdatedAt != nil {
					fmt.Printf("  Updated: %s\n", dataset.UpdatedAt.Format("2006-01-02 15:04:05"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newDatasetDeleteCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <dataset-id>",
		Short: "Delete a dataset",
		Long:  `Delete a dataset and all its associated data.`,
		Example: `  # Delete dataset
  kubiya memory dataset delete abc-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Delete dataset
			if err := client.DeleteDataset(datasetID); err != nil {
				return fmt.Errorf("failed to delete dataset: %w", err)
			}

			fmt.Printf("%s Dataset deleted successfully\n", style.SuccessStyle.Render("✓"))
			fmt.Printf("  Dataset ID: %s\n", style.DimStyle.Render(datasetID))

			return nil
		},
	}

	return cmd
}

func newDatasetPurgeCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "purge <dataset-id>",
		Short: "Purge all data from a dataset",
		Long: `Clear all data items from a dataset while preserving the dataset container.

This operation removes all memories, knowledge, and data entries from the dataset
but keeps the dataset itself with its permissions and metadata intact.

This is useful when you want to refresh the dataset content without recreating it.`,
		Example: `  # Purge all data from a dataset
  kubiya memory dataset purge abc-123

  # Purge with JSON output
  kubiya memory dataset purge abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			if outputFormat != "json" {
				fmt.Printf("🗑️  Purging dataset data\n")
				fmt.Printf("  Dataset: %s\n", style.HighlightStyle.Render(datasetID))
				fmt.Println()
			}

			// Purge dataset data
			resp, err := client.PurgeDatasetData(datasetID)
			if err != nil {
				return fmt.Errorf("failed to purge dataset data: %w", err)
			}

			// Output results
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Println(string(data))
			default:
				fmt.Println()
				fmt.Printf("%s Purge initiated successfully\n", style.SuccessStyle.Render("✓"))
				fmt.Printf("  Dataset ID: %s\n", style.DimStyle.Render(resp.DatasetID))
				fmt.Printf("  Items to purge: %s\n", style.DimStyle.Render(fmt.Sprintf("%d", resp.EstimatedCount)))

				if resp.JobID != nil && *resp.JobID != "" {
					fmt.Printf("  Job ID: %s\n", style.DimStyle.Render(*resp.JobID))
					fmt.Printf("  Status: %s\n\n", style.HighlightStyle.Render(resp.Status))
					fmt.Printf("  💡 Track progress: kubiya memory status %s\n", *resp.JobID)
				} else {
					fmt.Printf("  Status: %s\n", style.SuccessStyle.Render("completed"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json)")

	return cmd
}

func newDatasetGetDataCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get-data <dataset-id>",
		Short: "Get dataset data entries",
		Long:  `Retrieve all data entries from a dataset.`,
		Example: `  # Get dataset data
  kubiya memory dataset get-data abc-123

  # Get as JSON
  kubiya memory dataset get-data abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]

			// Create client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get dataset data
			dataResp, err := client.GetDatasetData(datasetID)
			if err != nil {
				return fmt.Errorf("failed to get dataset data: %w", err)
			}

			// Output
			switch outputFormat {
			case "json":
				data, _ := json.MarshalIndent(dataResp, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				data, _ := yaml.Marshal(dataResp)
				fmt.Println(string(data))
			default:
				fmt.Printf("📊 Dataset Data (%d entries)\n\n", dataResp.Count)

				if len(dataResp.Data) == 0 {
					fmt.Println(style.DimStyle.Render("  No data entries found"))
					return nil
				}

				for i, entry := range dataResp.Data {
					fmt.Printf("%s. Entry ID: %s\n",
						style.BoldStyle.Render(fmt.Sprintf("%d", i+1)),
						style.HighlightStyle.Render(entry.ID))

					if entry.CreatedAt != nil {
						fmt.Printf("   Created: %s\n", entry.CreatedAt.Format("2006-01-02 15:04"))
					}

					// Show content preview
					contentJSON, _ := json.MarshalIndent(entry.Content, "   ", "  ")
					fmt.Printf("   Content:\n%s\n", string(contentJSON))

					if i < len(dataResp.Data)-1 {
						fmt.Println()
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}
