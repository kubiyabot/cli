package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "valid env file with various formats",
			content: `
KEY1=value1
KEY2=value2
# Comment line
KEY3="quoted value"
KEY4='single quoted'
EMPTY_VALUE=
`,
			expected: map[string]string{
				"KEY1":        "value1",
				"KEY2":        "value2",
				"KEY3":        "quoted value",
				"KEY4":        "single quoted",
				"EMPTY_VALUE": "",
			},
			wantErr: false,
		},
		{
			name: "empty lines and comments",
			content: `
# This is a comment

KEY1=value1

# Another comment
KEY2=value2
`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name: "malformed lines are skipped",
			content: `
KEY1=value1
MALFORMED_LINE_WITHOUT_EQUALS
KEY2=value2
`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name: "values with equals signs",
			content: `
DATABASE_URL=postgres://user:pass@host:5432/db
API_KEY=abc=def=ghi
`,
			expected: map[string]string{
				"DATABASE_URL": "postgres://user:pass@host:5432/db",
				"API_KEY":      "abc=def=ghi",
			},
			wantErr: false,
		},
		{
			name: "values with spaces",
			content: `
MESSAGE=Hello World
PATH=/usr/local/bin:/usr/bin
`,
			expected: map[string]string{
				"MESSAGE": "Hello World",
				"PATH":    "/usr/local/bin:/usr/bin",
			},
			wantErr: false,
		},
		{
			name:     "empty file",
			content:  "",
			expected: map[string]string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "test-*.env")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			tmpFile.Close()

			// Test loading
			result, err := loadEnvFile(tmpFile.Name())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLoadEnvFile_Security(t *testing.T) {
	t.Run("rejects directory traversal paths", func(t *testing.T) {
		_, err := loadEnvFile("../../../etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal")
	})

	t.Run("rejects non-existent files", func(t *testing.T) {
		_, err := loadEnvFile("/nonexistent/file.env")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("rejects directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := loadEnvFile(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "directory traversal attack",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "directory traversal",
		},
		{
			name:    "non-existent file",
			path:    "/nonexistent/file.txt",
			wantErr: true,
		},
		{
			name:    "directory instead of file",
			path:    "/tmp",
			wantErr: true,
			errMsg:  "directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("valid file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		err = validateFilePath(tmpFile.Name())
		assert.NoError(t, err)
	})
}

func TestValidateWorkingDir(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "directory traversal attack",
			path:    "../../../etc",
			wantErr: true,
			errMsg:  "directory traversal",
		},
		{
			name:    "non-existent directory",
			path:    "/nonexistent/directory",
			wantErr: true,
			errMsg:  "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkingDir(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := validateWorkingDir(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		err = validateWorkingDir(tmpFile.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

func TestValidateSkillDir(t *testing.T) {
	t.Run("valid directory with files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a file in the directory
		testFile := filepath.Join(tmpDir, "skill.py")
		err := os.WriteFile(testFile, []byte("print('test')"), 0644)
		require.NoError(t, err)

		err = validateSkillDir(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("empty directory warns but succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := validateSkillDir(tmpDir)
		// Should not error, but will print warning
		assert.NoError(t, err)
	})

	t.Run("invalid directory", func(t *testing.T) {
		err := validateSkillDir("/nonexistent/skills")
		assert.Error(t, err)
	})

	t.Run("directory traversal attack", func(t *testing.T) {
		err := validateSkillDir("../../../usr")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal")
	})
}

func TestMergeEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		maps     []map[string]string
		expected map[string]string
	}{
		{
			name:     "single map",
			maps:     []map[string]string{{"KEY1": "value1", "KEY2": "value2"}},
			expected: map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name: "multiple maps with override",
			maps: []map[string]string{
				{"KEY1": "value1", "KEY2": "value2"},
				{"KEY2": "override2", "KEY3": "value3"},
			},
			expected: map[string]string{"KEY1": "value1", "KEY2": "override2", "KEY3": "value3"},
		},
		{
			name: "CLI overrides .env file",
			maps: []map[string]string{
				{"API_KEY": "from_file", "DEBUG": "false"},
				{"API_KEY": "from_cli", "ENVIRONMENT": "production"},
			},
			expected: map[string]string{
				"API_KEY":     "from_cli",
				"DEBUG":       "false",
				"ENVIRONMENT": "production",
			},
		},
		{
			name:     "empty maps",
			maps:     []map[string]string{{}, {}},
			expected: map[string]string{},
		},
		{
			name:     "no maps",
			maps:     []map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEnvVars(tt.maps...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadEnvFile_Integration(t *testing.T) {
	t.Run("realistic .env file", func(t *testing.T) {
		content := `
# Application Configuration
APP_NAME=MyApp
APP_ENV=production

# Database Configuration
DATABASE_URL=postgres://user:password@localhost:5432/mydb
DATABASE_POOL_SIZE=10

# API Keys (these are test keys)
STRIPE_API_KEY="sk_test_123456789"
TWILIO_API_KEY='AC123456789'

# Feature Flags
ENABLE_FEATURE_X=true
ENABLE_FEATURE_Y=false

# Empty values
OPTIONAL_CONFIG=
`

		tmpFile, err := os.CreateTemp("", "realistic-*.env")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		result, err := loadEnvFile(tmpFile.Name())
		require.NoError(t, err)

		expected := map[string]string{
			"APP_NAME":           "MyApp",
			"APP_ENV":            "production",
			"DATABASE_URL":       "postgres://user:password@localhost:5432/mydb",
			"DATABASE_POOL_SIZE": "10",
			"STRIPE_API_KEY":     "sk_test_123456789",
			"TWILIO_API_KEY":     "AC123456789",
			"ENABLE_FEATURE_X":   "true",
			"ENABLE_FEATURE_Y":   "false",
			"OPTIONAL_CONFIG":    "",
		}

		assert.Equal(t, expected, result)
	})
}
