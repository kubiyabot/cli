package util

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/kubiya"
)

// CountRequiredArgs counts the number of required arguments in a tool
func CountRequiredArgs(args interface{}) int {
	count := 0

	switch typedArgs := args.(type) {
	case []kubiya.Arg:
		for _, arg := range typedArgs {
			if arg.Required {
				count++
			}
		}
	case []kubiya.ToolArg:
		for _, arg := range typedArgs {
			if arg.Required {
				count++
			}
		}
	}

	return count
}

func PrintArgs(args []kubiya.Arg) {
	for _, arg := range args {
		fmt.Printf("%s: %s\n", arg.Name, arg.Description)
	}
}
