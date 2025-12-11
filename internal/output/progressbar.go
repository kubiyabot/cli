package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ProgressBar provides a simple progress bar interface
type ProgressBar struct {
	total   int
	current int
	message string
	mode    OutputMode
	writer  io.Writer
	width   int
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, message string, mode OutputMode) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		message: message,
		mode:    mode,
		writer:  os.Stderr,
		width:   40, // Default width of the progress bar
	}
}

// Update updates the progress bar to the given value
func (pb *ProgressBar) Update(current int) {
	pb.current = current
	pb.render()
}

// Increment increments the progress bar by one
func (pb *ProgressBar) Increment() {
	pb.current++
	pb.render()
}

// SetMessage updates the progress bar message
func (pb *ProgressBar) SetMessage(message string) {
	pb.message = message
	pb.render()
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	pb.current = pb.total
	pb.render()
	fmt.Fprintf(pb.writer, "\n")
}

// render displays the progress bar
func (pb *ProgressBar) render() {
	if pb.mode == OutputModeCI {
		// CI mode: just show percentage updates at intervals
		percentage := int((float64(pb.current) / float64(pb.total)) * 100)
		if percentage%25 == 0 || pb.current == pb.total {
			fmt.Fprintf(pb.writer, "⏳ %s: %d%%\n", pb.message, percentage)
		}
		return
	}

	// Interactive mode: render actual progress bar
	percentage := float64(pb.current) / float64(pb.total)
	filledWidth := int(percentage * float64(pb.width))

	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", pb.width-filledWidth)

	// Use carriage return to overwrite the line
	fmt.Fprintf(pb.writer, "\r%s [%s] %d%%", pb.message, bar, int(percentage*100))
}
