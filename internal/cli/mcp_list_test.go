package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
)

func TestMcpListCommandExecute(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{} // Not used by list, but needed for constructor

	// --- Filesystem Setup ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)
	require.NoError(t, fs.MkdirAll(providerDir, 0750), "Failed to create mock providers dir")

	// Create some dummy provider files
	providersToCreate := []string{"cursor_ide.yaml", "claude_desktop.yaml", "another-provider.yaml"}
	for _, fname := range providersToCreate {
		require.NoError(t, afero.WriteFile(fs, filepath.Join(providerDir, fname), []byte("name: test"), 0644), "Failed to write dummy provider file %s", fname)
	}
	// Create a non-yaml file that should be ignored
	require.NoError(t, afero.WriteFile(fs, filepath.Join(providerDir, "README.md"), []byte("readme"), 0644))

	// --- Execute Command ---
	cmd := newMcpListCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	err := cmd.Execute()

	// --- Assertions ---
	require.NoError(t, err, "mcp list should execute successfully")
	assert.Empty(t, errOut.String(), "Stderr should be empty on success")

	output := out.String()
	assert.Contains(t, output, "Available MCP Providers:")

	// Check that the expected providers (without .yaml) are listed
	assert.Contains(t, output, "- cursor_ide")
	assert.Contains(t, output, "- claude_desktop")
	assert.Contains(t, output, "- another-provider")

	// Check that the non-yaml file is NOT listed
	assert.NotContains(t, output, "README.md")
}

func TestMcpListCommandExecute_NoProviders(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{}

	// --- Filesystem Setup (Ensure provider dir exists but is empty) ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)
	require.NoError(t, fs.MkdirAll(providerDir, 0750), "Failed to create mock providers dir")

	// --- Execute Command ---
	cmd := newMcpListCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	err := cmd.Execute()

	// --- Assertions ---
	require.NoError(t, err, "mcp list should execute successfully even with no providers")
	assert.Empty(t, errOut.String(), "Stderr should be empty")

	output := out.String()
	assert.Contains(t, output, "Available MCP Providers:")
	assert.Contains(t, output, "No providers found in") // Check for the 'no providers' message
}

func TestMcpListCommandExecute_DirNotExist(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{}

	// --- Filesystem Setup (Provider dir does NOT exist) ---

	// --- Execute Command ---
	cmd := newMcpListCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	err := cmd.Execute()

	// --- Assertions ---
	// Expect an error because the directory it tries to read doesn't exist
	require.Error(t, err, "mcp list should return an error if the providers directory doesn't exist")
}
