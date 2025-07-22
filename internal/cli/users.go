package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// printJSON prints data as formatted JSON
func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func newUsersCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user",
		Aliases: []string{"users"},
		Short:   "👥 Manage users and groups",
		Long:    "List users and groups for access control management",
	}

	cmd.AddCommand(
		newListUsersCommand(cfg),
		newListGroupsCommand(cfg),
	)

	return cmd
}

func newListUsersCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all users",
		Long:    "Display a list of all users in the organization",
		Example: "  kubiya user list\n  kubiya user list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			users, err := client.ListUsers(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list users: %w", err)
			}

			if outputFormat == "json" {
				return printJSON(users)
			}

			return printUsersTable(users)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

func newListGroupsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "groups",
		Short:   "📋 List all groups",
		Long:    "Display a list of all groups in the organization",
		Example: "  kubiya user groups\n  kubiya user groups --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			groups, err := client.ListGroups(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list groups: %w", err)
			}

			if outputFormat == "json" {
				return printJSON(groups)
			}

			return printGroupsTable(groups)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

func printUsersTable(users []kubiya.User) error {
	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}

	// Sort users by email for consistent output
	sort.Slice(users, func(i, j int) bool {
		return users[i].Email < users[j].Email
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "EMAIL\tNAME\tUUID\tSTATUS\tGROUPS\n")

	for _, user := range users {
		name := user.Name
		if name == "" {
			name = "-"
		}
		
		groups := fmt.Sprintf("%d groups", len(user.Groups))
		if len(user.Groups) == 0 {
			groups = "none"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			user.Email,
			name,
			user.UUID,
			user.Status,
			groups)
	}

	return w.Flush()
}

func printGroupsTable(groups []kubiya.Group) error {
	if len(groups) == 0 {
		fmt.Println("No groups found.")
		return nil
	}

	// Sort groups - system groups first, then by name
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].System != groups[j].System {
			return groups[i].System // System groups first
		}
		return groups[i].Name < groups[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tUUID\tDESCRIPTION\tTYPE\n")

	for _, group := range groups {
		groupType := "Custom"
		if group.System {
			groupType = "System"
		}

		description := group.Description
		if description == "" {
			description = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			group.Name,
			group.UUID,
			description,
			groupType)
	}

	return w.Flush()
}