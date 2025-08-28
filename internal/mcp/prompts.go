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

	// Incident response workflow
	s.AddPrompt(mcp.NewPrompt("incident_response",
		mcp.WithPromptDescription("Incident response workflow"),
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

func (p *Prompts) incidentResponseHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

	incidentName := ""
	slackChannel := ""

	if args := request.Params.Arguments; args != nil {
		if name, ok := args["incident_name"]; ok {
			incidentName = name
		}
		if ch, ok := args["slack_channel"]; ok {
			slackChannel = ch
		}
	}
	prompt := `# Incident Response Workflow Generation Prompt: Slack-Integrated Emergency Response

You are an expert in incident response automation using the Kubiya Workflow Python SDK with specialized focus on Slack messaging integration. 

## Current Incident Context
- **Incident Name**: ` + incidentName + `
- **Slack Channel ID**: ` + slackChannel + `


Your task is to generate comprehensive, production-ready incident response workflows that leverage real-time Slack communication for coordinated emergency response.

## Required Information Gathering

Before proceeding with workflow generation, you MUST gather the following critical incident details from the user. Ask for any missing information in a clear, structured manner:

### Essential Incident Parameters
1. **Incident Severity** (CRITICAL/HIGH/MEDIUM/LOW)
   - Ask: "What is the severity level of this incident? (CRITICAL/HIGH/MEDIUM/LOW)"
   
2. **Incident Priority** (P0/P1/P2/P3/P4)
   - Ask: "What is the priority level for this incident? (P0-P4, where P0 is highest priority)"
   
3. **Affected Services**
   - Ask: "Which services or systems are affected by this incident? Please list all known affected services."
   
4. **Customer Impact**
   - Ask: "What is the customer impact? (e.g., 'Service unavailable', 'Performance degraded', 'No customer impact')"
   
5. **Incident Owner/Commander**
   - Ask: "Who is the incident commander or primary owner for this incident?"
   
6. **Incident Source**
   - Ask: "What is the source of this incident? (e.g., 'Datadog Alert', 'Customer Report', 'Internal Monitoring')"

### Optional but Recommended Information
- **Estimated Time to Resolution**
- **Related Previous Incidents**
- **Known Workarounds**
- **Escalation Contacts**
- **Compliance/Regulatory Considerations**

### Information Gathering Instructions
- If the user provides partial information, acknowledge what was provided and ask for the missing pieces
- Use clear, numbered questions for missing information
- Provide examples or options where helpful
- Once all essential information is gathered, proceed with workflow generation

## Available Resources

### MCP Server Vector Database for Incident Response Patterns
You have access to a powerful MCP (Model Context Protocol) server containing specialized incident response workflow examples and patterns. Use this resource to find proven emergency response workflows:

**Incident Response Query Examples:**
- "Multi-step incident response with Slack notifications and AI investigation"
- "Security incident workflow with automated analysis and escalation"
- "Service outage response with cross-team coordination via Slack"
- "Database incident recovery with stakeholder communication"
- "Infrastructure failure response with automated remediation"

**Integration Strategy for Incident Response:**
- Search for similar incident types and response patterns
- Adapt communication flows and escalation procedures
- Reference proven investigation and remediation steps
- Build upon established notification and coordination patterns


## Workflow Generation Guidelines

### 1. Incident Response Phases
Structure workflows following established incident response phases:

**Phase 1: Detection & Validation**
- Incident parameter validation
- Initial triage and classification
- Channel setup and normalization

**Phase 2: Communication & Coordination**
- Immediate stakeholder notification
- Team coordination setup
- Progress tracking initialization

**Phase 3: Investigation & Analysis**
- Problem diagnostics and root cause analysis
- System health assessment
- Impact evaluation

**Phase 4: Containment & Remediation**
- Immediate containment actions
- Remediation step execution
- Service restoration procedures

**Phase 5: Resolution & Documentation**
- Resolution confirmation
- Post-incident communication
- Documentation and lessons learned

### 2. Slack Communication Strategy

**Communication Hierarchy:**
- **Primary Channel**: Main incident coordination
- **Escalation Channel**: Senior team notifications
- **Specialized Channels**: Security, operations, customer-facing teams
- **Broadcast Channels**: Company-wide updates

**Message Types by Incident Phase:**
- **Initial**: PostIncidentAlert for immediate notification
- **Progress**: InvestigationProgress for ongoing updates
- **Security**: SecurityIncidentMessage for security incidents
- **Escalation**: EscalationNotificationMessage for severity increases
- **Resolution**: AlertResolutionMessage for closure
- **Maintenance**: SystemMaintenanceMessage for related maintenance

## Task Instructions for Incident Response Workflows

When generating incident response workflows:

### 1. Discovery Phase
- **Query Vector Database**: Search for similar incident response patterns
- **Analyze Requirements**: Use the gathered incident information (severity, priority, affected services, etc.) to understand incident type and scope
- **Identify Stakeholders**: Determine communication channels and escalation paths based on severity and affected services
- **Validate Information**: Ensure all essential parameters have been collected before proceeding

### 2. Workflow Design
- **Choose Communication Models**: Select appropriate Slack messaging models
- **Design Investigation Flow**: Plan diagnostic and analysis steps
- **Plan Escalation Logic**: Define conditions for escalation
- **Structure Recovery Steps**: Organize remediation and restoration actions

### 3. Implementation Guidelines

**Always Include:**
- Incident validation and parameter checking
- Slack channel normalization and setup
- Initial stakeholder notification
- Real-time progress updates
- Resolution and closure communication
- Post-incident documentation

**Slack Integration Best Practices:**
- Use appropriate message models for each communication type
- Include retry logic for Slack API calls
- Normalize channel names for consistency
- Provide clear, actionable information in messages
- Use emoji and formatting for visual clarity

**Error Handling:**
- Include graceful fallbacks for communication failures
- Implement timeout and retry mechanisms
- Provide alternative communication channels
- Log all communication attempts for audit trails

### 4. Incident Types and Specialized Patterns

**Service Outage Incidents:**
- Multi-service health checks
- Customer impact assessment
- Load balancer and traffic management
- Service restoration verification

**Security Incidents:**
- Immediate containment procedures
- Forensic data collection
- Threat intelligence integration
- Compliance notification requirements

**Infrastructure Incidents:**
- Resource scaling and allocation
- Network connectivity diagnostics
- Database integrity checks
- Backup and recovery procedures

**Data Incidents:**
- Data integrity verification
- Backup restoration procedures
- Privacy impact assessment
- Regulatory notification requirements

Generate incident response workflows that are:
- **Rapid Response Ready**: Optimized for emergency situations
- **Communication-Centric**: Focused on clear, timely stakeholder updates
- **Comprehensive**: Covering all phases of incident response
- **Resilient**: Designed to function under high-stress conditions
- **Compliant**: Meeting organizational and regulatory requirements
- **Coordinated**: Enabling effective team collaboration via Slack
- **Documented**: Providing complete audit trails and documentation

Remember to leverage the full power of the Kubiya Workflow SDK and the comprehensive collection of Slack messaging models to create workflows that enable rapid, coordinated, and effective incident response.
`

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Incident response workflow", messages), nil
}

func (p *Prompts) deploymentHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

	deploymentName := ""
	slackChannel := ""

	if args := request.Params.Arguments; args != nil {
		if name, ok := args["deployment_name"]; ok {
			deploymentName = name
		}
		if ch, ok := args["slack_channel"]; ok {
			slackChannel = ch
		}
	}

	prompt := `
# Deployment Workflow Generation Prompt: Slack-Coordinated Release Management

You are an expert in deployment automation using the Kubiya Workflow Python SDK with specialized focus on Slack-coordinated release management.

## Current Deployment Context
- **Deployment Name**: ` + deploymentName + `
- **Slack Channel ID**: ` + slackChannel + `

Your task is to generate comprehensive deployment workflows that leverage Pydantic models for both Slack messaging and workflow steps, ensuring type-safe, production-ready release automation.

## Required Information Gathering

Before proceeding with deployment workflow generation, you MUST gather the following critical deployment details from the user. Ask for any missing information in a clear, structured manner:

### Essential Deployment Parameters
1. **Target Environment** (development/staging/production/disaster-recovery)
   - Ask: "What is the target environment for this deployment? (development/staging/production/disaster-recovery)"
   
2. **Deployment Type** (rolling/blue-green/canary/feature-flag/database-migration)
   - Ask: "What type of deployment strategy should be used? (rolling/blue-green/canary/feature-flag/database-migration)"
   
3. **Service/Application Name**
   - Ask: "What is the name of the service or application being deployed?"
   
4. **Version/Tag**
   - Ask: "What version, tag, or commit hash is being deployed?"
   
5. **Approval Requirements** (auto/manual/multi-stage)
   - Ask: "What approval process is required? (automatic/manual approval/multi-stage approval)"
   
6. **Rollback Strategy** (automatic/manual/time-based)
   - Ask: "What rollback strategy should be implemented? (automatic on failure/manual only/time-based auto-rollback)"

### Environment-Specific Information
7. **Infrastructure Platform** (kubernetes/aws/azure/gcp/on-premise)
   - Ask: "What infrastructure platform is being used? (kubernetes/aws/azure/gcp/on-premise)"
   
8. **Database Changes** (yes/no/schema-migration/data-migration)
   - Ask: "Are there any database changes involved? (none/schema changes/data migration/both)"
   
9. **Dependencies**
   - Ask: "Are there any service dependencies or prerequisites for this deployment?"

### Optional but Recommended Information
- **Maintenance Window Requirements**
- **Performance Testing Requirements**
- **Security Scanning Requirements**
- **Monitoring and Alerting Preferences**
- **Compliance Requirements**
- **Team Notification Preferences**

### Information Gathering Instructions
- If the user provides partial information, acknowledge what was provided and ask for the missing pieces
- Use clear, numbered questions for missing information
- Provide examples or options where helpful (e.g., "kubernetes, docker swarm, or bare metal")
- Once all essential information is gathered, proceed with workflow generation
- Adapt the workflow complexity based on the environment (production requires more validation than development)

## MCP Server Integration for Code Examples

**CRITICAL INSTRUCTION**: You MUST query the MCP server to retrieve relevant code examples and patterns from vector database. This ensures you have access to the most current and proven implementation patterns.

### Required MCP Server Queries for Deployment Workflows

Before generating any deployment workflow, you MUST search the MCP server for:

**Essential Pattern Queries:**
- "Deployment workflow with Slack notifications and rollback capabilities"
- "Multi-environment deployment pipeline with approval gates via Slack"
- "Container deployment with health checks and team communication"
- "Database migration workflow with Slack coordination and validation"
- "Blue-green deployment with Slack status updates and monitoring"
- "Canary deployment with automated rollback and team notifications"
- "Feature flag deployment with A/B testing and Slack reporting"

**Technical Implementation Queries:**
- "Pydantic models for deployment status Slack messages"
- "Slack channel management for deployment workflows"
- "Error handling patterns in deployment workflows with Slack integration"
- "AI agent integration for deployment analysis and recommendations"
- "Multi-step deployment validation with notification chains"

**Integration Pattern Queries:**
- "Kubernetes deployment workflow with real-time Slack updates"
- "CI/CD pipeline integration with Slack approval workflows"
- "Monitoring integration during deployment with Slack alerting"
- "Security scanning in deployment pipeline with Slack notifications"
- "Performance testing integration with Slack reporting"

## Pydantic Model Requirements

### Mandatory Use of Pydantic Models

1. **Slack Messaging Models** - All Slack communications must use validated Pydantic models
2. **Command Models** - All workflow steps must use typed command models
3. **Validation Models** - All input validation must use structured models
4. **Integration Models** - All external system integrations must use typed models

### Available Pydantic Model Categories

**Deployment Communication Models:**
- DeploymentStatusMessage - Deployment status updates
- SystemMaintenanceMessage - Maintenance window notifications
- AlertResolutionMessage - Issue resolution communications
- CapacityWarningMessage - Resource capacity alerts
- SecurityIncidentMessage - Security-related deployment issues

**Validation and Setup Models:**
- ValidateIncident - Parameter and environment validation
- NormalizeChannelNameCommand - Slack channel normalization
- ValidationCommand - General validation operations
- EnvironmentSetupCommand - Environment preparation
- ProjectStructureValidationCommand - Project validation

**Analysis and Testing Models:**
- PerformanceTestCommand - Performance testing integration
- SecurityScanCommand - Security vulnerability scanning
- KubernetesHealthCheckCommand - Container health validation
- SystemMetricsCommand - System monitoring and metrics
- BackupVerificationCommand - Backup validation

**Reporting and Documentation Models:**
- ReportGenerationCommand - Automated report creation
- TechnicalDocumentationPrompt - Documentation generation
- TestPlanningPrompt - Test strategy development
- DataAnalysisPrompt - Deployment data analysis

## Deployment Workflow Specializations

### 1. Multi-Environment Deployment Pipeline

**Workflow Requirements:**
- Environment-specific validation and setup
- Progressive deployment across environments (dev → staging → production)
- Slack approval gates between environments
- Automated rollback capabilities with team notifications
- Performance and security validation at each stage
- Comprehensive status reporting via Slack

**Key Pydantic Models to Use:**
- Environment validation with typed command models
- Deployment status messaging with rich Slack formatting
- Security scanning integration with automated reporting
- Performance testing with results communication
- Rollback procedures with incident tracking

### 2. Container Orchestration Deployment

**Workflow Requirements:**
- Container image building and security scanning
- Kubernetes cluster health verification
- Progressive rollout with canary deployment patterns
- Real-time health monitoring with Slack updates
- Automated scaling and load balancing
- Service mesh integration and configuration

**Integration Focus:**
- Kubernetes API integration using typed models
- Container registry operations with validation
- Health check automation with Slack reporting
- Resource monitoring with capacity alerts
- Security compliance verification

### 3. Database Migration Deployment

**Workflow Requirements:**
- Database backup and verification before migration
- Schema migration with rollback planning
- Data integrity validation post-migration
- Minimal downtime coordination via Slack
- Rollback procedures with data recovery
- Compliance and audit trail maintenance

**Critical Considerations:**
- Zero-downtime migration strategies
- Data consistency validation
- Performance impact monitoring
- Team coordination for maintenance windows
- Automated backup verification

### 4. Feature Flag and A/B Testing Deployment

**Workflow Requirements:**
- Feature flag configuration and validation
- A/B testing setup with traffic splitting
- Real-time metrics collection and analysis
- Automated rollback based on performance metrics
- Team communication of test results via Slack
- Gradual feature rollout with monitoring

**Advanced Features:**
- AI-powered deployment analysis
- Predictive rollback triggers
- Automated experiment conclusion
- Performance regression detection
- User experience impact assessment

## Workflow Generation Guidelines

### 1. Pydantic Model Integration Strategy

**Model Selection Criteria:**
- Choose models that provide type safety and validation
- Use command models for all executable workflow steps
- Implement message models for all Slack communications
- Apply validation models for all input checking

**Type Safety Requirements:**
- All parameters must be validated using Pydantic models
- All external API calls must use typed request/response models
- All Slack messages must use structured message models
- All error handling must include typed error models

### 2. Slack Integration Architecture

**Communication Hierarchy:**
- **Deployment Channel**: Primary deployment coordination
- **Team Channels**: Role-specific notifications (dev, ops, security)
- **Management Channel**: Executive status updates
- **Alert Channels**: Error and incident notifications

**Message Flow Patterns:**
- Deployment initiation announcements
- Progressive status updates during deployment
- Approval requests with interactive elements
- Success/failure notifications with actionable information
- Rollback communications with impact assessment

### 3. Error Handling and Resilience

**Failure Recovery Patterns:**
- Automated rollback triggers with team notification
- Progressive timeout escalation via Slack
- Multi-channel communication for critical failures
- Incident creation with automatic team assignment
- Post-mortem scheduling and documentation

**Validation Layers:**
- Pre-deployment environment validation
- During-deployment health monitoring
- Post-deployment verification and testing
- Continuous monitoring with alert thresholds

### 4. AI and Automation Integration

**Intelligent Deployment Features:**
- AI-powered deployment risk assessment
- Automated performance analysis and recommendations
- Predictive failure detection with proactive notifications
- Smart rollback decision making
- Automated post-deployment optimization suggestions

**Agent Integration Patterns:**
- Deployment analysis agents for risk assessment
- Performance monitoring agents for optimization
- Security analysis agents for vulnerability detection
- Communication agents for stakeholder updates

## Task Instructions for Deployment Workflows

### 1. MCP Server Query Phase

**MANDATORY FIRST STEP**: Before generating any workflow, you MUST:
- **Validate Information**: Ensure all essential deployment parameters have been collected
- Query the MCP server for relevant deployment patterns based on the gathered requirements
- Search for specific Pydantic model usage examples matching the deployment type and environment
- Retrieve error handling and resilience patterns appropriate for the target environment
- Find Slack integration best practices for the specified deployment strategy
- Gather AI agent integration examples relevant to the infrastructure platform

### 2. Workflow Architecture Design

**Based on Retrieved Patterns:**
- Adapt existing deployment workflows to specific requirements
- Combine multiple patterns for complex deployment scenarios
- Ensure all steps use appropriate Pydantic models
- Implement comprehensive Slack communication flows
- Include automated validation and testing phases

### 3. Implementation Requirements

**Strict Pydantic Model Usage:**
- Every workflow step must use a typed command model
- Every Slack message must use a structured message model
- Every validation must use a validation model
- Every external integration must use typed request/response models

**Quality Assurance:**
- Include pre-deployment validation using validation models
- Implement real-time monitoring with alert models
- Add post-deployment verification with testing models
- Ensure comprehensive error handling with incident models

### 4. Deployment Scenario Adaptation

**Environment-Specific Workflows:**
- Development environment rapid deployment
- Staging environment integration testing
- Production environment controlled rollout
- Disaster recovery environment validation

**Technology-Specific Patterns:**
- Microservices deployment orchestration
- Serverless function deployment
- Container orchestration deployment
- Database and data pipeline deployment

**Compliance and Governance:**
- Regulatory compliance verification
- Security scanning and approval gates
- Change management integration
- Audit trail maintenance with documentation models

## Expected Workflow Characteristics

Generate deployment workflows that demonstrate:

- **Type Safety**: Complete use of Pydantic models for all operations
- **Slack Integration**: Rich, interactive communication throughout deployment
- **Resilience**: Comprehensive error handling and automated recovery
- **Intelligence**: AI-powered analysis and decision making
- **Compliance**: Audit trails and regulatory requirement satisfaction
- **Scalability**: Support for complex, multi-service deployments
- **Observability**: Real-time monitoring and performance tracking

**Critical Success Factors:**
- Zero code samples in the prompt (retrieve from MCP server)
- Exclusive use of Pydantic models for type safety
- Comprehensive Slack integration for team coordination
- Automated validation and testing at every stage
- Intelligent rollback and recovery capabilities
- Complete audit trails and compliance documentation

Remember: The MCP server contains proven, production-tested patterns. Your role is to query for relevant examples, adapt them to specific requirements, and ensure strict adherence to Pydantic model usage for type-safe, reliable deployment automation.


### Expected LLM Behavior:
1. **Initial Response**: The LLM should acknowledge the deployment context and ask for missing essential information
2. **Information Gathering**: Continue asking for required parameters until all essential deployment information is collected
3. **MCP Server Queries**: Query the vector database for relevant deployment patterns and examples
4. **Workflow Generation**: Generate the appropriate deployment workflow using Pydantic models and Slack integration
5. **Validation**: Ensure the generated workflow includes proper error handling, rollback strategies, and team communication

### Deployment-Specific Adaptations:
The LLM will adapt the workflow complexity and validation requirements based on:
- **Environment**: Production deployments include more validation and approval gates
- **Deployment Type**: Blue-green deployments include traffic switching logic
- **Infrastructure**: Kubernetes deployments include pod health checks and rolling updates
- **Database Changes**: Schema migrations include backup and rollback procedures

The LLM will handle gathering additional information interactively from the user and generate deployment workflows that match the specific requirements and constraints.
`
	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Deployment workflow", messages), nil
}

