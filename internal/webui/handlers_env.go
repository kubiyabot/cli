package webui

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Sensitive environment variable patterns
var sensitivePatterns = []string{
	"KEY", "SECRET", "TOKEN", "PASSWORD", "CREDENTIAL",
	"API_KEY", "APIKEY", "AUTH", "PRIVATE",
}

// handleEnvList handles GET/PUT /api/env - list or update environment variables
func (s *Server) handleEnvList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleEnvGet(w, r)
	case http.MethodPut:
		s.handleEnvUpdate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleEnvGet handles GET /api/env - list all environment variables
func (s *Server) handleEnvGet(w http.ResponseWriter, r *http.Request) {
	// Get all environment variables
	envVars := s.collectEnvVariables()

	// Sort by key
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Key < envVars[j].Key
	})

	writeJSON(w, map[string]interface{}{
		"variables": envVars,
		"count":     len(envVars),
		"env_file":  filepath.Join(s.config.WorkerDir, ".env"),
	})
}

// handleEnvUpdate handles PUT /api/env - update environment variables
func (s *Server) handleEnvUpdate(w http.ResponseWriter, r *http.Request) {
	var request EnvUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(request.Variables) == 0 {
		writeError(w, http.StatusBadRequest, "no variables provided")
		return
	}

	// Update environment variables in memory
	updatedKeys := make([]string, 0, len(request.Variables))
	for key, value := range request.Variables {
		if key == "" {
			continue
		}
		os.Setenv(key, value)
		updatedKeys = append(updatedKeys, key)
	}

	response := EnvUpdateResponse{
		Success:       true,
		Message:       "Environment variables updated",
		RestartNeeded: true, // Worker should restart to pick up new values
		UpdatedKeys:   updatedKeys,
	}

	// Save to file if requested
	if request.SaveToFile {
		envFilePath := filepath.Join(s.config.WorkerDir, ".env")
		if err := s.saveEnvFile(envFilePath, request.Variables); err != nil {
			response.Success = false
			response.Message = "Failed to save to .env file: " + err.Error()
		} else {
			response.EnvFilePath = envFilePath
			response.Message = "Environment variables updated and saved to .env"
		}
	}

	// Log the update
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "env",
		Message:   "Environment variables updated: " + strings.Join(updatedKeys, ", "),
	})

	writeJSON(w, response)
}

// handleEnvSave handles POST /api/env/save - save current env to .env file
func (s *Server) handleEnvSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Collect custom environment variables (not system ones)
	envVars := s.collectEnvVariables()
	customVars := make(map[string]string)
	for _, v := range envVars {
		if v.Source == EnvSourceCustom || v.Source == EnvSourceWorker {
			customVars[v.Key] = v.Value
		}
	}

	envFilePath := filepath.Join(s.config.WorkerDir, ".env")
	if err := s.saveEnvFile(envFilePath, customVars); err != nil {
		writeJSON(w, EnvUpdateResponse{
			Success: false,
			Message: "Failed to save .env file: " + err.Error(),
		})
		return
	}

	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "env",
		Message:   "Environment saved to .env file",
	})

	writeJSON(w, EnvUpdateResponse{
		Success:     true,
		Message:     "Environment saved to .env file",
		EnvFilePath: envFilePath,
	})
}

// handleEnvReload handles POST /api/env/reload - reload from .env file
func (s *Server) handleEnvReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	envFilePath := filepath.Join(s.config.WorkerDir, ".env")
	loadedVars, err := s.loadEnvFile(envFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, EnvUpdateResponse{
				Success: false,
				Message: "No .env file found at " + envFilePath,
			})
			return
		}
		writeJSON(w, EnvUpdateResponse{
			Success: false,
			Message: "Failed to load .env file: " + err.Error(),
		})
		return
	}

	// Apply loaded variables
	updatedKeys := make([]string, 0, len(loadedVars))
	for key, value := range loadedVars {
		os.Setenv(key, value)
		updatedKeys = append(updatedKeys, key)
	}

	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "env",
		Message:   "Environment reloaded from .env file: " + strings.Join(updatedKeys, ", "),
	})

	writeJSON(w, EnvUpdateResponse{
		Success:       true,
		Message:       "Environment reloaded from .env file",
		RestartNeeded: true,
		UpdatedKeys:   updatedKeys,
		EnvFilePath:   envFilePath,
	})
}

