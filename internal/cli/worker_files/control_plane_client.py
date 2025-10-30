"""
Control Plane Client - Clean API for worker to communicate with Control Plane.

This centralizes all HTTP communication between worker and Control Plane,
providing a clean interface for:
- Event streaming (real-time UI updates)
- Session persistence (history storage)
- Metadata caching (execution types)
- Toolset resolution

Usage:
    from control_plane_client import get_control_plane_client

    client = get_control_plane_client()
    client.publish_event(execution_id, "message_chunk", {...})
    client.persist_session(execution_id, session_id, user_id, messages)
"""

import os
import httpx
from datetime import datetime, timezone
from typing import Optional, Dict, List, Any
import structlog

logger = structlog.get_logger()


class ControlPlaneClient:
    """Client for communicating with the Control Plane API from workers."""

    def __init__(self, base_url: str, api_key: str):
        """
        Initialize Control Plane client.

        Args:
            base_url: Control Plane URL (e.g., http://localhost:8000)
            api_key: Kubiya API key for authentication
        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.headers = {"Authorization": f"UserKey {api_key}"}

        # Use connection pooling for performance
        self._client = httpx.Client(
            timeout=httpx.Timeout(10.0, connect=2.0),
            limits=httpx.Limits(max_connections=10, max_keepalive_connections=5),
        )

    def __del__(self):
        """Close the HTTP client on cleanup."""
        try:
            self._client.close()
        except:
            pass

    def publish_event(
        self,
        execution_id: str,
        event_type: str,
        data: Dict[str, Any],
    ) -> bool:
        """
        Publish a streaming event for real-time UI updates.

        Args:
            execution_id: Execution ID
            event_type: Event type (message_chunk, tool_started, etc.)
            data: Event payload

        Returns:
            True if successful, False otherwise
        """
        try:
            url = f"{self.base_url}/api/v1/executions/{execution_id}/events"
            payload = {
                "event_type": event_type,
                "data": data,
                "timestamp": datetime.now(timezone.utc).isoformat(),
            }

            response = self._client.post(url, json=payload, headers=self.headers)

            if response.status_code not in (200, 202):
                logger.warning(
                    "event_publish_failed",
                    status=response.status_code,
                    execution_id=execution_id[:8],
                    event_type=event_type,
                )
                return False

            return True

        except Exception as e:
            logger.warning(
                "event_publish_error",
                error=str(e),
                execution_id=execution_id[:8],
                event_type=event_type,
            )
            return False

    def cache_metadata(
        self,
        execution_id: str,
        execution_type: str,
    ) -> bool:
        """
        Cache execution metadata in Redis for fast SSE lookups.

        This eliminates the need for database queries on every SSE connection.

        Args:
            execution_id: Execution ID
            execution_type: "AGENT" or "TEAM"

        Returns:
            True if successful, False otherwise
        """
        return self.publish_event(
            execution_id=execution_id,
            event_type="metadata",
            data={"execution_type": execution_type},
        )

    def get_session(
        self,
        execution_id: str,
        session_id: Optional[str] = None,
    ) -> Optional[Dict[str, Any]]:
        """
        Retrieve session history from Control Plane database.

        This loads conversation history so workers can restore context
        across multiple execution turns.

        Args:
            execution_id: Execution ID
            session_id: Session ID (defaults to execution_id if not provided)

        Returns:
            Dict with session data including messages, or None if not found
        """
        try:
            session_id = session_id or execution_id
            url = f"{self.base_url}/api/v1/executions/{execution_id}/session"

            response = self._client.get(url, headers=self.headers)

            if response.status_code == 200:
                session_data = response.json()
                logger.info(
                    "session_loaded",
                    execution_id=execution_id[:8],
                    message_count=len(session_data.get("messages", [])),
                )
                return session_data
            elif response.status_code == 404:
                logger.info(
                    "session_not_found",
                    execution_id=execution_id[:8],
                )
                return None
            else:
                logger.warning(
                    "session_load_failed",
                    status=response.status_code,
                    execution_id=execution_id[:8],
                )
                return None

        except Exception as e:
            logger.warning(
                "session_load_error",
                error=str(e),
                execution_id=execution_id[:8],
            )
            return None

    def persist_session(
        self,
        execution_id: str,
        session_id: str,
        user_id: Optional[str],
        messages: List[Dict[str, Any]],
        metadata: Optional[Dict[str, Any]] = None,
    ) -> bool:
        """
        Persist session history to Control Plane database.

        This ensures history is available even when worker is offline.

        Args:
            execution_id: Execution ID
            session_id: Session ID
            user_id: User ID
            messages: List of session messages
            metadata: Optional metadata

        Returns:
            True if successful, False otherwise
        """
        try:
            url = f"{self.base_url}/api/v1/executions/{execution_id}/session"
            payload = {
                "session_id": session_id,
                "user_id": user_id,
                "messages": messages,
                "metadata": metadata or {},
            }

            response = self._client.post(url, json=payload, headers=self.headers)

            if response.status_code in (200, 201):
                logger.info(
                    "session_persisted",
                    execution_id=execution_id[:8],
                    message_count=len(messages),
                )
                return True
            else:
                logger.warning(
                    "session_persistence_failed",
                    status=response.status_code,
                    execution_id=execution_id[:8],
                )
                return False

        except Exception as e:
            logger.warning(
                "session_persistence_error",
                error=str(e),
                execution_id=execution_id[:8],
            )
            return False

    def get_toolsets(
        self,
        agent_id: str,
    ) -> List[Dict[str, Any]]:
        """
        Fetch resolved toolsets for an agent from Control Plane.

        This endpoint returns toolsets merged from all layers:
        - All agent environments (many-to-many)
        - Team toolsets (if agent has team)
        - All team environments (many-to-many)
        - Agent's own toolsets

        Args:
            agent_id: Agent ID

        Returns:
            List of toolset configurations with source and inheritance info
        """
        try:
            url = f"{self.base_url}/api/v1/toolsets/associations/agents/{agent_id}/toolsets/resolved"
            response = self._client.get(url, headers=self.headers)

            if response.status_code == 200:
                toolsets = response.json()
                logger.info(
                    "toolsets_fetched",
                    agent_id=agent_id[:8],
                    toolset_count=len(toolsets),
                )
                return toolsets
            else:
                logger.warning(
                    "toolsets_fetch_failed",
                    status=response.status_code,
                    agent_id=agent_id[:8],
                )
                return []

        except Exception as e:
            logger.warning(
                "toolsets_fetch_error",
                error=str(e),
                agent_id=agent_id[:8],
            )
            return []

    def get_team_toolsets(
        self,
        team_id: str,
    ) -> List[Dict[str, Any]]:
        """
        Fetch resolved toolsets for a team from Control Plane.

        This endpoint returns toolsets merged from all layers:
        - All team environments (many-to-many)
        - Team's own toolsets

        Args:
            team_id: Team ID

        Returns:
            List of toolset configurations with source and inheritance info
        """
        try:
            url = f"{self.base_url}/api/v1/toolsets/associations/teams/{team_id}/toolsets/resolved"
            response = self._client.get(url, headers=self.headers)

            if response.status_code == 200:
                toolsets = response.json()
                logger.info(
                    "team_toolsets_fetched",
                    team_id=team_id[:8],
                    toolset_count=len(toolsets),
                )
                return toolsets
            else:
                logger.warning(
                    "team_toolsets_fetch_failed",
                    status=response.status_code,
                    team_id=team_id[:8],
                )
                return []

        except Exception as e:
            logger.warning(
                "team_toolsets_fetch_error",
                error=str(e),
                team_id=team_id[:8],
            )
            return []

    def get_agent_execution_environment(
        self,
        agent_id: str,
    ) -> Dict[str, str]:
        """
        Fetch resolved execution environment for an agent from Control Plane.

        This endpoint returns a fully resolved environment variable dict with:
        - Custom env vars from agent configuration
        - Secret values (resolved from Kubiya vault)
        - Integration tokens (resolved and mapped to env var names like GH_TOKEN, JIRA_TOKEN)

        Args:
            agent_id: Agent ID

        Returns:
            Dict of environment variables ready to inject into agent execution
        """
        try:
            url = f"{self.base_url}/api/v1/execution-environment/agents/{agent_id}/resolved"
            response = self._client.get(url, headers=self.headers)

            if response.status_code == 200:
                env_vars = response.json()
                logger.info(
                    "agent_execution_environment_fetched",
                    agent_id=agent_id[:8],
                    env_var_count=len(env_vars),
                    env_var_keys=list(env_vars.keys()),
                )
                return env_vars
            else:
                logger.warning(
                    "agent_execution_environment_fetch_failed",
                    status=response.status_code,
                    agent_id=agent_id[:8],
                )
                return {}

        except Exception as e:
            logger.warning(
                "agent_execution_environment_fetch_error",
                error=str(e),
                agent_id=agent_id[:8],
            )
            return {}

    def get_team_execution_environment(
        self,
        team_id: str,
    ) -> Dict[str, str]:
        """
        Fetch resolved execution environment for a team from Control Plane.

        This endpoint returns a fully resolved environment variable dict with:
        - Custom env vars from team configuration
        - Secret values (resolved from Kubiya vault)
        - Integration tokens (resolved and mapped to env var names like GH_TOKEN, JIRA_TOKEN)

        Args:
            team_id: Team ID

        Returns:
            Dict of environment variables ready to inject into team execution
        """
        try:
            url = f"{self.base_url}/api/v1/execution-environment/teams/{team_id}/resolved"
            response = self._client.get(url, headers=self.headers)

            if response.status_code == 200:
                env_vars = response.json()
                logger.info(
                    "team_execution_environment_fetched",
                    team_id=team_id[:8],
                    env_var_count=len(env_vars),
                    env_var_keys=list(env_vars.keys()),
                )
                return env_vars
            else:
                logger.warning(
                    "team_execution_environment_fetch_failed",
                    status=response.status_code,
                    team_id=team_id[:8],
                )
                return {}

        except Exception as e:
            logger.warning(
                "team_execution_environment_fetch_error",
                error=str(e),
                team_id=team_id[:8],
            )
            return {}


# Singleton instance
_control_plane_client: Optional[ControlPlaneClient] = None


def get_control_plane_client() -> ControlPlaneClient:
    """
    Get or create the Control Plane client singleton.

    Reads configuration from environment variables:
    - CONTROL_PLANE_URL: Control Plane URL
    - KUBIYA_API_KEY: API key for authentication

    Returns:
        ControlPlaneClient instance

    Raises:
        ValueError: If required environment variables are not set
    """
    global _control_plane_client

    if _control_plane_client is None:
        base_url = os.environ.get("CONTROL_PLANE_URL")
        api_key = os.environ.get("KUBIYA_API_KEY")

        if not base_url:
            raise ValueError("CONTROL_PLANE_URL environment variable not set")
        if not api_key:
            raise ValueError("KUBIYA_API_KEY environment variable not set")

        _control_plane_client = ControlPlaneClient(base_url=base_url, api_key=api_key)

        logger.info(
            "control_plane_client_initialized",
            base_url=base_url,
        )

    return _control_plane_client
