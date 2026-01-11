package controlplane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreMemory(t *testing.T) {
	tests := []struct {
		name           string
		request        *entities.MemoryStoreRequest
		mockResponse   *entities.MemoryStoreResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "successful store",
			request: &entities.MemoryStoreRequest{
				Context: entities.MemoryContext{
					Title:   "Test Memory",
					Content: "Test content",
					Tags:    []string{"test", "memory"},
				},
			},
			mockResponse: &entities.MemoryStoreResponse{
				MemoryID: "mem_123",
				Status:   "stored",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "store with metadata",
			request: &entities.MemoryStoreRequest{
				Context: entities.MemoryContext{
					Title:   "AWS Config",
					Content: "Region: us-east-1",
				},
				Metadata: map[string]interface{}{
					"env":    "production",
					"region": "us-east-1",
				},
			},
			mockResponse: &entities.MemoryStoreResponse{
				MemoryID: "mem_456",
				Status:   "processing",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "api error",
			request: &entities.MemoryStoreRequest{
				Context: entities.MemoryContext{
					Title:   "Test",
					Content: "Content",
				},
			},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/memory/store", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.StoreMemory(tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.MemoryID, result.MemoryID)
				assert.Equal(t, tt.mockResponse.Status, result.Status)
			}
		})
	}
}

func TestRecallMemory(t *testing.T) {
	tests := []struct {
		name           string
		request        *entities.MemoryRecallRequest
		mockResponse   *entities.MemoryRecallResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "successful recall",
			request: &entities.MemoryRecallRequest{
				Query: "AWS configuration",
			},
			mockResponse: &entities.MemoryRecallResponse{
				Results: []entities.MemorySearchResult{
					{
						Content:         "Region: us-east-1",
						SimilarityScore: 0.95,
						Metadata: map[string]interface{}{
							"title": "AWS Config",
							"tags":  []string{"aws"},
						},
						Source: "test",
					},
				},
				Query: "AWS configuration",
				Count: 1,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "recall with filters",
			request: &entities.MemoryRecallRequest{
				Query:    "database",
				Tags:     []string{"production"},
				MinScore: 0.7,
			},
			mockResponse: &entities.MemoryRecallResponse{
				Results: []entities.MemorySearchResult{},
				Query:   "database",
				Count:   0,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "api error",
			request: &entities.MemoryRecallRequest{
				Query: "test",
			},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/memory/recall", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.RecallMemory(tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.mockResponse.Results), len(result.Results))
				assert.Equal(t, tt.mockResponse.Count, result.Count)
			}
		})
	}
}

func TestListMemories(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   []*entities.Memory
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "successful list",
			mockResponse: []*entities.Memory{
				{
					MemoryID: "mem_123",
					Context: entities.MemoryContext{
						Title:   "Memory 1",
						Content: "Content 1",
					},
				},
				{
					MemoryID: "mem_456",
					Context: entities.MemoryContext{
						Title:   "Memory 2",
						Content: "Content 2",
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty list",
			mockResponse:   []*entities.Memory{},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "api error",
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/memory/list", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.ListMemories()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.mockResponse), len(result))
			}
		})
	}
}

func TestGetMemoryJobStatus(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		datasetID      string
		mockResponse   *entities.MemoryJobStatus
		mockStatusCode int
		expectError    bool
	}{
		{
			name:      "successful status check",
			jobID:     "job_123",
			datasetID: "ds_123",
			mockResponse: &entities.MemoryJobStatus{
				JobID:    "job_123",
				Status:   "completed",
				Progress: 1.0,
				Message:  "Processing complete",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:      "processing status",
			jobID:     "job_456",
			datasetID: "ds_456",
			mockResponse: &entities.MemoryJobStatus{
				JobID:    "job_456",
				Status:   "processing",
				Progress: 0.5,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "job not found",
			jobID:          "job_999",
			datasetID:      "ds_999",
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/memory/status/"+tt.jobID, r.URL.Path)
				assert.Equal(t, tt.datasetID, r.URL.Query().Get("dataset_id"))
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.GetMemoryJobStatus(tt.jobID, tt.datasetID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.JobID, result.JobID)
				assert.Equal(t, tt.mockResponse.Status, result.Status)
			}
		})
	}
}

