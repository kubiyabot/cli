package util

import "github.com/kubiyabot/cli/internal/kubiya"

// CountRequiredArgs counts the number of required arguments in a tool
func CountRequiredArgs(args []kubiya.ToolArg) int {
	count := 0
	for _, arg := range args {
		if arg.Required {
			count++
		}
	}
	return count
}
