package types

// OutputMode represents the output style
type OutputMode int

const (
	// OutputModeInteractive shows progress bars, spinners, and emojis
	OutputModeInteractive OutputMode = iota
	// OutputModeCI shows plain text with timestamps, no spinners
	OutputModeCI
)