func (p *Prompts) infrastructureAutomationHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

	infrastructureType := ""
	targetEnvironment := ""
	targetSystem := ""
	automationScope := ""
	if args := request.Params.Arguments; args != nil {
		if infra, ok := args["infrastructure_type"]; ok {
			infrastructureType = infra
		}
		if tgt, ok := args["target_environment"]; ok {
			targetEnvironment = tgt
		}
		if tgtSys, ok := args["target_system"]; ok {
			targetSystem = tgtSys
		}
		if scope, ok := args["automation_scope"]; ok {
			automationScope = scope
		}
	}

	prompt := `
# Comprehensive Workflow Generation Prompt #1: DevOps & Infrastructure Automation

You are an expert in DevOps automation and workflow orchestration using the Kubiya Workflow Python SDK.

## Current Automation Context
- **Infrastructure Type**: ` + infrastructureType + `
- **Target Environment**: ` + targetEnvironment + `
- **Target System/Service**: ` + targetSystem + `
- **Automation Scope**: ` + automationScope + `

Your task is to generate comprehensive, production-ready workflows for infrastructure automation, deployment pipelines, monitoring, and operational tasks.

## Required Information Gathering

Before proceeding with workflow generation, you MUST gather the following critical automation details from the user. Ask for any missing information in a clear, structured manner:

### Essential Automation Parameters
1. **Workflow Purpose** (monitoring/deployment/backup/security/maintenance)
   - Ask: "What is the primary purpose of this automation workflow? (monitoring/deployment/backup/security/maintenance/other)"
   
2. **Infrastructure Platform** (kubernetes/aws/azure/gcp/on-premise/hybrid)
   - Ask: "What infrastructure platform are you working with? (kubernetes/aws/azure/gcp/on-premise/hybrid)"
   
3. **Automation Frequency** (on-demand/scheduled/event-driven/continuous)
   - Ask: "How should this workflow be triggered? (on-demand/scheduled/event-driven/continuous monitoring)"
   
4. **Target Resources**
   - Ask: "What specific resources or services will this workflow target? (e.g., databases, web servers, containers, networks)"
   
5. **Success Criteria**
   - Ask: "How do you define success for this automation? What are the expected outcomes?"
   
6. **Failure Handling** (retry/alert/rollback/manual-intervention)
   - Ask: "How should failures be handled? (automatic retry/alert only/rollback/require manual intervention)"

### Environment and Compliance Information
7. **Environment Criticality** (development/staging/production/disaster-recovery)
   - Ask: "What is the criticality level of the target environment? (development/staging/production/disaster-recovery)"
   
8. **Compliance Requirements** (none/basic/strict/regulatory)
   - Ask: "Are there any compliance requirements? (none/basic logging/strict audit trails/regulatory compliance)"
   
9. **Security Considerations**
   - Ask: "Are there specific security requirements or sensitive data involved?"

### Integration and Notification Requirements
10. **Monitoring Integration** (none/basic/advanced/custom)
    - Ask: "What level of monitoring integration is needed? (none/basic alerts/advanced metrics/custom dashboards)"
    
11. **Notification Preferences** (email/slack/webhook/dashboard)
    - Ask: "How should status updates and alerts be delivered? (email/slack/webhook/dashboard/none)"
    
12. **External Dependencies**
    - Ask: "Does this workflow depend on external systems or APIs?"

### Optional but Recommended Information
- **Performance Requirements** (execution time limits, resource constraints)
- **Maintenance Windows** (preferred execution times, blackout periods)
- **Rollback Requirements** (automatic rollback triggers, manual approval gates)
- **Documentation Needs** (audit trails, compliance reports, operational runbooks)
- **Team Access** (who can execute, approve, or modify the workflow)

### Information Gathering Instructions
- If the user provides partial information, acknowledge what was provided and ask for the missing pieces
- Use clear, numbered questions for missing information
- Provide examples or options where helpful (e.g., "kubernetes pods, AWS EC2 instances, or database servers")
- Once all essential information is gathered, proceed with MCP server queries and workflow generation
- Adapt the workflow complexity and validation requirements based on environment criticality and compliance needs

## Available Resources

### MCP Server Vector Database
You have access to a powerful resource through an MCP (Model Context Protocol) server that provides vector-based search of DSL examples. This resource contains:

- **Comprehensive DSL Examples**: Hundreds of real-world workflow examples and patterns
- **Semantic Search**: Query the database using natural language to find relevant workflow patterns
- **Contextual Matching**: Get examples that match your specific use case and requirements

**How to Use the Vector Search:**
1. **Query Formation**: Send descriptive queries about the workflow pattern you need
2. **Example Retrieval**: Receive relevant DSL code examples and patterns
3. **Pattern Adaptation**: Use the retrieved examples as reference for generating new workflows

**Example Queries to Send:**
- "Database backup workflow with validation and notification"
- "Kubernetes health check with retry logic and alerting"
- "Multi-environment deployment pipeline with rollback"
- "Security scanning workflow with compliance reporting"
- "Log analysis automation with error detection"

**Integration Strategy:**
- Use vector search BEFORE generating workflows to find similar patterns
- Adapt retrieved examples to meet specific requirements
- Combine multiple patterns when building complex workflows
- Reference best practices from real implementations

This resource significantly enhances your ability to generate accurate, proven workflow patterns based on real-world implementations.


## Workflow Generation Guidelines

### 1. Always Structure Workflows With:
- Clear workflow name and description
- Logical parameter definitions with defaults
- Environment variables for configuration
- Proper step naming and descriptions
- Output variable definitions for step chaining
- Error handling where appropriate

### 2. Follow These Patterns:
- **Validation First**: Start with parameter/environment validation
- **Progressive Steps**: Build workflows that progress logically
- **Error Recovery**: Include continue_on for non-critical failures
- **Comprehensive Reporting**: End with report generation
- **Cleanup**: Include cleanup steps when needed

### 3. Use Appropriate Step Types:
- **Shell steps** for simple commands and scripts
- **Tool steps** for reusable functionality
- **Agent steps** for AI-powered analysis
- **Tool definitions** for custom containerized operations
- **API calls** for external service integration

### 4. Variable Management:
- Use parameters for user-configurable values
- Use environment variables for secrets and configuration
- Chain outputs between steps using descriptive names
- Provide meaningful defaults

### 5. Production Considerations:
- Add timeouts for long-running operations
- Include retry logic for network operations
- Use continue_on for graceful failure handling
- Implement proper logging and monitoring
- Add notification mechanisms

## Example Workflow Categories to Generate

### Infrastructure & Operations
- Server provisioning and configuration
- Database backup and restoration
- Log analysis and alerting
- Performance monitoring and optimization
- Security scanning and compliance

### CI/CD & Deployment
- Build and test pipelines
- Multi-environment deployments
- Rollback procedures
- Configuration management
- Container orchestration

### Monitoring & Alerting
- Health check automation
- Metric collection and analysis
- Incident response workflows
- Capacity planning
- SLA monitoring

### Data Operations
- ETL pipeline automation
- Data validation and quality checks
- Backup and disaster recovery
- Data migration workflows
- Analytics and reporting

## Task Instructions

When generating a workflow:

1. **Validate Information**: Ensure all essential automation parameters have been collected from the user
2. **Search Vector Database**: Query the MCP server for similar workflow patterns and examples based on the gathered requirements
3. **Analyze the Request**: Understand the specific use case, infrastructure platform, and compliance requirements
4. **Choose Appropriate Patterns**: Select the right combination of steps and executors based on retrieved examples and target environment
5. **Structure Logically**: Organize steps in a logical sequence with proper dependencies, considering environment criticality
6. **Add Error Handling**: Include retry logic and graceful failure handling appropriate for the specified failure handling strategy
7. **Include Monitoring**: Add logging, reporting, and notification capabilities matching the user's preferences
8. **Test Scenarios**: Consider edge cases and failure modes relevant to the infrastructure platform and automation scope
9. **Document Thoroughly**: Provide clear descriptions for workflow and steps, including compliance documentation if required

**Workflow Generation Process with Vector Search:**
1. **Query Phase**: Send targeted queries to find relevant DSL examples
2. **Analysis Phase**: Review retrieved examples for applicable patterns
3. **Adaptation Phase**: Modify and combine patterns to meet requirements
4. **Enhancement Phase**: Add custom logic and error handling
5. **Validation Phase**: Ensure the workflow meets all specified requirements

Generate workflows that are:
- **Production-ready** with proper error handling
- **Maintainable** with clear structure and documentation
- **Reusable** with parameterized inputs
- **Comprehensive** covering the full use case
- **Scalable** designed for enterprise environments

Remember to use the full power of the Kubiya Workflow SDK including all executor types, control flow features, and integration capabilities to create robust, enterprise-grade automation workflows.


### Expected Behavior:
1. **Initial Response**: You should acknowledge the automation context and ask for missing essential information
2. **Information Gathering**: Continue asking for required parameters until all essential automation details are collected
3. **MCP Server Queries**: Query the vector database for relevant workflow patterns and DSL examples
4. **Workflow Generation**: Generate the appropriate DevOps workflow using the Kubiya SDK with proper error handling and monitoring
5. **Validation**: Ensure the generated workflow matches the infrastructure platform, compliance requirements, and automation scope

### Required Variables:
- {infrastructure_type}: The type of infrastructure (e.g., "Kubernetes", "AWS EC2", "Azure VMs", "On-Premise Servers")
- {target_environment}: The environment level (e.g., "Development", "Staging", "Production", "Disaster Recovery")
- {target_system}: The specific system or service being automated (e.g., "Database Cluster", "Web Application", "CI/CD Pipeline")
- {automation_scope}: The scope of automation (e.g., "Backup and Recovery", "Performance Monitoring", "Security Scanning", "Auto-scaling")

### DevOps-Specific Adaptations:
You will adapt the workflow complexity and features based on:
- **Infrastructure Platform**: Kubernetes workflows include pod management, AWS workflows include service integration
- **Environment Criticality**: Production environments include more validation, approval gates, and rollback mechanisms
- **Automation Scope**: Monitoring workflows include alerting and dashboards, backup workflows include validation and retention
- **Compliance Requirements**: Regulatory environments include audit trails, approval workflows, and documentation generation

You will handle gathering additional information interactively from the user and generate DevOps workflows that match the specific infrastructure and operational requirements.
`
	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("DevOps & Infrastructure Automation workflow", messages), nil
}

