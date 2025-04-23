package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Accept fs afero.Fs
func newMcpUpdateCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	// fs is now passed in
	// fs := afero.NewOsFs()

	cmd := &cobra.Command{
		Use:   "update",
		Short: "🔄 Update the local Kubiya MCP server (mcp-gateway)",
		Long: `Checks for updates to the mcp-gateway repository from origin/main.
If updates are found, it pulls the latest changes, reinstalls dependencies using uv, 
and updates the stored version information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			fmt.Fprintln(stdout, "🚀 Checking for MCP server updates...")

			// 1. Check if installed using afero
			gatewayDir, err := mcp.GetMcpGatewayDir()
			if err != nil {
				return fmt.Errorf("failed to determine mcp-gateway directory: %w", err)
			}
			if _, err := fs.Stat(gatewayDir); os.IsNotExist(err) {
				fmt.Fprintf(stderr, "Error: MCP Gateway directory not found at %s\n", gatewayDir)
				fmt.Fprintln(stderr, "Please run 'kubiya mcp install' first.")
				return fmt.Errorf("MCP gateway not installed")
			} else if err != nil {
				return fmt.Errorf("failed checking MCP Gateway directory %s: %w", gatewayDir, err)
			}

			// 2. Read current installed SHA using afero
			fmt.Fprint(stdout, "Reading local version... ")
			currentSHA, err := mcp.ReadGatewaySHA(fs)
			if err != nil {
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error reading installed SHA: %v\n", err)
				fmt.Fprintln(stderr, "Cannot determine the current version. Update check aborted.")
				// Consider allowing a forced update here?
				return err
			}
			fmt.Fprintf(stdout, "✅ Found: %s\n", currentSHA[:7]) // Show short SHA

			// 3. Fetch remote changes
			fmt.Fprint(stdout, "Fetching remote information... ")
			fetchCmd := exec.Command("git", "fetch", "origin")
			fetchCmd.Dir = gatewayDir
			// Use runCommandCapture to suppress noisy output unless there's an error
			_, fetchStderr, err := runCommandCapture(fetchCmd)
			if err != nil {
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error fetching remote changes: %v\n%s", err, fetchStderr)
				return fmt.Errorf("failed to fetch remote changes: %w", err)
			}
			fmt.Fprintln(stdout, "✅ Done.")

			// 4. Get remote SHA (origin/main)
			fmt.Fprint(stdout, "Checking latest version... ")
			remoteShaCmd := exec.Command("git", "rev-parse", "origin/main")
			remoteShaCmd.Dir = gatewayDir
			remoteSHAStr, remoteStderr, err := runCommandCapture(remoteShaCmd)
			if err != nil {
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error getting remote SHA: %v\n%s", err, remoteStderr)
				return fmt.Errorf("failed to get remote SHA: %w", err)
			}
			remoteSHA := strings.TrimSpace(remoteSHAStr)
			fmt.Fprintf(stdout, "✅ Latest: %s\n", remoteSHA[:7])

			// 5. Compare and Update
			if currentSHA == remoteSHA {
				fmt.Fprintln(stdout, "\n✨ MCP server is already up-to-date.")
				return nil
			}

			fmt.Fprintf(stdout, "\n🔄 Update available ( %s -> %s ). Updating...\n", currentSHA[:7], remoteSHA[:7])

			// 5a. Reset to remote branch
			fmt.Fprint(stdout, "Updating repository... ")
			// Use reset --hard to discard local changes and match remote exactly
			resetCmd := exec.Command("git", "reset", "--hard", "origin/main")
			resetCmd.Dir = gatewayDir
			if err := runCommand(resetCmd, stdout, stderr); err != nil { // Stream output for reset
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error resetting repository: %v\n", err)
				return err
			}
			// Output from git reset is usually sufficient confirmation
			// fmt.Fprintln(stdout, "✅ Repository updated.")

			// 5b. Reinstall dependencies
			fmt.Fprintln(stdout, "Reinstalling dependencies using uv...")
			installCmd := exec.Command("uv", "sync")
			installCmd.Dir = gatewayDir
			if err := runCommand(installCmd, stdout, stderr); err != nil { // Stream output
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error reinstalling dependencies: %v\n", err)
				return err
			}
			fmt.Fprintln(stdout, "✅ Dependencies reinstalled.")

			// 5c. Store the new SHA using afero
			fmt.Fprint(stdout, "Storing updated version information... ")
			if err := mcp.StoreGatewaySHA(fs, remoteSHA); err != nil {
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error storing updated gateway SHA: %v\n", err)
				fmt.Fprintln(stderr, "Warning: Could not store new repository version. Update checks may not function correctly next time.")
				// Don't fail the whole update just for this
			} else {
				fmt.Fprintln(stdout, "✅ Stored.")
			}

			// 6. Re-apply configurations
			fmt.Fprintln(stdout, "\n⚙️ Re-applying configurations for existing providers...")

			if cfg.APIKey == "" {
				fmt.Fprintln(stdout, "🟡 Skipping configuration re-apply: Kubiya API key not configured.")
				fmt.Fprintln(stdout, "   Run 'kubiya mcp apply <provider>' manually if needed after setting the API key.")
			} else {
				var allTeammateUUIDs []string
				client := kubiya.NewClient(cfg)
				fmt.Fprint(stdout, "  Fetching all teammates for re-apply... ")
				teammates, err := client.ListTeammates(context.Background())
				if err != nil {
					fmt.Fprintln(stdout, "❌ Error fetching teammates.")
					fmt.Fprintf(stderr, "  Warning: Could not fetch teammates for re-apply: %v\n", err)
					fmt.Fprintln(stderr, "  Will re-apply configurations without specific teammates.")
					allTeammateUUIDs = []string{}
				} else {
					for _, tm := range teammates {
						allTeammateUUIDs = append(allTeammateUUIDs, tm.UUID)
					}
					fmt.Fprintf(stdout, "✅ Using %d teammates.\n", len(allTeammateUUIDs))
				}

				mcpConfigDir, err := mcp.GetMcpConfigDir(fs)
				if err != nil {
					fmt.Fprintf(stderr, "  Warning: Could not access MCP config directory (%s) for re-apply: %v\n", mcpConfigDir, err)
				} else {
					files, err := afero.ReadDir(fs, mcpConfigDir)
					if err != nil {
						fmt.Fprintf(stderr, "  Warning: Could not list files in MCP config directory (%s) for re-apply: %v\n", mcpConfigDir, err)
					} else {
						fmt.Fprintln(stdout, "  Checking providers in", mcpConfigDir)
						appliedCount := 0
						for _, file := range files {
							if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
								providerName := strings.TrimSuffix(file.Name(), ".yaml")
								fmt.Fprintf(stdout, "  Re-applying configuration for: %s\n", providerName)
								if err := applyProviderConfiguration(providerName, cfg, fs, stdout, stderr, allTeammateUUIDs); err != nil {
									fmt.Fprintf(stderr, "  Error re-applying config for %s: %v\n", providerName, err)
								} else {
									appliedCount++
								}
							}
						}
						if appliedCount == 0 {
							fmt.Fprintln(stdout, "  No provider configurations found to re-apply.")
						}
					}
				}
			} // End API key check

			fmt.Fprintln(stdout, "\n✨ MCP server updated successfully!")

			return nil
		},
	}
	return cmd
}
