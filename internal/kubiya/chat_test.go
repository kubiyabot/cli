package kubiya

import (
	"context"
	"net/http"
	"testing"
)

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name       string
		agentUUID  string
		message    string
		response   string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful message",
			agentUUID:  "123",
			message:    "test message",
			response:   `{"content":"response message"}`,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error response",
			agentUUID:  "123",
			message:    "test message",
			response:   `{"error":"test error"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "empty agent UUID",
			agentUUID:  "",
			message:    "test message",
			response:   `{"error":"invalid agent"}`,
			statusCode: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if auth := r.Header.Get("Authorization"); auth != "UserKey test-key" {
					t.Errorf("Expected Authorization header, got %s", auth)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			})
			defer server.Close()

			resp, err := client.SendMessage(context.Background(), tt.agentUUID, tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && resp == nil {
				t.Error("SendMessage() returned nil response")
			}
		})
	}
} 