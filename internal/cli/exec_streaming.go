package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/output"
	"github.com/kubiyabot/cli/internal/streaming"
	"github.com/mattn/go-isatty"
)

// StreamingOptions holds resolved streaming configuration
type StreamingOptions struct {
	// Enabled indicates whether streaming is enabled
	Enabled bool

	// Format is the resolved output format (text or json)
	Format streaming.StreamFormat

	// Verbose indicates whether to show detailed tool inputs/outputs
	Verbose bool

	// EventsWriter is where streaming events are written (typically stderr)
	EventsWriter io.Writer

	// ResultWriter is where the final result is written (typically stdout)
	ResultWriter io.Writer
}

// resolveStreamOptions determines streaming configuration based on flags and environment
// Streaming is enabled by default for better UX
func resolveStreamOptions(noStreamFlag bool, streamFormat string, verbose bool) StreamingOptions {
	opts := StreamingOptions{
		Enabled:      !noStreamFlag, // Streaming is ON by default, --no-stream disables it
		Verbose:      verbose,
		EventsWriter: os.Stderr,
		ResultWriter: os.Stdout,
	}

	// Resolve format
	opts.Format = resolveStreamFormat(streamFormat)

	return opts
}

// resolveStreamFormat determines the actual format to use
// Text format is the default for all environments (TTY, CI, pipes) for better readability
// Use --stream-format=json explicitly when programmatic parsing is needed
func resolveStreamFormat(explicit string) streaming.StreamFormat {
	switch explicit {
	case "text":
		return streaming.StreamFormatText
	case "json":
		return streaming.StreamFormatJSON
	case "auto", "":
		// Default to text for all environments - more readable in CI logs and terminals
		return streaming.StreamFormatText
	default:
		// Unknown format, default to text
		return streaming.StreamFormatText
	}
}

// ShouldAutoEnableStreaming returns true if streaming should be auto-enabled
func ShouldAutoEnableStreaming() bool {
	return output.IsCI() || !isatty.IsTerminal(os.Stderr.Fd())
}

// StreamingExecutor orchestrates streaming execution
type StreamingExecutor struct {
	options  StreamingOptions
	pipeline *streaming.EventPipeline
}

// NewStreamingExecutor creates a new streaming executor
func NewStreamingExecutor(options StreamingOptions) *StreamingExecutor {
	// Create renderer based on format
	var renderer streaming.StreamRenderer
	switch options.Format {
	case streaming.StreamFormatText:
		renderer = streaming.NewTextRenderer(options.EventsWriter, options.Verbose)
	case streaming.StreamFormatJSON:
		renderer = streaming.NewJSONRenderer(options.EventsWriter, options.Verbose)
	default:
		renderer = streaming.NewJSONRenderer(options.EventsWriter, options.Verbose)
	}

	// Create pipeline with filters
	pipeline := streaming.NewEventPipeline(renderer)

	// Add verbosity filter if not verbose (strip tool inputs/outputs)
	if !options.Verbose {
		pipeline.AddFilter(streaming.NewVerbosityFilter(false))
	}

	// Add deduplication filter to avoid repeated events
	pipeline.AddFilter(streaming.NewDeduplicationFilter())

	// Add timestamp filter to ensure all events have timestamps
	pipeline.AddFilter(streaming.NewTimestampFilter())

	return &StreamingExecutor{
		options:  options,
		pipeline: pipeline,
	}
}

// ProcessEvent processes a single control plane event through the pipeline
func (se *StreamingExecutor) ProcessEvent(cpEvent entities.StreamEvent) error {
	// Map control plane event to streaming event
	event := streaming.MapControlPlaneEvent(cpEvent)

	// Process through pipeline
	return se.pipeline.Process(event)
}

// ProcessStreamEvent processes a streaming.StreamEvent directly
func (se *StreamingExecutor) ProcessStreamEvent(event streaming.StreamEvent) error {
	return se.pipeline.Process(event)
}

// StreamExecution streams execution events from a channel
func (se *StreamingExecutor) StreamExecution(ctx context.Context, eventChan <-chan entities.StreamEvent, errChan <-chan error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err, ok := <-errChan:
			if !ok {
				// Error channel closed
				continue
			}
			if err != nil {
				// Log error event and return
				se.ProcessStreamEvent(streaming.NewErrorEvent(err.Error(), "", false))
				return err
			}

		case event, ok := <-eventChan:
			if !ok {
				// Event channel closed - stream complete
				return nil
			}

			// Process the event
			if err := se.ProcessEvent(event); err != nil {
				// Log but don't fail on render errors
				continue
			}

			// Check if this is a terminal event
			if event.IsTerminalEvent() {
				return nil
			}
		}
	}
}

// Flush ensures all buffered output is written
func (se *StreamingExecutor) Flush() error {
	return se.pipeline.Flush()
}

// Close cleans up resources
func (se *StreamingExecutor) Close() error {
	return se.pipeline.Close()
}

// IsStreamingEnabled returns true if streaming is enabled for this execution
// Streaming is enabled by default unless --no-stream flag is set
func (ec *ExecCommand) IsStreamingEnabled() bool {
	// Streaming is enabled by default, only disabled if --no-stream is set
	return !ec.NoStream
}

// GetStreamingOptions resolves and returns streaming options for this execution
func (ec *ExecCommand) GetStreamingOptions() StreamingOptions {
	return resolveStreamOptions(ec.NoStream, ec.StreamFormat, ec.Verbose)
}

// streamExecutionWithPipeline streams execution output using the streaming pipeline
func (ec *ExecCommand) streamExecutionWithPipeline(ctx context.Context, executionID string, opts StreamingOptions) error {
	executor := NewStreamingExecutor(opts)
	defer executor.Close()

	// Send connected event
	executor.ProcessStreamEvent(streaming.NewConnectedEvent(executionID))

	// Stream execution output from the control plane
	eventChan, errChan := ec.client.StreamExecutionOutput(ctx, executionID)

	// Process events through the streaming executor
	err := executor.StreamExecution(ctx, eventChan, errChan)
	if err != nil {
		return err
	}

	// Send done event
	executor.ProcessStreamEvent(streaming.NewDoneEvent())

	return executor.Flush()
}

// printStreamingInfo prints information about streaming mode
func printStreamingInfo(opts StreamingOptions) {
	if !opts.Enabled {
		return
	}

	format := "text"
	if opts.Format == streaming.StreamFormatJSON {
		format = "json"
	}

	fmt.Fprintf(opts.EventsWriter, "[STREAM] Streaming mode enabled (format: %s)\n", format)
	if opts.Verbose {
		fmt.Fprintln(opts.EventsWriter, "[STREAM] Verbose mode: showing tool inputs/outputs")
	}
}