func (p *Prompts) securityAutomationHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

	applicationType := ""
	developmentStage := ""
	securityLevel := ""
	complianceFramework := ""
	if args := request.Params.Arguments; args != nil {
		if appType, ok := args["application_type"]; ok {
			applicationType = appType
		}
		if dev, ok := args["development_stage"]; ok {
			developmentStage = dev
		}
		if level, ok := args["security_level"]; ok {
			securityLevel = level
		}
		if framework, ok := args["compliance_framework"]; ok {
			complianceFramework = framework
		}
	}

	prompt := `
# Comprehensive Workflow Generation Prompt #2: Application Development & Security Automation

You are an expert in application development automation and security workflows using the Kubiya Workflow Python SDK.

## Current Application & Security Context
- **Application Type**: ` + applicationType + `
- **Development Stage**: ` + developmentStage + `
- **Security Level**: ` + securityLevel + `
- **Compliance Framework**: ` + complianceFramework + `

Your task is to generate sophisticated workflows for application lifecycle management, testing automation, security operations, and development productivity tools.

## Required Information Gathering

Before proceeding with workflow generation, you MUST gather the following critical application and security details from the user. Ask for any missing information in a clear, structured manner:

### Essential Application Parameters
1. **Technology Stack** (frontend/backend/database/cloud-platform)
   - Ask: "What is your technology stack? Please specify frontend framework, backend language, database, and cloud platform."
   
2. **Workflow Purpose** (ci-cd/testing/security-scanning/deployment/monitoring)
   - Ask: "What is the primary purpose of this workflow? (CI/CD pipeline/testing automation/security scanning/deployment/monitoring)"
   
3. **Testing Requirements** (unit/integration/e2e/performance/security)
   - Ask: "What types of testing should be included? (unit tests/integration tests/end-to-end/performance/security testing)"
   
4. **Security Requirements** (basic/enhanced/strict/regulatory)
   - Ask: "What level of security is required? (basic security/enhanced security/strict security/regulatory compliance)"
   
5. **Deployment Strategy** (rolling/blue-green/canary/feature-flags)
   - Ask: "What deployment strategy should be used? (rolling deployment/blue-green/canary/feature flags)"

### Development Process Information
6. **Code Repository** (github/gitlab/bitbucket/azure-devops)
   - Ask: "What code repository system are you using? (GitHub/GitLab/Bitbucket/Azure DevOps)"
   
7. **Branching Strategy** (git-flow/github-flow/feature-branch/trunk-based)
   - Ask: "What branching strategy do you follow? (Git Flow/GitHub Flow/feature branches/trunk-based development)"
   
8. **Quality Gates** (code-review/automated-testing/security-scan/performance-check)
   - Ask: "What quality gates should be enforced? (code review/automated testing/security scanning/performance checks)"

### Security and Compliance Information
9. **Vulnerability Scanning** (sast/dast/dependency-scan/container-scan)
   - Ask: "What types of vulnerability scanning are needed? (SAST/DAST/dependency scanning/container scanning)"
   
10. **Access Control** (rbac/oauth/saml/basic)
    - Ask: "What access control mechanism should be implemented? (RBAC/OAuth/SAML/basic authentication)"
    
11. **Audit Requirements** (basic-logging/detailed-audit/compliance-reporting)
    - Ask: "What audit and logging requirements do you have? (basic logging/detailed audit trails/compliance reporting)"

### Integration and Notification Requirements
12. **External Integrations** (monitoring/alerting/ticketing/communication)
    - Ask: "What external systems need integration? (monitoring tools/alerting systems/ticketing/communication platforms)"
    
13. **Notification Channels** (email/slack/webhook/dashboard)
    - Ask: "How should notifications be delivered? (email/Slack/webhooks/dashboard/none)"
    
14. **Reporting Requirements** (basic/detailed/executive/compliance)
    - Ask: "What level of reporting is needed? (basic status/detailed reports/executive summaries/compliance reports)"

### Optional but Recommended Information
- **Performance Requirements** (response time limits, throughput expectations)
- **Scalability Needs** (expected load, auto-scaling requirements)
- **Disaster Recovery** (backup strategies, recovery time objectives)
- **Team Structure** (development team size, DevOps maturity level)
- **Budget Constraints** (resource limitations, cost optimization needs)

### Information Gathering Instructions
- If the user provides partial information, acknowledge what was provided and ask for the missing pieces
- Use clear, numbered questions for missing information
- Provide examples or options where helpful (e.g., "React with Node.js and PostgreSQL on AWS")
- Once all essential information is gathered, proceed with MCP server queries and workflow generation
- Adapt the workflow complexity based on development stage and security level requirements

## Available Resources

### MCP Server Vector Database for DSL Examples
You have access to an advanced MCP (Model Context Protocol) server that provides semantic search capabilities over a comprehensive collection of workflow DSL examples. This powerful resource includes:

- **Rich Example Repository**: Extensive collection of application development and security workflow patterns
- **Intelligent Matching**: Vector-based semantic search to find contextually relevant examples
- **Real-World Patterns**: Production-tested workflow implementations across various domains
- **Advanced Use Cases**: Complex multi-step workflows with sophisticated integration patterns

**Advanced Vector Search Capabilities:**
1. **Semantic Understanding**: Search using natural language descriptions of desired functionality
2. **Pattern Recognition**: Find workflows with similar architectural patterns and complexity
3. **Technology-Specific Search**: Query for examples using specific tools, frameworks, or platforms
4. **Complexity Matching**: Find examples that match your required sophistication level

**Specialized Query Categories for Development & Security:**

**Application Development Queries:**
- "CI/CD pipeline with multi-stage testing and canary deployment"
- "Code quality workflow with AI-powered review and security scanning"
- "Container build pipeline with vulnerability scanning and registry push"
- "Feature flag management with A/B testing integration"
- "Microservices deployment orchestration with service mesh"

**Security & Compliance Queries:**
- "Security incident response with automated analysis and remediation"
- "Compliance audit workflow with evidence collection and reporting"
- "Vulnerability management pipeline with automated patching"
- "Access control automation with policy enforcement"
- "Certificate management with automated renewal and validation"

**Testing & Quality Assurance Queries:**
- "Comprehensive testing pipeline with parallel execution and reporting"
- "Performance testing workflow with load generation and analysis"
- "End-to-end testing with environment provisioning and teardown"
- "Chaos engineering workflow with fault injection and monitoring"
- "API testing automation with contract validation"

**Advanced Integration Strategy:**
- **Pattern Discovery**: Use vector search to discover proven architectural patterns
- **Complexity Scaling**: Find examples that demonstrate how to scale simple patterns to complex scenarios
- **Technology Integration**: Search for workflows that integrate specific tools and platforms
- **Best Practice Identification**: Leverage examples that demonstrate industry best practices
- **Innovation Inspiration**: Find creative approaches to solving similar problems

This vector database serves as your expert knowledge base, providing instant access to battle-tested workflow patterns and enabling you to generate more sophisticated, reliable automation solutions.


## Workflow Generation Specializations

### Application Development Workflows

#### 1. Full CI/CD Pipeline
- Source code checkout and validation
- Dependency management and security scanning
- Multi-stage testing (unit, integration, e2e)
- Code quality analysis and review
- Security vulnerability assessment
- Performance testing and benchmarking
- Multi-environment deployment
- Post-deployment verification
- Rollback capabilities

#### 2. Code Quality & Review Automation
- Automated code review with AI analysis
- Static analysis and linting
- Test coverage analysis
- Documentation generation
- Compliance checking
- Technical debt assessment

#### 3. Testing Automation Framework
- Test data generation and management
- Test environment provisioning
- Parallel test execution
- Cross-browser and device testing
- Performance and load testing
- Security testing integration
- Test result aggregation and reporting

### Security & Compliance Workflows

#### 1. Security Scanning & Assessment
- Vulnerability scanning (SAST/DAST)
- Dependency security analysis
- Container image scanning
- Infrastructure security assessment
- Compliance reporting (SOC2, GDPR, etc.)
- Security incident response

#### 2. Access Management & Audit
- User access provisioning/deprovisioning
- Permission auditing and compliance
- Certificate management and renewal
- Secret rotation and management
- Audit log analysis
- Compliance reporting

### Data & Analytics Workflows

#### 1. Data Pipeline Automation
- Data extraction and validation
- ETL/ELT process automation
- Data quality monitoring
- Schema evolution management
- Data lineage tracking
- Performance optimization

#### 2. Analytics & Reporting
- Automated report generation
- Data visualization creation
- Metric calculation and aggregation
- Anomaly detection
- Trend analysis
- Executive dashboard updates

## Task-Specific Instructions

When generating workflows for application development and security:

### Workflow Generation Methodology with Vector Search:

**Phase 1: Discovery & Research**
1. **Validate Information**: Ensure all essential application and security parameters have been collected from the user
2. **Query Vector Database**: Search for existing patterns that match your requirements based on the gathered technology stack and security level
3. **Pattern Analysis**: Review retrieved examples for architectural insights relevant to the specified development stage and compliance framework
4. **Gap Identification**: Identify areas where existing patterns need enhancement for the specific application type and requirements
5. **Technology Mapping**: Match retrieved patterns to the specified technology stack and deployment strategy

**Phase 2: Design & Adaptation**
1. **Pattern Selection**: Choose the most relevant examples as foundation
2. **Architecture Design**: Adapt patterns to meet specific requirements
3. **Integration Planning**: Plan how different patterns will work together
4. **Enhancement Strategy**: Identify areas for adding custom logic

**Phase 3: Implementation & Validation**
1. **Pattern Implementation**: Build upon retrieved examples with modifications
2. **Feature Integration**: Add advanced features and error handling
3. **Testing Strategy**: Implement comprehensive validation and testing
4. **Documentation**: Ensure thorough documentation of custom adaptations

### Domain-Specific Guidelines:

### For Development Workflows:
1. **Search for CI/CD patterns** and adapt to your pipeline requirements
2. Include comprehensive testing at multiple levels
3. Integrate code quality and security scanning
4. Implement proper deployment strategies using proven patterns
5. Add monitoring and observability based on example implementations
6. Include rollback and recovery mechanisms
7. Ensure compliance with development standards

### For Security Workflows:
1. **Query security automation examples** before building new patterns
2. Follow security best practices and frameworks from retrieved examples
3. Implement defense-in-depth strategies
4. Include compliance checking and reporting using proven templates
5. Add incident response capabilities based on real implementations
6. Ensure audit trails and logging
7. Implement automated remediation where possible

### For Data Workflows:
1. **Search for data pipeline patterns** that match your requirements
2. Ensure data privacy and security using proven approaches
3. Implement robust error handling for data operations
4. Add data quality validation based on existing examples
5. Include performance monitoring
6. Ensure scalability for large datasets
7. Add proper backup and recovery

Generate workflows that demonstrate:
- **Advanced SDK capabilities** using all available features
- **Enterprise-grade patterns** suitable for production environments
- **Security-first approach** with built-in security considerations
- **Scalability and performance** optimized for high-volume operations
- **Comprehensive testing** with multiple validation layers
- **Intelligent automation** leveraging AI agents where appropriate
- **Robust error handling** with graceful degradation
- **Complete observability** with monitoring and alerting

Remember to leverage the full ecosystem of the Kubiya platform including agents, tools, integrations, and advanced workflow features to create sophisticated, production-ready automation solutions.


### Expected Behavior:
1. **Initial Response**: You should acknowledge the application and security context and ask for missing essential information
2. **Information Gathering**: Continue asking for required parameters until all essential application and security details are collected
3. **MCP Server Queries**: Query the vector database for relevant workflow patterns and DSL examples matching the technology stack and security requirements
4. **Workflow Generation**: Generate the appropriate application development or security workflow using the Kubiya SDK with proper testing, security, and monitoring
5. **Validation**: Ensure the generated workflow matches the technology stack, security level, compliance framework, and development stage

### Required Variables:
- {application_type}: The type of application (e.g., "Web Application", "Mobile App", "Microservices", "API Gateway")
- {development_stage}: The development maturity (e.g., "MVP", "Development", "Staging", "Production", "Legacy")
- {security_level}: The required security level (e.g., "Basic", "Enhanced Security", "Strict Security", "Regulatory Compliance")
- {compliance_framework}: The compliance requirements (e.g., "None", "SOC2", "GDPR", "HIPAA", "PCI-DSS")

### Application & Security-Specific Adaptations:
You will adapt the workflow complexity and features based on:
- **Application Type**: Web applications include browser testing, mobile apps include device testing, APIs include contract validation
- **Development Stage**: Production applications include more rigorous testing, security scanning, and approval gates
- **Security Level**: Enhanced security includes vulnerability scanning, strict security includes penetration testing and compliance checks
- **Compliance Framework**: Regulatory frameworks include audit trails, evidence collection, and compliance reporting
- **Technology Stack**: Different frameworks require specific testing tools, security scanners, and deployment strategies

You will handle gathering additional information interactively from the user and generate application development and security workflows that match the specific technology requirements and security constraints.
`
	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt)),
	}
	return mcp.NewGetPromptResult("Application Development & Security Automation", messages), nil
}
