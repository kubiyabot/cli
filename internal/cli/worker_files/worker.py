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
import structlog
import httpx
import socket
import platform
import psutil
import time
from dataclasses import dataclass
from typing import Optional, List
from temporalio.worker import Worker
from temporalio.client import Client, TLSConfig
from collections import deque

# Import workflows and activities from local package
from workflows.agent_execution import AgentExecutionWorkflow
from workflows.team_execution import TeamExecutionWorkflow
from activities.agent_activities import (
    execute_agent_llm,
    update_execution_status,
    update_agent_status,
)
from activities.team_activities import (
    get_team_agents,
    execute_team_coordination,
)

# Configure structured logging
import logging


def pretty_console_renderer(logger, name, event_dict):
    """
    Render logs in a pretty, human-readable format instead of JSON.
    Uses colors and emojis for better readability.
    """
    level = event_dict.get("level", "info").upper()
    event = event_dict.get("event", "")
    timestamp = event_dict.get("timestamp", "")

    # Extract timestamp (just time part)
    if timestamp:
        try:
            time_part = timestamp.split("T")[1].split(".")[0]  # HH:MM:SS
        except:
            time_part = timestamp
    else:
        time_part = time.strftime("%H:%M:%S")

    # Color codes
    RESET = "\033[0m"
    GRAY = "\033[90m"
    GREEN = "\033[92m"
    YELLOW = "\033[93m"
    RED = "\033[91m"
    CYAN = "\033[96m"
    BOLD = "\033[1m"

    # Level icons and colors
    level_config = {
        "INFO": ("ℹ️", CYAN),
        "WARNING": ("⚠️", YELLOW),
        "ERROR": ("❌", RED),
        "DEBUG": ("🔍", GRAY),
    }

    icon, color = level_config.get(level, ("•", RESET))

    # Format the main message
    message = f"{GRAY}[{time_part}]{RESET} {icon}  {event}"

    # Add relevant context (skip internal keys)
    skip_keys = {"level", "event", "timestamp", "logger"}
    context_parts = []

    for key, value in event_dict.items():
        if key in skip_keys:
            continue
        # Format value nicely
        if isinstance(value, bool):
            value_str = "✓" if value else "✗"
        elif isinstance(value, str) and len(value) > 60:
            value_str = value[:57] + "..."
        else:
            value_str = str(value)

        context_parts.append(f"{GRAY}{key}={RESET}{value_str}")

    if context_parts:
        message += f" {GRAY}({', '.join(context_parts)}){RESET}"

    return message


structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.TimeStamper(fmt="iso"),
        pretty_console_renderer,
    ],
    wrapper_class=structlog.make_filtering_bound_logger(logging.INFO),
    logger_factory=structlog.PrintLoggerFactory(),
)

logger = structlog.get_logger()

# Global log buffer to collect logs since last heartbeat
log_buffer = deque(maxlen=500)  # Keep last 500 log lines
worker_start_time = time.time()


class ProgressUI:
    """Minimal animated UI for worker startup - minikube style"""

    @staticmethod
    def step(emoji: str, message: str, status: str = ""):
        """Print a step with emoji and optional status"""
        if status:
            print(f"{emoji}  {message} {status}")
        else:
            print(f"{emoji}  {message}")

    @staticmethod
    def success(emoji: str, message: str):
        """Print success message"""
        GREEN = "\033[92m"
        RESET = "\033[0m"
        print(f"{GREEN}{emoji}  {message}{RESET}")

    @staticmethod
    def error(emoji: str, message: str):
        """Print error message"""
        RED = "\033[91m"
        RESET = "\033[0m"
        print(f"{RED}{emoji}  {message}{RESET}")

    @staticmethod
    def header(text: str):
        """Print section header"""
        CYAN = "\033[96m"
        BOLD = "\033[1m"
        RESET = "\033[0m"
        print(f"\n{CYAN}{BOLD}{text}{RESET}")

    @staticmethod
    def banner():
        """Print startup banner"""
        CYAN = "\033[96m"
        BOLD = "\033[1m"
        RESET = "\033[0m"
        print(f"\n{CYAN}{BOLD}🚀 Kubiya Agent Worker{RESET}\n")


