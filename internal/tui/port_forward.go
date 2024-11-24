package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubiyabot/cli/internal/kubectl"
	"github.com/kubiyabot/cli/internal/portforward"
)

func (s *SourceBrowser) setupPortForward() tea.Cmd {
	return func() tea.Msg {
		// Cancel existing port-forward if any
		if s.portForward.cancel != nil {
			s.portForward.cancel()
		}

		// Create cancelable context with 15 second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		s.portForward.cancel = cancel

		s.execution.output = "⚡ Setting up connection to tool manager... (timeout: 15s)\n"

		// Get current context if not set
		currentCtx, err := kubectl.GetCurrentContext()
		if err != nil {
			return portForwardMsg{err: fmt.Errorf("Failed to get current context: %w", err)}
		}
		s.execution.context = currentCtx
		s.execution.output += fmt.Sprintf("Using Kubernetes context: %s\n", s.execution.context)

		// Initialize port forwarder with port 5001 instead of 80
		pf, err := portforward.NewPortForwarder("kubiya", "tool-manager", 5001, 5001)
		if err != nil {
			if strings.Contains(err.Error(), "No tool-manager pods found") {
				return portForwardMsg{err: fmt.Errorf(`
❌ No tool-manager pods found in namespace 'kubiya'

This usually means:
1. The Kubiya runner is not installed in this cluster
2. The deployment is not running
3. You might be using the wrong context

Current context: %s

To fix this:
1. Check if the runner is installed:
   kubectl get deployment -n kubiya tool-manager

2. If not installed, visit:
   https://docs.kubiya.ai/runner/installation`, s.execution.context)}
			}
			return portForwardMsg{err: fmt.Errorf("Failed to setup port forward: %w", err)}
		}

		// Set context
		if err := pf.SetContext(s.execution.context); err != nil {
			return portForwardMsg{err: fmt.Errorf("Failed to set context '%s': %w", s.execution.context, err)}
		}

		s.execution.output += "Starting port forward...\n"

		// Create error channel for the goroutine
		errChan := make(chan error, 1)

		// Start port forwarding in a goroutine
		go func() {
			if err := pf.Start(ctx); err != nil {
				errChan <- err
				return
			}
			close(errChan)
		}()

		// Wait for port forward to be ready
		select {
		case <-pf.Ready():
			s.portForward.forwarder = pf
			s.portForward.ready = true
			s.execution.output += "✅ Connection established\n"
			return portForwardMsg{ready: true}

		case err := <-errChan:
			pf.Stop()
			if err != nil {
				return portForwardMsg{err: fmt.Errorf("Failed to start port forward: %w", err)}
			}
			return portForwardMsg{err: fmt.Errorf("Port forward failed to start")}

		case err := <-pf.Errors():
			pf.Stop()
			return portForwardMsg{err: fmt.Errorf("Port forward error: %w", err)}

		case <-ctx.Done():
			pf.Stop()
			return portForwardMsg{err: fmt.Errorf("Timeout waiting for port forward to be ready (15s)")}
		}
	}
}
