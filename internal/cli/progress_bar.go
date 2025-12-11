package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kubiyabot/cli/internal/style"
)

// ProgressBar displays streaming progress for plan generation
type ProgressBar struct {
	current  int
	message  string
	stage    string
	mu       sync.Mutex
	lastDraw time.Time
	done     bool
}

// NewProgressBar creates a new progress bar
func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		current:  0,
		message:  "Initializing...",
		stage:    "",
		lastDraw: time.Now(),
		done:     false,
	}
}

// Update updates the progress bar with new progress and message
func (pb *ProgressBar) Update(progress int, message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.current = progress
	pb.message = message

	// Only redraw if enough time has passed (avoid flickering)
	now := time.Now()
	if now.Sub(pb.lastDraw) >= 100*time.Millisecond {
		pb.draw()
		pb.lastDraw = now
	}
}

// SetStage sets the current stage name
func (pb *ProgressBar) SetStage(stage string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.stage = stage
	pb.draw()
	pb.lastDraw = time.Now()
}

// Complete marks the progress as complete
func (pb *ProgressBar) Complete() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.done = true
	pb.current = 100
	pb.message = "Complete"
	pb.drawComplete()
}

// draw renders the progress bar
func (pb *ProgressBar) draw() {
	if pb.done {
		return
	}

	// Clear the line
	fmt.Print("\r\033[K")

	// Build progress bar
	barWidth := 30
	completed := int(float64(barWidth) * float64(pb.current) / 100.0)
	bar := strings.Repeat("█", completed) + strings.Repeat("░", barWidth-completed)

	// Format: [████░░░░] 45% - Analyzing resources...
	line := fmt.Sprintf("%s %s %3d%% - %s",
		style.ProgressBarStyle.Render("["+bar+"]"),
		style.ProgressDotStyle.Render("⏳"),
		pb.current,
		style.DimStyle.Render(pb.message))

	// Add stage if present
	if pb.stage != "" {
		line += style.DimStyle.Render(fmt.Sprintf(" (%s)", pb.stage))
	}

	fmt.Print(line)
}

// drawComplete draws the completion message
func (pb *ProgressBar) drawComplete() {
	// Clear the line
	fmt.Print("\r\033[K")

	// Show completion
	line := fmt.Sprintf("%s %s",
		style.SuccessStyle.Render("✓"),
		style.SuccessStyle.Render("Plan generation complete"))

	fmt.Println(line)
}

// ShowThinking shows a thinking indicator
func (pb *ProgressBar) ShowThinking(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	// Clear the line
	fmt.Print("\r\033[K")

	// Show thinking indicator
	fmt.Printf("  %s %s\n",
		style.DimStyle.Render("💭"),
		style.DimStyle.Render(message))
}

// ShowToolCall shows a tool call indicator
func (pb *ProgressBar) ShowToolCall(toolName string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	// Show tool call
	fmt.Printf("  %s Using tool: %s\n",
		style.RobotIconStyle.Render("🔧"),
		style.HighlightStyle.Render(toolName))
}

// ShowResourcesSummary shows a resources summary
func (pb *ProgressBar) ShowResourcesSummary(summary string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	// Show summary
	fmt.Printf("  %s %s\n",
		style.InfoStyle.Render("📊"),
		style.DimStyle.Render(summary))
}

// ShowError shows an error message
func (pb *ProgressBar) ShowError(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	// Clear the line
	fmt.Print("\r\033[K")

	// Show error
	fmt.Println(style.CreateErrorBox(message))
	pb.done = true
}

// Spinner creates a simple spinner animation
type Spinner struct {
	frames  []string
	current int
	message string
	stop    chan struct{}
	done    bool
	mu      sync.Mutex
}

// NewSpinner creates a new spinner
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		message: message,
		stop:    make(chan struct{}),
		done:    false,
	}
}

// Start starts the spinner animation
func (s *Spinner) Start() {
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.done {
					// Clear line and draw spinner
					fmt.Print("\r\033[K")
					frame := s.frames[s.current%len(s.frames)]
					fmt.Printf("%s %s",
						style.SpinnerStyle.Render(frame),
						style.DimStyle.Render(s.message))
					s.current++
				}
				s.mu.Unlock()
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.done {
		s.done = true
		close(s.stop)
		// Clear the line
		fmt.Print("\r\033[K")
	}
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}
