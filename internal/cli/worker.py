"""
Temporal worker for Agent Control Plane - Decoupled Architecture.

This worker:
1. Registers with Control Plane API on startup using KUBIYA_API_KEY
2. Gets dynamic configuration (Temporal credentials, task queue name, etc.)
3. Connects to Temporal Cloud with provided credentials
4. Sends periodic heartbeats to Control Plane
5. Has NO direct database access - all state managed via Control Plane API

Environment variables REQUIRED:
- KUBIYA_API_KEY: Kubiya API key for authentication (required)
- CONTROL_PLANE_URL: Control Plane API URL (e.g., https://control-plane.kubiya.ai)
- ENVIRONMENT_NAME: Environment/task queue name to join (default: "default")

Environment variables OPTIONAL:
- WORKER_HOSTNAME: Custom hostname for worker (default: auto-detected)
- HEARTBEAT_INTERVAL: Seconds between heartbeats (default: 30)
"""

import asyncio
import os
import sys
from pathlib import Path
import structlog
import httpx
import socket
from dataclasses import dataclass
from typing import Optional
from temporalio.worker import Worker
from temporalio.client import Client, TLSConfig

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

from app.workflows.agent_execution import AgentExecutionWorkflow
from app.workflows.team_execution import TeamExecutionWorkflow
from app.activities.agent_activities import (
    execute_agent_llm,
    update_execution_status,
    update_agent_status,
)
from app.activities.team_activities import (
    get_team_agents,
    execute_team_coordination,
)

# Configure structured logging
import logging

structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(logging.INFO),
    logger_factory=structlog.PrintLoggerFactory(),
)

logger = structlog.get_logger()


@dataclass
class WorkerConfig:
    """Configuration received from Control Plane registration"""
    worker_id: str
    worker_token: str
    environment_name: str  # Task queue name (org_id.environment)
    temporal_namespace: str
    temporal_host: str
    temporal_api_key: str
    organization_id: str
    control_plane_url: str


async def register_with_control_plane(
    control_plane_url: str,
    kubiya_api_key: str,
    environment_name: str,
    hostname: Optional[str] = None
) -> WorkerConfig:
    """
    Register worker with Control Plane and get configuration.

    Args:
        control_plane_url: Control Plane API URL
        kubiya_api_key: Kubiya API key for authentication
        environment_name: Environment/task queue to join
        hostname: Worker hostname (auto-detected if not provided)

    Returns:
        WorkerConfig with all necessary configuration

    Raises:
        Exception if registration fails
    """
    if not hostname:
        hostname = socket.gethostname()

    registration_data = {
        "environment_name": environment_name,
        "hostname": hostname,
        "worker_metadata": {
            "python_version": sys.version,
            "platform": sys.platform,
        }
    }

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.post(
                f"{control_plane_url}/api/v1/workers/register",
                json=registration_data,
                headers={"Authorization": f"Bearer {kubiya_api_key}"}
            )

            if response.status_code != 200:
                logger.error(
                    "worker_registration_failed",
                    status_code=response.status_code,
                    response=response.text[:500]
                )
                raise Exception(
                    f"Failed to register with Control Plane: {response.status_code} - {response.text[:200]}"
                )

            data = response.json()
            logger.info(
                "worker_registered_successfully",
                worker_id=data.get("worker_id"),
                environment_name=data.get("environment_name"),
                org_id=data.get("organization_id"),
            )

            return WorkerConfig(
                worker_id=data["worker_id"],
                worker_token=data["worker_token"],
                environment_name=data["environment_name"],
                temporal_namespace=data["temporal_namespace"],
                temporal_host=data["temporal_host"],
                temporal_api_key=data["temporal_api_key"],
                organization_id=data["organization_id"],
                control_plane_url=data["control_plane_url"],
            )

    except httpx.RequestError as e:
        logger.error("control_plane_connection_failed", error=str(e))
        raise Exception(f"Failed to connect to Control Plane: {e}")


async def send_heartbeat(
    config: WorkerConfig,
    kubiya_api_key: str,
    status: str = "active",
    tasks_processed: int = 0,
    current_task_id: Optional[str] = None
) -> bool:
    """
    Send heartbeat to Control Plane.

    Args:
        config: Worker configuration
        kubiya_api_key: Kubiya API key for authentication
        status: Worker status (active, idle, busy)
        tasks_processed: Number of tasks processed
        current_task_id: Currently executing task ID

    Returns:
        True if successful, False otherwise
    """
    heartbeat_data = {
        "worker_id": config.worker_id,
        "environment_name": config.environment_name.split(".")[-1],  # Extract environment name
        "status": status,
        "tasks_processed": tasks_processed,
        "current_task_id": current_task_id,
        "worker_metadata": {}
    }

    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.post(
                f"{config.control_plane_url}/api/v1/workers/heartbeat",
                json=heartbeat_data,
                headers={"Authorization": f"Bearer {kubiya_api_key}"}
            )

            if response.status_code in [200, 204]:
                logger.debug("heartbeat_sent", worker_id=config.worker_id)
                return True
            else:
                logger.warning(
                    "heartbeat_failed",
                    status_code=response.status_code,
                    response=response.text[:200]
                )
                return False

    except Exception as e:
        logger.warning("heartbeat_error", error=str(e))
        return False


