package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newKnowledgeCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "knowledge",
		Aliases: []string{"k"},
		Short:   "📚 Manage knowledge base",
		Long:    `Create, read, update, and delete knowledge items in your Kubiya knowledge base.`,
	}

	cmd.AddCommand(
		newListKnowledgeCommand(cfg),
		newGetKnowledgeCommand(cfg),
		newCreateKnowledgeCommand(cfg),
		newUpdateKnowledgeCommand(cfg),
		newDeleteKnowledgeCommand(cfg),
		newEditKnowledgeCommand(cfg),
	)

	return cmd
}

func newListKnowledgeCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all knowledge items",
		Example: "  kubiya knowledge list\n  kubiya knowledge list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			items, err := client.ListKnowledge(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(items)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "📚 KNOWLEDGE BASE")
				fmt.Fprintln(w, "UUID\tNAME\tDESCRIPTION\tLABELS")
				for _, k := range items {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						k.UUID,
						k.Name,
						k.Description,
						strings.Join(k.Labels, ", "),
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

func newGetKnowledgeCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get [uuid]",
		Short:   "📖 Get knowledge item details",
		Example: "  kubiya knowledge get abc-123\n  kubiya knowledge get abc-123 --output json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			item, err := client.GetKnowledge(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(item)
			case "text":
				fmt.Printf("📚 Knowledge Item: %s\n\n", item.Name)
				fmt.Printf("Description: %s\n", item.Description)
				fmt.Printf("Labels: %s\n", strings.Join(item.Labels, ", "))
				fmt.Printf("\nContent:\n%s\n", item.Content)
				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreateKnowledgeCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		desc        string
		labels      []string
		groups      []string
		contentFile string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "📝 Create new knowledge item",
		Example: `  kubiya knowledge create --name "Redis Setup" --desc "How to setup Redis" --labels devops,redis --content-file redis.md
  kubiya knowledge create --name "AWS Tips" --desc "AWS best practices" --labels aws,cloud`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var content string
			if contentFile != "" {
				data, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("failed to read content file: %w", err)
				}
				content = string(data)
			}

			item := kubiya.Knowledge{
				Name:        name,
				Description: desc,
				Labels:      labels,
				Groups:      groups,
				Content:     content,
				Type:        "knowledge",
				Source:      "manual",
				Properties:  make(map[string]string),
			}

			client := kubiya.NewClient(cfg)
			created, err := client.CreateKnowledge(cmd.Context(), item)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Created knowledge item: %s (%s)\n", created.Name, created.UUID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Knowledge item name")
	cmd.Flags().StringVarP(&desc, "desc", "d", "", "Knowledge item description")
	cmd.Flags().StringSliceVarP(&labels, "labels", "l", nil, "Labels (comma-separated)")
	cmd.Flags().StringSliceVarP(&groups, "groups", "g", nil, "Groups (comma-separated)")
	cmd.Flags().StringVarP(&contentFile, "content-file", "f", "", "File containing the content")

	cmd.MarkFlagRequired("name")
	return cmd
}

func newUpdateKnowledgeCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		desc        string
		labels      []string
		groups      []string
		contentFile string
		useEditor   bool
		yes         bool
	)

	cmd := &cobra.Command{
		Use:   "update [uuid]",
		Short: "��️ Update knowledge item",
		Long: `Update a knowledge item by UUID.
		
By default, this will open your default editor (set by $EDITOR) to edit the content.
You can also provide content from a file using --content-file.`,
		Example: `  # Update using default editor (recommended)
  kubiya knowledge update abc-123

  # Update using specific editor flag
  kubiya knowledge update abc-123 --editor
  
  # Update content from a file without confirmation
  kubiya knowledge update abc-123 --content-file new.md --yes
  
  # Update metadata only
  kubiya knowledge update abc-123 --name "Updated Name" --desc "New description"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If neither content-file is specified nor editor is explicitly disabled,
			// default to using the editor
			if contentFile == "" && !cmd.Flags().Changed("editor") {
				useEditor = true
			}

			// Ensure we have at least one update method
			if !useEditor && contentFile == "" && name == "" && desc == "" && len(labels) == 0 && len(groups) == 0 {
				return fmt.Errorf("no updates specified. Use --editor (default) to edit content, --content-file to update from file, or provide metadata updates")
			}

			client := kubiya.NewClient(cfg)
			existing, err := client.GetKnowledge(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			var contentUpdated bool
			var oldContent string

			if useEditor {
				// Store old content for diff
				oldContent = existing.Content

				// Create temp file and edit
				tmpfile, err := createTempFileWithMetadata(existing)
				if err != nil {
					return err
				}
				defer os.Remove(tmpfile.Name())

				if err := runEditor(getEditor(), tmpfile.Name()); err != nil {
					return fmt.Errorf("editor failed: %w", err)
				}

				// Read and parse updated content
				content, err := os.ReadFile(tmpfile.Name())
				if err != nil {
					return fmt.Errorf("failed to read updated content: %w", err)
				}

				parts := strings.SplitN(string(content), "---\n", 3)
				if len(parts) != 3 {
					return fmt.Errorf("invalid file format: expected metadata and content sections")
				}

				contentStr := strings.TrimSpace(parts[2])
				if len(contentStr) == 0 {
					return fmt.Errorf("content cannot be empty")
				}

				existing.Content = contentStr
				contentUpdated = true

			} else if contentFile != "" {
				data, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("failed to read content file: %w", err)
				}
				if len(strings.TrimSpace(string(data))) == 0 {
					return fmt.Errorf("content file is empty")
				}
				existing.Content = string(data)
				contentUpdated = true
			}

			// Update fields if provided
			if name != "" {
				if len(strings.TrimSpace(name)) == 0 {
					return fmt.Errorf("name cannot be empty")
				}
				existing.Name = name
			}
			if desc != "" {
				if len(strings.TrimSpace(desc)) == 0 {
					return fmt.Errorf("description cannot be empty")
				}
				existing.Description = desc
			}
			if len(labels) > 0 {
				// Validate labels
				for _, label := range labels {
					if len(strings.TrimSpace(label)) == 0 {
						return fmt.Errorf("labels cannot be empty")
					}
				}
				existing.Labels = labels
			}
			if len(groups) > 0 {
				// Validate groups
				for _, group := range groups {
					if len(strings.TrimSpace(group)) == 0 {
						return fmt.Errorf("groups cannot be empty")
					}
				}
				existing.Groups = groups
			}

			// Show changes based on update method
			if useEditor && contentUpdated {
				fmt.Println("\n📝 Content Changes:")
				if err := showDiff(oldContent, existing.Content); err != nil {
					// Fallback to simple change indication if diff fails
					fmt.Println("Content has been modified")
				}

				fmt.Println("\nMetadata:")
				fmt.Printf("Name: %s\n", existing.Name)
				fmt.Printf("Description: %s\n", existing.Description)
				fmt.Printf("Labels: %v\n", existing.Labels)
				fmt.Printf("Groups: %v\n", existing.Groups)

				// Always ask for confirmation when using editor
				fmt.Print("\nDo you want to proceed? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("update cancelled")
				}
			} else if !yes {
				// Show simple changes and ask for confirmation if --yes is not set
				fmt.Println("\n📝 Review changes:")
				fmt.Printf("UUID: %s\n", existing.UUID)
				fmt.Printf("Name: %s\n", existing.Name)
				fmt.Printf("Description: %s\n", existing.Description)
				fmt.Printf("Labels: %v\n", existing.Labels)
				fmt.Printf("Groups: %v\n", existing.Groups)
				if contentUpdated {
					fmt.Println("Content: [Updated from file]")
				}

				fmt.Print("\nDo you want to proceed? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			// Perform update
			updated, err := client.UpdateKnowledge(cmd.Context(), args[0], *existing)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Updated knowledge item: %s\n", updated.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "New name")
	cmd.Flags().StringVarP(&desc, "desc", "d", "", "New description")
	cmd.Flags().StringSliceVarP(&labels, "labels", "l", nil, "New labels (comma-separated)")
	cmd.Flags().StringSliceVarP(&groups, "groups", "g", nil, "New groups (comma-separated)")
	cmd.Flags().StringVarP(&contentFile, "content-file", "f", "", "File containing new content")
	cmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Open content in editor (default if no other content update method specified)")
	cmd.Flags().BoolVar(&useEditor, "no-editor", false, "Disable opening in editor")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")

	cmd.MarkFlagsMutuallyExclusive("content-file", "editor")
	cmd.MarkFlagsMutuallyExclusive("no-editor", "editor")

	return cmd
}

// Helper function to create a temporary file with metadata and content
func createTempFileWithMetadata(item *kubiya.Knowledge) (*os.File, error) {
	tmpfile, err := os.CreateTemp("", "kubiya-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write metadata section
	metadata := fmt.Sprintf(`---
Name: %s
Description: %s
Labels: %v
Groups: %v
UUID: %s
---

`, item.Name, item.Description, item.Labels, item.Groups, item.UUID)

	if _, err := tmpfile.WriteString(metadata); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write content section
	if _, err := tmpfile.WriteString(item.Content); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("failed to write content: %w", err)
	}

	if err := tmpfile.Close(); err != nil {
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpfile, nil
}

// Helper function to get the editor command
func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	return "vim" // fallback
}

func newDeleteKnowledgeCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "delete [uuid]",
		Short:   "🗑️ Delete knowledge item",
		Example: "  kubiya knowledge delete abc-123",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			if err := client.DeleteKnowledge(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("✅ Deleted knowledge item: %s\n", args[0])
			return nil
		},
	}
}

func newEditKnowledgeCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "edit [uuid]",
		Short:   "✏️ Edit knowledge item in your default editor",
		Example: "  kubiya knowledge edit abc-123",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get existing item
			item, err := client.GetKnowledge(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Create temporary file
			tmpfile, err := os.CreateTemp("", "kubiya-*.md")
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			defer os.Remove(tmpfile.Name())

			// Write current content
			if _, err := tmpfile.WriteString(item.Content); err != nil {
				return fmt.Errorf("failed to write to temp file: %w", err)
			}
			tmpfile.Close()

			// Open in editor
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim" // fallback
			}

			if err := runEditor(editor, tmpfile.Name()); err != nil {
				return fmt.Errorf("editor failed: %w", err)
			}

			// Read updated content
			content, err := os.ReadFile(tmpfile.Name())
			if err != nil {
				return fmt.Errorf("failed to read updated content: %w", err)
			}

			// Update the item
			item.Content = string(content)
			updated, err := client.UpdateKnowledge(cmd.Context(), args[0], *item)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Updated knowledge item: %s\n", updated.Name)
			return nil
		},
	}
}

func runEditor(editor, filename string) error {
	cmd := exec.Command(editor, filename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Helper function to show diff between old and new content
func showDiff(old, new string) error {
	// Create temporary files for diff
	oldFile, err := os.CreateTemp("", "kubiya-old-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(oldFile.Name())

	newFile, err := os.CreateTemp("", "kubiya-new-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(newFile.Name())

	// Write content to files
	if err := os.WriteFile(oldFile.Name(), []byte(old), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(newFile.Name(), []byte(new), 0644); err != nil {
		return err
	}

	// Run diff command
	cmd := exec.Command("diff", "-u", oldFile.Name(), newFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ignore error as diff returns non-zero if files are different
	_ = cmd.Run()
	return nil
}
