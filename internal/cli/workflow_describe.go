package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type WorkflowDescribe struct {
	Name         string                 `yaml:"name" json:"name"`
	Description  string                 `yaml:"description" json:"description"`
	Version      string                 `yaml:"version" json:"version"`
	Author       string                 `yaml:"author" json:"author"`
	Tags         []string               `yaml:"tags" json:"tags"`
	Env          []string               `yaml:"env" json:"env"`
	Variables    map[string]interface{} `yaml:"variables" json:"variables"`
	Steps        []WorkflowStepDescribe `yaml:"steps" json:"steps"`
	Triggers     []WorkflowTrigger      `yaml:"triggers" json:"triggers"`
	Conditions   []WorkflowCondition    `yaml:"conditions" json:"conditions"`
	Outputs      []WorkflowOutput       `yaml:"outputs" json:"outputs"`
	Timeout      string                 `yaml:"timeout" json:"timeout"`
	Retries      int                    `yaml:"retries" json:"retries"`
	Parallelism  int                    `yaml:"parallelism" json:"parallelism"`
	Dependencies []string               `yaml:"dependencies" json:"dependencies"`
}

type WorkflowStepDescribe struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Executor    WorkflowExecutorDescribe `yaml:"executor" json:"executor"`
	Depends     []string               `yaml:"depends" json:"depends"`
	Condition   string                 `yaml:"condition" json:"condition"`
	Timeout     string                 `yaml:"timeout" json:"timeout"`
	Retries     int                    `yaml:"retries" json:"retries"`
	Output      string                 `yaml:"output" json:"output"`
	OnFailure   string                 `yaml:"on_failure" json:"on_failure"`
	OnSuccess   string                 `yaml:"on_success" json:"on_success"`
	Variables   map[string]interface{} `yaml:"variables" json:"variables"`
}

type WorkflowExecutorDescribe struct {
	Type   string                 `yaml:"type" json:"type"`
	Config map[string]interface{} `yaml:"config" json:"config"`
}

type WorkflowTrigger struct {
	Type   string                 `yaml:"type" json:"type"`
	Config map[string]interface{} `yaml:"config" json:"config"`
}

type WorkflowCondition struct {
	Name      string `yaml:"name" json:"name"`
	Condition string `yaml:"condition" json:"condition"`
}

type WorkflowOutput struct {
	Name        string `yaml:"name" json:"name"`
	Value       string `yaml:"value" json:"value"`
	Description string `yaml:"description" json:"description"`
}

func newWorkflowDescribeCommand(cfg *config.Config) *cobra.Command {
	var format string
	var showSteps bool
	var showEnv bool
	var showDeps bool
	var showOutputs bool
	var showConfig bool

	cmd := &cobra.Command{
		Use:   "describe <workflow-file>",
		Short: "Describe a workflow with detailed information",
		Long: `Describe a workflow file showing comprehensive information including:
• Basic workflow metadata (name, description, version, author)
• Environment variables and configuration
• Step-by-step execution flow with dependencies
• Executor types and configurations
• Input/output parameters
• Triggers and conditions
• Performance and reliability settings

The output can be formatted as table (default), json, yaml, or mermaid for different use cases.`,
		Example: `  # Describe a workflow with default table format
  kubiya workflow describe my-workflow.yaml

  # Show full details including all sections
  kubiya workflow describe my-workflow.yaml --steps --env --deps --outputs --config

  # Export workflow description as JSON
  kubiya workflow describe my-workflow.yaml --format json

  # Generate Mermaid flowchart diagram
  kubiya workflow describe my-workflow.yaml --format mermaid

  # Show only basic info and dependencies
  kubiya workflow describe my-workflow.yaml --deps`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowDescribe(args[0], format, showSteps, showEnv, showDeps, showOutputs, showConfig)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml, mermaid)")
	cmd.Flags().BoolVarP(&showSteps, "steps", "s", false, "Show detailed step information")
	cmd.Flags().BoolVarP(&showEnv, "env", "e", false, "Show environment variables")
	cmd.Flags().BoolVarP(&showDeps, "deps", "d", false, "Show step dependencies")
	cmd.Flags().BoolVarP(&showOutputs, "outputs", "o", false, "Show workflow outputs")
	cmd.Flags().BoolVarP(&showConfig, "config", "c", false, "Show executor configurations")

	return cmd
}

