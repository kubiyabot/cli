"""Agent-related Temporal activities - Thin wrappers around services"""

from datetime import datetime, timezone
from temporalio import activity
import structlog
import os
import httpx
from typing import Dict, Any

from control_plane_client import get_control_plane_client
from models.inputs import (
    AgentExecutionInput,
    UpdateExecutionStatusInput,
    UpdateAgentStatusInput,
    CancelExecutionInput
)

# Backward compatibility aliases for workflows
ActivityExecuteAgentInput = AgentExecutionInput
ActivityUpdateExecutionInput = UpdateExecutionStatusInput
ActivityUpdateAgentInput = UpdateAgentStatusInput
ActivityCancelAgentInput = CancelExecutionInput
from services.agent_executor import AgentExecutorService
from services.session_service import SessionService
from services.cancellation_manager import cancellation_manager

logger = structlog.get_logger()


@activity.defn(name="execute_agent_llm")
async def execute_agent_llm(input: AgentExecutionInput) -> dict:
    """
    Execute an agent's LLM call with Agno and session management.

    This is a thin wrapper around AgentExecutorService which contains
    the actual business logic.

    Args:
        input: AgentExecutionInput with execution details

    Returns:
        Dict with response, usage, success flag, etc.
    """
    try:
        # Initialize services
        control_plane = get_control_plane_client()
        session_service = SessionService(control_plane)
        executor = AgentExecutorService(
            control_plane=control_plane,
            session_service=session_service,
            cancellation_manager=cancellation_manager
        )

        # Send heartbeat: Starting execution
        activity.heartbeat({"status": "Agent is processing your request..."})

        # Execute (all business logic in service)
        result = await executor.execute(input)

        # Send heartbeat: Completed
        activity.heartbeat({"status": "Agent execution completed"})

        return result

    except Exception as e:
        activity.logger.error(
            "agent_execution_failed",
            extra={"execution_id": input.execution_id, "error": str(e)}
        )
        return {
            "success": False,
            "error": str(e),
            "model": input.model_id,
            "usage": None,
            "finish_reason": "error",
        }


@activity.defn(name="update_execution_status")
async def update_execution_status(input: UpdateExecutionStatusInput) -> dict:
    """
    Update execution status in database via Control Plane API.

    Args:
        input: UpdateExecutionStatusInput with update details

    Returns:
        Dict with success flag
    """
    print(f"🔄 Updating execution status: {input.status} (execution_id: {input.execution_id[:8]}...)")

    activity.logger.info(
        "updating_execution_status",
        extra={
            "execution_id": input.execution_id,
            "status": input.status,
        }
    )

    try:
        # Get Control Plane URL and API key from environment
        control_plane_url = os.getenv("CONTROL_PLANE_URL")
        kubiya_api_key = os.getenv("KUBIYA_API_KEY")
        worker_id = os.getenv("WORKER_ID", "unknown")

        if not control_plane_url:
            raise ValueError("CONTROL_PLANE_URL environment variable not set")
        if not kubiya_api_key:
            raise ValueError("KUBIYA_API_KEY environment variable not set")

        # Collect worker system information
        import socket
        import platform
        worker_info = {
            "worker_id": worker_id,
            "hostname": socket.gethostname(),
            "platform": platform.platform(),
            "python_version": platform.python_version(),
        }

        # Build update payload
        update_payload = {}

        if input.status:
            update_payload["status"] = input.status

        if input.started_at:
            update_payload["started_at"] = input.started_at

        if input.completed_at:
            update_payload["completed_at"] = input.completed_at

        if input.response is not None:
            update_payload["response"] = input.response

        if input.error_message is not None:
            update_payload["error_message"] = input.error_message

        if input.usage:
            update_payload["usage"] = input.usage

        # Merge worker info into execution_metadata
        execution_metadata = input.execution_metadata or {}
        if not execution_metadata.get("worker_info"):
            execution_metadata["worker_info"] = worker_info
        update_payload["execution_metadata"] = execution_metadata

        # Call Control Plane API
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.patch(
                f"{control_plane_url}/api/v1/executions/{input.execution_id}",
                json=update_payload,
                headers={
                    "Authorization": f"Bearer {kubiya_api_key}",
                    "Content-Type": "application/json",
                }
            )

            if response.status_code == 404:
                raise Exception(f"Execution not found: {input.execution_id}")
            elif response.status_code != 200:
                raise Exception(f"Failed to update execution: {response.status_code} - {response.text}")

        print(f"✅ Status updated successfully: {input.status}\n")

        activity.logger.info(
            "execution_status_updated",
            extra={
                "execution_id": input.execution_id,
                "status": input.status,
            }
        )

        return {"success": True}

    except Exception as e:
        print(f"❌ Failed to update status: {str(e)}\n")

        activity.logger.error(
            "failed_to_update_execution_status",
            extra={
                "execution_id": input.execution_id,
                "error": str(e),
            }
        )
        raise


@activity.defn(name="update_agent_status")
async def update_agent_status(input: UpdateAgentStatusInput) -> dict:
    """
    Update agent status in database via Control Plane API.

    Args:
        input: UpdateAgentStatusInput with update details

    Returns:
        Dict with success flag
    """
    activity.logger.info(
        "updating_agent_status",
        extra={
            "agent_id": input.agent_id,
            "status": input.status,
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

        # Build update payload
        update_payload = {
            "status": input.status,
            "last_active_at": input.last_active_at,
        }

        if input.error_message is not None:
            update_payload["error_message"] = input.error_message

        if input.state:
            update_payload["state"] = input.state

        # Call Control Plane API
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.patch(
                f"{control_plane_url}/api/v1/agents/{input.agent_id}",
                json=update_payload,
                headers={
                    "Authorization": f"Bearer {kubiya_api_key}",
                    "Content-Type": "application/json",
                }
            )

            # For team executions, the "agent_id" is actually a team_id
            # This is expected and not an error
            if response.status_code == 404:
                activity.logger.info(
                    "agent_not_found_skipping",
                    extra={
                        "agent_id": input.agent_id,
                        "status": input.status,
                    }
                )
                return {"success": True, "skipped": True}
            elif response.status_code != 200:
                raise Exception(f"Failed to update agent: {response.status_code} - {response.text}")

        activity.logger.info(
            "agent_status_updated",
            extra={
                "agent_id": input.agent_id,
                "status": input.status,
            }
        )

        return {"success": True}

    except Exception as e:
        activity.logger.error(
            "failed_to_update_agent_status",
            extra={
                "agent_id": input.agent_id,
                "error": str(e),
            }
        )
        raise


@activity.defn(name="cancel_agent_run")
async def cancel_agent_run(input: CancelExecutionInput) -> dict:
    """
    Cancel an active agent run using Agno's cancel_run API.

    This is called when a user clicks the STOP button in the UI.

    Args:
        input: CancelExecutionInput with execution_id

    Returns:
        Dict with success status and cancellation details
    """
    print("\n" + "="*80)
    print("🛑 CANCEL AGENT RUN")
    print("="*80)
    print(f"Execution ID: {input.execution_id}\n")

    try:
        result = cancellation_manager.cancel(input.execution_id)

        if result["success"]:
            print(f"✅ Agent run cancelled successfully!\n")
        else:
            print(f"⚠️  Cancel failed: {result.get('error')}\n")

        return result

    except Exception as e:
        print(f"❌ Error cancelling run: {str(e)}\n")
        activity.logger.error(
            "cancel_agent_error",
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
