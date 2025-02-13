package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

type outputFormat string

const (
	formatText outputFormat = "text"
	formatJSON outputFormat = "json"
	formatYAML outputFormat = "yaml"
)

const (
	loadingFrames = `🧠 ⚡️ 💭 ✨` // Brain thinking animation frames
)

// GenerateToolUI is the Bubble Tea model (if you choose to run it interactively).
type GenerateToolUI struct {
	client        *kubiya.Client
	description   string
	sessionID     string
	messages      []kubiya.ToolGenerationChatMessage
	toolPreview   *kubiya.Tool
	spinner       spinner.Model
	viewport      viewport.Model
	width         int
	height        int
	err           error
	ready         bool
	format        outputFormat
	saved         bool
	showHelp      bool
	footer        string
	progress      progress.Model
	statusMsg     string
	P             *tea.Program
	cancel        context.CancelFunc
	loadingFrame  int
	targetDir     string
	generatedTool []struct {
		Content  string `json:"content"`
		FileName string `json:"file_name"`
	}
	buffer      strings.Builder
	fileBuffers map[string]*strings.Builder
}

type fileBuffer struct {
	content  strings.Builder
	fileName string
}

// newGenerateToolCommand returns the Cobra command for `generate-tool`.
func newGenerateToolCommand(cfg *config.Config) *cobra.Command {
	var (
		sessionID   string
		targetDir   string
		debug       bool
		fileBuffers = make(map[string]*fileBuffer)
	)

	cmd := &cobra.Command{
		Use:   "generate-tool",
		Short: "🛠️ Generate a new tool from description",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("tool description is required")
			}

			// Generate a session ID if not provided
			if sessionID == "" {
				sessionID = fmt.Sprintf("gen-%d", time.Now().Unix())
			}

			// If no --target directory is given, use the current working directory
			if targetDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
				targetDir = cwd
			}

			// Create session directory
			sessionDir := filepath.Join(targetDir, sessionID)
			if err := os.MkdirAll(sessionDir, 0755); err != nil {
				return fmt.Errorf("failed to create session directory: %w", err)
			}
			fmt.Printf("📁 Using session directory: %s\n", sessionDir)

			// Create debug log file if debug mode is enabled
			var debugLog *os.File
			var err error
			if debug {
				debugLogPath := filepath.Join(sessionDir, "debug.log")
				debugLog, err = os.Create(debugLogPath)
				if err != nil {
					return fmt.Errorf("failed to create debug log: %w", err)
				}
				fmt.Printf("📝 Debug log: %s\n", debugLogPath)

				// Write initial debug info
				fmt.Fprintf(debugLog, "=== Generation Started at %s ===\n", time.Now().Format(time.RFC3339))
				fmt.Fprintf(debugLog, "Description: %s\n", args[0])
				fmt.Fprintf(debugLog, "Session ID: %s\n", sessionID)
				fmt.Fprintf(debugLog, "Target Dir: %s\n\n", sessionDir)

				// Ensure writes are flushed
				_ = debugLog.Sync()
			}

			description := args[0]
			fmt.Printf("🎯 Generating tool from description: %s\n", description)
			fmt.Printf("🔑 Using session ID: %s\n", sessionID)

			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			fmt.Println("\n🚀 Starting tool generation...")
			messagesChan, err := client.GenerateTool(ctx, description, sessionID)
			if err != nil {
				return fmt.Errorf("failed to start generation: %w", err)
			}

			filesCreated := false

			// Receive SSE messages and write files
			for msg := range messagesChan {
				if debug {
					// Log raw message if debug is enabled
					fmt.Printf("=== Message Received at %s ===\n", time.Now().Format(time.RFC3339))
					fmt.Printf("Type: %s\n", msg.Type)
					fmt.Printf("GeneratedToolContent: %+v\n", msg.GeneratedToolContent)
				}

				// Skip empty messages
				if len(msg.GeneratedToolContent) == 0 {
					continue
				}

				if msg.Type == "error" {
					return fmt.Errorf("generation error: %s", msg.GeneratedToolContent[0].Content)
				}

				// Handle file content messages
				for _, content := range msg.GeneratedToolContent {
					// Skip if no filename or if filename is incomplete
					if content.FileName == "" || !strings.Contains(content.FileName, ".") {
						continue
					}

					// Get or create buffer for this file
					buf, exists := fileBuffers[content.FileName]
					if !exists {
						buf = &fileBuffer{
							fileName: content.FileName,
						}
						fileBuffers[content.FileName] = buf
					}

					// Clean up content by removing any partial message artifacts
					cleanContent := content.Content
					if strings.HasPrefix(cleanContent, "from kubiya_sdk") {
						// This is a complete file content, replace existing buffer
						buf.content.Reset()
						buf.content.WriteString(cleanContent)
					} else if strings.HasPrefix(cleanContent, "FileName:") ||
						strings.HasPrefix(cleanContent, "Content:") {
						// Skip metadata messages
						continue
					} else {
						// Append to existing content
						buf.content.WriteString(cleanContent)
					}
				}

				// Write all buffered files
				for _, buf := range fileBuffers {
					// Skip empty files
					if buf.content.Len() == 0 {
						continue
					}

					content := buf.content.String()
					// Skip if content is just metadata
					if strings.HasPrefix(content, "FileName:") {
						continue
					}

					fullPath := filepath.Join(sessionDir, buf.fileName)
					dir := filepath.Dir(fullPath)

					fmt.Printf("📥 Writing file: %s\n", buf.fileName)

					// Make sure the directory structure exists
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory %s: %w", dir, err)
					}

					// Write the file content to disk
					if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
						return fmt.Errorf("failed to write file %s: %w", fullPath, err)
					}

					fmt.Printf("✅ Created file: %s (%d bytes)\n", buf.fileName, len(content))
					filesCreated = true
				}
			}

			if !filesCreated {
				return fmt.Errorf("no files were created during tool generation")
			}

			fmt.Printf("\n✨ Tool generation completed successfully in: %s\n", sessionDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID for continuing previous generation")
	cmd.Flags().StringVarP(&targetDir, "target", "t", "", "Target directory for generated files (default: current directory)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	return cmd
}

// The following methods (Init, Update, View, etc.) are part of the Bubble Tea UI
// shown in your code snippet. If you do not actually run the TUI, you can ignore
// them. They're left here for completeness.

func (ui *GenerateToolUI) Run() error {
	p := tea.NewProgram(ui, tea.WithAltScreen())
	ui.P = p
	defer ui.cleanup()
	_, err := p.Run()
	return err
}

func (ui *GenerateToolUI) Init() tea.Cmd {
	ui.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
	)
	ui.fileBuffers = make(map[string]*strings.Builder)
	return tea.Batch(
		ui.spinner.Tick,
		ui.startGeneration,
	)
}

