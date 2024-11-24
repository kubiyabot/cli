package kubiya

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubiyabot/cli/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: "https://api.test",
		Debug:   false,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Error("NewClient() returned nil")
	}

	if client.cfg != cfg {
		t.Error("NewClient() did not set config correctly")
	}
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	t.Cleanup(func() { server.Close() })

	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Debug:   false,
	}

	return server, NewClient(cfg)
}

func TestListTeammates(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful response",
			response:   `[{"uuid":"123","name":"Test Agent"}]`,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error response",
			response:   `{"error":"test error"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if auth := r.Header.Get("Authorization"); auth != "UserKey test-key" {
					t.Errorf("Expected Authorization header, got %s", auth)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			})
			defer server.Close()

			teammates, err := client.ListTeammates(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ListTeammates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(teammates) == 0 {
				t.Error("ListTeammates() returned empty list")
			}
		})
	}
} 