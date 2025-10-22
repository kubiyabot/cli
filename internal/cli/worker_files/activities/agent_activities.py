"""Agent-related Temporal activities"""

from dataclasses import dataclass
from typing import Optional, Any
from datetime import datetime, timezone
from temporalio import activity
import structlog
import os
import httpx
from pathlib import Path

from agno.tools.shell import ShellTools
from agno.tools.python import PythonTools
from agno.tools.file import FileTools

logger = structlog.get_logger()


def cache_execution_metadata(execution_id: str, execution_type: str):
    """
    Cache execution metadata in Redis for fast SSE endpoint lookups.

    This eliminates the need for DB queries on every SSE connection.

    Args:
        execution_id: The execution ID
        execution_type: "AGENT" or "TEAM"
    """
    control_plane_url = os.environ.get("CONTROL_PLANE_URL", "http://localhost:8000")
    api_key = os.environ.get("KUBIYA_API_KEY")

    if not api_key:
        logger.warning("redis_cache_skipped_no_api_key", execution_id=execution_id)
        return

    try:
        # Use the events endpoint to publish metadata - Control Plane will cache it
        url = f"{control_plane_url}/api/v1/executions/{execution_id}/events"
        payload = {
            "event_type": "metadata",
            "data": {
                "execution_type": execution_type,
            },
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

        with httpx.Client(timeout=httpx.Timeout(1.0, connect=0.5)) as client:
            response = client.post(
                url,
                json=payload,
                headers={"Authorization": f"UserKey {api_key}"}
            )

            if response.status_code not in (200, 202):
                logger.warning(
                    "redis_cache_failed",
                    status=response.status_code,
                    execution_id=execution_id[:8],
                )

    except Exception as e:
        # Never fail the activity if caching fails
        logger.warning("redis_cache_error", error=str(e), execution_id=execution_id[:8])


def publish_streaming_event(execution_id: str, event_type: str, data: dict):
    """
    Publish a streaming event to Control Plane for real-time UI updates.

    Args:
        execution_id: The execution ID
        event_type: Type of event (tool_started, tool_completed, message, etc.)
        data: Event payload
    """
    control_plane_url = os.environ.get("CONTROL_PLANE_URL", "http://localhost:8000")
    api_key = os.environ.get("KUBIYA_API_KEY")

    if not api_key:
        logger.warning("streaming_event_skipped_no_api_key", execution_id=execution_id)
        return

    try:
        url = f"{control_plane_url}/api/v1/executions/{execution_id}/events"
        payload = {
            "event_type": event_type,
            "data": data,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }

        # Fast HTTP POST with short timeout (streaming must be fast)
        # Use a connection pool for better performance
        with httpx.Client(timeout=httpx.Timeout(1.0, connect=0.5)) as client:
            response = client.post(
                url,
                json=payload,
                headers={"Authorization": f"UserKey {api_key}"}
            )

            if response.status_code not in (200, 202):
                logger.warning(
                    "streaming_event_publish_failed",
                    status=response.status_code,
                    execution_id=execution_id[:8],
                    event_type=event_type,
                )

    except Exception as e:
        # Never fail the activity if streaming fails
        logger.warning("streaming_event_error", error=str(e), execution_id=execution_id[:8])


def instantiate_toolset(toolset_data: dict) -> Optional[Any]:
    """
    Instantiate an Agno toolkit based on toolset configuration from Control Plane.

    Args:
        toolset_data: Toolset data from Control Plane API containing:
            - type: Toolset type (file_system, shell, python, docker, etc.)
            - name: Toolset name
            - configuration: Dict with toolset-specific config
            - enabled: Whether toolset is enabled

    Returns:
        Instantiated Agno toolkit or None if type not supported/enabled
    """
    if not toolset_data.get("enabled", True):
        print(f"   ⊗ Skipping disabled toolset: {toolset_data.get('name')}")
        return None

    toolset_type = toolset_data.get("type", "").lower()
    config = toolset_data.get("configuration", {})
    name = toolset_data.get("name", "Unknown")

    try:
        # Map Control Plane toolset types to Agno toolkit classes
        if toolset_type in ["file_system", "file", "file_generation"]:
            # FileTools: file operations (read, write, list, search)
            # Note: file_generation is mapped to FileTools (save_file functionality)
            base_dir = config.get("base_dir")
            toolkit = FileTools(
                base_dir=Path(base_dir) if base_dir else None,
                enable_save_file=config.get("enable_save_file", True),
                enable_read_file=config.get("enable_read_file", True),
                enable_list_files=config.get("enable_list_files", True),
                enable_search_files=config.get("enable_search_files", True),
            )
            print(f"   ✓ Instantiated FileTools: {name}")
            if toolset_type == "file_generation":
                print(f"     - Type: File Generation (using FileTools.save_file)")
            print(f"     - Base Dir: {base_dir or 'Current directory'}")
            print(f"     - Read: {config.get('enable_read_file', True)}, Write: {config.get('enable_save_file', True)}")
            return toolkit

        elif toolset_type in ["shell", "bash"]:
            # ShellTools: shell command execution
            base_dir = config.get("base_dir")
            toolkit = ShellTools(
                base_dir=Path(base_dir) if base_dir else None,
                enable_run_shell_command=config.get("enable_run_shell_command", True),
            )
            print(f"   ✓ Instantiated ShellTools: {name}")
            print(f"     - Base Dir: {base_dir or 'Current directory'}")
            print(f"     - Run Commands: {config.get('enable_run_shell_command', True)}")
            return toolkit

        elif toolset_type == "python":
            # PythonTools: Python code execution
            base_dir = config.get("base_dir")
            toolkit = PythonTools(
                base_dir=Path(base_dir) if base_dir else None,
                safe_globals=config.get("safe_globals"),
                safe_locals=config.get("safe_locals"),
            )
            print(f"   ✓ Instantiated PythonTools: {name}")
            print(f"     - Base Dir: {base_dir or 'Current directory'}")
            return toolkit

        elif toolset_type == "docker":
            # DockerTools requires docker package and running Docker daemon
            try:
                from agno.tools.docker import DockerTools
                import docker

                # Check if Docker daemon is accessible
                try:
                    docker_client = docker.from_env()
                    docker_client.ping()

                    # Docker is available, instantiate toolkit
                    toolkit = DockerTools()
                    print(f"   ✓ Instantiated DockerTools: {name}")
                    print(f"     - Docker daemon: Connected")
                    docker_client.close()
                    return toolkit

                except Exception as docker_error:
                    print(f"   ⚠ Docker daemon not available - skipping: {name}")
                    print(f"     Error: {str(docker_error)}")
                    return None

            except ImportError:
                print(f"   ⚠ Docker toolset requires 'docker' package - skipping: {name}")
                print(f"     Install with: pip install docker")
                return None

        else:
            print(f"   ⚠ Unsupported toolset type '{toolset_type}': {name}")
            return None

    except Exception as e:
        print(f"   ❌ Error instantiating toolset '{name}' (type: {toolset_type}): {str(e)}")
        logger.error(
            f"Error instantiating toolset",
            extra={
                "toolset_name": name,
                "toolset_type": toolset_type,
                "error": str(e)
            }
        )
        return None


@dataclass
class ActivityExecuteAgentInput:
    """Input for execute_agent_llm activity"""
    execution_id: str
    agent_id: str
    organization_id: str
    prompt: str
    system_prompt: Optional[str] = None
    model_id: Optional[str] = None
    model_config: dict = None
    mcp_servers: dict = None  # MCP servers configuration
    session_id: Optional[str] = None  # Session ID for Agno session management (use execution_id)
    user_id: Optional[str] = None  # User ID for multi-user support
    # Note: control_plane_url and api_key are read from worker environment variables (CONTROL_PLANE_URL, KUBIYA_API_KEY)

    def __post_init__(self):
        if self.model_config is None:
            self.model_config = {}
        if self.mcp_servers is None:
            self.mcp_servers = {}


@dataclass
class ActivityUpdateExecutionInput:
    """Input for update_execution_status activity"""
    execution_id: str
    status: str
    started_at: Optional[str] = None
    completed_at: Optional[str] = None
    response: Optional[str] = None
    error_message: Optional[str] = None
    usage: dict = None
    execution_metadata: dict = None

    def __post_init__(self):
        if self.usage is None:
            self.usage = {}
        if self.execution_metadata is None:
            self.execution_metadata = {}


@dataclass
class ActivityUpdateAgentInput:
    """Input for update_agent_status activity"""
    agent_id: str
    organization_id: str
    status: str
    last_active_at: str
    error_message: Optional[str] = None
    state: dict = None

    def __post_init__(self):
        if self.state is None:
            self.state = {}


@activity.defn
async def execute_agent_llm(input: ActivityExecuteAgentInput) -> dict:
    """
    Execute an agent's LLM call with Agno Teams and session management.

    This activity uses Agno Teams with session support for persistent conversation history.
    The session_id should be set to execution_id for 1:1 mapping.

    Args:
        input: Activity input with execution details

    Returns:
        Dict with response, usage, success flag, session messages, etc.
    """
    print("\n" + "="*80)
    print("🤖 AGENT EXECUTION START")
    print("="*80)
    print(f"Execution ID: {input.execution_id}")
    print(f"Agent ID: {input.agent_id}")
    print(f"Organization: {input.organization_id}")
    print(f"Model: {input.model_id or 'default'}")
    print(f"Session ID: {input.session_id}")
    print(f"MCP Servers: {len(input.mcp_servers)} configured" if input.mcp_servers else "MCP Servers: None")
    print(f"Prompt: {input.prompt[:100]}..." if len(input.prompt) > 100 else f"Prompt: {input.prompt}")
    print("="*80 + "\n")

    activity.logger.info(
        f"Executing agent LLM call with Agno Sessions",
        extra={
            "execution_id": input.execution_id,
            "agent_id": input.agent_id,
            "organization_id": input.organization_id,
            "model_id": input.model_id,
            "has_mcp_servers": bool(input.mcp_servers),
            "mcp_server_count": len(input.mcp_servers) if input.mcp_servers else 0,
            "mcp_server_ids": list(input.mcp_servers.keys()) if input.mcp_servers else [],
            "session_id": input.session_id,
        }
    )

    try:
        # Get LiteLLM credentials from environment (set by worker from registration)
        litellm_api_base = os.getenv("LITELLM_API_BASE", "https://llm-proxy.kubiya.ai")
        litellm_api_key = os.getenv("LITELLM_API_KEY")

        if not litellm_api_key:
            raise ValueError("LITELLM_API_KEY environment variable not set")

        # Get model from input or use default
        model = input.model_id or os.environ.get("LITELLM_DEFAULT_MODEL", "kubiya/claude-sonnet-4")

        # Get Control Plane URL and API key from environment (worker has these set on startup)
        control_plane_url = os.getenv("CONTROL_PLANE_URL")
        api_key = os.getenv("KUBIYA_API_KEY")

        # Fetch resolved toolsets from Control Plane if available
        toolsets = []
        if control_plane_url and api_key and input.agent_id:
            print(f"🔧 Fetching toolsets from Control Plane...")
            try:
                async with httpx.AsyncClient(timeout=30.0) as client:
                    response = await client.get(
                        f"{control_plane_url}/api/v1/toolsets/associations/agents/{input.agent_id}/toolsets/resolved",
                        headers={"Authorization": f"Bearer {api_key}"}
                    )

                    if response.status_code == 200:
                        toolsets = response.json()
                        print(f"✅ Resolved {len(toolsets)} toolsets from Control Plane")
                        print(f"   Toolset Types: {[t.get('type') for t in toolsets]}")
                        print(f"   Toolset Sources: {[t.get('source') for t in toolsets]}")
                        print(f"   Toolset Names: {[t.get('name') for t in toolsets]}\n")

                        activity.logger.info(
                            f"Resolved toolsets from Control Plane",
                            extra={
                                "agent_id": input.agent_id,
                                "toolset_count": len(toolsets),
                                "toolset_types": [t.get("type") for t in toolsets],
                                "toolset_sources": [t.get("source") for t in toolsets],
                                "toolset_names": [t.get("name") for t in toolsets],
                            }
                        )
                    else:
                        print(f"⚠️  Failed to fetch toolsets: HTTP {response.status_code}")
                        print(f"   Response: {response.text[:200]}\n")
                        activity.logger.warning(
                            f"Failed to fetch toolsets from Control Plane: {response.status_code}",
                            extra={
                                "status_code": response.status_code,
                                "response_text": response.text[:500]
                            }
                        )
            except Exception as e:
                print(f"❌ Error fetching toolsets: {str(e)}\n")
                activity.logger.error(
                    f"Error fetching toolsets from Control Plane: {str(e)}",
                    extra={"error": str(e)}
                )
                # Continue execution without toolsets
        else:
            print(f"ℹ️  No Control Plane URL/API key in environment - skipping toolset resolution\n")

        # Instantiate Agno toolkits from Control Plane toolsets
        print(f"\n🔧 Instantiating Toolsets:")
        agno_toolkits = []
        if toolsets:
            for toolset in toolsets:
                toolkit = instantiate_toolset(toolset)
                if toolkit:
                    agno_toolkits.append(toolkit)

        if agno_toolkits:
            print(f"\n✅ Successfully instantiated {len(agno_toolkits)} toolset(s)")
        else:
            print(f"\nℹ️  No toolsets instantiated\n")

        print(f"📦 Total Tools Available:")
        print(f"   MCP Servers: {len(input.mcp_servers)}")
        print(f"   OS-Level Toolsets: {len(agno_toolkits)}\n")

        activity.logger.info(
            f"Using Agno Agent with sessions and toolsets",
            extra={
                "execution_id": input.execution_id,
                "session_id": input.session_id,
                "has_mcp_servers": bool(input.mcp_servers),
                "mcp_server_count": len(input.mcp_servers) if input.mcp_servers else 0,
                "mcp_servers": list(input.mcp_servers.keys()) if input.mcp_servers else [],
                "toolset_count": len(agno_toolkits),
                "model": model,
            }
        )

        # Import Agno libraries
        from agno.agent import Agent
        from agno.models.litellm import LiteLLM

        print(f"\n🤖 Creating Agno Agent:")
        print(f"   Model: {model}")
        print(f"   Toolsets: {len(agno_toolkits)}")

        # Send heartbeat: Creating agent
        activity.heartbeat({"status": "Creating agent with toolsets..."})

        # Track tool executions for real-time streaming
        tool_execution_messages = []

        # Create tool hook to capture tool execution for real-time streaming
        # Agno inspects the signature and passes matching parameters
        def tool_hook(name: str = None, function_name: str = None, function=None, arguments: dict = None, **kwargs):
            """Hook to capture tool execution and add to messages for streaming

            Agno passes these parameters based on our signature:
            - name or function_name: The tool function name
            - function: The callable being executed (this is the NEXT function in the chain)
            - arguments: Dict of arguments passed to the tool

            The hook must CALL the function and return its result.
            """
            # Get tool name from Agno's parameters
            tool_name = name or function_name or "unknown"
            tool_args = arguments or {}

            # Generate unique tool execution ID (tool_name + timestamp)
            import time
            tool_execution_id = f"{tool_name}_{int(time.time() * 1000000)}"

            print(f"   🔧 Tool Starting: {tool_name} (ID: {tool_execution_id})")
            if tool_args:
                args_preview = str(tool_args)[:200]
                print(f"      Args: {args_preview}{'...' if len(str(tool_args)) > 200 else ''}")

            # Publish streaming event to Control Plane (real-time UI update)
            publish_streaming_event(
                execution_id=input.execution_id,
                event_type="tool_started",
                data={
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,  # Unique ID for this execution
                    "tool_arguments": tool_args,
                    "message": f"🔧 Executing tool: {tool_name}",
                }
            )

            tool_execution_messages.append({
                "role": "system",
                "content": f"🔧 Executing tool: **{tool_name}**",
                "tool_name": tool_name,
                "tool_event": "started",
                "timestamp": datetime.now(timezone.utc).isoformat(),
            })

            # CRITICAL: Actually call the function and handle completion
            result = None
            error = None
            try:
                # Call the actual function (next in the hook chain)
                if function and callable(function):
                    result = function(**tool_args) if tool_args else function()
                else:
                    raise ValueError(f"Function not callable: {function}")

                status = "success"
                icon = "✅"
                print(f"   {icon} Tool Success: {tool_name}")

            except Exception as e:
                error = e
                status = "failed"
                icon = "❌"
                print(f"   {icon} Tool Failed: {tool_name} - {str(e)}")

            # Publish completion event to Control Plane (real-time UI update)
            publish_streaming_event(
                execution_id=input.execution_id,
                event_type="tool_completed",
                data={
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,  # Same ID to match the started event
                    "status": status,
                    "error": str(error) if error else None,
                    "message": f"{icon} Tool {status}: {tool_name}",
                }
            )

            tool_execution_messages.append({
                "role": "system",
                "content": f"{icon} Tool {status}: **{tool_name}**",
                "tool_name": tool_name,
                "tool_event": "completed",
                "tool_status": status,
                "timestamp": datetime.now(timezone.utc).isoformat(),
            })

            # If there was an error, re-raise it so Agno knows the tool failed
            if error:
                raise error

            # Return the result to continue the chain
            return result

        # Create Agno Agent with LiteLLM configuration
        # Use openai/ prefix for custom proxy compatibility
        agent = Agent(
            name=f"Agent {input.agent_id}",
            role=input.system_prompt or "You are a helpful AI assistant",
            model=LiteLLM(
                id=f"openai/{model}",
                api_base=litellm_api_base,
                api_key=litellm_api_key,
            ),
            tools=agno_toolkits if agno_toolkits else None,  # Add toolsets to agent
            tool_hooks=[tool_hook],  # Add hook for real-time tool updates
        )

        # Cache execution metadata in Redis for fast SSE lookups (avoid DB queries)
        cache_execution_metadata(input.execution_id, "AGENT")

        # Execute agent run with streaming
        print("⚡ Executing Agent Run with Streaming...\n")

        # Send heartbeat: Starting execution
        activity.heartbeat({"status": "Agent is processing your request..."})

        import asyncio

        # Stream the response and collect chunks
        response_chunks = []
        full_response = ""

        # Generate unique message ID for this turn (execution_id + timestamp)
        import time
        message_id = f"{input.execution_id}_{int(time.time() * 1000000)}"

        def stream_agent_run():
            """Run agent with streaming and collect response"""
            nonlocal full_response
            try:
                # Run with streaming enabled
                run_response = agent.run(input.prompt, stream=True)

                # Iterate over streaming chunks
                for chunk in run_response:
                    if hasattr(chunk, 'content') and chunk.content:
                        content = str(chunk.content)
                        full_response += content
                        response_chunks.append(content)
                        print(content, end='', flush=True)

                        # Stream chunk to Control Plane for real-time UI updates
                        # Include message_id so UI knows which message these chunks belong to
                        publish_streaming_event(
                            execution_id=input.execution_id,
                            event_type="message_chunk",
                            data={
                                "role": "assistant",
                                "content": content,
                                "is_chunk": True,
                                "message_id": message_id,  # Unique ID for this turn
                            }
                        )

                        # Note: Cannot send heartbeat from sync context (thread pool)

                print()  # New line after streaming

                # Return the iterator's final result
                return run_response
            except Exception as e:
                print(f"\n❌ Streaming error: {str(e)}")
                # Fall back to non-streaming
                return agent.run(input.prompt, stream=False)

        # Execute in thread pool
        result = await asyncio.to_thread(stream_agent_run)

        # Send heartbeat: Completed
        activity.heartbeat({"status": "Agent execution completed, preparing response..."})

        print("✅ Agent Execution Completed!")
        print(f"   Response Length: {len(full_response)} chars\n")

        activity.logger.info(
            f"Agent LLM call completed",
            extra={
                "execution_id": input.execution_id,
                "has_content": bool(full_response),
            }
        )

        # Use the streamed response content
        response_content = full_response if full_response else (result.content if hasattr(result, "content") else str(result))

        # Extract tool call messages for UI streaming
        tool_messages = []
        if hasattr(result, "messages") and result.messages:
            for msg in result.messages:
                # Check if message has tool calls
                if hasattr(msg, "tool_calls") and msg.tool_calls:
                    for tool_call in msg.tool_calls:
                        tool_name = getattr(tool_call, "function", {}).get("name") if hasattr(tool_call, "function") else str(tool_call)
                        tool_args = getattr(tool_call, "function", {}).get("arguments") if hasattr(tool_call, "function") else {}

                        print(f"   🔧 Tool Call: {tool_name}")

                        tool_messages.append({
                            "role": "tool",
                            "content": f"Executing {tool_name}...",
                            "tool_name": tool_name,
                            "tool_input": tool_args,
                            "timestamp": datetime.now(timezone.utc).isoformat(),
                        })

        if tool_messages:
            print(f"\n🔧 Tool Calls Captured: {len(tool_messages)}")

        # Extract usage metrics if available
        usage = {}
        if hasattr(result, "metrics") and result.metrics:
            metrics = result.metrics
            usage = {
                "prompt_tokens": getattr(metrics, "input_tokens", 0),
                "completion_tokens": getattr(metrics, "output_tokens", 0),
                "total_tokens": getattr(metrics, "total_tokens", 0),
            }
            print(f"📊 Token Usage:")
            print(f"   Input Tokens: {usage.get('prompt_tokens', 0)}")
            print(f"   Output Tokens: {usage.get('completion_tokens', 0)}")
            print(f"   Total Tokens: {usage.get('total_tokens', 0)}\n")

        print(f"📝 Response Preview:")
        print(f"   {response_content[:200]}..." if len(response_content) > 200 else f"   {response_content}")


        print("\n" + "="*80)
        print("🏁 AGENT EXECUTION END")
        print("="*80 + "\n")

        return {
            "success": True,
            "response": response_content,
            "usage": usage,
            "model": model,
            "finish_reason": "stop",
            "mcp_tools_used": 0,  # TODO: Track MCP tool usage
            "tool_messages": tool_messages,  # Include tool call messages for UI
            "tool_execution_messages": tool_execution_messages,  # Include real-time tool execution status
        }

    except Exception as e:
        print("\n" + "="*80)
        print("❌ AGENT EXECUTION FAILED")
        print("="*80)
        print(f"Error: {str(e)}")
        print("="*80 + "\n")

        activity.logger.error(
            f"Agent LLM call failed",
            extra={
                "execution_id": input.execution_id,
                "error": str(e),
            }
        )
        return {
            "success": False,
            "error": str(e),
            "model": input.model_id,
            "usage": None,
            "finish_reason": "error",
        }


@activity.defn
async def update_execution_status(input: ActivityUpdateExecutionInput) -> dict:
    """
    Update execution status in database via Control Plane API.

    This activity calls the Control Plane API to update execution records.
    Also records which worker processed this execution.

    Args:
        input: Activity input with update details

    Returns:
        Dict with success flag
    """
    print(f"🔄 Updating execution status: {input.status} (execution_id: {input.execution_id[:8]}...)")

    activity.logger.info(
        f"Updating execution status via Control Plane API",
        extra={
            "execution_id": input.execution_id,
            "status": input.status,
        }
    )

    try:
        # Get Control Plane URL and Kubiya API key from environment
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
            f"Execution status updated via API",
            extra={
                "execution_id": input.execution_id,
                "status": input.status,
            }
        )

        return {"success": True}

    except Exception as e:
        print(f"❌ Failed to update status: {str(e)}\n")

        activity.logger.error(
            f"Failed to update execution status",
            extra={
                "execution_id": input.execution_id,
                "error": str(e),
            }
        )
        raise


