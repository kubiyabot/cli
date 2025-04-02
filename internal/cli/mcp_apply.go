package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2" // Use survey for the prompt
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya" // Need client for listing teammates
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero" // Add afero import
	"github.com/spf13/cobra"
)

// TemplateContext holds the data passed to the Go template during rendering.
type TemplateContext struct {
	ApiKey                      string
	TeammateUUIDsCommaSeparated string
	McpGatewayPath              string // Absolute path to the mcp-gateway checkout
	WrapperScriptPath           string // Path to the generated run script
	OS                          string // Add OS field
}

const wrapperScriptName = "run_mcp.sh"

// applyProviderConfiguration handles the logic of applying a single provider's configuration.
// It takes the list of teammate UUIDs to use.
func applyProviderConfiguration(providerName string, cfg *config.Config, fs afero.Fs, stdout, stderr io.Writer, teammateUUIDs []string) error {
	fmt.Fprintf(stdout, "Applying configuration for provider: %s...\n", providerName)

	// 1. Load Provider Config
	fmt.Fprint(stdout, "  Loading configuration... ")
	providerConfig, err := mcp.LoadProviderConfig(fs, providerName)
	if err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error loading provider config: %v\n", err)
		return err // Return error to caller
	}
	fmt.Fprintln(stdout, "✅ Loaded.")

	// 2. Check OS Compatibility
	currentOS := runtime.GOOS
	if providerConfig.OS != "" && providerConfig.OS != currentOS {
		fmt.Fprintf(stdout, "  🟡 Skipping: Provider '%s' requires OS '%s', but current OS is '%s'.\n",
			providerConfig.Name, providerConfig.OS, currentOS)
		return nil // Not an error, just skip
	}

	// 3. Gather Context Data
	fmt.Fprint(stdout, "  Gathering context data... ")
	var templateCtx TemplateContext

	// 3a. API Key
	templateCtx.ApiKey = cfg.APIKey

	// 3b. Teammate UUIDs (Comma-separated)
	commaSeparatedUUIDs := strings.Join(teammateUUIDs, ",")
	templateCtx.TeammateUUIDsCommaSeparated = commaSeparatedUUIDs

	// 3c. McpGatewayPath
	gatewayDir, err := mcp.GetMcpGatewayDir()
	if err != nil {
		fmt.Fprintln(stdout, "❌ Failed determining gateway path.")
		fmt.Fprintf(stderr, "  Error determining MCP Gateway path: %v\n", err)
		return err
	}
	if _, err := os.Stat(gatewayDir); os.IsNotExist(err) {
		fmt.Fprintln(stdout, "❌ Failed, gateway not found.")
		fmt.Fprintf(stderr, "  Error: MCP Gateway directory not found at %s. Run 'kubiya mcp install'.\n", gatewayDir)
		return fmt.Errorf("MCP gateway not installed")
	} else if err != nil {
		fmt.Fprintln(stdout, "❌ Failed checking gateway path.")
		fmt.Fprintf(stderr, "  Error checking MCP Gateway directory %s: %v\n", gatewayDir, err)
		return err
	}
	templateCtx.McpGatewayPath = gatewayDir

	// 3d. Find uv and create wrapper script
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintln(stdout, "❌ Failed, uv not found.")
			fmt.Fprintf(stderr, "  Error: 'uv' command not found in PATH. It's required by MCP Gateway.\n")
			return fmt.Errorf("uv not found")
		}
		fmt.Fprintln(stdout, "❌ Failed finding uv.")
		fmt.Fprintf(stderr, "  Error finding uv command: %v\n", err)
		return err
	}

	// Define wrapper script path and content
	wrapperScriptPath := filepath.Join(gatewayDir, wrapperScriptName)
	// Use absolute path for uv and gateway dir in the script
	// Ensure proper quoting for paths that might contain spaces
	wrapperScriptContent := fmt.Sprintf(`#!/bin/sh
# Wrapper script to run the Kubiya MCP Gateway

exec "%s" --directory "%s" run main.py server
`, uvPath, gatewayDir)

	// Write the wrapper script using afero and set executable permissions
	if err := afero.WriteFile(fs, wrapperScriptPath, []byte(wrapperScriptContent), 0750); err != nil {
		fmt.Fprintln(stdout, "❌ Failed writing wrapper script.")
		fmt.Fprintf(stderr, "  Error writing MCP wrapper script '%s': %v\n", wrapperScriptPath, err)
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}
	templateCtx.WrapperScriptPath = wrapperScriptPath // Store path for template

	templateCtx.OS = currentOS
	fmt.Fprintln(stdout, "✅ Context gathered.")

	// 4. Render Template
	fmt.Fprint(stdout, "  Rendering configuration template... ")
	tmpl, err := template.New(providerName).Parse(providerConfig.Template)
	if err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error parsing template for %s: %v\n", providerName, err)
		return fmt.Errorf("template parse error: %w", err)
	}

	var renderedOutput bytes.Buffer
	if err := tmpl.Execute(&renderedOutput, templateCtx); err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error executing template for %s: %v\n", providerName, err)
		return fmt.Errorf("template execute error: %w", err)
	}
	fmt.Fprintln(stdout, "✅ Rendered.")

	// 5. Determine Target Path
	fmt.Fprint(stdout, "  Resolving target file path... ")
	var targetPath string
	isCursor := strings.Contains(strings.ToLower(providerConfig.Name), "cursor")
	if isCursor {
		targetPath, err = mcp.ExpandTilde("~/.cursor/mcp.json")
	} else {
		if providerConfig.TargetFile == "" {
			return fmt.Errorf("provider '%s' requires 'target_file' in YAML", providerName)
		}
		targetPath, err = mcp.ExpandTilde(providerConfig.TargetFile)
	}
	if err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error resolving target file path '%s': %v\n", providerConfig.TargetFile, err)
		return err
	}
	fmt.Fprintf(stdout, "✅ Resolved: %s\n", targetPath)

	// 6. Ensure Target Directory Exists
	targetDir := filepath.Dir(targetPath)
	fmt.Fprintf(stdout, "  Ensuring target directory exists: %s... ", targetDir)
	if err := fs.MkdirAll(targetDir, 0750); err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error creating target directory '%s': %v\n", targetDir, err)
		return err
	}
	fmt.Fprintln(stdout, "✅ OK.")

	// 7. Write/Merge Target File
	fmt.Fprint(stdout, "  Writing/Merging target file... ")
	if isCursor {
		err = mergeMcpJson(fs, targetPath, renderedOutput.Bytes())
	} else {
		err = afero.WriteFile(fs, targetPath, renderedOutput.Bytes(), 0640)
	}
	if err != nil {
		fmt.Fprintln(stdout, "❌ Failed.")
		fmt.Fprintf(stderr, "  Error writing/merging target file '%s': %v\n", targetPath, err)
		return err
	}
	fmt.Fprintln(stdout, "✅ Written/Merged.")

	fmt.Fprintf(stdout, "✨ Successfully applied MCP configuration for %s!\n", providerConfig.Name)
	fmt.Fprintf(stdout, "   Target file updated: %s\n", targetPath)
	return nil
}

