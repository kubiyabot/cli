package cli

import (
	"encoding/json"
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/formatter"
	"github.com/kubiyabot/cli/internal/style"
	"gopkg.in/yaml.v3"
)

// Control Plane V2 Agent Operations

func createAgentV2(cfg *config.Config, name, description, model, systemPrompt, runtime string, teamID *string, environmentIDs []string) error {
	client, err := controlplane.New(cfg.APIKey, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}

	req := &entities.AgentCreateRequest{
		Name: name,
	}

	if description != "" {
		req.Description = &description
	}
	if model != "" {
		req.Model = &model
	}
	if systemPrompt != "" {
		req.SystemPrompt = &systemPrompt
	}
	if runtime != "" {
		rt := entities.RuntimeType(runtime)
		req.Runtime = &rt
	}
	if teamID != nil {
		req.TeamID = teamID
	}
	if len(environmentIDs) > 0 {
		req.EnvironmentIDs = environmentIDs
	}

	agent, err := client.CreateAgent(req)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Printf("%s Agent %s created successfully (ID: %s)\n",
		style.SuccessStyle.Render("✓"),
		style.HighlightStyle.Render(agent.Name),
		agent.ID)

	return nil
}

func listAgentsV2(cfg *config.Config, outputFormat string) error {
	client, err := controlplane.New(cfg.APIKey, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}

	agents, err := client.ListAgents()
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		formatter.EmptyListMessage("agents")
		return nil
	}

	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal agents: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(agents)
		if err != nil {
			return fmt.Errorf("failed to marshal agents: %w", err)
		}
		fmt.Print(string(data))
	default:
		formatter.ListOutput("Agents", "🤖", len(agents), func() {
			table := formatter.NewTable("ID", "NAME", "RUNTIME", "STATUS", "TEAM", "CREATED")
			for _, agent := range agents {
				teamID := "-"
				if agent.TeamID != nil {
					teamID = formatter.StyledDim(*agent.TeamID)
				}

				status := string(agent.Status)
				if status == "" {
					status = "unknown"
				}

				runtime := string(agent.Runtime)
				if runtime == "" {
					runtime = "default"
				}

				table.AddRow(
					formatter.StyledID(agent.ID),
					formatter.StyledName(agent.Name),
					formatter.StyledValue(runtime),
					formatter.FormatStatus(status),
					teamID,
					formatter.FormatCustomTime(agent.CreatedAt),
				)
			}
			table.Render()
		})
	}

	return nil
}

func getAgentV2(cfg *config.Config, agentID string, outputFormat string) error {
	client, err := controlplane.New(cfg.APIKey, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}

	agent, err := client.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(agent, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal agent: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(agent)
		if err != nil {
			return fmt.Errorf("failed to marshal agent: %w", err)
		}
		fmt.Print(string(data))
	default:
		fields := map[string]string{
			"ID":      agent.ID,
			"Name":    agent.Name,
			"Runtime": string(agent.Runtime),
			"Status":  string(agent.Status),
		}

		if agent.Description != nil {
			fields["Description"] = *agent.Description
		}

		if agent.TeamID != nil {
			fields["Team ID"] = *agent.TeamID
		}

		if agent.ModelID != nil {
			fields["Model ID"] = *agent.ModelID
		}

		if agent.Model != nil {
			fields["Model"] = *agent.Model
		}

		if agent.SystemPrompt != nil {
			fields["System Prompt"] = formatter.TruncateString(*agent.SystemPrompt, 100)
		}

		if len(agent.EnvironmentIDs) > 0 {
			fields["Environments"] = fmt.Sprintf("%d environments", len(agent.EnvironmentIDs))
		}

		if agent.CreatedAt != nil {
			fields["Created"] = agent.CreatedAt.Format("2006-01-02 15:04:05")
		}

		if agent.UpdatedAt != nil {
			fields["Updated"] = agent.UpdatedAt.Format("2006-01-02 15:04:05")
		}

		formatter.DetailOutput("Agent Details", "🤖", fields)
	}

	return nil
}

func deleteAgentV2(cfg *config.Config, agentID string) error {
	client, err := controlplane.New(cfg.APIKey, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}

	// Get agent name before deleting
	agent, err := client.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if err := client.DeleteAgent(agentID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	fmt.Printf("%s Agent %s deleted successfully\n",
		style.SuccessStyle.Render("✓"),
		style.HighlightStyle.Render(agent.Name))

	return nil
}

func updateAgentV2(cfg *config.Config, agentID string, updates map[string]interface{}) error {
	client, err := controlplane.New(cfg.APIKey, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}

	req := &entities.AgentUpdateRequest{}

	if name, ok := updates["name"].(string); ok && name != "" {
		req.Name = &name
	}
	if desc, ok := updates["description"].(string); ok && desc != "" {
		req.Description = &desc
	}
	if model, ok := updates["model"].(string); ok && model != "" {
		req.Model = &model
	}
	if prompt, ok := updates["system_prompt"].(string); ok && prompt != "" {
		req.SystemPrompt = &prompt
	}
	if runtime, ok := updates["runtime"].(string); ok && runtime != "" {
		rt := entities.RuntimeType(runtime)
		req.Runtime = &rt
	}
	if teamID, ok := updates["team_id"].(string); ok && teamID != "" {
		req.TeamID = &teamID
	}

	agent, err := client.UpdateAgent(agentID, req)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	fmt.Printf("%s Agent %s updated successfully\n",
		style.SuccessStyle.Render("✓"),
		style.HighlightStyle.Render(agent.Name))

	return nil
}