func runWorkflowDescribe(filename, format string, showSteps, showEnv, showDeps, showOutputs, showConfig bool) error {
	// Parse workflow file
	workflow, err := parseWorkflowDescribeFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse workflow file: %w", err)
	}

	// Handle different output formats
	switch format {
	case "json":
		return outputJSON(workflow)
	case "yaml":
		return outputYAML(workflow)
	case "mermaid":
		return outputMermaid(workflow)
	case "table":
		return outputTable(workflow, showSteps, showEnv, showDeps, showOutputs, showConfig)
	default:
		return fmt.Errorf("unsupported format: %s (supported: table, json, yaml, mermaid)", format)
	}
}

func parseWorkflowDescribeFile(filename string) (*WorkflowDescribe, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var workflow WorkflowDescribe
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(content, &workflow)
	case ".json":
		err = json.Unmarshal(content, &workflow)
	default:
		// Try YAML first, then JSON
		err = yaml.Unmarshal(content, &workflow)
		if err != nil {
			err = json.Unmarshal(content, &workflow)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	return &workflow, nil
}

func outputJSON(workflow *WorkflowDescribe) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(workflow)
}

func outputYAML(workflow *WorkflowDescribe) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(workflow)
}

func outputMermaid(workflow *WorkflowDescribe) error {
	fmt.Println("```mermaid")
	fmt.Println("flowchart TD")
	fmt.Println()
	
	// Add workflow title and description as a comment
	if workflow.Name != "" {
		fmt.Printf("    %% Workflow: %s\n", workflow.Name)
	}
	if workflow.Description != "" {
		fmt.Printf("    %% Description: %s\n", workflow.Description)
	}
	fmt.Println()
	
	// Create step nodes with styling based on executor type
	stepNodes := make(map[string]string)
	executorColors := map[string]string{
		"http":            "#FFE5B4",  // Peach
		"docker":          "#E5F3FF",  // Light Blue
		"command":         "#E5FFE5",  // Light Green
		"slack":           "#FFE5FF",  // Light Pink
		"jq":              "#FFFACD",  // Light Yellow
		"llm_completion":  "#F0E68C",  // Khaki
		"tool":            "#DDA0DD",  // Plum
		"kubernetes":      "#B0E0E6",  // Powder Blue
		"aws":             "#FFA07A",  // Light Salmon
		"git":             "#F5DEB3",  // Wheat
		"file":            "#D3D3D3",  // Light Gray
		"default":         "#F0F0F0",  // Light Gray
	}
	
	if len(workflow.Steps) == 0 {
		fmt.Println("    EmptyWorkflow[\"No steps defined\"]")
		fmt.Println("    EmptyWorkflow:::emptyStyle")
		fmt.Println()
		fmt.Println("    classDef emptyStyle fill:#ffcccc,stroke:#ff6666,stroke-width:2px")
		fmt.Println("```")
		return nil
	}
	
	// Generate step nodes
	for i, step := range workflow.Steps {
		nodeId := fmt.Sprintf("step%d", i+1)
		stepNodes[step.Name] = nodeId
		
		// Create node label with step name and executor type
		label := step.Name
		if step.Executor.Type != "" {
			label += fmt.Sprintf("\\n[%s]", step.Executor.Type)
		}
		
		// Choose node shape based on executor type
		var nodeShape string
		switch step.Executor.Type {
		case "http":
			nodeShape = fmt.Sprintf("(%s)", label)
		case "docker":
			nodeShape = fmt.Sprintf("[[%s]]", label)
		case "command":
			nodeShape = fmt.Sprintf("[\"%s\"]", label)
		case "slack":
			nodeShape = fmt.Sprintf("{{%s}}", label)
		case "jq":
			nodeShape = fmt.Sprintf("[(%s)]", label)
		case "llm_completion":
			nodeShape = fmt.Sprintf("((%s))", label)
		default:
			nodeShape = fmt.Sprintf("[\"%s\"]", label)
		}
		
		fmt.Printf("    %s%s\n", nodeId, nodeShape)
	}
	
	fmt.Println()
	
	// Generate start and end nodes
	startNode := "START"
	endNode := "END"
	fmt.Printf("    %s([\"🚀 START\"])\n", startNode)
	fmt.Printf("    %s([\"✅ END\"])\n", endNode)
	fmt.Println()
	
	// Create dependency connections
	stepWithoutDeps := make([]string, 0)
	stepWithDeps := make(map[string][]string)
	
	for _, step := range workflow.Steps {
		if len(step.Depends) == 0 {
			stepWithoutDeps = append(stepWithoutDeps, step.Name)
		} else {
			stepWithDeps[step.Name] = step.Depends
		}
	}
	
	// Connect start to steps without dependencies
	for _, stepName := range stepWithoutDeps {
		if nodeId, exists := stepNodes[stepName]; exists {
			fmt.Printf("    %s --> %s\n", startNode, nodeId)
		}
	}
	
	// Connect dependent steps
	for stepName, deps := range stepWithDeps {
		if nodeId, exists := stepNodes[stepName]; exists {
			for _, dep := range deps {
				if depNodeId, depExists := stepNodes[dep]; depExists {
					fmt.Printf("    %s --> %s\n", depNodeId, nodeId)
				}
			}
		}
	}
	
	// Connect final steps to end
	finalSteps := make([]string, 0)
	for _, step := range workflow.Steps {
		isFinal := true
		for _, otherStep := range workflow.Steps {
			for _, dep := range otherStep.Depends {
				if dep == step.Name {
					isFinal = false
					break
				}
			}
			if !isFinal {
				break
			}
		}
		if isFinal {
			finalSteps = append(finalSteps, step.Name)
		}
	}
	
	for _, stepName := range finalSteps {
		if nodeId, exists := stepNodes[stepName]; exists {
			fmt.Printf("    %s --> %s\n", nodeId, endNode)
		}
	}
	
	fmt.Println()
	
	// Add styling classes
	fmt.Println("    %% Styling")
	fmt.Printf("    classDef startEndStyle fill:#90EE90,stroke:#006400,stroke-width:3px,color:#000000\n")
	fmt.Printf("    class %s,%s startEndStyle\n", startNode, endNode)
	fmt.Println()
	
	// Add executor-specific styling
	executorSteps := make(map[string][]string)
	for i, step := range workflow.Steps {
		nodeId := fmt.Sprintf("step%d", i+1)
		execType := step.Executor.Type
		if execType == "" {
			execType = "default"
		}
		executorSteps[execType] = append(executorSteps[execType], nodeId)
	}
	
	for execType, steps := range executorSteps {
		color := executorColors[execType]
		if color == "" {
			color = executorColors["default"]
		}
		
		className := fmt.Sprintf("%sStyle", execType)
		fmt.Printf("    classDef %s fill:%s,stroke:#333,stroke-width:2px\n", className, color)
		fmt.Printf("    class %s %s\n", strings.Join(steps, ","), className)
	}
	
	fmt.Println()
	fmt.Println("```")
	
	// Add additional information as comments
	fmt.Println()
	fmt.Println("---")
	fmt.Println()
	fmt.Printf("**Workflow Information:**\n")
	if workflow.Name != "" {
		fmt.Printf("- **Name:** %s\n", workflow.Name)
	}
	if workflow.Description != "" {
		fmt.Printf("- **Description:** %s\n", workflow.Description)
	}
	fmt.Printf("- **Total Steps:** %d\n", len(workflow.Steps))
	
	// Count executor types
	if len(workflow.Steps) > 0 {
		executorCount := make(map[string]int)
		for _, step := range workflow.Steps {
			execType := step.Executor.Type
			if execType == "" {
				execType = "undefined"
			}
			executorCount[execType]++
		}
		
		fmt.Printf("- **Executor Types:** ")
		var execTypes []string
		for execType, count := range executorCount {
			execTypes = append(execTypes, fmt.Sprintf("%s(%d)", execType, count))
		}
		sort.Strings(execTypes)
		fmt.Printf("%s\n", strings.Join(execTypes, ", "))
	}
	
	if len(workflow.Env) > 0 {
		fmt.Printf("- **Environment Variables:** %d\n", len(workflow.Env))
	}
	
	if len(workflow.Variables) > 0 {
		fmt.Printf("- **Variables:** %d\n", len(workflow.Variables))
	}
	
	fmt.Println()
	fmt.Println("**Legend:**")
	fmt.Println("- 🚀 **START** - Workflow entry point")
	fmt.Println("- ✅ **END** - Workflow completion")
	fmt.Println("- **Rectangle** - Command executor")
	fmt.Println("- **Round** - HTTP executor")
	fmt.Println("- **Double Rectangle** - Docker executor")
	fmt.Println("- **Hexagon** - Slack executor")
	fmt.Println("- **Round Rectangle** - JQ executor")
	fmt.Println("- **Circle** - LLM Completion executor")
	
	return nil
}

