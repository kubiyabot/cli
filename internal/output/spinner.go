package output

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

// Spinner provides a simple interface for showing progress
type Spinner struct {
	spinner *spinner.Spinner
	mode    OutputMode
	message string
	writer  io.Writer
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string, mode OutputMode) *Spinner {
	s := &Spinner{
		mode:    mode,
		message: message,
		writer:  os.Stderr,
	}

	if mode == OutputModeInteractive {
		// Use an animated spinner (CharSet 14 is dots - smooth and clean!)
		s.spinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.spinner.Suffix = " " + message
		s.spinner.Writer = s.writer
		s.spinner.Color("blue", "bold")
	}

	return s
}

// Start starts the spinner
func (s *Spinner) Start() {
	if s.mode == OutputModeInteractive && s.spinner != nil {
		s.spinner.Start()
	} else {
		// CI mode: just print the message
		fmt.Fprintf(s.writer, "⏳ %s...\n", s.message)
	}
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.mode == OutputModeInteractive && s.spinner != nil {
		s.spinner.Stop()
	}
}

// Success stops the spinner and shows a success message
func (s *Spinner) Success(message string) {
	s.Stop()
	fmt.Fprintf(s.writer, "✓ %s\n", message)
}

// Fail stops the spinner and shows a failure message
func (s *Spinner) Fail(message string) {
	s.Stop()
	fmt.Fprintf(s.writer, "✗ %s\n", message)
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.message = message
	if s.mode == OutputModeInteractive && s.spinner != nil {
		s.spinner.Suffix = " " + message
	}
}
