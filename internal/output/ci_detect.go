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

// IsTTY returns true if stdout is a TTY
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// IsStderrTTY returns true if stderr is a TTY
func IsStderrTTY() bool {
	return isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
}

// ShouldAutoEnableStreaming returns true if streaming should be auto-enabled
// This happens in CI environments or when output is piped
func ShouldAutoEnableStreaming() bool {
	return IsCI() || !IsTTY()
}

// ResolveStreamFormat determines the stream format based on explicit setting and environment
// Returns "text" for TTY, "json" for CI/pipes
func ResolveStreamFormat(explicit string) string {
	switch explicit {
	case "text":
		return "text"
	case "json":
		return "json"
	case "auto", "":
		if IsCI() || !IsStderrTTY() {
			return "json"
		}
		return "text"
	default:
		return "json"
	}
}
