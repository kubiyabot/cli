package cli

import (
	"sort"
	"strings"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// EntityMatch represents a matched entity with its score
type EntityMatch struct {
	Entity     interface{} // *entities.Agent or *entities.Team
	EntityType string      // "agent" or "team"
	EntityID   string
	EntityName string
	Score      float64
	MatchedOn  []string // Fields that matched: "name", "description", "capabilities"
}

// EntityMatcher handles fuzzy matching of agents and teams
type EntityMatcher struct {
	agents []*entities.Agent
	teams  []*entities.Team
}

// NewEntityMatcher creates a matcher with the given entities
func NewEntityMatcher(agents []*entities.Agent, teams []*entities.Team) *EntityMatcher {
	return &EntityMatcher{
		agents: agents,
		teams:  teams,
	}
}

// FindMatches searches for entities matching the pattern
// Returns matches sorted by score (highest first)
func (m *EntityMatcher) FindMatches(pattern string, threshold float64) []EntityMatch {
	var matches []EntityMatch

	// Search agents
	for _, agent := range m.agents {
		score, matchedOn := m.calculateAgentScore(pattern, agent)
		if score >= threshold {
			matches = append(matches, EntityMatch{
				Entity:     agent,
				EntityType: "agent",
				EntityID:   agent.ID,
				EntityName: agent.Name,
				Score:      score,
				MatchedOn:  matchedOn,
			})
		}
	}

	// Search teams
	for _, team := range m.teams {
		score, matchedOn := m.calculateTeamScore(pattern, team)
		if score >= threshold {
			matches = append(matches, EntityMatch{
				Entity:     team,
				EntityType: "team",
				EntityID:   team.ID,
				EntityName: team.Name,
				Score:      score,
				MatchedOn:  matchedOn,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// calculateAgentScore computes weighted fuzzy score for an agent
// Weights: Name (3x), Description (2x), Capabilities (1x)
func (m *EntityMatcher) calculateAgentScore(pattern string, agent *entities.Agent) (float64, []string) {
	var totalScore float64
	var matchedFields []string
	patternLower := strings.ToLower(pattern)

	// Check for exact name match first (case-insensitive)
	if strings.EqualFold(agent.Name, pattern) {
		return 1.0, []string{"name (exact)"}
	}

	// Name matching (weight: 3)
	nameScore := m.fuzzyScore(patternLower, strings.ToLower(agent.Name))
	if nameScore > 0 {
		totalScore += nameScore * 3.0
		matchedFields = append(matchedFields, "name")
	}

	// Description matching (weight: 2)
	if agent.Description != nil && *agent.Description != "" {
		descScore := m.fuzzyScore(patternLower, strings.ToLower(*agent.Description))
		if descScore > 0 {
			totalScore += descScore * 2.0
			matchedFields = append(matchedFields, "description")
		}
	}

	// Capabilities matching (weight: 1) - best match among all capabilities
	var bestCapScore float64
	for _, cap := range agent.Capabilities {
		capScore := m.fuzzyScore(patternLower, strings.ToLower(cap))
		if capScore > bestCapScore {
			bestCapScore = capScore
		}
	}
	if bestCapScore > 0 {
		totalScore += bestCapScore * 1.0
		matchedFields = append(matchedFields, "capabilities")
	}

	// Normalize by max possible score (3 + 2 + 1 = 6)
	normalizedScore := totalScore / 6.0

	return normalizedScore, matchedFields
}

// calculateTeamScore computes weighted fuzzy score for a team
// Weights: Name (3x), Description (2x), TeamType (1x)
func (m *EntityMatcher) calculateTeamScore(pattern string, team *entities.Team) (float64, []string) {
	var totalScore float64
	var matchedFields []string
	patternLower := strings.ToLower(pattern)

	// Check for exact name match first (case-insensitive)
	if strings.EqualFold(team.Name, pattern) {
		return 1.0, []string{"name (exact)"}
	}

	// Name matching (weight: 3)
	nameScore := m.fuzzyScore(patternLower, strings.ToLower(team.Name))
	if nameScore > 0 {
		totalScore += nameScore * 3.0
		matchedFields = append(matchedFields, "name")
	}

	// Description matching (weight: 2)
	if team.Description != nil && *team.Description != "" {
		descScore := m.fuzzyScore(patternLower, strings.ToLower(*team.Description))
		if descScore > 0 {
			totalScore += descScore * 2.0
			matchedFields = append(matchedFields, "description")
		}
	}

	// TeamType matching (weight: 1)
	if team.TeamType != nil && *team.TeamType != "" {
		typeScore := m.fuzzyScore(patternLower, strings.ToLower(*team.TeamType))
		if typeScore > 0 {
			totalScore += typeScore * 1.0
			matchedFields = append(matchedFields, "type")
		}
	}

	// Normalize by max possible score (3 + 2 + 1 = 6)
	normalizedScore := totalScore / 6.0

	return normalizedScore, matchedFields
}

// fuzzyScore calculates a score between 0 and 1 for how well pattern matches target
// Uses substring matching and fuzzy matching for flexibility
func (m *EntityMatcher) fuzzyScore(pattern, target string) float64 {
	if pattern == "" || target == "" {
		return 0
	}

	// Exact substring match gets highest score
	if strings.Contains(target, pattern) {
		// Score based on how much of the target the pattern covers
		return 0.8 + (0.2 * float64(len(pattern)) / float64(len(target)))
	}

	// Check if pattern is contained in target using fuzzy matching
	if fuzzy.Match(pattern, target) {
		// Use RankMatch to get distance - lower is better
		distance := fuzzy.RankMatch(pattern, target)
		if distance >= 0 {
			// Convert distance to score: 0 distance = 1.0 score, higher distance = lower score
			return 1.0 / (1.0 + float64(distance)*0.1)
		}
	}

	// No match
	return 0
}
