package output

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsCI detects if the CLI is running in a CI environment
// Checks multiple common CI environment variables and TTY status
func IsCI() bool {
	ciEnvVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"KUBIYA_CI_MODE",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"JENKINS_HOME",
		"JENKINS_URL",
		"BUILDKITE",
		"TRAVIS",
		"TEAMCITY_VERSION",
		"BITBUCKET_PIPELINES",
		"DRONE",
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	// Check if stdout is not a TTY (piped or redirected)
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}

	return false
}

// IsInteractive returns true if the output should be interactive
// (opposite of IsCI)
func IsInteractive() bool {
	return !IsCI()
}