func TestCreateDataset(t *testing.T) {
	tests := []struct {
		name           string
		request        *entities.DatasetCreateRequest
		mockResponse   *entities.Dataset
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "successful create org dataset",
			request: &entities.DatasetCreateRequest{
				Name:        "Production Context",
				Description: "Production environment context",
				Scope:       "org",
			},
			mockResponse: &entities.Dataset{
				ID:          "ds_123",
				Name:        "Production Context",
				Description: "Production environment context",
				Scope:       "org",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "create role dataset",
			request: &entities.DatasetCreateRequest{
				Name:         "Team Dataset",
				Scope:        "role",
				AllowedRoles: []string{"developer", "devops"},
			},
			mockResponse: &entities.Dataset{
				ID:           "ds_456",
				Name:         "Team Dataset",
				Scope:        "role",
				AllowedRoles: []string{"developer", "devops"},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "validation error",
			request: &entities.DatasetCreateRequest{
				Name:  "",
				Scope: "org",
			},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/datasets", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.CreateDataset(tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.ID, result.ID)
				assert.Equal(t, tt.mockResponse.Name, result.Name)
				assert.Equal(t, tt.mockResponse.Scope, result.Scope)
			}
		})
	}
}

func TestListDatasets(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   []*entities.Dataset
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "successful list",
			mockResponse: []*entities.Dataset{
				{
					ID:    "ds_123",
					Name:  "Dataset 1",
					Scope: "org",
				},
				{
					ID:    "ds_456",
					Name:  "Dataset 2",
					Scope: "user",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty list",
			mockResponse:   []*entities.Dataset{},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/datasets", r.URL.Path)

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.ListDatasets()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.mockResponse), len(result))
			}
		})
	}
}

func TestGetDataset(t *testing.T) {
	tests := []struct {
		name           string
		datasetID      string
		mockResponse   *entities.Dataset
		mockStatusCode int
		expectError    bool
	}{
		{
			name:      "successful get",
			datasetID: "ds_123",
			mockResponse: &entities.Dataset{
				ID:          "ds_123",
				Name:        "Production Context",
				Description: "Prod env context",
				Scope:       "org",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "dataset not found",
			datasetID:      "ds_999",
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/datasets/"+tt.datasetID, r.URL.Path)

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.GetDataset(tt.datasetID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResponse.ID, result.ID)
			}
		})
	}
}

func TestDeleteDataset(t *testing.T) {
	tests := []struct {
		name           string
		datasetID      string
		mockStatusCode int
		expectError    bool
	}{
		{
			name:           "successful delete",
			datasetID:      "ds_123",
			mockStatusCode: http.StatusNoContent,
			expectError:    false,
		},
		{
			name:           "dataset not found",
			datasetID:      "ds_999",
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/datasets/"+tt.datasetID, r.URL.Path)

				w.WriteHeader(tt.mockStatusCode)
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			err = client.DeleteDataset(tt.datasetID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDatasetData(t *testing.T) {
	tests := []struct {
		name           string
		datasetID      string
		mockResponse   *entities.DatasetDataResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name:      "successful get data",
			datasetID: "ds_123",
			mockResponse: &entities.DatasetDataResponse{
				DatasetID: "ds_123",
				Data: []entities.DatasetDataEntry{
					{
						ID:        "entry_1",
						DatasetID: "ds_123",
						Content:   map[string]interface{}{"key": "value"},
					},
				},
				Count: 1,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:      "empty data",
			datasetID: "ds_456",
			mockResponse: &entities.DatasetDataResponse{
				DatasetID: "ds_456",
				Data:      []entities.DatasetDataEntry{},
				Count:     0,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/context-graph/api/v1/graph/datasets/"+tt.datasetID+"/data", r.URL.Path)

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client, err := NewWithURL("test-api-key", server.URL, false)
			require.NoError(t, err)

			result, err := client.GetDatasetData(tt.datasetID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResponse.DatasetID, result.DatasetID)
				assert.Equal(t, tt.mockResponse.Count, result.Count)
			}
		})
	}
}
