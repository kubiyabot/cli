package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newPolicyCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policy",
		Aliases: []string{"policies", "pol"},
		Short:   "🛡️  Manage OPA policies for access control",
		Long:    `Create, update, validate, and manage Open Policy Agent (OPA) policies.`,
	}

	cmd.AddCommand(
		newListPoliciesCommand(cfg),
		newGetPolicyCommand(cfg),
		newCreatePolicyCommand(cfg),
		newUpdatePolicyCommand(cfg),
		newDeletePolicyCommand(cfg),
	)

	return cmd
}

func newListPoliciesCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List all policies",
		Example: `  # List all policies
  kubiya policy list

  # Output in JSON format
  kubiya policy list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			fmt.Println("🔍 Fetching policies...")
			policies, err := client.ListPolicies()
			if err != nil {
				return fmt.Errorf("failed to list policies: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(policies)
			}

			if len(policies) == 0 {
				fmt.Println("\n╭─────────────────────╮")
				fmt.Println("│                     │")
				fmt.Println("│  No policies found  │")
				fmt.Println("│                     │")
				fmt.Println("╰─────────────────────╯")
				fmt.Println("\nTo create a new policy:")
				fmt.Println("  kubiya policy create --name \"My Policy\" --file policy.rego")
				return nil
			}

			// Display policies in a table
			fmt.Printf("\n╔════════════════════════════╗\n")
			fmt.Printf("║                            ║\n")
			fmt.Printf("║   🛡️  Policies (%d)   ║\n", len(policies))
			fmt.Printf("║                            ║\n")
			fmt.Printf("╚════════════════════════════╝\n")
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, " ID\tNAME\tENABLED\tDESCRIPTION\tCREATED")
			fmt.Fprintln(w, strings.Repeat("─", 100))

			for _, p := range policies {
				desc := "-"
				if p.Description != nil && *p.Description != "" {
					desc = *p.Description
					if len(desc) > 40 {
						desc = desc[:37] + "..."
					}
				}

				created := "-"
				if p.CreatedAt != nil {
					created = p.CreatedAt.Format("2006-01-02 15:04")
				}

				enabledStr := "✗"
				if p.Enabled {
					enabledStr = style.SuccessStyle.Render("✓")
				} else {
					enabledStr = style.ErrorStyle.Render("✗")
				}

				// Shorten ID for display
				displayID := p.ID
				if len(displayID) > 12 {
					displayID = displayID[:8] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					style.DimStyle.Render(displayID),
					style.HighlightStyle.Render(p.Name),
					enabledStr,
					desc,
					created,
				)
			}
			w.Flush()

			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newGetPolicyCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get [policy-id]",
		Short: "📝 Get policy details",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get policy details
  kubiya policy get abc123

  # Output in JSON format
  kubiya policy get abc123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			policyID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			policy, err := client.GetPolicy(policyID)
			if err != nil {
				return fmt.Errorf("failed to get policy: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(policy)
			}

			// Display policy details
			fmt.Printf("\n╔═══════════════════════╗\n")
			fmt.Printf("║                       ║\n")
			fmt.Printf("║   🛡️  Policy Details ║\n")
			fmt.Printf("║                       ║\n")
			fmt.Printf("╚═══════════════════════╝\n")
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", policy.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(policy.Name))

			if policy.Description != nil && *policy.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *policy.Description)
			}

			enabledStr := "Disabled"
			if policy.Enabled {
				enabledStr = style.SuccessStyle.Render("Enabled")
			} else {
				enabledStr = style.ErrorStyle.Render("Disabled")
			}
			fmt.Fprintf(w, "Status:\t%s\n", enabledStr)

			if policy.CreatedAt != nil {
				fmt.Fprintf(w, "Created:\t%s\n", policy.CreatedAt.Format("2006-01-02 15:04:05"))
			}

			if policy.UpdatedAt != nil {
				fmt.Fprintf(w, "Updated:\t%s\n", policy.UpdatedAt.Format("2006-01-02 15:04:05"))
			}

			w.Flush()

			// Display Rego policy content
			if policy.Rego != "" {
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Policy Content (Rego):"))
				fmt.Println(policy.Rego)
			}

			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		regoFile    string
		enabled     bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "➕ Create a new policy",
		Example: `  # Create a policy from a Rego file
  kubiya policy create --name "Production Policy" --file policy.rego

  # Create with description
  kubiya policy create --name "My Policy" --description "Access control policy" --file policy.rego

  # Create disabled policy
  kubiya policy create --name "My Policy" --file policy.rego --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Read Rego file
			regoContent, err := os.ReadFile(regoFile)
			if err != nil {
				return fmt.Errorf("failed to read rego file: %w", err)
			}

			var desc *string
			if description != "" {
				desc = &description
			}

			enabledPtr := &enabled

			req := &entities.PolicyCreateRequest{
				Name:        name,
				Description: desc,
				Rego:        string(regoContent),
				Enabled:     enabledPtr,
			}

			fmt.Println("🛠️  Creating policy...")
			policy, err := client.CreatePolicy(req)
			if err != nil {
				return fmt.Errorf("failed to create policy: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(policy)
			}

			fmt.Printf("\n%s\n\n", style.TitleStyle.Render("✅ Policy created successfully!"))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", policy.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(policy.Name))
			if policy.Description != nil && *policy.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *policy.Description)
			}
			enabledStr := "Disabled"
			if policy.Enabled {
				enabledStr = style.SuccessStyle.Render("Enabled")
			}
			fmt.Fprintf(w, "Status:\t%s\n", enabledStr)
			w.Flush()

			fmt.Println()
			fmt.Printf("To view policy details:\n  %s\n\n",
				style.DimStyle.Render(fmt.Sprintf("kubiya policy get %s", policy.ID)))

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Policy name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Policy description")
	cmd.Flags().StringVarP(&regoFile, "file", "f", "", "Path to Rego policy file (required)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable policy immediately")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("file")

	return cmd
}

func newUpdatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		regoFile    string
		enabled     bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "update [policy-id]",
		Short: "🔄 Update a policy",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update policy name
  kubiya policy update abc123 --name "New Name"

  # Update Rego content
  kubiya policy update abc123 --file updated-policy.rego

  # Disable policy
  kubiya policy update abc123 --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			policyID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.PolicyUpdateRequest{}

			if name != "" {
				req.Name = &name
			}
			if description != "" {
				req.Description = &description
			}
			if regoFile != "" {
				regoContent, err := os.ReadFile(regoFile)
				if err != nil {
					return fmt.Errorf("failed to read rego file: %w", err)
				}
				regoStr := string(regoContent)
				req.Rego = &regoStr
			}
			if cmd.Flags().Changed("enabled") {
				req.Enabled = &enabled
			}

			fmt.Println("🔄 Updating policy...")
			policy, err := client.UpdatePolicy(policyID, req)
			if err != nil {
				return fmt.Errorf("failed to update policy: %w", err)
			}

			if outputFormat == "json" {
				return json.NewEncoder(os.Stdout).Encode(policy)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Policy updated successfully!"))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID:\t%s\n", policy.ID)
			fmt.Fprintf(w, "Name:\t%s\n", style.HighlightStyle.Render(policy.Name))
			if policy.Description != nil && *policy.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", *policy.Description)
			}
			enabledStr := "Disabled"
			if policy.Enabled {
				enabledStr = style.SuccessStyle.Render("Enabled")
			}
			fmt.Fprintf(w, "Status:\t%s\n", enabledStr)
			w.Flush()

			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "New policy name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New policy description")
	cmd.Flags().StringVarP(&regoFile, "file", "f", "", "Path to new Rego policy file")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Enable/disable policy")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

func newDeletePolicyCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [policy-id]",
		Short: "🗑️ Delete a policy",
		Args:  cobra.ExactArgs(1),
		Example: `  # Delete a policy
  kubiya policy delete abc123

  # Force delete without confirmation
  kubiya policy delete abc123 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			policyID := args[0]
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get policy details first
			policy, err := client.GetPolicy(policyID)
			if err != nil {
				return fmt.Errorf("failed to get policy: %w", err)
			}

			// Confirm deletion unless forced
			if !force {
				fmt.Printf("Are you sure you want to delete policy '%s' (%s)? [y/N] ", policy.Name, policyID)
				var confirm string
				fmt.Scanln(&confirm)

				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			// Delete the policy
			if err := client.DeletePolicy(policyID); err != nil {
				return fmt.Errorf("failed to delete policy: %w", err)
			}

			fmt.Printf("\n%s\n\n", style.HighlightStyle.Render("✅ Policy deleted successfully!"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	return cmd
}
