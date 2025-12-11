package errors

const (
	// ExitCodeSuccess indicates successful execution
	ExitCodeSuccess = 0

	// ExitCodeRuntime indicates a general runtime error
	ExitCodeRuntime = 1

	// ExitCodeValidation indicates a usage/validation error (follows bash convention)
	ExitCodeValidation = 2

	// ExitCodeAuth indicates an authentication failure
	ExitCodeAuth = 3

	// ExitCodeAPI indicates an API/service error
	ExitCodeAPI = 4

	// ExitCodeNetwork indicates a network connectivity error
	ExitCodeNetwork = 5

	// ExitCodeConfig indicates a configuration error
	ExitCodeConfig = 6
)

// ExitCode returns the appropriate exit code for an error type
func ExitCode(t ErrorType) int {
	switch t {
	case ErrorTypeValidation:
		return ExitCodeValidation
	case ErrorTypeAuth:
		return ExitCodeAuth
	case ErrorTypeAPI:
		return ExitCodeAPI
	case ErrorTypeNetwork:
		return ExitCodeNetwork
	case ErrorTypeConfig:
		return ExitCodeConfig
	case ErrorTypeRuntime:
		return ExitCodeRuntime
	default:
		return ExitCodeRuntime
	}
}

// ExitCodeFromError extracts the exit code from an error
// Returns ExitCodeRuntime for non-CLIError types
func ExitCodeFromError(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	if cliErr, ok := err.(*CLIError); ok {
		return ExitCode(cliErr.Type)
	}

	return ExitCodeRuntime
}