def collect_system_info() -> dict:
    """
    Collect current system metrics and information.
    """
    try:
        cpu_percent = psutil.cpu_percent(interval=0.1)
        memory = psutil.virtual_memory()
        disk = psutil.disk_usage('/')

        # Get Kubiya CLI version from environment variable (set by CLI) - skipped for now
        cli_version = None

        # Check Docker availability
        docker_available = False
        docker_version = None
        try:
            import subprocess
            import shutil

            # First try to find docker in PATH using shutil.which
            docker_path = shutil.which('docker')
            logger.debug("docker_which_result", path=docker_path)

            # Fallback to common locations if not in PATH
            if not docker_path:
                docker_paths = [
                    '/usr/local/bin/docker',
                    '/usr/bin/docker',
                    '/opt/homebrew/bin/docker',
                ]
                for path in docker_paths:
                    logger.debug("docker_checking_path", path=path, exists=os.path.exists(path))
                    if os.path.exists(path):
                        docker_path = path
                        break

            if docker_path:
                logger.debug("docker_running_version_check", path=docker_path)
                result = subprocess.run(
                    [docker_path, '--version'],
                    capture_output=True,
                    text=True,
                    timeout=3,
                    shell=False
                )
                logger.debug(
                    "docker_version_output",
                    returncode=result.returncode,
                    stdout=result.stdout[:200],
                    stderr=result.stderr[:200] if result.stderr else None
                )
                if result.returncode == 0:
                    docker_available = True
                    # Parse "Docker version 28.1.1, build 4eba377"
                    output = result.stdout.strip()
                    if ',' in output:
                        docker_version = output.split(',')[0].replace('Docker version', '').strip()
                    else:
                        docker_version = output.replace('Docker version', '').strip()
                    logger.debug("docker_detected", version=docker_version, path=docker_path)
                else:
                    logger.warning("docker_version_check_failed", returncode=result.returncode)
            else:
                logger.warning("docker_not_found_in_path_or_common_locations")
        except Exception as e:
            # Log for debugging but don't fail
            logger.warning("docker_detection_failed", error=str(e), error_type=type(e).__name__)
            import traceback
            logger.debug("docker_detection_traceback", traceback=traceback.format_exc())

        # Parse OS details from platform
        os_name = platform.system()  # Darwin, Linux, Windows
        os_version = platform.release()

        return {
            "hostname": socket.gethostname(),
            "platform": platform.platform(),
            "os_name": os_name,
            "os_version": os_version,
            "python_version": platform.python_version(),
            "cli_version": cli_version,
            "docker_available": docker_available,
            "docker_version": docker_version,
            "cpu_count": psutil.cpu_count(),
            "cpu_percent": cpu_percent,
            "memory_total": memory.total,
            "memory_used": memory.used,
            "memory_percent": memory.percent,
            "disk_total": disk.total,
            "disk_used": disk.used,
            "disk_percent": disk.percent,
            "uptime_seconds": time.time() - worker_start_time,
        }
    except Exception as e:
        logger.warning("failed_to_collect_system_info", error=str(e))
        return {
            "hostname": socket.gethostname(),
            "platform": platform.platform(),
        }


def get_recent_logs() -> List[str]:
    """
    Get logs collected since last heartbeat and clear the buffer.
    """
    logs = list(log_buffer)
    log_buffer.clear()
    return logs


def log_to_buffer(message: str):
    """
    Add a log message to the buffer for sending in next heartbeat.
    """
    log_buffer.append(message)


@dataclass
class WorkerConfig:
    """Configuration received from Control Plane registration"""
    worker_id: str
    environment_name: str  # Task queue name (org_id.environment)
    temporal_namespace: str
    temporal_host: str
    temporal_api_key: str
    organization_id: str
    control_plane_url: str
    litellm_api_url: str = "https://llm-proxy.kubiya.ai"
    litellm_api_key: str = ""


