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

// Embed root __init__.py
//go:embed worker_files/__init__.py
var appInit string