// Accept fs afero.Fs
func newMcpApplyCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	var interactive bool
	var nonInteractive bool
	var teammateUUIDs []string

	cmd := &cobra.Command{
		Use:   "apply <provider_name>",
		Short: "⚙️ Apply MCP configuration for a specific provider",
		Long: `Reads the provider's YAML configuration (e.g., claude_desktop.yaml), 
gathers context (API key, teammate IDs, paths), renders the template, 
and writes the target application's configuration file (e.g., for Claude Desktop).

Default behavior is interactive teammate selection unless --non-interactive is specified or specific --teammate-uuid flags are used.`,
		Args: cobra.ExactArgs(1), // Requires exactly one argument: the provider name
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAPIKey(cmd, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			providerName := args[0]
			client := kubiya.NewClient(cfg)

			var finalTeammateUUIDs []string

			// Determine teammate selection mode
			useExplicitUUIDs := cmd.Flags().Changed("teammate-uuid")
			useNonInteractive := nonInteractive
			fileInfo, _ := os.Stdout.Stat()
			isTty := (fileInfo.Mode() & os.ModeCharDevice) != 0
			useInteractive := interactive || (isTty && !useNonInteractive && !useExplicitUUIDs)

			if useExplicitUUIDs {
				// Use UUIDs provided via flags
				finalTeammateUUIDs = teammateUUIDs
				fmt.Fprintf(stdout, "Using %d teammate UUID(s) provided via flags.\n", len(finalTeammateUUIDs))
			} else if useInteractive {
				// Interactive selection
				fmt.Println("Fetching available teammates for selection...")
				teammates, err := client.ListTeammates(context.Background())
				if err != nil {
					fmt.Fprintf(stderr, "Error fetching teammates: %v\n", err)
					return fmt.Errorf("failed to list teammates: %w", err)
				}
				if len(teammates) == 0 {
					fmt.Fprintln(stderr, "Error: No teammates found in your workspace.")
					return fmt.Errorf("no teammates available to select")
				}

				options := make([]string, len(teammates))
				optionMap := make(map[string]string)
				for i, tm := range teammates {
					display := fmt.Sprintf("%s (%s)", tm.Name, tm.UUID)
					options[i] = display
					optionMap[display] = tm.UUID
				}

				var selectedOptions []string
				prompt := &survey.MultiSelect{
					Message:  "Select teammates to include in the MCP configuration:",
					Options:  options,
					Help:     `These teammates' context will be available via the local MCP server.`,
					PageSize: 10,
				}
				err = survey.AskOne(prompt, &selectedOptions, survey.WithKeepFilter(true))
				if err != nil {
					return fmt.Errorf("teammate selection failed: %w", err)
				}
				if len(selectedOptions) == 0 {
					return fmt.Errorf("no teammates selected")
				}
				for _, opt := range selectedOptions {
					finalTeammateUUIDs = append(finalTeammateUUIDs, optionMap[opt])
				}
				fmt.Fprintf(stdout, "Selected %d teammate(s).\n", len(finalTeammateUUIDs))
			} else {
				// Non-interactive default: Use all teammates
				fmt.Fprintln(stdout, "Non-interactive mode: Fetching all teammates...")
				teammates, err := client.ListTeammates(context.Background())
				if err != nil {
					fmt.Fprintf(stderr, "Error fetching all teammates: %v\n", err)
					return fmt.Errorf("failed to list all teammates: %w", err)
				}
				if len(teammates) == 0 {
					fmt.Fprintln(stderr, "Warning: No teammates found in workspace. MCP configuration will have no teammates.")
				}
				for _, tm := range teammates {
					finalTeammateUUIDs = append(finalTeammateUUIDs, tm.UUID)
				}
				fmt.Fprintf(stdout, "Using all %d available teammate(s).\n", len(finalTeammateUUIDs))
			}

			// Call the refactored core logic
			err := applyProviderConfiguration(providerName, cfg, fs, stdout, stderr, finalTeammateUUIDs)
			if err != nil {
				// Error is already logged within the helper function
				return fmt.Errorf("failed to apply configuration for provider %s", providerName)
			}

			return nil
		},
	}

	// Flags for explicit control
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Force interactive teammate selection (overrides TTY detection)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Force non-interactive mode (uses all teammates unless --teammate-uuid is specified)")
	cmd.Flags().StringSliceVar(&teammateUUIDs, "teammate-uuid", nil, "Specify teammate UUIDs directly (implies non-interactive)")

	return cmd
}