async def start_worker_for_queue(
    control_plane_url: str,
    kubiya_api_key: str,
    queue_id: str,
) -> WorkerConfig:
    """
    Start a worker for a specific queue ID.

    Args:
        control_plane_url: Control Plane API URL
        kubiya_api_key: Kubiya API key for authentication
        queue_id: Worker queue ID (UUID)

    Returns:
        WorkerConfig with all necessary configuration

    Raises:
        Exception if start fails
    """
    logger.info(
        "starting_worker_for_queue",
        queue_id=queue_id,
        control_plane_url=control_plane_url,
    )

    try:
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                f"{control_plane_url}/api/v1/worker-queues/{queue_id}/start",
                headers={"Authorization": f"Bearer {kubiya_api_key}"}
            )

            # Success case
            if response.status_code == 200:
                data = response.json()

                ProgressUI.success("✓", f"Registered with control plane")
                logger.info(
                    "worker_registered",
                    worker_id=data.get("worker_id")[:8],
                    queue_name=data.get("queue_name"),
                )

                # The task_queue_name is now just the queue UUID
                return WorkerConfig(
                    worker_id=data["worker_id"],
                    environment_name=data["task_queue_name"],  # This is now the queue UUID
                    temporal_namespace=data["temporal_namespace"],
                    temporal_host=data["temporal_host"],
                    temporal_api_key=data["temporal_api_key"],
                    organization_id=data["organization_id"],
                    control_plane_url=data["control_plane_url"],
                    litellm_api_url=data.get("litellm_api_url", "https://llm-proxy.kubiya.ai"),
                    litellm_api_key=data.get("litellm_api_key", ""),
                )

            # Handle errors
            else:
                # Try to extract error detail from response
                error_message = response.text
                try:
                    error_data = response.json()
                    error_message = error_data.get("detail", response.text)
                except:
                    pass

                ProgressUI.error("✗", "Worker registration failed")
                print(f"   {error_message}\n")

                logger.error(
                    "worker_start_failed",
                    status_code=response.status_code,
                    queue_id=queue_id,
                )
                sys.exit(1)

    except httpx.RequestError as e:
        ProgressUI.error("✗", f"Connection failed: {control_plane_url}")
        print(f"   {str(e)}\n")
        logger.error("control_plane_connection_failed", error=str(e))
        sys.exit(1)


async def send_heartbeat(
    config: WorkerConfig,
    kubiya_api_key: str,
    status: str = "active",
    tasks_processed: int = 0,
    current_task_id: Optional[str] = None
) -> bool:
    """
    Send heartbeat to Control Plane with system info and logs.

    Args:
        config: Worker configuration
        kubiya_api_key: Kubiya API key for authentication
        status: Worker status (active, idle, busy)
        tasks_processed: Number of tasks processed
        current_task_id: Currently executing task ID

    Returns:
        True if successful, False otherwise
    """
    # Collect system info
    system_info = collect_system_info()

    # Get logs since last heartbeat
    logs = get_recent_logs()

    heartbeat_data = {
        "status": status,
        "tasks_processed": tasks_processed,
        "current_task_id": current_task_id,
        "worker_metadata": {},
        "system_info": system_info,
        "logs": logs if logs else None,
    }

    try:
        url = f"{config.control_plane_url}/api/v1/workers/{config.worker_id}/heartbeat"

        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.post(
                url,
                json=heartbeat_data,
                headers={"Authorization": f"Bearer {kubiya_api_key}"}
            )

            if response.status_code in [200, 204]:
                logger.debug("heartbeat_sent", worker_id=config.worker_id)
                log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Heartbeat sent successfully")
                return True
            else:
                logger.warning(
                    "heartbeat_failed",
                    status_code=response.status_code,
                    response=response.text[:200]
                )
                log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Heartbeat failed: HTTP {response.status_code}")
                return False

    except Exception as e:
        logger.warning("heartbeat_error", error=str(e))
        log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Heartbeat error: {str(e)[:100]}")
        return False


async def create_temporal_client(config: WorkerConfig) -> Client:
    """
    Create Temporal client using configuration from Control Plane.

    Args:
        config: Worker configuration from Control Plane registration

    Returns:
        Connected Temporal client instance
    """
    try:
        # Connect to Temporal Cloud with API key
        client = await Client.connect(
            config.temporal_host,
            namespace=config.temporal_namespace,
            tls=TLSConfig(),  # TLS enabled
            rpc_metadata={"authorization": f"Bearer {config.temporal_api_key}"}
        )

        return client

    except Exception as e:
        logger.error("connection_failed", error=str(e))
        ProgressUI.error("✗", f"Temporal connection failed: {str(e)}")
        raise


