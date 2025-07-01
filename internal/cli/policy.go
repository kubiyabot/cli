package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newPolicyCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policy",
		Aliases: []string{"pol"},
		Short:   "🛡️  Manage OPA policies for access control",
		Long:    `Create, update, validate, and manage Open Policy Agent (OPA) policies for controlling access to tools and workflows.`,
	}

	cmd.AddCommand(
		newListPoliciesCommand(cfg),
		newGetPolicyCommand(cfg),
		newCreatePolicyCommand(cfg),
		newUpdatePolicyCommand(cfg),
		newDeletePolicyCommand(cfg),
		newValidatePolicyCommand(cfg),
		newEvaluatePolicyCommand(cfg),
		newTestToolPermissionCommand(cfg),
		newTestWorkflowPermissionCommand(cfg),
	)

	return cmd
}

func newListPoliciesCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all OPA policies",
		Example: "  kubiya policy list\n  kubiya policy list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			policies, err := client.ListPolicies(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(policies)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🛡️  OPA POLICIES")
				fmt.Fprintln(w, "NAME\tENVIRONMENTS\tPOLICY SIZE")
				for _, policy := range policies {
					envs := strings.Join(policy.Env, ", ")
					if envs == "" {
						envs = "all"
					}
					fmt.Fprintf(w, "%s\t%s\t%d chars\n", policy.Name, envs, len(policy.Policy))
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newGetPolicyCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get [policy-name]",
		Short:   "📖 Get policy details",
		Example: "  kubiya policy get my-policy\n  kubiya policy get my-policy --output json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			policy, err := client.GetPolicy(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(policy)
			case "text":
				fmt.Printf("🛡️  Policy: %s\n\n", policy.Name)
				fmt.Printf("Environments: %s\n", strings.Join(policy.Env, ", "))
				fmt.Printf("\nPolicy Content:\n%s\n", policy.Policy)
				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		envs        []string
		policyFile  string
		policyText  string
		validate    bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "📝 Create a new OPA policy",
		Example: `  kubiya policy create --name "tool-access" --env "prod,staging" --file policy.rego
  kubiya policy create --name "workflow-access" --env "prod" --policy "package workflows; allow = true"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("policy name is required")
			}

			var policyContent string
			if policyFile != "" {
				content, err := ioutil.ReadFile(policyFile)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}
				policyContent = string(content)
			} else if policyText != "" {
				policyContent = policyText
			} else {
				return fmt.Errorf("either --file or --policy must be provided")
			}

			policy := kubiya.Policy{
				Name:   name,
				Env:    envs,
				Policy: policyContent,
			}

			client := kubiya.NewClient(cfg)

			// Validate policy if requested
			if validate {
				fmt.Println("🔍 Validating policy...")
				validation := kubiya.PolicyValidationRequest{
					Name:   policy.Name,
					Env:    policy.Env,
					Policy: policy.Policy,
				}

				result, err := client.ValidatePolicy(cmd.Context(), validation)
				if err != nil {
					return fmt.Errorf("policy validation failed: %w", err)
				}

				if !result.Valid {
					fmt.Println("❌ Policy validation failed:")
					for _, error := range result.Errors {
						fmt.Printf("  - %s\n", error)
					}
					return fmt.Errorf("policy has validation errors")
				}
				fmt.Println("✅ Policy validation passed")
			}

			created, err := client.CreatePolicy(cmd.Context(), policy)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Created policy: %s\n", created.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Policy name (required)")
	cmd.Flags().StringSliceVarP(&envs, "env", "e", nil, "Target environments (comma-separated)")
	cmd.Flags().StringVarP(&policyFile, "file", "f", "", "Policy file path")
	cmd.Flags().StringVarP(&policyText, "policy", "p", "", "Policy content directly")
	cmd.Flags().BoolVar(&validate, "validate", true, "Validate policy before creating")

	cmd.MarkFlagRequired("name")
	return cmd
}

func newUpdatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		envs        []string
		policyFile  string
		policyText  string
		validate    bool
	)

	cmd := &cobra.Command{
		Use:   "update [policy-name]",
		Short: "✏️ Update an existing OPA policy",
		Example: `  kubiya policy update my-policy --file new-policy.rego
  kubiya policy update my-policy --env "prod,staging" --policy "package tools; allow = false"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			
			// Get existing policy
			existing, err := client.GetPolicy(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Update fields if provided
			if len(envs) > 0 {
				existing.Env = envs
			}

			if policyFile != "" {
				content, err := ioutil.ReadFile(policyFile)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}
				existing.Policy = string(content)
			} else if policyText != "" {
				existing.Policy = policyText
			}

			// Validate policy if requested
			if validate {
				fmt.Println("🔍 Validating policy...")
				validation := kubiya.PolicyValidationRequest{
					Name:   existing.Name,
					Env:    existing.Env,
					Policy: existing.Policy,
				}

				result, err := client.ValidatePolicy(cmd.Context(), validation)
				if err != nil {
					return fmt.Errorf("policy validation failed: %w", err)
				}

				if !result.Valid {
					fmt.Println("❌ Policy validation failed:")
					for _, error := range result.Errors {
						fmt.Printf("  - %s\n", error)
					}
					return fmt.Errorf("policy has validation errors")
				}
				fmt.Println("✅ Policy validation passed")
			}

			updated, err := client.UpdatePolicy(cmd.Context(), args[0], *existing)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Updated policy: %s\n", updated.Name)
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&envs, "env", "e", nil, "Target environments (comma-separated)")
	cmd.Flags().StringVarP(&policyFile, "file", "f", "", "Policy file path")
	cmd.Flags().StringVarP(&policyText, "policy", "p", "", "Policy content directly")
	cmd.Flags().BoolVar(&validate, "validate", true, "Validate policy before updating")

	return cmd
}