@activity.defn
async def update_agent_status(input: ActivityUpdateAgentInput) -> dict:
    """
    Update agent status in database via Control Plane API.

    This activity calls the Control Plane API to update agent records.

    Args:
        input: Activity input with update details

    Returns:
        Dict with success flag
    """
    activity.logger.info(
        f"Updating agent status via Control Plane API",
        extra={
            "agent_id": input.agent_id,
            "status": input.status,
        }
    )

    try:
        # Get Control Plane URL and Kubiya API key from environment
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

            # For team executions, the "agent_id" is actually a team_id, so it won't be found in agents table
            # This is expected and not an error - just log and return success
            if response.status_code == 404:
                activity.logger.info(
                    f"Agent not found (likely a team execution) - skipping agent status update",
                    extra={
                        "agent_id": input.agent_id,
                        "status": input.status,
                    }
                )
                return {"success": True, "skipped": True}
            elif response.status_code != 200:
                raise Exception(f"Failed to update agent: {response.status_code} - {response.text}")

        activity.logger.info(
            f"Agent status updated via API",
            extra={
                "agent_id": input.agent_id,
                "status": input.status,
            }
        )

        return {"success": True}

    except Exception as e:
        activity.logger.error(
            f"Failed to update agent status",
            extra={
                "agent_id": input.agent_id,
                "error": str(e),
            }
        )
        raise
