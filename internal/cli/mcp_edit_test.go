package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
)

// Helper to get the default editor for testing
func getDefaultEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vim" // Default to vim for Unix-like systems
		}
	}
	return editor
}

func TestMcpEditCommandExecute(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{} // Not used by edit, but needed for constructor

	// --- Filesystem Setup ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)
	require.NoError(t, fs.MkdirAll(providerDir, 0750), "Failed to create mock providers dir")

	providerName := "my-test-provider"
	providerFilename := providerName + ".yaml"
	providerFilePath := filepath.Join(providerDir, providerFilename)

	// Create the provider file
	require.NoError(t, afero.WriteFile(fs, providerFilePath, []byte("name: test"), 0644), "Failed to write dummy provider file")

	// --- Mocking Setup ---
	startMockCommander()
	defer stopMockCommander()

	// Mock the editor command execution
	editorCmd := getDefaultEditor()
	// We don't care about output/error, just that it was called
	addMockCommand(editorCmd+" "+providerFilePath, "", nil)
	// Mock LookPath for the editor itself (assume it exists)
	addMockLookPath(editorCmd, "/usr/bin/"+editorCmd, nil)

	// --- Execute Command ---
	cmd := newMcpEditCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{providerName})

	err := cmd.Execute()

	// --- Assertions ---
	require.NoError(t, err, "mcp edit should execute successfully")
	assert.Empty(t, errOut.String(), "Stderr should be empty on success")

	output := out.String()
	assert.Contains(t, output, fmt.Sprintf("Opening provider configuration file for '%s' in editor (%s): %s", providerName, editorCmd, providerFilePath))

	// Check recorded command calls
	calls := getRecordedCommandCalls()
	require.Len(t, calls, 1, "Expected 1 command (editor) to be called")
	assert.Equal(t, editorCmd, calls[0].Name)
	assert.Equal(t, []string{providerFilePath}, calls[0].Args)
	assert.Empty(t, calls[0].Dir, "Editor command should not have a specific directory set")
}

func TestMcpEditCommandExecute_ProviderNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{}

	// --- Filesystem Setup (Provider dir exists, but file does not) ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)
	require.NoError(t, fs.MkdirAll(providerDir, 0750), "Failed to create mock providers dir")

	providerName := "non-existent-provider"

	// --- Execute Command ---
	cmd := newMcpEditCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{providerName})

	err := cmd.Execute()

	// --- Assertions ---
	require.Error(t, err, "mcp edit should return an error if provider file not found")
	assert.Contains(t, err.Error(), "provider configuration file not found")
	assert.Contains(t, errOut.String(), "Error: provider configuration file not found")

	// Check no commands were called
	calls := getRecordedCommandCalls()
	assert.Empty(t, calls, "No commands should be called if provider file not found")
}

func TestMcpEditCommandExecute_EditorNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{}

	// --- Filesystem Setup ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)
	require.NoError(t, fs.MkdirAll(providerDir, 0750))
	providerName := "my-test-provider"
	providerFilename := providerName + ".yaml"
	providerFilePath := filepath.Join(providerDir, providerFilename)
	require.NoError(t, afero.WriteFile(fs, providerFilePath, []byte("name: test"), 0644))

	// --- Mocking Setup ---
	startMockCommander()
	defer stopMockCommander()

	// Mock LookPath for the editor to return error
	editorCmd := getDefaultEditor()
	addMockLookPath(editorCmd, "", exec.ErrNotFound)

	// --- Execute Command ---
	cmd := newMcpEditCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{providerName})

	err := cmd.Execute()

	// --- Assertions ---
	require.Error(t, err, "mcp edit should return an error if editor command not found")
	assert.Contains(t, err.Error(), "failed to find editor")
	assert.Contains(t, err.Error(), editorCmd)
	assert.Contains(t, errOut.String(), fmt.Sprintf("Error: failed to find editor '%s' in PATH", editorCmd))

	// Check no commands were called
	calls := getRecordedCommandCalls()
	assert.Empty(t, calls, "No editor command should be called if editor not found")
}