func (ui *GenerateToolUI) startGeneration() tea.Msg {
	ui.statusMsg = "🚀 Starting tool generation..."
	ctx, cancel := context.WithCancel(context.Background())
	ui.cancel = cancel

	go func() {
		messagesChan, err := ui.client.GenerateTool(ctx, ui.description, ui.sessionID)
		if err != nil {
			ui.P.Send(fmt.Errorf("failed to start generation: %w", err))
			return
		}

		for msg := range messagesChan {
			if msg.Type == "error" {
				for _, file := range msg.GeneratedToolContent {
					ui.P.Send(fmt.Errorf(file.Content))
					return
				}
			}
			ui.messages = append(ui.messages, msg)
			ui.P.Send(tickMsg(time.Now()))
		}

		ui.ready = true
		ui.P.Send(tickMsg(time.Now()))
	}()

	return ui.spinner.Tick()
}

type tickMsg time.Time

func (ui *GenerateToolUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		ui.err = msg
		return ui, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return ui, tea.Quit
		case "?":
			ui.showHelp = !ui.showHelp
		}

	case tea.WindowSizeMsg:
		ui.width = msg.Width
		ui.height = msg.Height
		ui.viewport.Width = msg.Width
		ui.viewport.Height = msg.Height - 4
		return ui, nil

	case tickMsg:
		if !ui.ready {
			return ui, tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
		}

	case kubiya.ToolGenerationChatMessage:
		ui.messages = append(ui.messages, msg)

		if msg.Type == "generated_tool" {
			fmt.Printf("📥 Received generated_tool message: %+v\n", msg)

			for _, file := range msg.GeneratedToolContent {
				filePath := filepath.Join(ui.targetDir, file.FileName)

				// Append the new content to the file
				err := appendToFile(filePath, file.Content)
				if err != nil {
					ui.err = fmt.Errorf("failed to write file %s: %w", filePath, err)
					return ui, tea.Quit
				}

				fmt.Printf("📝 Appending to file: %s\n", filePath)
			}
		} else if msg.Type == "message" {
			for _, file := range msg.GeneratedToolContent {
				fmt.Printf("📝 Received message: %s\n", file.Content)
			}
		} else if msg.Type == "error" {
			for _, file := range msg.GeneratedToolContent {
				ui.err = fmt.Errorf("error generating tool: %s", file.Content)
				return ui, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	ui.spinner, cmd = ui.spinner.Update(msg)
	return ui, cmd
}

func appendToFile(filePath, content string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return err
	}

	return nil
}

func (ui *GenerateToolUI) View() string {
	if ui.err != nil {
		return fmt.Sprintf("\n  ❌ Error: %v\n\nPress any key to exit...", ui.err)
	}

	if !ui.ready {
		frames := strings.Split(loadingFrames, " ")
		frame := frames[ui.loadingFrame%len(frames)]
		ui.loadingFrame++

		loadingStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 3)

		loading := fmt.Sprintf("%s Generating your tool...\n\n%s", frame, ui.spinner.View())
		if ui.statusMsg != "" {
			loading += "\n\n" + ui.statusMsg
		}

		return loadingStyle.Render(loading)
	}

	// If generation is ready, render final content (if you wish).
	return "Tool generation complete. Press 'q' to quit."
}

func (ui *GenerateToolUI) cleanup() {
	if ui.cancel != nil {
		ui.cancel()
	}
}

func (ui *GenerateToolUI) saveFiles() error {
	for filePath, buffer := range ui.fileBuffers {
		dir := filepath.Dir(filePath)

		fmt.Printf("💾 Saving file: %s\n", filePath)

		// Make sure the directory structure exists
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write the file content to disk
		if err := os.WriteFile(filePath, []byte(buffer.String()), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filePath, err)
		}

		fmt.Printf("✅ Created file: %s (%d bytes)\n", filePath, buffer.Len())
	}

	return nil
}