func newDeletePolicyCommand(cfg *config.Config) *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:     "delete [policy-name]",
		Short:   "🗑️ Delete an OPA policy",
		Example: "  kubiya policy delete my-policy --confirm",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				fmt.Printf("⚠️  Are you sure you want to delete policy '%s'? Use --confirm to proceed.\n", args[0])
				return nil
			}

			client := kubiya.NewClient(cfg)
			result, err := client.DeletePolicy(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("✅ Policy deleted: %s\n", result.Status)
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm deletion")
	return cmd
}

func newValidatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		name       string
		envs       []string
		policyFile string
		policyText string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "🔍 Validate an OPA policy",
		Example: `  kubiya policy validate --name "test-policy" --file policy.rego
  kubiya policy validate --name "test-policy" --policy "package tools; allow = true"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("policy name is required")
			}

			var policyContent string
			if policyFile != "" {
				content, err := ioutil.ReadFile(policyFile)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}
				policyContent = string(content)
			} else if policyText != "" {
				policyContent = policyText
			} else {
				return fmt.Errorf("either --file or --policy must be provided")
			}

			validation := kubiya.PolicyValidationRequest{
				Name:   name,
				Env:    envs,
				Policy: policyContent,
			}

			client := kubiya.NewClient(cfg)
			result, err := client.ValidatePolicy(cmd.Context(), validation)
			if err != nil {
				return err
			}

			if result.Valid {
				fmt.Println("✅ Policy is valid")
			} else {
				fmt.Println("❌ Policy validation failed:")
				for _, error := range result.Errors {
					fmt.Printf("  - %s\n", error)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Policy name (required)")
	cmd.Flags().StringSliceVarP(&envs, "env", "e", nil, "Target environments (comma-separated)")
	cmd.Flags().StringVarP(&policyFile, "file", "f", "", "Policy file path")
	cmd.Flags().StringVarP(&policyText, "policy", "p", "", "Policy content directly")

	cmd.MarkFlagRequired("name")
	return cmd
}

func newEvaluatePolicyCommand(cfg *config.Config) *cobra.Command {
	var (
		inputFile   string
		inputText   string
		policyFile  string
		policyText  string
		dataFile    string
		dataText    string
		query       string
	)

	cmd := &cobra.Command{
		Use:   "evaluate",
		Short: "⚖️  Evaluate a policy with input data",
		Example: `  kubiya policy evaluate --policy-file policy.rego --input-file input.json --query "data.tools.allow"
  kubiya policy evaluate --policy "package tools; allow = true" --input '{"tool": "kubectl"}' --query "allow"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse input
			var input map[string]interface{}
			if inputFile != "" {
				content, err := ioutil.ReadFile(inputFile)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}
				if err := json.Unmarshal(content, &input); err != nil {
					return fmt.Errorf("failed to parse input JSON: %w", err)
				}
			} else if inputText != "" {
				if err := json.Unmarshal([]byte(inputText), &input); err != nil {
					return fmt.Errorf("failed to parse input JSON: %w", err)
				}
			} else {
				input = make(map[string]interface{})
			}

			// Parse policy
			var policyContent string
			if policyFile != "" {
				content, err := ioutil.ReadFile(policyFile)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}
				policyContent = string(content)
			} else if policyText != "" {
				policyContent = policyText
			} else {
				return fmt.Errorf("either --policy-file or --policy must be provided")
			}

			// Parse data
			var data map[string]interface{}
			if dataFile != "" {
				content, err := ioutil.ReadFile(dataFile)
				if err != nil {
					return fmt.Errorf("failed to read data file: %w", err)
				}
				if err := json.Unmarshal(content, &data); err != nil {
					return fmt.Errorf("failed to parse data JSON: %w", err)
				}
			} else if dataText != "" {
				if err := json.Unmarshal([]byte(dataText), &data); err != nil {
					return fmt.Errorf("failed to parse data JSON: %w", err)
				}
			} else {
				data = make(map[string]interface{})
			}

			if query == "" {
				query = "data"
			}

			evaluation := kubiya.PolicyEvaluationRequest{
				Input:  input,
				Policy: policyContent,
				Data:   data,
				Query:  query,
			}

			client := kubiya.NewClient(cfg)
			result, err := client.EvaluatePolicy(cmd.Context(), evaluation)
			if err != nil {
				return err
			}

			if result.Error != "" {
				fmt.Printf("❌ Evaluation error: %s\n", result.Error)
				return nil
			}

			fmt.Println("✅ Policy evaluation result:")
			resultJSON, err := json.MarshalIndent(result.Result, "", "  ")
			if err != nil {
				fmt.Printf("%v\n", result.Result)
			} else {
				fmt.Printf("%s\n", resultJSON)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "input-file", "", "Input JSON file")
	cmd.Flags().StringVar(&inputText, "input", "", "Input JSON string")
	cmd.Flags().StringVar(&policyFile, "policy-file", "", "Policy file path")
	cmd.Flags().StringVar(&policyText, "policy", "", "Policy content directly")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "Data JSON file")
	cmd.Flags().StringVar(&dataText, "data", "", "Data JSON string")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query string (default: 'data')")

	return cmd
}

