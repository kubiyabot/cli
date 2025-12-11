package version

import (
	"testing"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latestVersion  string
		expectedUpdate bool
		description    string
	}{
		{
			name:           "Patch update available",
			currentVersion: "v2.6.0",
			latestVersion:  "v2.6.1",
			expectedUpdate: true,
			description:    "v2.6.1 > v2.6.0",
		},
		{
			name:           "Minor update available",
			currentVersion: "v2.6.0",
			latestVersion:  "v2.7.0",
			expectedUpdate: true,
			description:    "v2.7.0 > v2.6.0",
		},
		{
			name:           "Major update available",
			currentVersion: "v2.6.0",
			latestVersion:  "v3.0.0",
			expectedUpdate: true,
			description:    "v3.0.0 > v2.6.0",
		},
		{
			name:           "Double digit minor version",
			currentVersion: "v2.9.0",
			latestVersion:  "v2.10.0",
			expectedUpdate: true,
			description:    "v2.10.0 > v2.9.0",
		},
		{
			name:           "Double digit major version",
			currentVersion: "v9.0.0",
			latestVersion:  "v10.0.0",
			expectedUpdate: true,
			description:    "v10.0.0 > v9.0.0",
		},
		{
			name:           "Same version",
			currentVersion: "v2.6.0",
			latestVersion:  "v2.6.0",
			expectedUpdate: false,
			description:    "v2.6.0 == v2.6.0",
		},
		{
			name:           "Older version",
			currentVersion: "v2.7.0",
			latestVersion:  "v2.6.0",
			expectedUpdate: false,
			description:    "v2.6.0 < v2.7.0",
		},
		{
			name:           "Dev version",
			currentVersion: "dev",
			latestVersion:  "v2.6.0",
			expectedUpdate: false,
			description:    "dev version should not update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with the OLD buggy implementation for comparison
			hasUpdateBuggy := tt.latestVersion > tt.currentVersion

			// Test with the NEW fixed implementation
			hasUpdateFixed, err := compareVersions(tt.currentVersion, tt.latestVersion)
			if err != nil {
				t.Fatalf("compareVersions failed: %v", err)
			}

			// Log both results
			t.Logf("Testing: %s", tt.description)
			t.Logf("  OLD (string comparison): %v", hasUpdateBuggy)
			t.Logf("  NEW (semver comparison):  %v", hasUpdateFixed)
			t.Logf("  Expected:                 %v", tt.expectedUpdate)

			// Verify the new implementation is correct
			if hasUpdateFixed != tt.expectedUpdate {
				t.Errorf("FAILED: compareVersions returned %v, expected %v", hasUpdateFixed, tt.expectedUpdate)
			}

			// Show if the old implementation was broken
			if hasUpdateBuggy != tt.expectedUpdate {
				t.Logf("  ✓ Fixed: Old implementation was BROKEN for this case")
			}
		})
	}
}

func TestGetUpdateMessage(t *testing.T) {
	// Save original version
	originalVersion := Version
	defer func() { Version = originalVersion }()

	// Test with dev version (should return empty)
	Version = "dev"
	msg := GetUpdateMessage()
	if msg != "" {
		t.Errorf("Expected empty message for dev version, got: %s", msg)
	}

	t.Logf("✓ GetUpdateMessage works correctly for dev version")
}
