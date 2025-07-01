package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Prompts manages MCP prompts
type Prompts struct{}

// NewPrompts creates a new prompts manager
func NewPrompts() *Prompts {
	return &Prompts{}
}

// Register adds all prompts to the MCP server
func (p *Prompts) Register(s *server.MCPServer) error {
	// Kubernetes troubleshooting workflow
	s.AddPrompt(mcp.NewPrompt("kubernetes_troubleshooting",
		mcp.WithPromptDescription("Comprehensive Kubernetes troubleshooting workflow"),
		mcp.WithArgument("namespace", mcp.ArgumentDescription("Kubernetes namespace to troubleshoot")),
		mcp.WithArgument("resource_type", mcp.ArgumentDescription("Resource type (pod, service, deployment, etc.)")),
		mcp.WithArgument("resource_name", mcp.ArgumentDescription("Specific resource name (optional)")),
	), p.kubernetesTroubleshootingHandler)

	// Deployment automation workflow
	s.AddPrompt(mcp.NewPrompt("deployment_automation",
		mcp.WithPromptDescription("Automated deployment workflow with best practices"),
		mcp.WithArgument("app_name", mcp.RequiredArgument(), mcp.ArgumentDescription("Application name to deploy")),
		mcp.WithArgument("environment", mcp.RequiredArgument(), mcp.ArgumentDescription("Target environment (dev, staging, prod)")),
		mcp.WithArgument("image_tag", mcp.ArgumentDescription("Docker image tag (optional, defaults to latest)")),
	), p.deploymentAutomationHandler)

	// Agent communication workflow
	s.AddPrompt(mcp.NewPrompt("agent_communication",
		mcp.WithPromptDescription("Inter-agent communication and coordination workflow"),
		mcp.WithArgument("source_agent", mcp.RequiredArgument(), mcp.ArgumentDescription("Source agent name")),
		mcp.WithArgument("target_agent", mcp.RequiredArgument(), mcp.ArgumentDescription("Target agent name")),
		mcp.WithArgument("task_description", mcp.RequiredArgument(), mcp.ArgumentDescription("Description of the task to coordinate")),
	), p.agentCommunicationHandler)

	return nil
}

func (p *Prompts) kubernetesTroubleshootingHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	namespace := ""
	resourceType := "all"
	resourceName := ""

	if args := request.Params.Arguments; args != nil {
		if ns, ok := args["namespace"]; ok {
			namespace = ns
		}
		if rt, ok := args["resource_type"]; ok {
			resourceType = rt
		}
		if rn, ok := args["resource_name"]; ok {
			resourceName = rn
		}
	}

	prompt := `You are a Kubernetes troubleshooting expert. Help diagnose and resolve issues in the cluster.

## Current Context
- Namespace: ` + namespace + `
- Resource Type: ` + resourceType + `
- Resource Name: ` + resourceName + `

## Troubleshooting Workflow

1. **Initial Assessment**
   - Check cluster health and node status
   - Verify namespace exists and is accessible
   - Review recent events and logs

2. **Resource-Specific Analysis**
   - Examine resource status and configuration
   - Check resource dependencies
   - Analyze networking and storage

3. **Common Issues to Check**
   - Resource limits and requests
   - Image pull issues
   - ConfigMap and Secret availability
   - Network policies and service connectivity
   - RBAC permissions

4. **Tools to Use**
   - kubectl get/describe for resource inspection
   - kubectl logs for application logs
   - kubectl top for resource usage
   - Network testing tools

Please start by checking the overall cluster health and then focus on the specific resource issues.`

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Kubernetes troubleshooting workflow", messages), nil
}

func (p *Prompts) deploymentAutomationHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	appName := ""
	environment := ""
	imageTag := "latest"

	if args := request.Params.Arguments; args != nil {
		if app, ok := args["app_name"]; ok {
			appName = app
		}
		if env, ok := args["environment"]; ok {
			environment = env
		}
		if tag, ok := args["image_tag"]; ok {
			imageTag = tag
		}
	}

	prompt := `You are a deployment automation specialist. Execute a comprehensive deployment workflow.

## Deployment Context
- Application: ` + appName + `
- Environment: ` + environment + `
- Image Tag: ` + imageTag + `

## Deployment Workflow

1. **Pre-Deployment Checks**
   - Verify target environment health
   - Check resource availability
   - Validate deployment configuration
   - Ensure backup/rollback strategy

2. **Deployment Process**
   - Build and tag container image
   - Update deployment manifests
   - Apply rolling deployment strategy
   - Monitor deployment progress

3. **Post-Deployment Validation**
   - Health checks and readiness probes
   - Service connectivity testing
   - Performance validation
   - Logging and monitoring setup

4. **Best Practices**
   - Use blue-green or canary deployment
   - Implement proper resource limits
   - Configure horizontal pod autoscaling
   - Set up alerts and monitoring

5. **Rollback Plan**
   - Document rollback procedures
   - Test rollback in staging
   - Monitor for issues post-deployment

Execute this deployment workflow step by step, ensuring each phase completes successfully before proceeding.`

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Deployment automation workflow", messages), nil
}

func (p *Prompts) agentCommunicationHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	sourceAgent := ""
	targetAgent := ""
	taskDescription := ""

	if args := request.Params.Arguments; args != nil {
		if src, ok := args["source_agent"]; ok {
			sourceAgent = src
		}
		if tgt, ok := args["target_agent"]; ok {
			targetAgent = tgt
		}
		if desc, ok := args["task_description"]; ok {
			taskDescription = desc
		}
	}

	prompt := `You are coordinating communication between multiple AI agents. Facilitate effective collaboration.

## Communication Context
- Source Agent: ` + sourceAgent + `
- Target Agent: ` + targetAgent + `
- Task: ` + taskDescription + `

## Agent Communication Protocol

1. **Task Assessment**
   - Analyze task complexity and requirements
   - Identify agent capabilities and specializations
   - Determine optimal collaboration strategy

2. **Communication Flow**
   - Initialize secure communication channel
   - Exchange task context and requirements
   - Establish shared understanding of goals

3. **Coordination Strategy**
   - Define roles and responsibilities
   - Set up progress tracking mechanisms
   - Establish error handling procedures

4. **Execution Monitoring**
   - Track individual agent progress
   - Facilitate information exchange
   - Resolve conflicts and dependencies

5. **Quality Assurance**
   - Validate intermediate results
   - Ensure consistency across agents
   - Perform final integration testing

## Communication Guidelines
- Use clear, structured messages
- Include context and error handling
- Maintain audit trail of decisions
- Ensure graceful failure handling

Begin by establishing communication with both agents and coordinating the task execution.`

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Agent communication workflow", messages), nil
}