func newTestToolPermissionCommand(cfg *config.Config) *cobra.Command {
	var (
		toolName string
		argsFile string
		argsText string
		runner   string
	)

	cmd := &cobra.Command{
		Use:   "test-tool",
		Short: "🔧 Test tool execution permission",
		Example: `  kubiya policy test-tool --tool kubectl --args '{"command": "get pods"}' --runner prod
  kubiya policy test-tool --tool aws --args-file args.json --runner staging`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if toolName == "" {
				return fmt.Errorf("tool name is required")
			}

			// Parse args
			var toolArgs map[string]interface{}
			if argsFile != "" {
				content, err := ioutil.ReadFile(argsFile)
				if err != nil {
					return fmt.Errorf("failed to read args file: %w", err)
				}
				if err := json.Unmarshal(content, &toolArgs); err != nil {
					return fmt.Errorf("failed to parse args JSON: %w", err)
				}
			} else if argsText != "" {
				if err := json.Unmarshal([]byte(argsText), &toolArgs); err != nil {
					return fmt.Errorf("failed to parse args JSON: %w", err)
				}
			} else {
				toolArgs = make(map[string]interface{})
			}

			if runner == "" {
				runner = "default"
			}

			client := kubiya.NewClient(cfg)
			allowed, message, err := client.ValidateToolExecution(cmd.Context(), toolName, toolArgs, runner)
			if err != nil {
				return err
			}

			if allowed {
				fmt.Printf("✅ Permission granted to execute tool '%s'\n", toolName)
				if message != "" {
					fmt.Printf("   %s\n", message)
				}
			} else {
				fmt.Printf("❌ Permission denied to execute tool '%s'\n", toolName)
				if message != "" {
					fmt.Printf("   Reason: %s\n", message)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&toolName, "tool", "t", "", "Tool name (required)")
	cmd.Flags().StringVar(&argsFile, "args-file", "", "Tool arguments JSON file")
	cmd.Flags().StringVar(&argsText, "args", "", "Tool arguments JSON string")
	cmd.Flags().StringVarP(&runner, "runner", "r", "", "Runner name")

	cmd.MarkFlagRequired("tool")
	return cmd
}

func newTestWorkflowPermissionCommand(cfg *config.Config) *cobra.Command {
	var (
		workflowFile string
		paramsFile   string
		paramsText   string
		runner       string
	)

	cmd := &cobra.Command{
		Use:   "test-workflow",
		Short: "🔄 Test workflow execution permission",
		Example: `  kubiya policy test-workflow --file workflow.yaml --params '{"env": "prod"}' --runner prod
  kubiya policy test-workflow --file workflow.yaml --params-file params.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workflowFile == "" {
				return fmt.Errorf("workflow file is required")
			}

			// Parse workflow
			workflowContent, err := ioutil.ReadFile(workflowFile)
			if err != nil {
				return fmt.Errorf("failed to read workflow file: %w", err)
			}

			var workflowDef map[string]interface{}
			if err := json.Unmarshal(workflowContent, &workflowDef); err != nil {
				return fmt.Errorf("failed to parse workflow JSON: %w", err)
			}

			// Parse params
			var params map[string]interface{}
			if paramsFile != "" {
				content, err := ioutil.ReadFile(paramsFile)
				if err != nil {
					return fmt.Errorf("failed to read params file: %w", err)
				}
				if err := json.Unmarshal(content, &params); err != nil {
					return fmt.Errorf("failed to parse params JSON: %w", err)
				}
			} else if paramsText != "" {
				if err := json.Unmarshal([]byte(paramsText), &params); err != nil {
					return fmt.Errorf("failed to parse params JSON: %w", err)
				}
			} else {
				params = make(map[string]interface{})
			}

			if runner == "" {
				runner = "default"
			}

			client := kubiya.NewClient(cfg)
			allowed, issues, err := client.ValidateWorkflowExecution(cmd.Context(), workflowDef, params, runner)
			if err != nil {
				return err
			}

			if allowed && len(issues) == 0 {
				fmt.Printf("✅ Permission granted to execute workflow\n")
			} else if allowed && len(issues) > 0 {
				fmt.Printf("⚠️  Permission granted but with warnings:\n")
				for _, issue := range issues {
					fmt.Printf("   - %s\n", issue)
				}
			} else {
				fmt.Printf("❌ Permission denied to execute workflow\n")
				for _, issue := range issues {
					fmt.Printf("   - %s\n", issue)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&workflowFile, "file", "f", "", "Workflow definition file (required)")
	cmd.Flags().StringVar(&paramsFile, "params-file", "", "Workflow parameters JSON file")
	cmd.Flags().StringVar(&paramsText, "params", "", "Workflow parameters JSON string")
	cmd.Flags().StringVarP(&runner, "runner", "r", "", "Runner name")

	cmd.MarkFlagRequired("file")
	return cmd
}