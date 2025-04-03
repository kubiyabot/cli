package cli

import (
	"os/exec"
	"sync"
)

// CommandExecutor defines the function signature for executing commands.
// Allows replacing exec.Command during tests.
type CommandExecutor func(name string, arg ...string) *exec.Cmd

// LookPathExecutor defines the function signature for looking up command paths.
type LookPathExecutor func(file string) (string, error)

// Package-level variables to hold the active command/lookup functions.
// These are swapped during tests.
var execCommand CommandExecutor
var execLookPath LookPathExecutor

// originalExecCommand holds the original exec.Command function.
var originalExecCommand = exec.Command

// originalLookPath holds the original exec.LookPath function.
var originalLookPath = exec.LookPath

// mockExecCommand is the function that replaces exec.Command during tests.
// It records calls and can return specific outputs or errors.
var mockExecCommand CommandExecutor

// mockLookPath is the function that replaces exec.LookPath during tests.
var mockLookPath LookPathExecutor

// commandCalls stores the details of commands that were supposed to be executed.
type commandCall struct {
	Name string
	Args []string
	Dir  string
}

var (
	recordedCmdCalls []commandCall
	muCmdCalls       sync.Mutex
	mockOutputs      map[string]string // Map command string to stdout output
	mockErrors       map[string]error  // Map command string to error
	mockPaths        map[string]string // Map command name to mock path
	mockPathErrors   map[string]error  // Map command name to mock LookPath error
)

// startMockCommander replaces exec.Command and exec.LookPath with mocking functions.
// It resets recorded calls and mock data.
func startMockCommander() {
	muCmdCalls.Lock()
	recordedCmdCalls = []commandCall{}
	mockOutputs = make(map[string]string)
	mockErrors = make(map[string]error)
	mockPaths = make(map[string]string)
	mockPathErrors = make(map[string]error)
	muCmdCalls.Unlock()

	// Define a simple mock function just for pointer comparison
	mockExecCommand = func(name string, arg ...string) *exec.Cmd {
		// This function's body doesn't strictly matter now,
		// as the logic is moved to runCommand/runCommandCapture.
		// It still needs to return a valid *exec.Cmd for type correctness.
		// We return the original command struct but won't call its Run method directly
		// within the helpers when mocking is active.
		return originalExecCommand(name, arg...)
	}

	// --- Mock exec.LookPath ---
	mockLookPath = func(file string) (string, error) {
		muCmdCalls.Lock()
		path, hasPath := mockPaths[file]
		err, hasError := mockPathErrors[file]
		muCmdCalls.Unlock()

		if hasError {
			return "", err // Return mock error
		}
		if hasPath {
			return path, nil // Return mock path
		}
		// Default behavior if not mocked: simulate not found
		return "", exec.ErrNotFound
	}

	// Replace the package-level variables used by the CLI commands
	execCommand = mockExecCommand
	execLookPath = mockLookPath
}

// stopMockCommander restores the original exec.Command and exec.LookPath functions.
func stopMockCommander() {
	execCommand = originalExecCommand
	execLookPath = originalLookPath
}

// addMockCommand sets the expected output and error for a specific command execution call.
func addMockCommand(cmdStr string, output string, err error) {
	muCmdCalls.Lock()
	defer muCmdCalls.Unlock()
	if output != "" {
		mockOutputs[cmdStr] = output
	}
	if err != nil {
		mockErrors[cmdStr] = err
	}
}

// addMockLookPath sets the expected path or error for a specific exec.LookPath call.
func addMockLookPath(cmdName string, path string, err error) {
	muCmdCalls.Lock()
	defer muCmdCalls.Unlock()
	if path != "" {
		mockPaths[cmdName] = path
	}
	if err != nil {
		mockPathErrors[cmdName] = err
	}
}

// getRecordedCommandCalls returns a copy of the recorded command calls.
func getRecordedCommandCalls() []commandCall {
	muCmdCalls.Lock()
	defer muCmdCalls.Unlock()
	// Return a copy to avoid race conditions
	callsCopy := make([]commandCall, len(recordedCmdCalls))
	copy(callsCopy, recordedCmdCalls)
	return callsCopy
}

// --- Modifications needed in command execution helpers ---
// Example modification for runCommandCapture (in mcp_install.go or helpers.go)
/*
var execCommand = exec.Command // Make exec.Command a variable

func runCommandCapture(cmd *exec.Cmd) (string, string, error) {
    // ... existing setup ...
    // Instead of creating cmd directly, assume it was created using execCommand
    err := cmd.Run() // Run should now be the mocked version during tests
    // ... existing return logic ...
}
*/
