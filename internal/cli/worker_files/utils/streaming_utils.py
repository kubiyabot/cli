"""Streaming utilities for agent and team execution"""

from typing import Dict, Any, Callable, Optional
import structlog

logger = structlog.get_logger()


class StreamingHelper:
    """
    Helper for handling streaming from Agno Agent/Team executions.

    Provides utilities for:
    - Publishing events to Control Plane
    - Tracking run_id from streaming chunks
    - Collecting response content
    - Publishing tool execution events
    - Handling member message streaming
    - Tracking tool IDs for proper start/complete matching
    """

    def __init__(self, control_plane_client, execution_id: str):
        self.control_plane = control_plane_client
        self.execution_id = execution_id
        self.run_id_published = False
        self.response_content = []
        self.member_message_ids = {}  # Track message_id per member
        self.active_streaming_member = None  # Track which member is streaming
        self.tool_execution_ids = {}  # Track tool IDs for matching start/complete events

    def handle_run_id(self, chunk: Any, on_run_id: Optional[Callable[[str], None]] = None) -> None:
        """
        Capture and publish run_id from first streaming chunk.

        Args:
            chunk: Streaming chunk from Agno
            on_run_id: Optional callback when run_id is captured
        """
        if not self.run_id_published and hasattr(chunk, 'run_id') and chunk.run_id:
            run_id = chunk.run_id

            logger.info("run_id_captured", execution_id=self.execution_id[:8], run_id=run_id[:16])

            # Publish to Control Plane for UI
            self.control_plane.publish_event(
                execution_id=self.execution_id,
                event_type="run_started",
                data={
                    "run_id": run_id,
                    "execution_id": self.execution_id,
                    "cancellable": True,
                }
            )

            self.run_id_published = True

            # Call callback if provided (for cancellation manager)
            if on_run_id:
                on_run_id(run_id)

    def handle_content_chunk(
        self,
        chunk: Any,
        message_id: str,
        print_to_console: bool = True
    ) -> Optional[str]:
        """
        Handle content chunk from streaming response.

        Args:
            chunk: Streaming chunk
            message_id: Unique message ID for this turn
            print_to_console: Whether to print to stdout

        Returns:
            Content string if present, None otherwise
        """
        if hasattr(chunk, 'content') and chunk.content:
            content = str(chunk.content)
            self.response_content.append(content)

            if print_to_console:
                print(content, end='', flush=True)

            # Stream to Control Plane for real-time UI updates
            self.control_plane.publish_event(
                execution_id=self.execution_id,
                event_type="message_chunk",
                data={
                    "role": "assistant",
                    "content": content,
                    "is_chunk": True,
                    "message_id": message_id,
                }
            )

            return content

        return None

    def get_full_response(self) -> str:
        """Get the complete response accumulated from all chunks."""
        return ''.join(self.response_content)

    def handle_member_content_chunk(
        self,
        member_name: str,
        content: str,
        print_to_console: bool = True
    ) -> str:
        """
        Handle content chunk from a team member.

        Args:
            member_name: Name of the team member
            content: Content string
            print_to_console: Whether to print to stdout

        Returns:
            The member's message_id
        """
        import time

        # Generate unique message ID for this member if not exists
        if member_name not in self.member_message_ids:
            self.member_message_ids[member_name] = f"{self.execution_id}_{member_name}_{int(time.time() * 1000000)}"

            # Print member name header once when they start
            if print_to_console:
                print(f"\n[{member_name}] ", end='', flush=True)

        # If switching to a different member, mark the previous one as complete
        if self.active_streaming_member and self.active_streaming_member != member_name:
            self.publish_member_complete(self.active_streaming_member)

        # Track that this member is now actively streaming
        self.active_streaming_member = member_name

        # Print content without repeated member name prefix
        if print_to_console:
            print(content, end='', flush=True)

        # Stream member chunk to Control Plane
        message_id = self.member_message_ids[member_name]
        self.control_plane.publish_event(
            execution_id=self.execution_id,
            event_type="member_message_chunk",
            data={
                "role": "assistant",
                "content": content,
                "is_chunk": True,
                "message_id": message_id,
                "source": "team_member",
                "member_name": member_name,
            }
        )

        return message_id

    def publish_member_complete(self, member_name: str) -> None:
        """
        Publish member_message_complete event.

        Args:
            member_name: Name of the member to mark as complete
        """
        if member_name in self.member_message_ids:
            self.control_plane.publish_event(
                execution_id=self.execution_id,
                event_type="member_message_complete",
                data={
                    "message_id": self.member_message_ids[member_name],
                    "member_name": member_name,
                    "source": "team_member",
                }
            )

    def finalize_streaming(self) -> None:
        """
        Finalize streaming by marking any active member as complete.
        Call this when streaming ends.
        """
        if self.active_streaming_member:
            self.publish_member_complete(self.active_streaming_member)
            self.active_streaming_member = None

    def publish_tool_start(
        self,
        tool_name: str,
        tool_execution_id: str,
        tool_args: Optional[Dict[str, Any]] = None,
        source: str = "agent",
        member_name: Optional[str] = None
    ) -> str:
        """
        Publish tool execution start event.

        Args:
            tool_name: Name of the tool
            tool_execution_id: Unique ID for this tool execution
            tool_args: Tool arguments
            source: "agent" or "team_member" or "team_leader"  or "team"
            member_name: Name of member (if tool is from a member)

        Returns:
            message_id for this tool execution
        """
        import time

        message_id = f"{self.execution_id}_tool_{tool_execution_id}"
        is_member_tool = member_name is not None
        parent_message_id = self.member_message_ids.get(member_name) if is_member_tool else None

        # Store tool info for matching with completion event
        tool_key = f"{member_name or 'leader'}_{tool_name}_{int(time.time())}"
        self.tool_execution_ids[tool_key] = {
            "tool_execution_id": tool_execution_id,
            "message_id": message_id,
            "tool_name": tool_name,
            "member_name": member_name,
            "parent_message_id": parent_message_id,
        }

        event_type = "member_tool_started" if is_member_tool else "tool_started"

        self.control_plane.publish_event(
            execution_id=self.execution_id,
            event_type=event_type,
            data={
                "tool_name": tool_name,
                "tool_execution_id": tool_execution_id,
                "message_id": message_id,
                "tool_arguments": tool_args,
                "source": "team_member" if is_member_tool else "team_leader",
                "member_name": member_name,
                "parent_message_id": parent_message_id,
                "message": f"🔧 Executing tool: {tool_name}",
            }
        )

        return message_id

    def publish_tool_complete(
        self,
        tool_name: str,
        tool_execution_id: str,
        status: str = "success",
        output: Optional[str] = None,
        error: Optional[str] = None,
        source: str = "agent",
        member_name: Optional[str] = None
    ) -> None:
        """
        Publish tool execution completion event.

        Args:
            tool_name: Name of the tool
            tool_execution_id: Unique ID for this tool execution
            status: "success" or "failed"
            output: Tool output (if successful)
            error: Error message (if failed)
            source: "agent" or "team_member" or "team_leader" or "team"
            member_name: Name of member (if tool is from a member)
        """
        import time

        # Find the stored tool info from the start event
        tool_key_pattern = f"{member_name or 'leader'}_{tool_name}"
        matching_tool = None
        for key, tool_info in list(self.tool_execution_ids.items()):
            if key.startswith(tool_key_pattern):
                matching_tool = tool_info
                # Remove from tracking dict
                del self.tool_execution_ids[key]
                break

        if matching_tool:
            message_id = matching_tool["message_id"]
            parent_message_id = matching_tool["parent_message_id"]
            # Use the stored tool_execution_id from the start event
            tool_execution_id = matching_tool["tool_execution_id"]
        else:
            # Fallback if start event wasn't captured
            message_id = f"{self.execution_id}_tool_{tool_execution_id}"
            parent_message_id = self.member_message_ids.get(member_name) if member_name else None
            logger.warning("tool_completion_without_start", tool_name=tool_name, member_name=member_name)

        is_member_tool = member_name is not None
        event_type = "member_tool_completed" if is_member_tool else "tool_completed"

        self.control_plane.publish_event(
            execution_id=self.execution_id,
            event_type=event_type,
            data={
                "tool_name": tool_name,
                "tool_execution_id": tool_execution_id,  # Now uses the stored ID from start event
                "message_id": message_id,
                "status": status,
                "tool_output": output[:1000] if output else None,  # Limit size
                "tool_error": error,
                "source": "team_member" if is_member_tool else "team_leader",
                "member_name": member_name,
                "parent_message_id": parent_message_id,
                "message": f"{'✅' if status == 'success' else '❌'} Tool {status}: {tool_name}",
            }
        )


def create_tool_hook(control_plane_client, execution_id: str):
    """
    Create a tool hook function for Agno Agent/Team.

    This hook is called before and after each tool execution
    to publish real-time updates to the Control Plane.

    Args:
        control_plane_client: Control Plane client instance
        execution_id: Execution ID

    Returns:
        Hook function compatible with Agno tool_hooks
    """
    import time

    def tool_hook(tool_name: str, tool_args: dict, result: Any = None, error: Exception = None):
        """Tool hook for real-time updates"""
        tool_execution_id = f"{tool_name}_{int(time.time() * 1000000)}"

        if error is None and result is None:
            # Tool starting
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="tool_started",
                data={
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,
                    "tool_arguments": tool_args,
                    "message": f"🔧 Starting: {tool_name}",
                }
            )
        else:
            # Tool completed
            status = "failed" if error else "success"
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="tool_completed",
                data={
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,
                    "status": status,
                    "tool_output": str(result)[:1000] if result else None,
                    "tool_error": str(error) if error else None,
                    "message": f"{'✅' if status == 'success' else '❌'} {status}: {tool_name}",
                }
            )

    return tool_hook
