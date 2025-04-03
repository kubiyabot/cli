package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
)

func TestMcpUpdateCommandExecute(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{}
	kubiyaDir, _ := mcp.GetKubiyaDir()
	gatewayDir := filepath.Join(kubiyaDir, "mcp-gateway")

	// --- Initial Setup (Simulate installed state) ---
	oldSHA := "11111111111111111111"
	newSHA := "22222222222222222222"
	require.NoError(t, fs.MkdirAll(gatewayDir, 0750), "Failed to create mock gateway dir")
	require.NoError(t, mcp.StoreGatewaySHA(fs, oldSHA), "Failed to store initial SHA")

	// --- Test Case 1: Update Required ---
	t.Run("Update Required", func(t *testing.T) {
		startMockCommander()
		defer stopMockCommander()

		// Mock command outputs/errors
		addMockCommand("git fetch origin", "", nil) // No output needed for fetch success
		addMockCommand("git rev-parse origin/main", newSHA, nil)
		addMockCommand("git reset --hard origin/main", "HEAD is now at "+newSHA, nil)
		addMockCommand("uv sync", "Synchronized environment.", nil)

		// Execute Command
		cmd := newMcpUpdateCommand(cfg, fs)
		out := new(bytes.Buffer)
		errOut := new(bytes.Buffer)
		cmd.SetOut(out)
		cmd.SetErr(errOut)

		err := cmd.Execute()
		require.NoError(t, err, "mcp update should succeed when update is needed")
		output := out.String()

		// Assertions
		assert.Empty(t, errOut.String(), "Stderr should be empty on success")
		assert.Contains(t, output, "Checking for MCP server updates")
		assert.Contains(t, output, fmt.Sprintf("Reading local version... ✅ Found: %s", oldSHA[:7]))
		assert.Contains(t, output, "Fetching remote information... ✅ Done.")
		assert.Contains(t, output, fmt.Sprintf("Checking latest version... ✅ Latest: %s", newSHA[:7]))
		assert.Contains(t, output, fmt.Sprintf("Update available ( %s -> %s ). Updating...", oldSHA[:7], newSHA[:7]))
		assert.Contains(t, output, "Updating repository...")
		assert.Contains(t, output, "Reinstalling dependencies using uv...")
		assert.Contains(t, output, "Dependencies reinstalled.")
		assert.Contains(t, output, "Storing updated version information... ✅ Stored.")
		assert.Contains(t, output, "MCP server updated successfully!")

		// Check recorded command calls
		calls := getRecordedCommandCalls()
		require.Len(t, calls, 4, "Expected 4 commands to be called during update")
		assert.Equal(t, "git", calls[0].Name)
		assert.Equal(t, []string{"fetch", "origin"}, calls[0].Args)
		assert.Equal(t, gatewayDir, calls[0].Dir)

		assert.Equal(t, "git", calls[1].Name)
		assert.Equal(t, []string{"rev-parse", "origin/main"}, calls[1].Args)
		assert.Equal(t, gatewayDir, calls[1].Dir)

		assert.Equal(t, "git", calls[2].Name)
		assert.Equal(t, []string{"reset", "--hard", "origin/main"}, calls[2].Args)
		assert.Equal(t, gatewayDir, calls[2].Dir)

		assert.Equal(t, "uv", calls[3].Name)
		assert.Equal(t, []string{"sync"}, calls[3].Args)
		assert.Equal(t, gatewayDir, calls[3].Dir)

		// Check filesystem state (SHA file updated)
		finalSHA, err := mcp.ReadGatewaySHA(fs)
		require.NoError(t, err, "Should be able to read gateway SHA after update")
		assert.Equal(t, newSHA, finalSHA, "Stored SHA should match the new SHA")
	})

	// --- Test Case 2: Already Up-to-date ---
	// Reset SHA to newSHA (from previous test) before running this one
	require.NoError(t, mcp.StoreGatewaySHA(fs, newSHA), "Failed to set SHA for up-to-date test")
	t.Run("Already Up-to-date", func(t *testing.T) {
		startMockCommander()
		defer stopMockCommander()

		// Mock commands needed for check
		addMockCommand("git fetch origin", "", nil)
		addMockCommand("git rev-parse origin/main", newSHA, nil) // Remote SHA matches local

		// Execute Command
		cmd := newMcpUpdateCommand(cfg, fs)
		out := new(bytes.Buffer)
		errOut := new(bytes.Buffer)
		cmd.SetOut(out)
		cmd.SetErr(errOut)

		err := cmd.Execute()
		require.NoError(t, err, "mcp update should succeed when already up-to-date")
		output := out.String()

		// Assertions
		assert.Empty(t, errOut.String(), "Stderr should be empty")
		assert.Contains(t, output, "Checking for MCP server updates")
		assert.Contains(t, output, fmt.Sprintf("Reading local version... ✅ Found: %s", newSHA[:7]))
		assert.Contains(t, output, "Fetching remote information... ✅ Done.")
		assert.Contains(t, output, fmt.Sprintf("Checking latest version... ✅ Latest: %s", newSHA[:7]))
		assert.Contains(t, output, "MCP server is already up-to-date.")
		assert.NotContains(t, output, "Updating repository...") // Should not attempt update steps
		assert.NotContains(t, output, "Reinstalling dependencies...")

		// Check recorded command calls
		calls := getRecordedCommandCalls()
		require.Len(t, calls, 2, "Expected only 2 commands (fetch, rev-parse) when up-to-date")
		assert.Equal(t, "git", calls[0].Name)
		assert.Equal(t, []string{"fetch", "origin"}, calls[0].Args)
		assert.Equal(t, "git", calls[1].Name)
		assert.Equal(t, []string{"rev-parse", "origin/main"}, calls[1].Args)

		// Check filesystem state (SHA file unchanged)
		finalSHA, err := mcp.ReadGatewaySHA(fs)
		require.NoError(t, err, "Should be able to read gateway SHA")
		assert.Equal(t, newSHA, finalSHA, "Stored SHA should remain unchanged")
	})

	// TODO: Add tests for error cases:
	// - Gateway directory not found
	// - Cannot read local SHA
	// - git fetch fails
	// - git rev-parse fails
	// - git reset fails
	// - uv pip install fails -> uv sync fails
	// - Cannot store new SHA
}
