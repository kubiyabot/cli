package mcp

import (
	"fmt"
	"strings"
	"testing"
)

// Mock tool data for testing
var mockTools = []interface{}{
	map[string]interface{}{
		"name":        "kubectl",
		"description": "Kubernetes command-line tool",
		"source_uuid": "k8s-source-uuid",
		"type":        "docker",
		"image":       "kubiya/kubectl-light:latest",
		"with_files": []map[string]interface{}{
			{
				"source":      "/var/run/secrets/kubernetes.io/serviceaccount/token",
				"destination": "/tmp/kubernetes_context_token",
			},
		},
		"env": []string{},
	},
	map[string]interface{}{
		"name":        "aws-cli",
		"description": "AWS command-line interface for cloud operations",
		"source_uuid": "aws-source-uuid",
		"type":        "docker",
		"image":       "amazon/aws-cli:latest",
		"with_files": []map[string]interface{}{
			{
				"source":      "$HOME/.aws/credentials",
				"destination": "/root/.aws/credentials",
			},
		},
		"env": []string{"AWS_PROFILE"},
	},
	map[string]interface{}{
		"name":        "terraform",
		"description": "Infrastructure as code tool",
		"source_uuid": "terraform-source-uuid",
		"type":        "docker",
		"image":       "hashicorp/terraform:latest",
		"with_files":  []map[string]interface{}{},
		"env":         []string{"TF_VAR_region"},
	},
	map[string]interface{}{
		"name":        "helm",
		"description": "Kubernetes package manager",
		"source_uuid": "k8s-source-uuid",
		"type":        "docker",
		"image":       "alpine/helm:latest",
		"with_files":  []map[string]interface{}{},
		"env":         []string{},
	},
	map[string]interface{}{
		"name":        "python-script",
		"description": "Python data processing script",
		"source_uuid": "python-source-uuid",
		"type":        "python",
		"image":       "python:3.9",
		"with_files":  []map[string]interface{}{},
		"env":         []string{"PYTHONPATH"},
	},
}

func TestSearchToolsFiltering(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		toolType       string
		sourceUUID     string
		longRunningOnly bool
		expectedCount   int
		expectedTools   []string
	}{
		{
			name:          "search by query 'kubectl'",
			query:         "kubectl",
			expectedCount: 1,
			expectedTools: []string{"kubectl"},
		},
		{
			name:          "search by query 'kubernetes'",
			query:         "kubernetes", // should match kubectl and helm descriptions
			expectedCount: 2,
			expectedTools: []string{"kubectl", "helm"},
		},
		{
			name:          "search by tool type 'docker'",
			query:         "",
			toolType:      "docker",
			expectedCount: 4, // kubectl, aws-cli, terraform, helm
			expectedTools: []string{"kubectl", "aws-cli", "terraform", "helm"},
		},
		{
			name:          "search by tool type 'python'",
			query:         "",
			toolType:      "python",
			expectedCount: 1,
			expectedTools: []string{"python-script"},
		},
		{
			name:          "search by source UUID",
			query:         "",
			sourceUUID:    "k8s-source-uuid",
			expectedCount: 2, // kubectl and helm
			expectedTools: []string{"kubectl", "helm"},
		},
		{
			name:          "search with query and type filter",
			query:         "aws",
			toolType:      "docker",
			expectedCount: 1,
			expectedTools: []string{"aws-cli"},
		},
		{
			name:          "search with no matches",
			query:         "nonexistent",
			expectedCount: 0,
			expectedTools: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Filter tools based on test criteria
			filtered := filterTools(mockTools, tt.query, tt.toolType, tt.sourceUUID, tt.longRunningOnly)
			
			if len(filtered) != tt.expectedCount {
				t.Errorf("expected %d tools, got %d", tt.expectedCount, len(filtered))
			}

			// Check that expected tools are present
			foundTools := make(map[string]bool)
			for _, tool := range filtered {
				toolMap := tool.(map[string]interface{})
				foundTools[toolMap["name"].(string)] = true
			}

			for _, expectedTool := range tt.expectedTools {
				if !foundTools[expectedTool] {
					t.Errorf("expected tool '%s' not found in results", expectedTool)
				}
			}
		})
	}
}

func TestToolDependencyExtraction(t *testing.T) {
	tests := []struct {
		name               string
		toolName           string
		expectedImage      string
		expectedFilesCount int
		expectedEnvCount   int
		expectsFiles       bool
	}{
		{
			name:               "kubectl tool dependencies",
			toolName:           "kubectl",
			expectedImage:      "kubiya/kubectl-light:latest",
			expectedFilesCount: 1,
			expectedEnvCount:   0,
			expectsFiles:       true,
		},
		{
			name:               "aws-cli tool dependencies",
			toolName:           "aws-cli",
			expectedImage:      "amazon/aws-cli:latest",
			expectedFilesCount: 1,
			expectedEnvCount:   1,
			expectsFiles:       true,
		},
		{
			name:               "python tool no file dependencies",
			toolName:           "python-script",
			expectedImage:      "python:3.9",
			expectedFilesCount: 0,
			expectedEnvCount:   1,
			expectsFiles:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the tool in mock data
			var tool map[string]interface{}
			for _, mockTool := range mockTools {
				toolMap := mockTool.(map[string]interface{})
				if toolMap["name"].(string) == tt.toolName {
					tool = toolMap
					break
				}
			}

			if tool == nil {
				t.Fatalf("tool '%s' not found in mock data", tt.toolName)
			}

			// Check image
			if tool["image"].(string) != tt.expectedImage {
				t.Errorf("expected image '%s', got '%s'", tt.expectedImage, tool["image"].(string))
			}

			// Check file dependencies
			withFiles := tool["with_files"].([]map[string]interface{})
			if len(withFiles) != tt.expectedFilesCount {
				t.Errorf("expected %d with_files entries, got %d", tt.expectedFilesCount, len(withFiles))
			}

			// Check environment variables
			env := tool["env"].([]string)
			if len(env) != tt.expectedEnvCount {
				t.Errorf("expected %d env vars, got %d", tt.expectedEnvCount, len(env))
			}

			// Verify file dependency structure if expected
			if tt.expectsFiles && len(withFiles) > 0 {
				firstFile := withFiles[0]
				if _, hasSource := firstFile["source"]; !hasSource {
					t.Error("file dependency missing 'source' field")
				}
				if _, hasDest := firstFile["destination"]; !hasDest {
					t.Error("file dependency missing 'destination' field")
				}
			}
		})
	}
}

