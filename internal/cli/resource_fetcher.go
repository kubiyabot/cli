package cli

import (
	"context"
	"fmt"
	"sync"

	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// ResourceFetcher fetches planning resources in parallel
type ResourceFetcher struct {
	client *controlplane.Client
}

// NewResourceFetcher creates a new resource fetcher
func NewResourceFetcher(client *controlplane.Client) *ResourceFetcher {
	return &ResourceFetcher{
		client: client,
	}
}

// PlanningResources contains all resources needed for planning
type PlanningResources struct {
	Agents       []kubiya.AgentInfo
	Teams        []kubiya.TeamInfo
	Environments []kubiya.EnvironmentInfo
	Queues       []kubiya.WorkerQueueInfo
}

// FetchAllResources fetches all planning resources in parallel
func (rf *ResourceFetcher) FetchAllResources(ctx context.Context) (*PlanningResources, error) {
	var (
		agents []kubiya.AgentInfo
		teams  []kubiya.TeamInfo
		envs   []kubiya.EnvironmentInfo
		queues []kubiya.WorkerQueueInfo
		wg     sync.WaitGroup
		mu     sync.Mutex
		errs   []error
	)

	wg.Add(4)

	// Fetch agents
	go func() {
		defer wg.Done()
		agentsList, err := rf.client.ListAgents()
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to fetch agents: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		agents = convertAgentsToInfo(agentsList)
		mu.Unlock()
	}()

	// Fetch teams
	go func() {
		defer wg.Done()
		teamsList, err := rf.client.ListTeams()
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to fetch teams: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		teams = convertTeamsToInfo(teamsList)
		mu.Unlock()
	}()

	// Fetch environments
	go func() {
		defer wg.Done()
		envsList, err := rf.client.ListEnvironments()
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to fetch environments: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		envs = convertEnvironmentsToInfo(envsList)
		mu.Unlock()
	}()

	// Fetch worker queues
	go func() {
		defer wg.Done()
		queuesList, err := rf.client.ListWorkerQueues()
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to fetch worker queues: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		queues = convertWorkerQueuesToInfo(queuesList)
		mu.Unlock()
	}()

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to fetch resources: %v", errs)
	}

	return &PlanningResources{
		Agents:       agents,
		Teams:        teams,
		Environments: envs,
		Queues:       queues,
	}, nil
}

// convertAgentsToInfo converts control plane agents to plan agent info
func convertAgentsToInfo(agents []*entities.Agent) []kubiya.AgentInfo {
	result := make([]kubiya.AgentInfo, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}

		info := kubiya.AgentInfo{
			ID:           agent.ID,
			Name:         agent.Name,
			Capabilities: agent.Capabilities,
			ModelID:      agent.ModelID,
		}

		if agent.Description != nil {
			info.Description = *agent.Description
		}

		result = append(result, info)
	}
	return result
}

// convertTeamsToInfo converts control plane teams to plan team info
func convertTeamsToInfo(teams []*entities.Team) []kubiya.TeamInfo {
	result := make([]kubiya.TeamInfo, 0, len(teams))
	for _, team := range teams {
		if team == nil {
			continue
		}

		info := kubiya.TeamInfo{
			ID:     team.ID,
			Name:   team.Name,
			Agents: team.Members, // Members are agent IDs
		}

		if team.Description != nil {
			info.Description = *team.Description
		}

		result = append(result, info)
	}
	return result
}

// convertEnvironmentsToInfo converts control plane environments to plan environment info
func convertEnvironmentsToInfo(envs []*entities.Environment) []kubiya.EnvironmentInfo {
	result := make([]kubiya.EnvironmentInfo, 0, len(envs))
	for _, env := range envs {
		if env == nil {
			continue
		}

		info := kubiya.EnvironmentInfo{
			ID:       env.ID,
			Name:     env.Name,
			Metadata: make(map[string]interface{}),
		}

		if env.Description != nil {
			info.Description = *env.Description
		}

		// Add configuration to metadata
		if env.Configuration != nil {
			info.Metadata["configuration"] = env.Configuration
		}

		// Add variables to metadata
		if env.Variables != nil && len(env.Variables) > 0 {
			info.Metadata["variables"] = env.Variables
		}

		// Add secrets to metadata
		if env.Secrets != nil && len(env.Secrets) > 0 {
			info.Metadata["secrets"] = env.Secrets
		}

		// Add integrations to metadata
		if env.Integrations != nil && len(env.Integrations) > 0 {
			info.Metadata["integrations"] = env.Integrations
		}

		result = append(result, info)
	}
	return result
}

// convertWorkerQueuesToInfo converts control plane worker queues to plan worker queue info
func convertWorkerQueuesToInfo(queues []*entities.WorkerQueue) []kubiya.WorkerQueueInfo {
	result := make([]kubiya.WorkerQueueInfo, 0, len(queues))
	for _, queue := range queues {
		if queue == nil {
			continue
		}

		info := kubiya.WorkerQueueInfo{
			ID:            queue.ID,
			Name:          queue.Name,
			EnvironmentID: queue.EnvironmentID,
			Status:        "active", // Default status
		}

		result = append(result, info)
	}
	return result
}
