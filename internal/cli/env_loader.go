package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadEnvFile loads environment variables from a .env file with security validations.
// It parses KEY=VALUE format, skips comments, and removes quotes from values.
func loadEnvFile(path string) (map[string]string, error) {
	// Validate path for security
	if err := validateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid env file path: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}
	defer file.Close()

	envMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Skip malformed lines with a warning
			fmt.Fprintf(os.Stderr, "Warning: Skipping malformed line %d in %s: %s\n", lineNumber, path, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Validate key is not empty
		if key == "" {
			fmt.Fprintf(os.Stderr, "Warning: Skipping line %d with empty key in %s\n", lineNumber, path)
			continue
		}

		// Remove surrounding quotes if present (both single and double)
		value = strings.Trim(value, `"'`)

		// Basic sanitization: ensure key doesn't contain suspicious characters
		if strings.ContainsAny(key, "\n\r\t") {
			return nil, fmt.Errorf("line %d: key contains invalid characters", lineNumber)
		}

		envMap[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading env file: %w", err)
	}

	return envMap, nil
}

// validateFilePath validates a file path for security concerns.
// It prevents directory traversal and ensures the file exists and is readable.
func validateFilePath(path string) error {
	// Prevent directory traversal attacks
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains directory traversal sequence (..)")
	}

	// Convert to absolute path for validation
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Ensure it's a file, not a directory
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("file is not readable: %w", err)
	}
	file.Close()

	return nil
}

// validateWorkingDir validates a working directory path for security concerns.
// It prevents directory traversal and ensures the directory exists.
func validateWorkingDir(dir string) error {
	// Prevent directory traversal in relative paths
	if strings.Contains(dir, "..") {
		return fmt.Errorf("working directory contains directory traversal sequence (..)")
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("working directory does not exist: %s", dir)
		}
		return fmt.Errorf("failed to access directory: %w", err)
	}

	// Ensure it's a directory
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	return nil
}

// validateSkillDir validates a skill directory path for security concerns.
// It uses the same validation as working directory.
func validateSkillDir(dir string) error {
	// Same validation as working directory
	if err := validateWorkingDir(dir); err != nil {
		return fmt.Errorf("invalid skill directory: %w", err)
	}

	// Additional check: ensure directory is not empty (optional warning)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read skill directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: Skill directory is empty: %s\n", dir)
	}

	return nil
}

// mergeEnvVars merges multiple environment variable maps.
// Later maps override earlier ones in case of key conflicts.
func mergeEnvVars(maps ...map[string]string) map[string]string {
	result := make(map[string]string)

	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}

	return result
}
