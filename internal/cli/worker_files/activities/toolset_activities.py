"""
Toolset Activities for Agent Control Plane Worker.

These activities handle toolset resolution and instantiation for agent execution.
"""

import structlog
from dataclasses import dataclass
from typing import Any, Optional
from temporalio import activity
import httpx

logger = structlog.get_logger()


@dataclass
class ToolSetDefinition:
    """Resolved toolset definition with merged configuration"""
    id: str
    name: str
    type: str
    description: str
    enabled: bool
    configuration: dict
    source: str  # 'environment', 'team', 'agent'


@activity.defn
async def resolve_agent_toolsets(
    agent_id: str,
    control_plane_url: str,
    api_key: str
) -> list[dict]:
    """
    Resolve toolsets for an agent by calling Control Plane API.

    The Control Plane handles all inheritance logic (Environment → Team → Agent)
    and returns the merged, resolved toolset list.

    Args:
        agent_id: Agent ID
        control_plane_url: Control Plane API URL (e.g., https://control-plane.kubiya.ai)
        api_key: API key for authentication

    Returns:
        List of resolved toolset definitions with merged configurations
    """
    logger.info(
        "resolving_agent_toolsets_from_control_plane",
        agent_id=agent_id,
        control_plane_url=control_plane_url
    )

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            # Call Control Plane API to resolve toolsets with inheritance
            response = await client.get(
                f"{control_plane_url}/api/v1/toolsets/associations/agents/{agent_id}/toolsets/resolved",
                headers={"Authorization": f"Bearer {api_key}"}
            )

            if response.status_code != 200:
                logger.error(
                    "toolset_resolution_failed",
                    status_code=response.status_code,
                    response=response.text[:500],
                    agent_id=agent_id
                )
                # Return empty list on failure - agent can still run without tools
                return []

            # Response is list of resolved toolsets
            toolsets = response.json()

            logger.info(
                "toolsets_resolved_from_control_plane",
                agent_id=agent_id,
                toolset_count=len(toolsets),
                toolset_types=[t.get("type") for t in toolsets],
                toolset_sources=[t.get("source") for t in toolsets]
            )

            return toolsets

    except Exception as e:
        logger.error(
            "toolset_resolution_error",
            error=str(e),
            agent_id=agent_id
        )
        return []


@activity.defn
async def instantiate_agent_tools(
    toolset_definitions: list[dict]
) -> list[Any]:
    """
    Instantiate agno tool instances from toolset definitions.

    Args:
        toolset_definitions: List of resolved toolset definitions

    Returns:
        List of instantiated agno tool objects
    """
    # Import agno tools
    try:
        from agno.tools.file import FileTools
        from agno.tools.shell import ShellTools
        from agno.tools.docker import DockerTools
        from agno.tools.sleep import SleepTools
        from agno.tools.file_generation import FileGenerationTools
    except ImportError as e:
        logger.error("agno_tools_import_failed", error=str(e))
        return []

    # Tool registry
    TOOLSET_REGISTRY = {
        "file_system": FileTools,
        "shell": ShellTools,
        "docker": DockerTools,
        "sleep": SleepTools,
        "file_generation": FileGenerationTools,
    }

    tools = []

    for toolset_def in toolset_definitions:
        if not toolset_def.get("enabled", True):
            logger.debug(
                "skipping_disabled_toolset",
                toolset_name=toolset_def.get("name")
            )
            continue

        toolset_type = toolset_def.get("type")
        tool_class = TOOLSET_REGISTRY.get(toolset_type)

        if not tool_class:
            logger.warning(
                "unknown_toolset_type",
                toolset_type=toolset_type,
                toolset_name=toolset_def.get("name")
            )
            continue

        # Get configuration
        config = toolset_def.get("configuration", {})

        # Instantiate tool with configuration
        try:
            tool_instance = tool_class(**config)
            tools.append(tool_instance)

            logger.info(
                "toolset_instantiated",
                toolset_name=toolset_def.get("name"),
                toolset_type=toolset_type,
                configuration=config
            )
        except Exception as e:
            logger.error(
                "toolset_instantiation_failed",
                toolset_name=toolset_def.get("name"),
                toolset_type=toolset_type,
                error=str(e)
            )
            # Continue with other tools even if one fails

    logger.info(
        "agent_tools_instantiated",
        tool_count=len(tools),
        tool_types=[type(t).__name__ for t in tools]
    )

    return tools


@activity.defn
async def instantiate_custom_toolset(
    toolset_definition: dict,
    organization_id: str,
    control_plane_url: str,
    api_key: str
) -> Optional[Any]:
    """
    Instantiate a custom toolset by loading user-provided Python code.

    Args:
        toolset_definition: Toolset definition with custom_class path
        organization_id: Organization ID
        control_plane_url: Control Plane API URL
        api_key: API key for authentication

    Returns:
        Instantiated tool instance or None if failed
    """
    logger.info(
        "instantiating_custom_toolset",
        toolset_name=toolset_definition.get("name"),
        organization_id=organization_id
    )

    try:
        # Get custom class path from configuration
        custom_class = toolset_definition.get("configuration", {}).get("custom_class")
        if not custom_class:
            logger.error("custom_toolset_missing_class", toolset_definition=toolset_definition)
            return None

        # Fetch custom toolset code from Control Plane
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.get(
                f"{control_plane_url}/api/v1/toolsets/{toolset_definition['id']}/code",
                headers={"Authorization": f"Bearer {api_key}"},
                params={"organization_id": organization_id}
            )

            if response.status_code != 200:
                logger.error(
                    "custom_toolset_code_fetch_failed",
                    status_code=response.status_code
                )
                return None

            code_data = response.json()
            python_code = code_data.get("code")

            if not python_code:
                logger.error("custom_toolset_no_code")
                return None

        # Execute code in isolated namespace
        namespace = {}
        exec(python_code, namespace)

        # Extract class from namespace
        class_parts = custom_class.split(".")
        tool_class = namespace.get(class_parts[-1])

        if not tool_class:
            logger.error(
                "custom_toolset_class_not_found",
                custom_class=custom_class
            )
            return None

        # Instantiate with configuration
        config = toolset_definition.get("configuration", {}).get("custom_config", {})
        tool_instance = tool_class(**config)

        logger.info(
            "custom_toolset_instantiated",
            toolset_name=toolset_definition.get("name"),
            custom_class=custom_class
        )

        return tool_instance

    except Exception as e:
        logger.error(
            "custom_toolset_instantiation_error",
            toolset_name=toolset_definition.get("name"),
            error=str(e)
        )
        return None
