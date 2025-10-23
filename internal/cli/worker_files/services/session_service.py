"""Session management service - handles loading and persisting conversation history"""

from typing import List, Dict, Any, Optional
from datetime import datetime, timezone
import structlog
import httpx

from control_plane_client import ControlPlaneClient
from utils.retry_utils import retry_with_backoff

logger = structlog.get_logger()


class SessionService:
    """
    Manages session history loading and persistence via Control Plane API.

    Workers don't have database access, so all session operations go through
    the Control Plane which provides Redis caching for hot loads.
    """

    def __init__(self, control_plane: ControlPlaneClient):
        self.control_plane = control_plane

    @retry_with_backoff(max_retries=3, initial_delay=1.0)
    def load_session(
        self,
        execution_id: str,
        session_id: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """
        Load session history from Control Plane (with retry).

        Returns:
            List of message dicts with role, content, timestamp, etc.
            Empty list if session not found or on error.
        """
        if not session_id:
            return []

        try:
            session_data = self.control_plane.get_session(
                execution_id=execution_id,
                session_id=session_id
            )

            if session_data and session_data.get("messages"):
                messages = session_data["messages"]
                logger.info(
                    "session_loaded",
                    execution_id=execution_id[:8],
                    message_count=len(messages)
                )
                return messages

            return []

        except httpx.TimeoutException:
            logger.warning(
                "session_load_timeout",
                execution_id=execution_id[:8]
            )
            raise  # Let retry decorator handle it
        except Exception as e:
            logger.warning(
                "session_load_error",
                execution_id=execution_id[:8],
                error=str(e)
            )
            return []  # Don't retry on non-timeout errors

    @retry_with_backoff(max_retries=3, initial_delay=1.0)
    def persist_session(
        self,
        execution_id: str,
        session_id: str,
        user_id: Optional[str],
        messages: List[Dict[str, Any]],
        metadata: Optional[Dict[str, Any]] = None
    ) -> bool:
        """
        Persist session history to Control Plane (with retry).

        Returns:
            True if successful, False otherwise
        """
        if not messages:
            logger.info("session_persist_skipped_no_messages", execution_id=execution_id[:8])
            return True

        try:
            success = self.control_plane.persist_session(
                execution_id=execution_id,
                session_id=session_id or execution_id,
                user_id=user_id,
                messages=messages,
                metadata=metadata or {}
            )

            if success:
                logger.info(
                    "session_persisted",
                    execution_id=execution_id[:8],
                    message_count=len(messages)
                )

            return success

        except Exception as e:
            logger.error(
                "session_persist_error",
                execution_id=execution_id[:8],
                error=str(e)
            )
            return False

    def build_conversation_context(
        self,
        session_messages: List[Dict[str, Any]]
    ) -> List[Dict[str, str]]:
        """
        Convert Control Plane session messages to Agno format.

        Args:
            session_messages: Messages from Control Plane

        Returns:
            List of dicts with 'role' and 'content' for Agno
        """
        context = []
        for msg in session_messages:
            context.append({
                "role": msg.get("role", "user"),
                "content": msg.get("content", ""),
            })
        return context

    def extract_messages_from_result(
        self,
        result: Any,
        user_id: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """
        Extract messages from Agno Agent/Team result.

        Args:
            result: Agno RunResponse object
            user_id: Optional user ID to attach

        Returns:
            List of message dicts ready for persistence
        """
        messages = []

        if hasattr(result, "messages") and result.messages:
            for msg in result.messages:
                messages.append({
                    "role": msg.role,
                    "content": msg.content,
                    "timestamp": (
                        getattr(msg, "created_at", datetime.now(timezone.utc)).isoformat()
                        if hasattr(msg, "created_at")
                        else datetime.now(timezone.utc).isoformat()
                    ),
                    "user_id": getattr(msg, "user_id", user_id),
                    "user_name": getattr(msg, "user_name", None),
                    "user_email": getattr(msg, "user_email", None),
                })

        return messages