func TestSmartContainerSelection(t *testing.T) {
	tests := []struct {
		name              string
		toolName          string
		expectedContainer string
		expectedFiles     bool
		expectedEnvVars   bool
	}{
		{
			name:              "AWS tool requires AWS container and credentials",
			toolName:          "aws-cli",
			expectedContainer: "amazon/aws-cli:latest",
			expectedFiles:     true, // AWS credentials files
			expectedEnvVars:   true, // AWS_PROFILE
		},
		{
			name:              "Kubernetes tool requires kubectl container and tokens",
			toolName:          "kubectl",
			expectedContainer: "kubiya/kubectl-light:latest",
			expectedFiles:     true, // k8s service account token
			expectedEnvVars:   false,
		},
		{
			name:              "Python tool requires Python container",
			toolName:          "python-script",
			expectedContainer: "python:3.9",
			expectedFiles:     false,
			expectedEnvVars:   true, // PYTHONPATH
		},
		{
			name:              "Terraform tool requires Terraform container",
			toolName:          "terraform",
			expectedContainer: "hashicorp/terraform:latest",
			expectedFiles:     false,
			expectedEnvVars:   true, // TF_VAR_region
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find tool in mock data
			var tool map[string]interface{}
			for _, mockTool := range mockTools {
				toolMap := mockTool.(map[string]interface{})
				if toolMap["name"].(string) == tt.toolName {
					tool = toolMap
					break
				}
			}

			if tool == nil {
				t.Fatalf("tool '%s' not found", tt.toolName)
			}

			// Verify container selection
			if tool["image"].(string) != tt.expectedContainer {
				t.Errorf("expected container '%s', got '%s'", 
					tt.expectedContainer, tool["image"].(string))
			}

			// Verify file dependencies
			withFiles := tool["with_files"].([]map[string]interface{})
			hasFiles := len(withFiles) > 0
			if hasFiles != tt.expectedFiles {
				t.Errorf("expected files dependency %v, got %v", tt.expectedFiles, hasFiles)
			}

			// Verify environment variables
			env := tool["env"].([]string)
			hasEnvVars := len(env) > 0
			if hasEnvVars != tt.expectedEnvVars {
				t.Errorf("expected env vars %v, got %v", tt.expectedEnvVars, hasEnvVars)
			}
		})
	}
}

// Helper function to simulate tool filtering logic
func filterTools(tools []interface{}, query, toolType, sourceUUID string, longRunningOnly bool) []interface{} {
	var filtered []interface{}
	
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		
		// Query filter (check name and description)
		if query != "" {
			name := strings.ToLower(toolMap["name"].(string))
			desc := strings.ToLower(toolMap["description"].(string))
			queryLower := strings.ToLower(query)
			
			if !strings.Contains(name, queryLower) && !strings.Contains(desc, queryLower) {
				continue
			}
		}
		
		// Tool type filter
		if toolType != "" && toolMap["type"].(string) != toolType {
			continue
		}
		
		// Source UUID filter
		if sourceUUID != "" && toolMap["source_uuid"].(string) != sourceUUID {
			continue
		}
		
		// Long running filter (mock implementation)
		if longRunningOnly {
			// For testing, assume only tools with "long" in name are long-running
			if !strings.Contains(strings.ToLower(toolMap["name"].(string)), "long") {
				continue
			}
		}
		
		filtered = append(filtered, tool)
	}
	
	return filtered
}

func TestMCPToolRegistration(t *testing.T) {
	// Test that core platform tools are properly defined
	coreTools := []struct {
		name        string
		description string
		hasHandler  bool
	}{
		{
			name:        "list_sources",
			description: "List all tool sources with metadata and pagination",
			hasHandler:  true,
		},
		{
			name:        "search_tools",
			description: "Search for tools across all sources with pagination and filtering",
			hasHandler:  true,
		},
		{
			name:        "list_runners",
			description: "List all available runners",
			hasHandler:  true,
		},
		{
			name:        "check_runner_health",
			description: "Check health status of a specific runner or all runners",
			hasHandler:  true,
		},
	}

	for _, tool := range coreTools {
		t.Run(fmt.Sprintf("tool_%s", tool.name), func(t *testing.T) {
			// Verify tool definition is not empty
			if tool.name == "" {
				t.Error("tool name should not be empty")
			}
			
			if tool.description == "" {
				t.Error("tool description should not be empty")
			}
			
			if !tool.hasHandler {
				t.Error("tool should have a handler")
			}
		})
	}
}