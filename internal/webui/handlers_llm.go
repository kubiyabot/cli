package webui

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// LLM insights cache
var (
	llmCache     *LLMInsights
	llmCacheMu   sync.RWMutex
	llmCacheTime time.Time
	llmCacheTTL  = 5 * time.Minute
)

// handleLLMModels handles GET /api/llm/models
func (s *Server) handleLLMModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	models, err := s.fetchModelsFromControlPlane()
	if err != nil {
		// Return empty list with error message rather than failing
		writeJSON(w, map[string]interface{}{
			"models":  []ModelInfo{},
			"error":   err.Error(),
			"message": "Failed to fetch models from control plane",
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// handleLLMProviders handles GET /api/llm/providers
func (s *Server) handleLLMProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	providers, err := s.fetchProvidersFromControlPlane()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"providers": []ProviderStatus{},
			"error":     err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	})
}

// handleLLMInsights handles GET /api/llm/insights (combined models + providers)
func (s *Server) handleLLMInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check cache
	llmCacheMu.RLock()
	if llmCache != nil && time.Since(llmCacheTime) < llmCacheTTL {
		cached := *llmCache
		llmCacheMu.RUnlock()
		writeJSON(w, cached)
		return
	}
	llmCacheMu.RUnlock()

	// Fetch fresh data
	insights, err := s.buildLLMInsights()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"error":   err.Error(),
			"models":  []ModelInfo{},
			"providers": []ProviderStatus{},
		})
		return
	}

	// Update cache
	llmCacheMu.Lock()
	llmCache = insights
	llmCacheTime = time.Now()
	llmCacheMu.Unlock()

	writeJSON(w, insights)
}

// handleLLMTest handles POST /api/llm/test - test model connectivity
func (s *Server) handleLLMTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request ModelTestRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if request.ModelID == "" {
		writeError(w, http.StatusBadRequest, "model_id required")
		return
	}

	// For now, model testing is not implemented - would require actual LLM call
	result := ModelTestResponse{
		Success:   false,
		ModelID:   request.ModelID,
		LatencyMS: 0,
		Error:     "Model testing not yet implemented",
	}

	writeJSON(w, result)
}

// handleLLMDefault handles GET /api/llm/default - get the default model
func (s *Server) handleLLMDefault(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defaultModel, err := s.fetchDefaultModel()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, defaultModel)
}

// fetchModelsFromControlPlane fetches models from the control plane API
func (s *Server) fetchModelsFromControlPlane() ([]ModelInfo, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	// Parse response - control plane returns []*Model
	var rawModels []struct {
		ID                 string                 `json:"id"`
		Value              string                 `json:"value"`
		Label              string                 `json:"label"`
		Provider           string                 `json:"provider"`
		Logo               *string                `json:"logo"`
		Enabled            bool                   `json:"enabled"`
		Recommended        bool                   `json:"recommended"`
		Capabilities       map[string]interface{} `json:"capabilities"`
		Pricing            map[string]interface{} `json:"pricing"`
		CompatibleRuntimes []string               `json:"compatible_runtimes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawModels); err != nil {
		return nil, err
	}

	// Convert to our type
	models := make([]ModelInfo, 0, len(rawModels))
	for _, m := range rawModels {
		logo := ""
		if m.Logo != nil {
			logo = *m.Logo
		}
		models = append(models, ModelInfo{
			ID:                 m.ID,
			Value:              m.Value,
			Label:              m.Label,
			Provider:           m.Provider,
			Logo:               logo,
			Enabled:            m.Enabled,
			Recommended:        m.Recommended,
			Capabilities:       m.Capabilities,
			Pricing:            m.Pricing,
			CompatibleRuntimes: m.CompatibleRuntimes,
		})
	}

	return models, nil
}

// fetchProvidersFromControlPlane fetches provider names from control plane
func (s *Server) fetchProvidersFromControlPlane() ([]ProviderStatus, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/models/providers", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var providerNames []string
	if err := json.NewDecoder(resp.Body).Decode(&providerNames); err != nil {
		return nil, err
	}

	// Convert to ProviderStatus
	providers := make([]ProviderStatus, 0, len(providerNames))
	for _, name := range providerNames {
		providers = append(providers, ProviderStatus{
			Name:      name,
			Connected: true, // Assume connected if returned
		})
	}

	return providers, nil
}

// fetchDefaultModel fetches the default model from control plane
func (s *Server) fetchDefaultModel() (*ModelInfo, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/models/default", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var rawModel struct {
		ID                 string                 `json:"id"`
		Value              string                 `json:"value"`
		Label              string                 `json:"label"`
		Provider           string                 `json:"provider"`
		Logo               *string                `json:"logo"`
		Enabled            bool                   `json:"enabled"`
		Recommended        bool                   `json:"recommended"`
		Capabilities       map[string]interface{} `json:"capabilities"`
		Pricing            map[string]interface{} `json:"pricing"`
		CompatibleRuntimes []string               `json:"compatible_runtimes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawModel); err != nil {
		return nil, err
	}

	logo := ""
	if rawModel.Logo != nil {
		logo = *rawModel.Logo
	}

	return &ModelInfo{
		ID:                 rawModel.ID,
		Value:              rawModel.Value,
		Label:              rawModel.Label,
		Provider:           rawModel.Provider,
		Logo:               logo,
		Enabled:            rawModel.Enabled,
		Recommended:        rawModel.Recommended,
		Capabilities:       rawModel.Capabilities,
		Pricing:            rawModel.Pricing,
		CompatibleRuntimes: rawModel.CompatibleRuntimes,
	}, nil
}

// buildLLMInsights builds combined LLM insights
func (s *Server) buildLLMInsights() (*LLMInsights, error) {
	models, modelsErr := s.fetchModelsFromControlPlane()
	providers, providersErr := s.fetchProvidersFromControlPlane()
	defaultModel, _ := s.fetchDefaultModel()

	// Build provider model counts
	providerCounts := make(map[string]int)
	if models != nil {
		for _, m := range models {
			providerCounts[m.Provider]++
		}
	}

	// Enrich providers with model counts
	if providers != nil {
		for i := range providers {
			providers[i].ModelCount = providerCounts[providers[i].Name]
		}
	}

	insights := &LLMInsights{
		Models:       models,
		Providers:    providers,
		DefaultModel: defaultModel,
		LastUpdated:  time.Now(),
		CachedUntil:  time.Now().Add(llmCacheTTL),
	}

	// Return error only if both failed
	if modelsErr != nil && providersErr != nil {
		return insights, modelsErr
	}

	return insights, nil
}
