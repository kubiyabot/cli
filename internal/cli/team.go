package cli

import (
	"encoding/json"
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/formatter"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newTeamCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "team",
		Aliases: []string{"teams", "tm"},
		Short:   "👥 Manage teams",
		Long:    `Create, list, update, and delete teams in the control plane.`,
	}

	cmd.AddCommand(
		newCreateTeamCommand(cfg),
		newListTeamsCommand(cfg),
		newGetTeamCommand(cfg),
		newUpdateTeamCommand(cfg),
		newDeleteTeamCommand(cfg),
		newTeamInteractiveChatCommand(cfg),
		newTeamExecCommand(cfg),
	)

	return cmd
}

func newCreateTeamCommand(cfg *config.Config) *cobra.Command {
	var (
		name           string
		description    string
		members        []string
		environmentIDs []string
		teamType       string
		outputFormat   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new team",
		Example: `  # Create a team
  kubiya team create --name "DevOps Team" --description "Handles DevOps tasks"

  # Create with members
  kubiya team create --name "Backend Team" --member agent-123 --member agent-456

  # Create with environments
  kubiya team create --name "Production Team" --environment env-prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.TeamCreateRequest{
				Name:           name,
				Members:        members,
				EnvironmentIDs: environmentIDs,
			}

			if description != "" {
				req.Description = &description
			}
			if teamType != "" {
				req.TeamType = &teamType
			}

			team, err := client.CreateTeam(req)
			if err != nil {
				return fmt.Errorf("failed to create team: %w", err)
			}

			fmt.Printf("%s Team %s created (ID: %s)\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(team.Name),
				team.ID)

			if outputFormat == "json" {
				data, _ := json.MarshalIndent(team, "", "  ")
				fmt.Println(string(data))
			} else if outputFormat == "yaml" {
				data, _ := yaml.Marshal(team)
				fmt.Print(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Team name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Team description")
	cmd.Flags().StringSliceVar(&members, "member", []string{}, "Agent IDs to add as members (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&environmentIDs, "environment", []string{}, "Environment IDs (can be specified multiple times)")
	cmd.Flags().StringVar(&teamType, "type", "", "Team type")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newListTeamsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all teams",
		Example: `  # List teams
  kubiya team list

  # List as JSON
  kubiya team list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			teams, err := client.ListTeams()
			if err != nil {
				return fmt.Errorf("failed to list teams: %w", err)
			}

			if len(teams) == 0 {
				formatter.EmptyListMessage("teams")
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(teams, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(teams)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				formatter.ListOutput("Teams", "👥", len(teams), func() {
					table := formatter.NewTable("ID", "NAME", "MEMBERS", "ENVIRONMENTS", "CREATED")
					for _, team := range teams {
						table.AddRow(
							formatter.StyledID(team.ID),
							formatter.StyledName(team.Name),
							formatter.FormatCount(len(team.Members)),
							formatter.FormatCount(len(team.EnvironmentIDs)),
							formatter.FormatCustomTime(team.CreatedAt),
						)
					}
					table.Render()
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newGetTeamCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get TEAM_ID",
		Aliases: []string{"describe", "show"},
		Short:   "Get team details",
		Args:    cobra.ExactArgs(1),
		Example: `  # Get team details
  kubiya team get team-123

  # Get as JSON
  kubiya team get team-123 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			team, err := client.GetTeam(teamID)
			if err != nil {
				return fmt.Errorf("failed to get team: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(team, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(team)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				fields := map[string]string{
					"ID":   team.ID,
					"Name": team.Name,
				}

				if team.Description != nil {
					fields["Description"] = *team.Description
				}

				if team.TeamType != nil {
					fields["Type"] = *team.TeamType
				}

				if len(team.Members) > 0 {
					fields["Members"] = fmt.Sprintf("%d agents", len(team.Members))
				}

				if len(team.EnvironmentIDs) > 0 {
					fields["Environments"] = fmt.Sprintf("%d environments", len(team.EnvironmentIDs))
				}

				if team.CreatedAt != nil {
					fields["Created"] = team.CreatedAt.Format("2006-01-02 15:04:05")
				}

				formatter.DetailOutput("Team Details", "👥", fields)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newUpdateTeamCommand(cfg *config.Config) *cobra.Command {
	var (
		name           string
		description    string
		members        []string
		environmentIDs []string
		teamType       string
	)

	cmd := &cobra.Command{
		Use:   "update TEAM_ID",
		Short: "Update a team",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update team name
  kubiya team update team-123 --name "New Team Name"

  # Update members
  kubiya team update team-123 --member agent-789`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.TeamUpdateRequest{}

			if name != "" {
				req.Name = &name
			}
			if description != "" {
				req.Description = &description
			}
			if len(members) > 0 {
				req.Members = members
			}
			if len(environmentIDs) > 0 {
				req.EnvironmentIDs = environmentIDs
			}
			if teamType != "" {
				req.TeamType = &teamType
			}

			team, err := client.UpdateTeam(teamID, req)
			if err != nil {
				return fmt.Errorf("failed to update team: %w", err)
			}

			fmt.Printf("%s Team %s updated\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(team.Name))

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Team name")
	cmd.Flags().StringVar(&description, "description", "", "Team description")
	cmd.Flags().StringSliceVar(&members, "member", []string{}, "Agent IDs (replaces existing members)")
	cmd.Flags().StringSliceVar(&environmentIDs, "environment", []string{}, "Environment IDs (replaces existing environments)")
	cmd.Flags().StringVar(&teamType, "type", "", "Team type")

	return cmd
}

func newDeleteTeamCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete TEAM_ID",
		Aliases: []string{"remove", "rm"},
		Short:   "Delete a team",
		Args:    cobra.ExactArgs(1),
		Example: `  # Delete a team
  kubiya team delete team-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get team name before deleting
			team, err := client.GetTeam(teamID)
			if err != nil {
				return fmt.Errorf("failed to get team: %w", err)
			}

			if err := client.DeleteTeam(teamID); err != nil {
				return fmt.Errorf("failed to delete team: %w", err)
			}

			fmt.Printf("%s Team %s deleted\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(team.Name))

			return nil
		},
	}

	return cmd
}