async def create_temporal_client(config: WorkerConfig) -> Client:
    """
    Create Temporal client using configuration from Control Plane.

    Args:
        config: Worker configuration from Control Plane registration

    Returns:
        Connected Temporal client instance
    """
    logger.info(
        "connecting_to_temporal",
        host=config.temporal_host,
        namespace=config.temporal_namespace,
    )

    try:
        # Connect to Temporal Cloud with API key
        client = await Client.connect(
            config.temporal_host,
            namespace=config.temporal_namespace,
            tls=TLSConfig(),  # TLS enabled
            rpc_metadata={"authorization": f"Bearer {config.temporal_api_key}"}
        )

        logger.info(
            "temporal_client_connected",
            host=config.temporal_host,
            namespace=config.temporal_namespace,
        )

        return client

    except Exception as e:
        logger.error("temporal_client_connection_failed", error=str(e))
        raise


async def heartbeat_loop(config: WorkerConfig, kubiya_api_key: str, interval: int = 30):
    """
    Background task to send periodic heartbeats to Control Plane.

    Args:
        config: Worker configuration
        kubiya_api_key: Kubiya API key for authentication
        interval: Seconds between heartbeats
    """
    tasks_processed = 0

    while True:
        try:
            await asyncio.sleep(interval)
            await send_heartbeat(
                config=config,
                kubiya_api_key=kubiya_api_key,
                status="active",
                tasks_processed=tasks_processed
            )
        except asyncio.CancelledError:
            logger.info("heartbeat_loop_cancelled")
            break
        except Exception as e:
            logger.warning("heartbeat_loop_error", error=str(e))


async def run_worker():
    """
    Run the Temporal worker with decoupled architecture.

    The worker:
    1. Registers with Control Plane API
    2. Gets dynamic configuration (Temporal credentials, task queue, etc.)
    3. Connects to Temporal Cloud
    4. Starts heartbeat loop
    5. Registers workflows and activities
    6. Polls for tasks and executes them
    """
    # Get configuration from environment
    kubiya_api_key = os.environ.get("KUBIYA_API_KEY")
    control_plane_url = os.environ.get("CONTROL_PLANE_URL")
    environment_name = os.environ.get("ENVIRONMENT_NAME", "default")
    worker_hostname = os.environ.get("WORKER_HOSTNAME")
    heartbeat_interval = int(os.environ.get("HEARTBEAT_INTERVAL", "30"))

    # Validate required configuration
    if not kubiya_api_key:
        logger.error(
            "configuration_error",
            message="KUBIYA_API_KEY environment variable is required"
        )
        sys.exit(1)

    if not control_plane_url:
        logger.error(
            "configuration_error",
            message="CONTROL_PLANE_URL environment variable is required"
        )
        sys.exit(1)

    logger.info(
        "worker_starting",
        control_plane_url=control_plane_url,
        environment_name=environment_name,
    )

    try:
        # Register with Control Plane
        logger.info("registering_with_control_plane")
        config = await register_with_control_plane(
            control_plane_url=control_plane_url,
            kubiya_api_key=kubiya_api_key,
            environment_name=environment_name,
            hostname=worker_hostname
        )

        logger.info(
            "worker_configured",
            worker_id=config.worker_id,
            task_queue=config.environment_name,
            temporal_namespace=config.temporal_namespace,
            organization_id=config.organization_id,
        )

        # Create Temporal client
        client = await create_temporal_client(config)

        # Start heartbeat loop in background
        heartbeat_task = asyncio.create_task(
            heartbeat_loop(config, kubiya_api_key, heartbeat_interval)
        )

        # Create worker
        worker = Worker(
            client,
            task_queue=config.environment_name,
            workflows=[
                AgentExecutionWorkflow,
                TeamExecutionWorkflow,
            ],
            activities=[
                execute_agent_llm,
                update_execution_status,
                update_agent_status,
                get_team_agents,
                execute_team_coordination,
            ],
            max_concurrent_activities=10,
            max_concurrent_workflow_tasks=10,
        )

        logger.info(
            "temporal_worker_started",
            worker_id=config.worker_id,
            task_queue=config.environment_name,
            workflows=["AgentExecutionWorkflow", "TeamExecutionWorkflow"],
            activities=5,
        )

        # Run worker (blocks until interrupted)
        await worker.run()

        # Cancel heartbeat task when worker stops
        heartbeat_task.cancel()
        try:
            await heartbeat_task
        except asyncio.CancelledError:
            pass

    except KeyboardInterrupt:
        logger.info("temporal_worker_stopping", reason="keyboard_interrupt")
    except Exception as e:
        import traceback
        logger.error("temporal_worker_error", error=str(e), traceback=traceback.format_exc())
        raise


def main():
    """Main entry point"""
    logger.info("worker_starting")

    try:
        asyncio.run(run_worker())
    except KeyboardInterrupt:
        logger.info("worker_stopped")
    except Exception as e:
        logger.error("worker_failed", error=str(e))
        sys.exit(1)


if __name__ == "__main__":
    main()
