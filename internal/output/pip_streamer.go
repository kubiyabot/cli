package output

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

// PipStreamer shows clean, live-updating output from pip installation
type PipStreamer struct {
	mode       OutputMode
	mu         sync.Mutex
	packages   []string
	hasError   bool
	fullOutput []string
	lastPrint  string
}

// NewPipStreamer creates a new pip output streamer
func NewPipStreamer(mode OutputMode) *PipStreamer {
	ps := &PipStreamer{
		mode:       mode,
		packages:   make([]string, 0),
		fullOutput: make([]string, 0),
	}

	if mode == OutputModeInteractive {
		// Print initial message
		fmt.Fprint(os.Stderr, "   📦 Preparing installation...")
	}

	return ps
}

// Write implements io.Writer interface
func (ps *PipStreamer) Write(p []byte) (n int, err error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}

	// Store full output for error display
	ps.fullOutput = append(ps.fullOutput, line)

	lineLower := strings.ToLower(line)

	// Only track package names, don't show every line
	if strings.Contains(lineLower, "collecting") {
		if pkg := extractPackageName(line); pkg != "" {
			ps.packages = append(ps.packages, pkg)
			ps.updateLine()
		}
	} else if strings.Contains(lineLower, "successfully installed") {
		// Extract installed packages
		installed := extractInstalledPackages(line)
		if len(installed) > 0 {
			ps.packages = installed
		}
		ps.updateLine()
	} else if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "warning") {
		ps.hasError = true
	}

	return len(p), nil
}

// updateLine updates the current line in place
func (ps *PipStreamer) updateLine() {
	if ps.mode != OutputModeInteractive {
		return
	}

	// Build status line
	var statusLine string
	pkgCount := len(ps.packages)

	if pkgCount == 0 {
		statusLine = "   📦 Preparing installation..."
	} else if pkgCount == 1 {
		statusLine = fmt.Sprintf("   📦 Installing %s...", ps.packages[0])
	} else {
		statusLine = fmt.Sprintf("   📦 Installing %d packages...", pkgCount)
	}

	// Clear the previous line and print new one
	// \r returns to start of line, then we print spaces to clear old text, then \r again and new text
	clearLine := "\r" + strings.Repeat(" ", len(ps.lastPrint)) + "\r"
	fmt.Fprint(os.Stderr, clearLine+statusLine)
	ps.lastPrint = statusLine
}

// Finish completes the streaming and shows final summary
func (ps *PipStreamer) Finish() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.mode == OutputModeInteractive {
		// Clear the updating line
		if ps.lastPrint != "" {
			clearLine := "\r" + strings.Repeat(" ", len(ps.lastPrint)) + "\r"
			fmt.Fprint(os.Stderr, clearLine)
		}

		// Show final message
		if ps.hasError {
			fmt.Fprintln(os.Stderr, "   ⚠️  Installation completed with warnings")
			// Show last few error lines
			lastLines := getLastNLines(ps.fullOutput, 3)
			for _, line := range lastLines {
				fmt.Fprintf(os.Stderr, "      %s\n", line)
			}
		} else if len(ps.packages) > 0 {
			// Show success with package summary
			summary := formatPackageSummary(ps.packages)
			fmt.Fprintf(os.Stderr, "   ✓ Installed: %s\n", summary)
		} else {
			fmt.Fprintln(os.Stderr, "   ✓ Installation complete")
		}
	}
}

// extractPackageName extracts package name from pip output
func extractPackageName(line string) string {
	// Match "Collecting package-name" or "Downloading package-name-1.2.3"
	re := regexp.MustCompile(`(?:Collecting|Downloading|Using cached)\s+([a-zA-Z0-9_-]+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractInstalledPackages extracts package names from "Successfully installed" line
func extractInstalledPackages(line string) []string {
	parts := strings.Split(line, "Successfully installed")
	if len(parts) < 2 {
		return nil
	}

	packages := []string{}
	items := strings.Fields(parts[1])
	for _, item := range items {
		// Remove version suffix (package-1.2.3 -> package)
		pkg := strings.Split(item, "-")[0]
		if pkg != "" {
			packages = append(packages, pkg)
		}
	}
	return packages
}

// formatPackageSummary creates a concise summary
func formatPackageSummary(packages []string) string {
	if len(packages) == 0 {
		return "dependencies"
	}
	if len(packages) <= 3 {
		return strings.Join(packages, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(packages[:2], ", "), len(packages)-2)
}

// getLastNLines returns the last N non-empty lines
func getLastNLines(lines []string, n int) []string {
	result := []string{}
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			result = append([]string{lines[i]}, result...)
		}
	}
	return result
}

// CreatePipWriter creates an io.Writer that streams pip output beautifully
func CreatePipWriter(mode OutputMode) io.Writer {
	if mode == OutputModeCI {
		// In CI mode, return stderr for full output
		return os.Stderr
	}

	streamer := NewPipStreamer(mode)

	// Create a pipe to capture output line by line
	pr, pw := io.Pipe()

	// Start a goroutine to read and stream
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			streamer.Write([]byte(line + "\n"))
		}
		streamer.Finish()
	}()

	return pw
}
