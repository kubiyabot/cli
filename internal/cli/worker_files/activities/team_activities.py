"""Team-related Temporal activities"""

import os
from dataclasses import dataclass
from typing import Optional, List, Any
from datetime import datetime, timezone
from temporalio import activity
import structlog
import httpx
from pathlib import Path

from agno.agent import Agent
from agno.team import Team
from agno.models.litellm import LiteLLM
from agno.tools.shell import ShellTools
from agno.tools.python import PythonTools
from agno.tools.file import FileTools

from activities.agent_activities import update_execution_status, ActivityUpdateExecutionInput

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
    mcp_servers: dict = None  # MCP servers configuration
    session_id: Optional[str] = None  # Session ID for Agno session management
    user_id: Optional[str] = None  # User ID for multi-user support
    # Note: control_plane_url and api_key are read from worker environment variables (CONTROL_PLANE_URL, KUBIYA_API_KEY)

    def __post_init__(self):
        if self.agents is None:
            self.agents = []
        if self.team_config is None:
            self.team_config = {}
        if self.mcp_servers is None:
            self.mcp_servers = {}


@activity.defn
async def get_team_agents(input: ActivityGetTeamAgentsInput) -> dict:
    """
    Get all agents in a team via Control Plane API.

    This activity fetches team details including member agents from the Control Plane.

    Args:
        input: Activity input with team details

    Returns:
        Dict with agents list
    """
    print(f"\n\n=== GET_TEAM_AGENTS START ===")
    print(f"team_id: {input.team_id} (type: {type(input.team_id).__name__})")
    print(f"organization_id: {input.organization_id} (type: {type(input.organization_id).__name__})")
    print(f"================================\n")

    activity.logger.info(
        f"[DEBUG] Getting team agents START",
        extra={
            "team_id": input.team_id,
            "team_id_type": type(input.team_id).__name__,
            "organization_id": input.organization_id,
            "organization_id_type": type(input.organization_id).__name__,
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

        print(f"Fetching team from Control Plane API: {control_plane_url}")

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
                print(f"Team not found!")
                activity.logger.error(
                    f"[DEBUG] Team not found",
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
        # The API returns a TeamWithAgentsResponse which includes the agents array
        agents = team_data.get("agents", [])

        print(f"Query executed. Agents found: {len(agents)}")

        activity.logger.info(
            f"[DEBUG] Query executed, processing results",
            extra={
                "agents_found": len(agents),
                "agent_ids": [a.get("id") for a in agents],
            }
        )

        print(f"Agents found: {len(agents)}")
        if agents:
            for agent in agents:
                print(f"  - {agent.get('name')} (ID: {agent.get('id')})")

        activity.logger.info(
            f"[DEBUG] Retrieved team agents via API",
            extra={
                "team_id": input.team_id,
                "agent_count": len(agents),
                "agent_names": [a.get("name") for a in agents],
                "agent_ids": [a.get("id") for a in agents],
            }
        )

        if not agents:
            print(f"\n!!! NO AGENTS FOUND - Team may have no members !!!")
            activity.logger.warning(
                f"[DEBUG] WARNING: No agents found for team",
                extra={
                    "team_id": input.team_id,
                    "organization_id": input.organization_id,
                }
            )

        print(f"\n=== GET_TEAM_AGENTS END: Returning {len(agents)} agents ===\n\n")
        return {
            "agents": agents,
            "count": len(agents),
        }

    except Exception as e:
        print(f"\n!!! EXCEPTION in get_team_agents: {type(e).__name__}: {str(e)} !!!\n")
        activity.logger.error(
            f"[DEBUG] EXCEPTION in get_team_agents",
            extra={
                "team_id": input.team_id,
                "organization_id": input.organization_id,
                "error": str(e),
                "error_type": type(e).__name__,
            }
        )
        raise


@activity.defn
async def execute_team_coordination(input: ActivityExecuteTeamInput) -> dict:
    """
    Execute team coordination using Agno Teams.

    This activity creates an Agno Team with member Agents and executes
    the team run, allowing Agno to handle coordination.

    Args:
        input: Activity input with team execution details

    Returns:
        Dict with aggregated response, usage, success flag
    """
    print("\n" + "="*80)
    print("🚀 TEAM EXECUTION START")
    print("="*80)
    print(f"Execution ID: {input.execution_id}")
    print(f"Team ID: {input.team_id}")
    print(f"Organization: {input.organization_id}")
    print(f"Agent Count: {len(input.agents)}")
    print(f"MCP Servers: {len(input.mcp_servers)} configured" if input.mcp_servers else "MCP Servers: None")
    print(f"Session ID: {input.session_id}")
    print(f"Prompt: {input.prompt[:100]}..." if len(input.prompt) > 100 else f"Prompt: {input.prompt}")
    print("="*80 + "\n")

    activity.logger.info(
        f"Executing team coordination with Agno Teams",
        extra={
            "execution_id": input.execution_id,
            "team_id": input.team_id,
            "organization_id": input.organization_id,
            "agent_count": len(input.agents),
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

        # Get Control Plane URL and API key from environment (worker has these set on startup)
        control_plane_url = os.getenv("CONTROL_PLANE_URL")
        api_key = os.getenv("KUBIYA_API_KEY")

        # Fetch resolved toolsets from Control Plane if available
        toolsets = []
        if control_plane_url and api_key and input.team_id:
            print(f"🔧 Fetching toolsets for TEAM from Control Plane...")
            try:
                async with httpx.AsyncClient(timeout=30.0) as client:
                    response = await client.get(
                        f"{control_plane_url}/api/v1/toolsets/associations/teams/{input.team_id}/toolsets/resolved",
                        headers={"Authorization": f"Bearer {api_key}"}
                    )

                    if response.status_code == 200:
                        toolsets = response.json()
                        print(f"✅ Resolved {len(toolsets)} toolsets from Control Plane for TEAM")
                        print(f"   Toolset Types: {[t.get('type') for t in toolsets]}")
                        print(f"   Toolset Sources: {[t.get('source') for t in toolsets]}")
                        print(f"   Toolset Names: {[t.get('name') for t in toolsets]}\n")

                        activity.logger.info(
                            f"Resolved toolsets for team from Control Plane",
                            extra={
                                "team_id": input.team_id,
                                "toolset_count": len(toolsets),
                                "toolset_types": [t.get("type") for t in toolsets],
                                "toolset_sources": [t.get("source") for t in toolsets],
                                "toolset_names": [t.get("name") for t in toolsets],
                            }
                        )
                    else:
                        print(f"⚠️  Failed to fetch toolsets for team: HTTP {response.status_code}")
                        print(f"   Response: {response.text[:200]}\n")
                        activity.logger.warning(
                            f"Failed to fetch toolsets for team from Control Plane: {response.status_code}",
                            extra={
                                "status_code": response.status_code,
                                "response_text": response.text[:500]
                            }
                        )
            except Exception as e:
                print(f"❌ Error fetching toolsets for team: {str(e)}\n")
                activity.logger.error(
                    f"Error fetching toolsets for team from Control Plane: {str(e)}",
                    extra={"error": str(e)}
                )
                # Continue execution without toolsets
        else:
            print(f"ℹ️  No Control Plane URL/API key in environment for team - skipping toolset resolution\n")

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

        # Create Agno Agent objects for each team member
        print("\n📋 Creating Team Members:")
        member_agents = []
        for i, agent_data in enumerate(input.agents, 1):
            # Get model ID (default to kubiya/claude-sonnet-4 if not specified)
            model_id = agent_data.get("model_id") or "kubiya/claude-sonnet-4"

            print(f"  {i}. {agent_data['name']}")
            print(f"     Model: {model_id}")
            print(f"     Role: {agent_data.get('description', agent_data['name'])[:60]}...")

            # Create Agno Agent with explicit LiteLLM proxy configuration
            # IMPORTANT: Use openai/ prefix for custom proxy compatibility
            member_agent = Agent(
                name=agent_data["name"],
                role=agent_data.get("description", agent_data["name"]),
                model=LiteLLM(
                    id=f"openai/{model_id}",  # e.g., "openai/kubiya/claude-sonnet-4"
                    api_base=litellm_api_base,
                    api_key=litellm_api_key,
                ),
            )
            member_agents.append(member_agent)

            activity.logger.info(
                f"Created Agno Agent",
                extra={
                    "agent_name": agent_data["name"],
                    "model": model_id,
                }
            )

        # Create Agno Team with member agents and LiteLLM model for coordination
        # Get coordinator model from team configuration (if specified by user in UI)
        # Falls back to default if not configured
        team_model = (
            input.team_config.get("llm", {}).get("model")
            or "kubiya/claude-sonnet-4"  # Default coordinator model
        )

        print(f"\n🤖 Creating Agno Team:")
        print(f"   Coordinator Model: {team_model}")
        print(f"   Members: {len(member_agents)}")
        print(f"   Toolsets: {len(agno_toolkits)}")

        # Send heartbeat: Creating team
        activity.heartbeat({"status": "Creating team with agents and toolsets..."})

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

        # Create Team with openai/ prefix for custom proxy compatibility
        # NOTE: Session persistence is handled by Control Plane's agno_service.py
        # The worker just executes and returns results - no database required here
        team = Team(
            members=member_agents,
            name=f"Team {input.team_id}",
            model=LiteLLM(
                id=f"openai/{team_model}",  # e.g., "openai/kubiya/claude-sonnet-4"
                api_base=litellm_api_base,
                api_key=litellm_api_key,
            ),
            tools=agno_toolkits if agno_toolkits else None,  # Add toolsets to team
            tool_hooks=[tool_hook],  # Add hook for real-time tool updates
            # No db parameter - worker doesn't persist sessions
            # Session tracking happens via session_id in run() call
        )

        activity.logger.info(
            f"Created Agno Team with {len(member_agents)} members",
            extra={
                "coordinator_model": team_model,
                "member_count": len(member_agents),
            }
        )

        # Cache execution metadata in Redis for fast SSE lookups (avoid DB queries)
        cache_execution_metadata(input.execution_id, "TEAM")

        # Execute team run with streaming in a thread pool
        # This prevents blocking the async event loop in Temporal
        print("\n⚡ Executing Team Run with Streaming...")
        print(f"   Prompt: {input.prompt}\n")

        # Send heartbeat: Starting execution
        activity.heartbeat({"status": "Team is processing your request..."})

        import asyncio

        # Stream the response and collect chunks + tool messages
        response_chunks = []
        full_response = ""

        # Generate unique message ID for this turn (execution_id + timestamp)
        import time
        message_id = f"{input.execution_id}_{int(time.time() * 1000000)}"

        def stream_team_run():
            """Run team with streaming and collect response"""
            nonlocal full_response
            try:
                # Run with streaming enabled
                # NOTE: session_id is passed for in-memory session tracking during execution
                # Actual persistence happens on Control Plane side
                run_kwargs = {"stream": True}
                if input.session_id:
                    run_kwargs["session_id"] = input.session_id
                if input.user_id:
                    run_kwargs["user_id"] = input.user_id

                run_response = team.run(input.prompt, **run_kwargs)

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

                print()  # New line after streaming

                # Return the iterator's final result
                return run_response
            except Exception as e:
                print(f"\n❌ Streaming error: {str(e)}")
                import traceback
                traceback.print_exc()
                # Fall back to non-streaming
                run_kwargs_fallback = {"stream": False}
                if input.session_id:
                    run_kwargs_fallback["session_id"] = input.session_id
                if input.user_id:
                    run_kwargs_fallback["user_id"] = input.user_id
                return team.run(input.prompt, **run_kwargs_fallback)

        # Execute in thread pool
        result = await asyncio.to_thread(stream_team_run)

        # Send heartbeat: Completed
        activity.heartbeat({"status": "Team execution completed, preparing response..."})

        print("\n✅ Team Execution Completed!")
        print(f"   Response Length: {len(full_response)} chars")

        activity.logger.info(
            f"Agno Team execution completed",
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
                "input_tokens": getattr(metrics, "input_tokens", 0),
                "output_tokens": getattr(metrics, "output_tokens", 0),
                "total_tokens": getattr(metrics, "total_tokens", 0),
            }
            print(f"\n📊 Token Usage:")
            print(f"   Input Tokens: {usage.get('input_tokens', 0)}")
            print(f"   Output Tokens: {usage.get('output_tokens', 0)}")
            print(f"   Total Tokens: {usage.get('total_tokens', 0)}")

        print(f"\n📝 Response Preview:")
        print(f"   {response_content[:200]}..." if len(response_content) > 200 else f"   {response_content}")

        print("\n" + "="*80)
        print("🏁 TEAM EXECUTION END")
        print("="*80 + "\n")

        return {
            "success": True,
            "response": response_content,
            "usage": usage,
            "coordination_type": "agno_team",
            "tool_messages": tool_messages,  # Include tool call messages for UI
            "tool_execution_messages": tool_execution_messages,  # Include real-time tool execution status
        }

    except Exception as e:
        print("\n" + "="*80)
        print("❌ TEAM EXECUTION FAILED")
        print("="*80)
        print(f"Error: {str(e)}")
        print("="*80 + "\n")

        activity.logger.error(
            f"Team coordination failed",
            extra={
                "execution_id": input.execution_id,
                "error": str(e),
            }
        )
        return {
            "success": False,
            "error": str(e),
            "coordination_type": "agno_team",
            "usage": {},
        }
