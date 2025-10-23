package cli

import (
	_ "embed"
)

// Embed worker Python script
//go:embed worker_files/worker.py
var workerPyScript string

// Embed requirements.txt
//go:embed requirements.txt
var requirementsTxt string

// Embed workflows
//go:embed worker_files/workflows/__init__.py
var workflowsInit string

//go:embed worker_files/workflows/agent_execution.py
var agentExecutionWorkflow string

//go:embed worker_files/workflows/team_execution.py
var teamExecutionWorkflow string

// Embed activities
//go:embed worker_files/activities/__init__.py
var activitiesInit string

//go:embed worker_files/activities/agent_activities.py
var agentActivities string

//go:embed worker_files/activities/team_activities.py
var teamActivities string

//go:embed worker_files/activities/toolset_activities.py
var toolsetActivities string

// Embed root __init__.py
//go:embed worker_files/__init__.py
var appInit string

// Embed control_plane_client
//go:embed worker_files/control_plane_client.py
var controlPlaneClient string

// Embed models
//go:embed worker_files/models/__init__.py
var modelsInit string

//go:embed worker_files/models/inputs.py
var modelsInputs string

// Embed services
//go:embed worker_files/services/__init__.py
var servicesInit string

//go:embed worker_files/services/agent_executor.py
var agentExecutorService string

//go:embed worker_files/services/agent_executor_v2.py
var agentExecutorV2Service string

//go:embed worker_files/services/cancellation_manager.py
var cancellationManagerService string

//go:embed worker_files/services/session_service.py
var sessionService string

//go:embed worker_files/services/team_executor.py
var teamExecutorService string

//go:embed worker_files/services/toolset_factory.py
var toolsetFactoryService string

// Embed runtimes
//go:embed worker_files/runtimes/__init__.py
var runtimesInit string

//go:embed worker_files/runtimes/base.py
var runtimesBase string

//go:embed worker_files/runtimes/claude_code_runtime.py
var runtimesClaudeCode string

//go:embed worker_files/runtimes/default_runtime.py
var runtimesDefault string

//go:embed worker_files/runtimes/factory.py
var runtimesFactory string

// Embed utils
//go:embed worker_files/utils/__init__.py
var utilsInit string

//go:embed worker_files/utils/retry_utils.py
var utilsRetry string

//go:embed worker_files/utils/streaming_utils.py
var utilsStreaming string
