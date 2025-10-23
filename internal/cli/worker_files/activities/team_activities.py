"""Team-related Temporal activities - Thin wrappers around services"""

import os
import httpx
from dataclasses import dataclass
from typing import Optional, List, Dict, Any
from datetime import datetime, timezone
from temporalio import activity
import structlog

from control_plane_client import get_control_plane_client
from models.inputs import (
    TeamExecutionInput,
    CancelExecutionInput
)
from services.team_executor import TeamExecutorService
from services.session_service import SessionService
from services.cancellation_manager import cancellation_manager

logger = structlog.get_logger()


@dataclass
class ActivityGetTeamAgentsInput:
    """Input for get_team_agents activity"""
    team_id: str
    organization_id: str


@dataclass
class ActivityExecuteTeamInput:
    """Input for execute_team_coordination activity"""
    execution_id: str
    team_id: str
    organization_id: str
    prompt: str
    system_prompt: Optional[str] = None
    agents: List[dict] = None
    team_config: dict = None
    mcp_servers: dict = None
    session_id: Optional[str] = None
    user_id: Optional[str] = None
    model_id: Optional[str] = None

    def __post_init__(self):
        if self.agents is None:
            self.agents = []
        if self.team_config is None:
            self.team_config = {}
        if self.mcp_servers is None:
            self.mcp_servers = {}


@dataclass
class ActivityCancelTeamInput:
    execution_id: str


# Backward compatibility aliases for workflows
ActivityCancelExecutionInput = CancelExecutionInput


@activity.defn(name="get_team_agents")
async def get_team_agents(input: ActivityGetTeamAgentsInput) -> dict:
    """
    Get all agents in a team via Control Plane API.

    Args:
        input: ActivityGetTeamAgentsInput with team details

    Returns:
        Dict with agents list
    """
    print(f"\n🔍 Getting team agents...")
    print(f"   Team ID: {input.team_id}")
    print(f"   Organization: {input.organization_id}\n")

    activity.logger.info(
        "getting_team_agents",
        extra={
            "team_id": input.team_id,
            "organization_id": input.organization_id,
        }
    )

    try:
        # Get Control Plane URL and API key from environment
        control_plane_url = os.getenv("CONTROL_PLANE_URL")
        kubiya_api_key = os.getenv("KUBIYA_API_KEY")

        if not control_plane_url:
            raise ValueError("CONTROL_PLANE_URL environment variable not set")
        if not kubiya_api_key:
            raise ValueError("KUBIYA_API_KEY environment variable not set")

        # Call Control Plane API to get team with agents
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.get(
                f"{control_plane_url}/api/v1/teams/{input.team_id}",
                headers={
                    "Authorization": f"Bearer {kubiya_api_key}",
                    "Content-Type": "application/json",
                }
            )

            if response.status_code == 404:
                print(f"⚠️  Team not found\n")
                activity.logger.error(
                    "team_not_found",
                    extra={
                        "team_id": input.team_id,
                        "organization_id": input.organization_id,
                    }
                )
                return {"agents": [], "count": 0}
            elif response.status_code != 200:
                raise Exception(f"Failed to get team: {response.status_code} - {response.text}")

            team_data = response.json()

        # Extract agents from the API response
        agents = team_data.get("agents", [])

        print(f"✅ Found {len(agents)} agents")
        if agents:
            for agent in agents:
                print(f"   - {agent.get('name')} (ID: {agent.get('id')})")
        print()

        activity.logger.info(
            "team_agents_retrieved",
            extra={
                "team_id": input.team_id,
                "agent_count": len(agents),
                "agent_names": [a.get("name") for a in agents],
                "agent_ids": [a.get("id") for a in agents],
            }
        )

        if not agents:
            activity.logger.warning(
                "no_agents_in_team",
                extra={
                    "team_id": input.team_id,
                    "organization_id": input.organization_id,
                }
            )

        return {
            "agents": agents,
            "count": len(agents),
        }

    except Exception as e:
        print(f"❌ Error getting team agents: {str(e)}\n")
        activity.logger.error(
            "get_team_agents_error",
            extra={
                "team_id": input.team_id,
                "organization_id": input.organization_id,
                "error": str(e),
                "error_type": type(e).__name__,
            }
        )
        raise


@activity.defn(name="execute_team_coordination")
async def execute_team_coordination(input: ActivityExecuteTeamInput) -> dict:
    """
    Execute team coordination using Agno Teams.

    This is a thin wrapper around TeamExecutorService which contains
    the actual business logic.

    Args:
        input: ActivityExecuteTeamInput with team execution details

    Returns:
        Dict with response, usage, success flag
    """
    try:
        # Initialize services
        control_plane = get_control_plane_client()
        session_service = SessionService(control_plane)
        executor = TeamExecutorService(
            control_plane=control_plane,
            session_service=session_service,
            cancellation_manager=cancellation_manager
        )

        # Send heartbeat: Starting execution
        activity.heartbeat({"status": "Team is processing your request..."})

        # Execute (all business logic in service)
        result = await executor.execute(input)

        # Send heartbeat: Completed
        activity.heartbeat({"status": "Team execution completed"})

        return result

    except Exception as e:
        activity.logger.error(
            "team_execution_failed",
            extra={"execution_id": input.execution_id, "error": str(e)}
        )
        return {
            "success": False,
            "error": str(e),
            "model": input.model_id,
            "usage": None,
            "finish_reason": "error",
        }


@activity.defn(name="cancel_team_run")
async def cancel_team_run(input: ActivityCancelTeamInput) -> dict:
    """
    Cancel an active team run using Agno's cancel_run API.

    This is called when a user clicks the STOP button in the UI.

    Args:
        input: ActivityCancelTeamInput with execution_id

    Returns:
        Dict with success status and cancellation details
    """
    print("\n" + "="*80)
    print("🛑 CANCEL TEAM RUN")
    print("="*80)
    print(f"Execution ID: {input.execution_id}\n")

    try:
        result = cancellation_manager.cancel(input.execution_id)

        if result["success"]:
            print(f"✅ Team run cancelled successfully!\n")
        else:
            print(f"⚠️  Cancel failed: {result.get('error')}\n")

        return result

    except Exception as e:
        print(f"❌ Error cancelling run: {str(e)}\n")
        activity.logger.error(
            "cancel_team_error",
            extra={
                "execution_id": input.execution_id,
                "error": str(e),
            }
        )
        return {
            "success": False,
            "error": str(e),
            "execution_id": input.execution_id,
        }