// collectEnvVariables collects all environment variables with metadata
func (s *Server) collectEnvVariables() []EnvVariable {
	// Get custom vars from .env file
	envFilePath := filepath.Join(s.config.WorkerDir, ".env")
	customVars, _ := s.loadEnvFile(envFilePath)

	// System-level vars we know about
	systemVars := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "SHELL": true,
		"TERM": true, "LANG": true, "LC_ALL": true, "PWD": true,
		"TMPDIR": true, "TEMP": true, "TMP": true,
	}

	// Worker-related vars
	workerVarPrefixes := []string{
		"KUBIYA_", "LITELLM_", "LANGFUSE_", "OPENAI_", "ANTHROPIC_",
		"AZURE_", "AWS_", "GOOGLE_", "VERTEX_",
	}

	var envVars []EnvVariable
	seen := make(map[string]bool)

	// Process all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		if seen[key] {
			continue
		}
		seen[key] = true

		// Determine source
		source := EnvSourceInherited
		if _, isCustom := customVars[key]; isCustom {
			source = EnvSourceCustom
		} else if systemVars[key] {
			source = EnvSourceSystem
		} else {
			for _, prefix := range workerVarPrefixes {
				if strings.HasPrefix(key, prefix) {
					source = EnvSourceWorker
					break
				}
			}
		}

		// Check if sensitive
		sensitive := isSensitiveKey(key)

		// Mask sensitive values
		displayValue := value
		if sensitive {
			displayValue = maskValue(value)
		}

		envVars = append(envVars, EnvVariable{
			Key:       key,
			Value:     displayValue,
			Source:    source,
			Sensitive: sensitive,
			Editable:  source != EnvSourceSystem,
		})
	}

	return envVars
}

// isSensitiveKey checks if a key name indicates sensitive data
func isSensitiveKey(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}
	return false
}

// maskValue masks a sensitive value
func maskValue(value string) string {
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

// saveEnvFile saves variables to a .env file
func (s *Server) saveEnvFile(path string, vars map[string]string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Read existing file to preserve comments and order
	existingVars, comments := s.readEnvFileWithComments(path)

	// Merge with new vars
	for k, v := range vars {
		existingVars[k] = v
	}

	// Write file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header comment
	file.WriteString("# Kubiya Worker Environment Variables\n")
	file.WriteString("# Generated by Worker WebUI\n")
	file.WriteString("# Last updated: " + time.Now().Format(time.RFC3339) + "\n\n")

	// Write preserved comments
	for _, comment := range comments {
		file.WriteString(comment + "\n")
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(existingVars))
	for k := range existingVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write variables
	for _, k := range keys {
		v := existingVars[k]
		// Quote values with spaces or special chars
		if strings.ContainsAny(v, " \t\n\"'$") {
			v = "\"" + strings.ReplaceAll(v, "\"", "\\\"") + "\""
		}
		file.WriteString(k + "=" + v + "\n")
	}

	return nil
}

// loadEnvFile loads variables from a .env file
func (s *Server) loadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Handle escaped quotes
		value = strings.ReplaceAll(value, "\\\"", "\"")
		value = strings.ReplaceAll(value, "\\'", "'")

		vars[key] = value
	}

	return vars, scanner.Err()
}

// readEnvFileWithComments reads a .env file preserving comments
func (s *Server) readEnvFileWithComments(path string) (map[string]string, []string) {
	vars := make(map[string]string)
	comments := []string{}

	file, err := os.Open(path)
	if err != nil {
		return vars, comments
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Preserve non-header comments
		if strings.HasPrefix(line, "#") && !strings.Contains(line, "Generated by") &&
			!strings.Contains(line, "Last updated") && !strings.Contains(line, "Kubiya Worker") {
			comments = append(comments, line)
			continue
		}

		if line == "" {
			continue
		}

		// Parse variable
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
			vars[key] = value
		}
	}

	return vars, comments
}