async def send_disconnect(
    config: WorkerConfig,
    kubiya_api_key: str,
    reason: str = "shutdown",
    exit_code: Optional[int] = None,
    error_message: Optional[str] = None
) -> bool:
    """
    Notify Control Plane that worker is disconnecting/exiting.

    Args:
        config: Worker configuration
        kubiya_api_key: Kubiya API key for authentication
        reason: Disconnect reason (shutdown, error, crash, etc.)
        exit_code: Exit code if applicable
        error_message: Error message if applicable

    Returns:
        True if successful, False otherwise
    """
    disconnect_data = {
        "reason": reason,
        "exit_code": exit_code,
        "error_message": error_message
    }

    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.post(
                f"{config.control_plane_url}/api/v1/workers/{config.worker_id}/disconnect",
                json=disconnect_data,
                headers={"Authorization": f"Bearer {kubiya_api_key}"}
            )

            if response.status_code in [200, 204]:
                logger.info(
                    "worker_disconnected",
                    worker_id=config.worker_id,
                    reason=reason,
                    exit_code=exit_code
                )
                return True
            else:
                logger.warning(
                    "disconnect_notification_failed",
                    status_code=response.status_code,
                    response=response.text[:200]
                )
                return False

    except Exception as e:
        logger.warning("disconnect_notification_error", error=str(e))
        return False


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
    queue_id = os.environ.get("QUEUE_ID")
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

    if not queue_id:
        logger.error(
            "configuration_error",
            message="QUEUE_ID environment variable is required"
        )
        sys.exit(1)

    log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Worker starting for queue {queue_id}")

    try:
        # Print banner
        ProgressUI.banner()

        # Step 1: Register with control plane
        ProgressUI.step("⏳", "Registering with control plane...")
        log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Registering with control plane...")
        config = await start_worker_for_queue(
            control_plane_url=control_plane_url,
            kubiya_api_key=kubiya_api_key,
            queue_id=queue_id,
        )
        log_to_buffer(f"[{time.strftime('%H:%M:%S')}] Worker registered: {config.worker_id}")

        # Set environment variables for activities to use
        os.environ["CONTROL_PLANE_URL"] = config.control_plane_url
        os.environ["KUBIYA_API_KEY"] = kubiya_api_key
        os.environ["WORKER_ID"] = config.worker_id
        os.environ["LITELLM_API_BASE"] = config.litellm_api_url
        os.environ["LITELLM_API_KEY"] = config.litellm_api_key

        # Step 2: Connect to Temporal
        ProgressUI.step("⏳", "Connecting to Temporal...")
        client = await create_temporal_client(config)
        ProgressUI.success("✓", "Connected to Temporal")

        # Step 3: Send initial heartbeat
        ProgressUI.step("⏳", "Sending heartbeat...")
        await send_heartbeat(
            config=config,
            kubiya_api_key=kubiya_api_key,
            status="active",
            tasks_processed=0
        )
        ProgressUI.success("✓", "Worker visible in UI")

        # Start heartbeat loop in background
        heartbeat_task = asyncio.create_task(
            heartbeat_loop(config, kubiya_api_key, heartbeat_interval)
        )

        # Step 4: Create worker
        ProgressUI.step("⏳", "Starting worker...")
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

        ProgressUI.success("✓", "Worker ready")
        ProgressUI.header("📡 Listening for tasks... (Ctrl+C to stop)")

        logger.info(
            "worker_ready",
            worker_id=config.worker_id[:8],
        )

        # Run worker (blocks until interrupted)
        await worker.run()

        # Cancel heartbeat task when worker stops
        heartbeat_task.cancel()
        try:
            await heartbeat_task
        except asyncio.CancelledError:
            pass

        # Notify control plane of graceful shutdown
        print()
        ProgressUI.step("⏳", "Shutting down gracefully...")
        await send_disconnect(
            config=config,
            kubiya_api_key=kubiya_api_key,
            reason="shutdown",
            exit_code=0
        )
        ProgressUI.success("✓", "Worker stopped")
        print()

    except KeyboardInterrupt:
        print()
        ProgressUI.step("⏳", "Shutting down...")
        # Notify control plane of keyboard interrupt
        try:
            await send_disconnect(
                config=config,
                kubiya_api_key=kubiya_api_key,
                reason="shutdown",
                exit_code=0
            )
            ProgressUI.success("✓", "Worker stopped")
        except Exception as e:
            logger.warning("disconnect_on_interrupt_failed", error=str(e))
    except Exception as e:
        import traceback
        logger.error("temporal_worker_error", error=str(e), traceback=traceback.format_exc())
        # Notify control plane of error
        try:
            await send_disconnect(
                config=config,
                kubiya_api_key=kubiya_api_key,
                reason="error",
                exit_code=1,
                error_message=str(e)[:500]
            )
        except Exception as disconnect_error:
            logger.warning("disconnect_on_error_failed", error=str(disconnect_error))
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
