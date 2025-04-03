package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
)

const mockProviderYAML = `
name: "Mock Cursor IDE"
os: "%s"
target_file: "~/.cursor/mcp.json"
template: |
  {
    "mcp.servers": [
      {
        "name": "kubiya-local",
        "command": "{{.UvPath}}",
        "args": [
          "run",
          "main.py",
          "server"
        ],
        "cwd": "{{.McpGatewayPath}}",
        "env": {
          "KUBIYA_API_KEY": "{{.ApiKey}}",
          "TEAMMATE_UUIDS": "{{.TeammateUUIDsJSON}}"
        }
      }
    ]
  }
`

func TestMcpApplyCommandExecute_NonInteractive(t *testing.T) {
	fs := afero.NewMemMapFs()
	cfg := &config.Config{APIKey: "test-api-key-123"}

	// --- Filesystem Setup ---
	kubiyaDir, _ := mcp.GetKubiyaDir()
	providerDir := filepath.Join(kubiyaDir, "providers")
	gatewayDir := filepath.Join(kubiyaDir, "mcp-gateway")
	providerFilePath := filepath.Join(providerDir, "cursor_ide.yaml")

	// Create necessary directories
	require.NoError(t, fs.MkdirAll(providerDir, 0750))
	require.NoError(t, fs.MkdirAll(gatewayDir, 0750)) // Simulate gateway installed

	// Write mock provider config, matching current OS
	formattedYAML := fmt.Sprintf(mockProviderYAML, runtime.GOOS)
	require.NoError(t, afero.WriteFile(fs, providerFilePath, []byte(formattedYAML), 0644))

	// Determine target path (handling tilde expansion manually for test)
	homeDir, _ := os.UserHomeDir()
	targetPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	targetDir := filepath.Dir(targetPath)

	// --- Mocking ---
	startMockCommander()
	defer stopMockCommander()

	mockUvPath := "/fake/path/to/uv"
	addMockLookPath("uv", mockUvPath, nil)

	// --- Execute Command ---
	cmd := newMcpApplyCommand(cfg, fs)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	// Run non-interactively
	cmd.SetArgs([]string{"cursor_ide", "--non-interactive"})
	err := cmd.Execute()

	// --- Assertions ---
	require.NoError(t, err, "mcp apply should succeed in non-interactive mode")
	output := out.String()
	errorOutput := errOut.String()
	assert.Empty(t, errorOutput, "Stderr should be empty on success")

	// Check output messages
	assert.Contains(t, output, "Applying MCP configuration for provider: cursor_ide")
	assert.Contains(t, output, "Loading configuration... ✅ Loaded.")
	assert.Contains(t, output, "Gathering context data...")
	assert.Contains(t, output, "Checking API Key... ✅ Found.")
	assert.Contains(t, output, "Fetching Teammate UUIDs... ✅ Using placeholder UUIDs (non-interactive mode).")
	assert.Contains(t, output, "Checking MCP Gateway installation... ✅ Found.")
	assert.Contains(t, output, "Checking uv command... ✅ Found.")
	assert.Contains(t, output, "Rendering configuration template... ✅ Rendered.")
	assert.Contains(t, output, fmt.Sprintf("Resolving target file path for Cursor (~/.cursor/mcp.json)... ✅ Resolved: %s", targetPath))
	assert.Contains(t, output, fmt.Sprintf("Ensuring target directory exists: %s... ✅ OK.", targetDir))
	assert.Contains(t, output, "✅ Written/Merged.")
	assert.Contains(t, output, "Successfully applied MCP configuration for Mock Cursor IDE")
	assert.Contains(t, output, fmt.Sprintf("Target file updated: %s", targetPath))

	// Check that no external commands were *executed*
	assert.Empty(t, getRecordedCommandCalls(), "No commands should be executed by apply (only LookPath)")

	// Check filesystem state (target file content)
	targetContentBytes, err := afero.ReadFile(fs, targetPath)
	require.NoError(t, err, "Should be able to read target file %s", targetPath)

	var targetJSON map[string]interface{}
	err = json.Unmarshal(targetContentBytes, &targetJSON)
	require.NoError(t, err, "Target file content should be valid JSON")

	// Deep check of the merged JSON structure
	serversRaw, ok := targetJSON["mcp.servers"]
	require.True(t, ok, "Target JSON should contain 'mcp.servers' key")
	serversList, ok := serversRaw.([]interface{})
	require.True(t, ok, "'mcp.servers' should be a list")
	require.Len(t, serversList, 1, "'mcp.servers' list should contain one server entry")

	serverEntry, ok := serversList[0].(map[string]interface{})
	require.True(t, ok, "Server entry should be a map")

	assert.Equal(t, "kubiya-local", serverEntry["name"], "Server name mismatch")
	assert.Equal(t, mockUvPath, serverEntry["command"], "Server command (uv path) mismatch")
	assert.Equal(t, gatewayDir, serverEntry["cwd"], "Server cwd (gateway path) mismatch")

	// Check args
	argsExpected := []interface{}{"run", "main.py", "server"}
	assert.Equal(t, argsExpected, serverEntry["args"], "Server args mismatch")

	// Check env vars
	envMap, ok := serverEntry["env"].(map[string]interface{})
	require.True(t, ok, "'env' should be a map")
	assert.Equal(t, cfg.APIKey, envMap["KUBIYA_API_KEY"], "API Key in env mismatch")

	// Check placeholder UUIDs were correctly marshalled and embedded
	placeholderUUIDs := []string{"PLACEHOLDER_UUID_1", "PLACEHOLDER_UUID_2"}
	placeholderJSONBytes, _ := json.Marshal(placeholderUUIDs)
	assert.Equal(t, string(placeholderJSONBytes), envMap["TEAMMATE_UUIDS"], "Teammate UUIDs JSON string mismatch")

	// TODO: Add tests for:
	// - Interactive mode (requires mocking survey and kubiya API client)
	// - OS mismatch error
	// - Provider file not found
	// - Invalid provider YAML
	// - Missing API key (should be caught by PreRunE)
	// - Gateway not installed error
	// - uv not found error
	// - Template parsing/execution errors
	// - Target file path resolution errors
	// - Filesystem errors (MkdirAll, WriteFile, ReadFile)
	// - JSON merging edge cases (existing invalid JSON, existing servers)
}

// TODO: Add tests for:
// - Interactive mode (requires mocking API client and survey)
// - Other providers (e.g., claude_desktop)
// - Error cases:
//   - Provider config not found
//   - Provider config invalid YAML or missing template/target
//   - OS mismatch
//   - API Key missing
//   - Gateway dir not found
//   - uv not found (mock LookPath to return ErrNotFound)
//   - Template parsing error
//   - Template execution error
//   - Cannot create target directory
//   - Cannot write/merge target file
//   - Merging with existing invalid JSON file