func outputTable(workflow *WorkflowDescribe, showSteps, showEnv, showDeps, showOutputs, showConfig bool) error {
	// Color definitions
	headerColor := color.New(color.FgCyan, color.Bold)
	keyColor := color.New(color.FgGreen)
	valueColor := color.New(color.FgWhite)
	stepColor := color.New(color.FgYellow, color.Bold)
	executorColor := color.New(color.FgMagenta)
	dependsColor := color.New(color.FgBlue)
	envColor := color.New(color.FgCyan)

	// Main header
	fmt.Println()
	headerColor.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	headerColor.Printf("│                            WORKFLOW DESCRIPTION                            │\n")
	headerColor.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Basic Information
	headerColor.Println("📋 BASIC INFORMATION")
	headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
	
	if workflow.Name != "" {
		keyColor.Print("Name: ")
		valueColor.Printf("%s\n", workflow.Name)
	}
	if workflow.Description != "" {
		keyColor.Print("Description: ")
		valueColor.Printf("%s\n", workflow.Description)
	}
	if workflow.Version != "" {
		keyColor.Print("Version: ")
		valueColor.Printf("%s\n", workflow.Version)
	}
	if workflow.Author != "" {
		keyColor.Print("Author: ")
		valueColor.Printf("%s\n", workflow.Author)
	}
	if len(workflow.Tags) > 0 {
		keyColor.Print("Tags: ")
		valueColor.Printf("%s\n", strings.Join(workflow.Tags, ", "))
	}
	if workflow.Timeout != "" {
		keyColor.Print("Timeout: ")
		valueColor.Printf("%s\n", workflow.Timeout)
	}
	if workflow.Retries > 0 {
		keyColor.Print("Retries: ")
		valueColor.Printf("%d\n", workflow.Retries)
	}
	if workflow.Parallelism > 0 {
		keyColor.Print("Parallelism: ")
		valueColor.Printf("%d\n", workflow.Parallelism)
	}
	fmt.Println()

	// Environment Variables
	if showEnv || len(workflow.Env) > 0 {
		headerColor.Println("🔧 ENVIRONMENT VARIABLES")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		if len(workflow.Env) == 0 {
			valueColor.Println("No environment variables defined")
		} else {
			for _, env := range workflow.Env {
				if strings.Contains(env, "=") {
					parts := strings.SplitN(env, "=", 2)
					envColor.Printf("  %s", parts[0])
					valueColor.Printf(" = %s\n", parts[1])
				} else {
					envColor.Printf("  %s\n", env)
				}
			}
		}
		fmt.Println()
	}

	// Variables
	if len(workflow.Variables) > 0 {
		headerColor.Println("⚙️  VARIABLES")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		for key, value := range workflow.Variables {
			keyColor.Printf("  %s: ", key)
			valueColor.Printf("%v\n", value)
		}
		fmt.Println()
	}

	// Steps Overview
	headerColor.Println("📝 STEPS OVERVIEW")
	headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
	
	if len(workflow.Steps) == 0 {
		valueColor.Println("No steps defined")
	} else {
		keyColor.Print("Total Steps: ")
		valueColor.Printf("%d\n", len(workflow.Steps))
		
		// Count executors
		executorCount := make(map[string]int)
		for _, step := range workflow.Steps {
			executorCount[step.Executor.Type]++
		}
		
		keyColor.Print("Executor Types: ")
		var execTypes []string
		for execType, count := range executorCount {
			execTypes = append(execTypes, fmt.Sprintf("%s(%d)", execType, count))
		}
		sort.Strings(execTypes)
		valueColor.Printf("%s\n", strings.Join(execTypes, ", "))
	}
	fmt.Println()

	// Detailed Steps
	if showSteps && len(workflow.Steps) > 0 {
		headerColor.Println("🔍 DETAILED STEPS")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		for i, step := range workflow.Steps {
			stepColor.Printf("Step %d: %s\n", i+1, step.Name)
			
			if step.Description != "" {
				keyColor.Print("  Description: ")
				valueColor.Printf("%s\n", step.Description)
			}
			
			executorColor.Print("  Executor: ")
			valueColor.Printf("%s\n", step.Executor.Type)
			
			if showConfig && len(step.Executor.Config) > 0 {
				keyColor.Println("  Configuration:")
				for key, value := range step.Executor.Config {
					keyColor.Printf("    %s: ", key)
					valueColor.Printf("%v\n", value)
				}
			}
			
			if len(step.Depends) > 0 {
				dependsColor.Print("  Dependencies: ")
				valueColor.Printf("%s\n", strings.Join(step.Depends, ", "))
			}
			
			if step.Condition != "" {
				keyColor.Print("  Condition: ")
				valueColor.Printf("%s\n", step.Condition)
			}
			
			if step.Output != "" {
				keyColor.Print("  Output: ")
				valueColor.Printf("%s\n", step.Output)
			}
			
			if step.Timeout != "" {
				keyColor.Print("  Timeout: ")
				valueColor.Printf("%s\n", step.Timeout)
			}
			
			if step.Retries > 0 {
				keyColor.Print("  Retries: ")
				valueColor.Printf("%d\n", step.Retries)
			}
			
			fmt.Println()
		}
	}

	// Dependencies Graph
	if showDeps && len(workflow.Steps) > 0 {
		headerColor.Println("🔗 DEPENDENCY GRAPH")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		printDependencyGraph(workflow.Steps)
		fmt.Println()
	}

	// Triggers
	if len(workflow.Triggers) > 0 {
		headerColor.Println("🚀 TRIGGERS")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		for i, trigger := range workflow.Triggers {
			keyColor.Printf("Trigger %d: ", i+1)
			valueColor.Printf("%s\n", trigger.Type)
			
			if showConfig && len(trigger.Config) > 0 {
				keyColor.Println("  Configuration:")
				for key, value := range trigger.Config {
					keyColor.Printf("    %s: ", key)
					valueColor.Printf("%v\n", value)
				}
			}
		}
		fmt.Println()
	}

	// Outputs
	if showOutputs && len(workflow.Outputs) > 0 {
		headerColor.Println("📤 OUTPUTS")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		for _, output := range workflow.Outputs {
			keyColor.Printf("%s: ", output.Name)
			valueColor.Printf("%s\n", output.Value)
			
			if output.Description != "" {
				keyColor.Print("  Description: ")
				valueColor.Printf("%s\n", output.Description)
			}
		}
		fmt.Println()
	}

	// Dependencies
	if len(workflow.Dependencies) > 0 {
		headerColor.Println("📦 WORKFLOW DEPENDENCIES")
		headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
		
		for _, dep := range workflow.Dependencies {
			dependsColor.Printf("  • %s\n", dep)
		}
		fmt.Println()
	}

	// Footer
	headerColor.Println("──────────────────────────────────────────────────────────────────────────────")
	keyColor.Print("💡 Tip: Use ")
	valueColor.Print("--steps --env --deps --outputs --config")
	keyColor.Println(" for full details")
	fmt.Println()

	return nil
}

func printDependencyGraph(steps []WorkflowStepDescribe) {
	// Create step name to index mapping
	stepMap := make(map[string]int)
	for i, step := range steps {
		stepMap[step.Name] = i
	}

	// Print dependency relationships
	keyColor := color.New(color.FgGreen)
	valueColor := color.New(color.FgWhite)
	dependsColor := color.New(color.FgBlue)

	for i, step := range steps {
		if len(step.Depends) == 0 {
			keyColor.Printf("  %d. %s", i+1, step.Name)
			valueColor.Println(" (no dependencies)")
		} else {
			keyColor.Printf("  %d. %s", i+1, step.Name)
			valueColor.Print(" depends on: ")
			
			var depList []string
			for _, dep := range step.Depends {
				if idx, exists := stepMap[dep]; exists {
					depList = append(depList, fmt.Sprintf("%s(#%d)", dep, idx+1))
				} else {
					depList = append(depList, dep)
				}
			}
			dependsColor.Println(strings.Join(depList, ", "))
		}
	}
}