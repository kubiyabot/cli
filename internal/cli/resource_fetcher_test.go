package cli

import (
	"testing"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockControlPlaneClient for testing
type MockControlPlaneClient struct {
	agents       []*entities.Agent
	teams        []*entities.Team
	environments []*entities.Environment
	queues       []*entities.WorkerQueue
	shouldError  bool
}

func (m *MockControlPlaneClient) ListAgents() ([]*entities.Agent, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.agents, nil
}

func (m *MockControlPlaneClient) ListTeams() ([]*entities.Team, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.teams, nil
}

func (m *MockControlPlaneClient) ListEnvironments() ([]*entities.Environment, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.environments, nil
}

func (m *MockControlPlaneClient) ListWorkerQueues() ([]*entities.WorkerQueue, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.queues, nil
}

func TestResourceFetcher(t *testing.T) {
	t.Run("FetchAllResources_Success", func(t *testing.T) {
		// Setup mock client with test data
		agentDesc := "Test agent description"
		teamDesc := "Test team description"
		envDesc := "Test environment description"

		mockClient := &MockControlPlaneClient{
			agents: []*entities.Agent{
				{
					ID:           "agent-1",
					Name:         "Agent One",
					Description:  &agentDesc,
					Capabilities: []string{"tool1", "tool2"},
				},
				{
					ID:   "agent-2",
					Name: "Agent Two",
				},
			},
			teams: []*entities.Team{
				{
					ID:          "team-1",
					Name:        "Team Alpha",
					Description: &teamDesc,
					Members:     []string{"agent-1", "agent-2"},
				},
			},
			environments: []*entities.Environment{
				{
					ID:          "env-1",
					Name:        "Production",
					Description: &envDesc,
					Variables: map[string]string{
						"KEY1": "value1",
					},
				},
			},
			queues: []*entities.WorkerQueue{
				{
					ID:            "queue-1",
					Name:          "Default Queue",
					EnvironmentID: "env-1",
				},
			},
		}

		// Create fetcher - we need to adapt it to use our mock
		// Since ResourceFetcher expects a *controlplane.Client, we'll test the conversion functions directly
		agents := convertAgentsToInfo(mockClient.agents)
		teams := convertTeamsToInfo(mockClient.teams)
		envs := convertEnvironmentsToInfo(mockClient.environments)
		queues := convertWorkerQueuesToInfo(mockClient.queues)

		// Verify agents conversion
		require.Len(t, agents, 2)
		assert.Equal(t, "agent-1", agents[0].ID)
		assert.Equal(t, "Agent One", agents[0].Name)
		assert.Equal(t, "Test agent description", agents[0].Description)
		assert.Equal(t, []string{"tool1", "tool2"}, agents[0].Capabilities)

		assert.Equal(t, "agent-2", agents[1].ID)
		assert.Equal(t, "Agent Two", agents[1].Name)
		assert.Empty(t, agents[1].Description)

		// Verify teams conversion
		require.Len(t, teams, 1)
		assert.Equal(t, "team-1", teams[0].ID)
		assert.Equal(t, "Team Alpha", teams[0].Name)
		assert.Equal(t, "Test team description", teams[0].Description)
		assert.Equal(t, []string{"agent-1", "agent-2"}, teams[0].Agents)

		// Verify environments conversion
		require.Len(t, envs, 1)
		assert.Equal(t, "env-1", envs[0].ID)
		assert.Equal(t, "Production", envs[0].Name)
		assert.Equal(t, "Test environment description", envs[0].Description)
		assert.NotNil(t, envs[0].Metadata)
		assert.Contains(t, envs[0].Metadata, "variables")

		// Verify queues conversion
		require.Len(t, queues, 1)
		assert.Equal(t, "queue-1", queues[0].ID)
		assert.Equal(t, "Default Queue", queues[0].Name)
		assert.Equal(t, "env-1", queues[0].EnvironmentID)
		assert.Equal(t, "active", queues[0].Status)
	})

	t.Run("ConvertAgentsToInfo_HandleNil", func(t *testing.T) {
		agents := []*entities.Agent{
			{
				ID:   "agent-1",
				Name: "Test Agent",
			},
			nil, // Should be skipped
			{
				ID:   "agent-2",
				Name: "Another Agent",
			},
		}

		result := convertAgentsToInfo(agents)
		assert.Len(t, result, 2)
		assert.Equal(t, "agent-1", result[0].ID)
		assert.Equal(t, "agent-2", result[1].ID)
	})

	t.Run("ConvertTeamsToInfo_HandleNil", func(t *testing.T) {
		teams := []*entities.Team{
			{
				ID:   "team-1",
				Name: "Test Team",
			},
			nil, // Should be skipped
		}

		result := convertTeamsToInfo(teams)
		assert.Len(t, result, 1)
		assert.Equal(t, "team-1", result[0].ID)
	})

	t.Run("ConvertEnvironmentsToInfo_HandleNil", func(t *testing.T) {
		envs := []*entities.Environment{
			{
				ID:   "env-1",
				Name: "Test Env",
			},
			nil, // Should be skipped
		}

		result := convertEnvironmentsToInfo(envs)
		assert.Len(t, result, 1)
		assert.Equal(t, "env-1", result[0].ID)
	})

	t.Run("ConvertEnvironmentsToInfo_WithMetadata", func(t *testing.T) {
		envs := []*entities.Environment{
			{
				ID:   "env-1",
				Name: "Rich Env",
				Configuration: map[string]interface{}{
					"setting1": "value1",
				},
				Variables: map[string]string{
					"VAR1": "val1",
				},
				Secrets:      []string{"secret1"},
				Integrations: []string{"integration1"},
			},
		}

		result := convertEnvironmentsToInfo(envs)
		require.Len(t, result, 1)

		metadata := result[0].Metadata
		assert.Contains(t, metadata, "configuration")
		assert.Contains(t, metadata, "variables")
		assert.Contains(t, metadata, "secrets")
		assert.Contains(t, metadata, "integrations")
	})

	t.Run("ConvertWorkerQueuesToInfo_HandleNil", func(t *testing.T) {
		queues := []*entities.WorkerQueue{
			{
				ID:   "queue-1",
				Name: "Test Queue",
			},
			nil, // Should be skipped
		}

		result := convertWorkerQueuesToInfo(queues)
		assert.Len(t, result, 1)
		assert.Equal(t, "queue-1", result[0].ID)
	})

	t.Run("ConvertAgentsToInfo_EmptyCapabilities", func(t *testing.T) {
		agents := []*entities.Agent{
			{
				ID:           "agent-1",
				Name:         "Agent",
				Capabilities: []string{},
			},
		}

		result := convertAgentsToInfo(agents)
		require.Len(t, result, 1)
		assert.NotNil(t, result[0].Capabilities)
		assert.Len(t, result[0].Capabilities, 0)
	})

	t.Run("ConvertTeamsToInfo_EmptyMembers", func(t *testing.T) {
		teams := []*entities.Team{
			{
				ID:      "team-1",
				Name:    "Solo Team",
				Members: []string{},
			},
		}

		result := convertTeamsToInfo(teams)
		require.Len(t, result, 1)
		assert.NotNil(t, result[0].Agents)
		assert.Len(t, result[0].Agents, 0)
	})
}
