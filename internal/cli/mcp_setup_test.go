package cli

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
)

func TestMcpSetupCommandExecute(t *testing.T) {
	// Use memory map filesystem for testing
	fs := afero.NewMemMapFs()
	kubiyaDir, _ := mcp.GetKubiyaDir()
	mcpDir := filepath.Join(kubiyaDir, mcp.DefaultMcpDirName)

	cfg := &config.Config{}

	// Create and execute the command
	cmd := newMcpSetupCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	// Execute first time
	err := cmd.Execute()
	require.NoError(t, err, "Command execution should succeed the first time")
	output := out.String()

	// Assert output contains expected messages
	assert.Contains(t, output, "Ensured MCP configuration directory exists")
	assert.Contains(t, output, "Created default configuration: cursor_ide.yaml")
	if runtime.GOOS == "darwin" { // Check Claude only on macOS
		assert.Contains(t, output, "Created default configuration: claude_desktop.yaml")
	} else {
		assert.NotContains(t, output, "claude_desktop.yaml")
	}
	assert.Contains(t, output, "MCP setup complete")
	assert.Empty(t, errOut.String(), "Stderr should be empty on success")

	// Assert files were created correctly
	defaultConfigs := mcp.GetDefaultConfigs()
	for filename, expectedContent := range defaultConfigs {
		targetPath := filepath.Join(mcpDir, filename)
		fileExists, _ := afero.Exists(fs, targetPath)
		assert.True(t, fileExists, "File %s should exist after setup", filename)
		actualContent, _ := afero.ReadFile(fs, targetPath)
		assert.Equal(t, expectedContent, string(actualContent), "Content of %s should match", filename)
	}

	// Execute second time (Idempotency check)
	out.Reset()
	errOut.Reset()
	err = cmd.Execute()
	require.NoError(t, err, "Command execution should succeed the second time")
	output2 := out.String()

	// Assert output indicates skipping
	assert.Contains(t, output2, "Ensured MCP configuration directory exists")
	assert.Contains(t, output2, "Skipping existing configuration: cursor_ide.yaml")
	if runtime.GOOS == "darwin" {
		assert.Contains(t, output2, "Skipping existing configuration: claude_desktop.yaml")
	}
	assert.NotContains(t, output2, "Created default configuration:") // Should not create again
	assert.Contains(t, output2, "All applicable default configurations already exist")
	assert.Empty(t, errOut.String(), "Stderr should be empty on second run")

	// Verify file content hasn't changed (double check)
	for filename, expectedContent := range defaultConfigs {
		targetPath := filepath.Join(mcpDir, filename)
		actualContent, _ := afero.ReadFile(fs, targetPath)
		assert.Equal(t, expectedContent, string(actualContent), "Content of %s should be unchanged", filename)
	}
}