// mergeMcpJson reads the target file, merges the new server definition,
// and writes the result back using the provided filesystem.
func mergeMcpJson(fs afero.Fs, targetPath string, newMcpData []byte) error {
	// 1. Read existing file content using afero
	var existingData map[string]interface{}
	content, err := afero.ReadFile(fs, targetPath) // Use fs
	if err == nil {
		// File exists, try to unmarshal it
		if err := json.Unmarshal(content, &existingData); err != nil {
			// File exists but isn't valid JSON, or structure is unexpected. Overwrite might be safer?
			// Or return an error asking user to fix the file.
			fmt.Fprintf(os.Stderr, "Warning: Existing file %s is not valid JSON or has unexpected structure. Overwriting. Error: %v\n", targetPath, err)
			existingData = make(map[string]interface{}) // Reset to empty map for overwrite
		}
	} else if !os.IsNotExist(err) {
		// Error reading file (not a 'does not exist' error)
		return fmt.Errorf("failed to read existing file %s: %w", targetPath, err)
	} else {
		// File does not exist, initialize empty map
		existingData = make(map[string]interface{})
	}

	// 2. Unmarshal the new MCP server definition (from the template)
	var newData map[string]interface{}
	if err := json.Unmarshal(newMcpData, &newData); err != nil {
		return fmt.Errorf("failed to unmarshal new MCP data template: %w", err)
	}

	// 3. Get the new server list from the template output
	newServersRaw, ok := newData["mcpServers"] // Correct key is likely mcpServers (camelCase)
	if !ok {
		// Attempt snake_case as fallback, though template uses camelCase
		newServersRaw, ok = newData["mcp_servers"]
		if !ok {
			return fmt.Errorf("rendered template does not contain 'mcpServers' key")
		}
	}
	newServersContainer, ok := newServersRaw.(map[string]interface{}) // It's a map containing the server entry
	if !ok || len(newServersContainer) == 0 {
		// Check if it's maybe directly the server entry (less likely based on template)
		newServersContainer, ok = newServersRaw.(map[string]interface{})
		if !ok || len(newServersContainer) == 0 {
			return fmt.Errorf("rendered template 'mcpServers' is not a non-empty map")
		}
	}

	// Assuming the template defines one server under a key like "kubiya-local"
	var newServerName string
	var newServerEntry map[string]interface{}
	for key, val := range newServersContainer {
		newServerName = key
		newServerEntry, ok = val.(map[string]interface{})
		if ok {
			break // Found the first server entry
		}
	}
	if newServerName == "" || newServerEntry == nil {
		return fmt.Errorf("could not extract server name and details from rendered template's mcpServers")
	}

	// 4. Get or create the existing server map ('mcpServers')
	var existingServers map[string]interface{}
	if serversRaw, exists := existingData["mcpServers"]; exists {
		if serversMap, ok := serversRaw.(map[string]interface{}); ok {
			existingServers = serversMap
		} else {
			// mcpServers exists but is not a map. Overwrite it.
			fmt.Fprintf(os.Stderr, "Warning: Existing 'mcpServers' in %s is not a map. It will be overwritten.\n", targetPath)
			existingServers = make(map[string]interface{})
		}
	} else {
		existingServers = make(map[string]interface{})
	}

	// 5. Merge: Add/replace the entry with the new server name
	existingServers[newServerName] = newServerEntry

	// 6. Update the map and write back to file using afero
	existingData["mcpServers"] = existingServers
	finalJson, err := json.MarshalIndent(existingData, "", "  ") // Pretty print
	if err != nil {
		return fmt.Errorf("failed to marshal final merged JSON: %w", err)
	}

	if err := afero.WriteFile(fs, targetPath, finalJson, 0640); err != nil { // Use fs
		return fmt.Errorf("failed to write merged file %s: %w", targetPath, err)
	}

	return nil
}
