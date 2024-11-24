package tui

import (
	"fmt"
	"strings"

	"github.com/kubiyabot/cli/internal/style"
)

func (s *SourceBrowser) renderToolDetail() string {
	if s.currentTool == nil {
		return "No tool selected"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  Tool: %s ", s.currentTool.Name))))

	if s.currentTool.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n\n", s.currentTool.Description))
	}

	if len(s.currentTool.Args) > 0 {
		b.WriteString(style.SubtitleStyle.Render("Arguments:\n"))
		for _, arg := range s.currentTool.Args {
			required := style.DimStyle.Render("(optional)")
			if arg.Required {
				required = style.HighlightStyle.Render("(required)")
			}
			b.WriteString(fmt.Sprintf("  • %s %s\n", style.HighlightStyle.Render(arg.Name), required))
			if arg.Description != "" {
				b.WriteString(fmt.Sprintf("    %s\n", arg.Description))
			}
		}
		b.WriteString("\n")
	}

	if len(s.currentTool.Env) > 0 {
		b.WriteString(style.SubtitleStyle.Render("Environment Variables:\n"))
		for _, env := range s.currentTool.Env {
			b.WriteString(fmt.Sprintf("  • %s\n", env))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(style.DimStyle.Render("Press 'x' to execute, 'esc' to go back, '?' for help\n"))

	return b.String()
}

func (s *SourceBrowser) renderToolExecution() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  Execute: %s ", s.currentTool.Name))))

	if len(s.inputs) > 0 {
		b.WriteString("Enter arguments:\n\n")
		for i, input := range s.inputs {
			if i == s.execution.activeInput {
				b.WriteString(style.HighlightStyle.Render("> "))
			} else {
				b.WriteString("  ")
			}
			b.WriteString(input.View() + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(style.DimStyle.Render("Press 'tab' to navigate, 'enter' to continue, 'esc' to go back\n"))

	return b.String()
}

func (s *SourceBrowser) renderEnvVarSelection() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  Environment Variables: %s ", s.currentTool.Name))))

	if len(s.currentTool.Env) > 0 {
		b.WriteString("Select source for each environment variable:\n\n")
		for i, env := range s.execution.envVarNames {
			if i == s.execution.activeInput {
				b.WriteString(style.HighlightStyle.Render("> "))
			} else {
				b.WriteString("  ")
			}

			// Show status if value is set
			status := "not set"
			if s.execution.envVars[env] != nil && s.execution.envVars[env].Value != "" {
				status = "✓ set"
			}

			b.WriteString(fmt.Sprintf("%s (%s)\n", env, style.DimStyle.Render(status)))
		}
	}

	b.WriteString("\n")
	b.WriteString(style.DimStyle.Render("Options:\n"))
	b.WriteString(style.DimStyle.Render("• Enter: Choose source for selected variable\n"))
	b.WriteString(style.DimStyle.Render("• e: Use environment variable\n"))
	b.WriteString(style.DimStyle.Render("• s: Use secret\n"))
	b.WriteString(style.DimStyle.Render("• m: Enter value manually\n"))
	b.WriteString(style.DimStyle.Render("• Space: Skip variable\n"))
	b.WriteString(style.DimStyle.Render("• Esc: Go back\n"))

	return b.String()
}

func (s *SourceBrowser) renderEnvVarOptions() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  %s ", s.execution.currentEnvVarName))))

	for i, option := range s.execution.envVarOptions {
		if i == s.execution.activeOption {
			b.WriteString(style.HighlightStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%s %s\n", option.icon, option.label))
	}

	b.WriteString("\n")
	b.WriteString(style.DimStyle.Render("Press 'enter' to select, 'esc' to go back\n"))

	return b.String()
}

func (s *SourceBrowser) renderEnvVarValueInput() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  Enter value for %s ", s.execution.currentEnvVarName))))
	b.WriteString(s.execution.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(style.DimStyle.Render("Press 'enter' to confirm, 'esc' to go back\n"))
	return b.String()
}

func (s *SourceBrowser) renderExecutionConfirmation() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(" 🚀 Execute Tool ")))
	b.WriteString(fmt.Sprintf("Tool: %s\n", s.currentTool.Name))
	b.WriteString(fmt.Sprintf("Source: %s\n\n", s.currentSource.Name))

	b.WriteString("Arguments:\n")
	for name, value := range s.execution.args {
		b.WriteString(fmt.Sprintf("  • %s: %s\n", name, value))
	}
	b.WriteString("\n")

	if len(s.execution.envVars) > 0 {
		b.WriteString("Environment Variables:\n")
		for name, value := range s.execution.envVars {
			if value != nil {
				b.WriteString(fmt.Sprintf("  • %s: %s\n", name, value.Value))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(style.DimStyle.Render("Press 'enter' to execute, 'esc' to go back\n"))
	return b.String()
}

func (s *SourceBrowser) renderExecutionProgress() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(" 🚀 Executing ")))
	b.WriteString(s.execution.output)
	return b.String()
}

func (s *SourceBrowser) renderExecutionSummary() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s\n\n", titleStyle.Render(" 📝 Execution Summary ")))
	b.WriteString(fmt.Sprintf("Tool: %s\n", s.currentTool.Name))
	b.WriteString(fmt.Sprintf("Source: %s\n\n", s.currentSource.Name))

	b.WriteString("Arguments:\n")
	for name, value := range s.execution.args {
		b.WriteString(fmt.Sprintf("  • %s: %s\n", name, value))
	}
	b.WriteString("\n")

	if len(s.execution.envVars) > 0 {
		b.WriteString("Environment Variables:\n")
		for name, value := range s.execution.envVars {
			if value != nil {
				b.WriteString(fmt.Sprintf("  • %s: %s\n", name, value.Value))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (s *SourceBrowser) validateToolExecution() []string {
	var validationErrors []string

	// Validate required arguments
	for _, arg := range s.currentTool.Args {
		if arg.Required {
			if value, exists := s.execution.args[arg.Name]; !exists || value == "" {
				validationErrors = append(validationErrors, fmt.Sprintf("Missing required argument: %s", arg.Name))
			}
		}
	}

	// Validate required environment variables
	for _, env := range s.currentTool.Env {
		if value, exists := s.execution.envVars[env]; !exists || value == nil || value.Value == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("Missing environment variable: %s", env))
		}
	}

	return validationErrors
}

func (s *SourceBrowser) getEnvVarsMap() map[string]string {
	envVars := make(map[string]string)
	for name, status := range s.execution.envVars {
		if status != nil {
			envVars[name] = status.Value
		}
	}
	return envVars
}
