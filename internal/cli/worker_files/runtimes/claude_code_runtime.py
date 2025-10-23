"""
Claude Code runtime implementation using Claude Code SDK.

This runtime adapter integrates the Claude Code SDK to power agents with
advanced coding capabilities, file operations, and specialized tools.
"""

from typing import Dict, Any, Optional, AsyncIterator, Callable, TYPE_CHECKING
import structlog
import os

from .base import (
    RuntimeType,
    RuntimeExecutionResult,
    RuntimeExecutionContext,
    AgentRuntime,
)

if TYPE_CHECKING:
    from control_plane_client import ControlPlaneClient
    from services.cancellation_manager import CancellationManager

logger = structlog.get_logger(__name__)


class ClaudeCodeRuntime:
    """
    Runtime implementation using Claude Code SDK.

    This runtime leverages Claude Code's specialized capabilities for
    software engineering tasks, file operations, and developer workflows.
    """

    def __init__(
        self,
        control_plane_client: "ControlPlaneClient",
        cancellation_manager: "CancellationManager",
        **kwargs,
    ):
        """
        Initialize the Claude Code runtime.

        Args:
            control_plane_client: Client for Control Plane API
            cancellation_manager: Manager for execution cancellation
            **kwargs: Additional configuration options
        """
        self.control_plane_client = control_plane_client
        self.cancellation_manager = cancellation_manager
        self.logger = structlog.get_logger(__name__)
        self.config = kwargs

        # Track active SDK clients for cancellation
        self._active_clients: Dict[str, Any] = {}

    async def execute(
        self, context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """
        Execute agent using Claude Code SDK (non-streaming).

        Args:
            context: Execution context with prompt, history, config

        Returns:
            RuntimeExecutionResult with response and metadata
        """
        try:
            from claude_agent_sdk import ClaudeSDKClient

            # Build Claude Code options
            options = self._build_claude_options(context)

            # Create and connect client
            client = ClaudeSDKClient(options=options)
            self._active_clients[context.execution_id] = client

            await client.connect()

            # Query with prompt
            await client.query(context.prompt)

            # Collect complete response
            response_text = ""
            usage = {}
            tool_messages = []
            finish_reason = None

            async for message in client.receive_response():
                # Extract content
                if hasattr(message, "content"):
                    for block in message.content:
                        if hasattr(block, "text"):
                            response_text += block.text
                        elif hasattr(block, "name"):  # Tool use
                            tool_messages.append(
                                {
                                    "tool": block.name,
                                    "input": block.input if hasattr(block, "input") else {},
                                }
                            )

                # Extract usage from result message
                if hasattr(message, "usage") and message.usage:
                    usage = message.usage

                # Extract finish reason
                if hasattr(message, "subtype"):
                    if message.subtype in ["success", "error"]:
                        finish_reason = message.subtype

            # Disconnect
            await client.disconnect()
            del self._active_clients[context.execution_id]

            return RuntimeExecutionResult(
                response=response_text,
                usage=usage,
                success=True,
                finish_reason=finish_reason or "stop",
                tool_messages=tool_messages,
                model=context.model_id,
            )

        except Exception as e:
            self.logger.error(
                "Claude Code execution failed",
                execution_id=context.execution_id,
                error=str(e),
            )

            # Cleanup
            if context.execution_id in self._active_clients:
                try:
                    await self._active_clients[context.execution_id].disconnect()
                except:
                    pass
                del self._active_clients[context.execution_id]

            return RuntimeExecutionResult(
                response="",
                usage={},
                success=False,
                error=str(e),
            )

    async def stream_execute(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable[[Dict], None]] = None,
    ) -> AsyncIterator[RuntimeExecutionResult]:
        """
        Execute agent with streaming using Claude Code SDK.

        Args:
            context: Execution context
            event_callback: Optional callback for real-time events

        Yields:
            RuntimeExecutionResult chunks as they arrive
        """
        try:
            from claude_agent_sdk import ClaudeSDKClient, AssistantMessage, TextBlock, ToolUseBlock

            # Build Claude Code options
            options = self._build_claude_options(context)

            # Create and connect client
            client = ClaudeSDKClient(options=options)
            self._active_clients[context.execution_id] = client

            await client.connect()

            # Cache execution metadata
            self.control_plane_client.cache_metadata(context.execution_id, "AGENT")

            # Query with prompt
            await client.query(context.prompt)

            # Stream messages
            accumulated_usage = {}
            tool_messages = []

            async for message in client.receive_messages():
                # Handle assistant messages
                if isinstance(message, AssistantMessage):
                    for block in message.content:
                        if isinstance(block, TextBlock):
                            # Stream text content
                            if event_callback:
                                event_callback(
                                    {
                                        "type": "content_chunk",
                                        "content": block.text,
                                        "execution_id": context.execution_id,
                                    }
                                )

                            yield RuntimeExecutionResult(
                                response=block.text,
                                usage={},
                                success=True,
                            )

                        elif isinstance(block, ToolUseBlock):
                            # Tool use event
                            tool_info = {
                                "tool": block.name,
                                "input": block.input,
                                "tool_use_id": block.id,
                            }
                            tool_messages.append(tool_info)

                            if event_callback:
                                event_callback(
                                    {
                                        "type": "tool_start",
                                        "tool_name": block.name,
                                        "tool_args": block.input,
                                        "tool_execution_id": block.id,
                                        "execution_id": context.execution_id,
                                    }
                                )

                # Handle result message (final)
                if hasattr(message, "subtype") and message.subtype in [
                    "success",
                    "error",
                ]:
                    if hasattr(message, "usage") and message.usage:
                        accumulated_usage = message.usage

                    # Final result message
                    yield RuntimeExecutionResult(
                        response="",  # Already streamed
                        usage=accumulated_usage,
                        success=message.subtype == "success",
                        finish_reason=message.subtype,
                        tool_messages=tool_messages,
                        model=context.model_id,
                    )
                    break

            # Disconnect
            await client.disconnect()
            del self._active_clients[context.execution_id]

        except Exception as e:
            self.logger.error(
                "Claude Code streaming failed",
                execution_id=context.execution_id,
                error=str(e),
            )

            # Cleanup
            if context.execution_id in self._active_clients:
                try:
                    await self._active_clients[context.execution_id].disconnect()
                except:
                    pass
                del self._active_clients[context.execution_id]

            yield RuntimeExecutionResult(
                response="",
                usage={},
                success=False,
                error=str(e),
            )

    async def cancel(self, execution_id: str) -> bool:
        """
        Cancel an in-progress execution via Claude SDK interrupt.

        Args:
            execution_id: ID of execution to cancel

        Returns:
            True if cancellation succeeded
        """
        if execution_id in self._active_clients:
            try:
                client = self._active_clients[execution_id]
                await client.interrupt()
                self.logger.info("Claude Code execution interrupted", execution_id=execution_id)
                return True
            except Exception as e:
                self.logger.error(
                    "Failed to interrupt Claude Code execution",
                    execution_id=execution_id,
                    error=str(e),
                )
                return False
        return False

    async def get_usage(self, execution_id: str) -> Dict[str, Any]:
        """
        Get usage metrics for an execution.

        Note: Claude Code SDK doesn't maintain execution history,
        so this returns empty dict unless called during execution.

        Args:
            execution_id: ID of execution

        Returns:
            Usage metrics dict
        """
        return {}

    def supports_streaming(self) -> bool:
        """Claude Code SDK supports streaming."""
        return True

    def supports_tools(self) -> bool:
        """Claude Code SDK supports tool calling."""
        return True

    def supports_mcp(self) -> bool:
        """Claude Code SDK has native MCP support."""
        return True

    def get_runtime_type(self) -> RuntimeType:
        """Return RuntimeType.CLAUDE_CODE."""
        return RuntimeType.CLAUDE_CODE

    def get_runtime_info(self) -> Dict[str, Any]:
        """Get information about Claude Code runtime."""
        return {
            "runtime_type": "claude_code",
            "framework": "claude_code_sdk",
            "supports_streaming": True,
            "supports_tools": True,
            "supports_mcp": True,
        }

    # Private helper methods

    def _build_claude_options(self, context: RuntimeExecutionContext) -> Any:
        """
        Build ClaudeAgentOptions from execution context.

        Args:
            context: Execution context

        Returns:
            ClaudeAgentOptions instance
        """
        from claude_agent_sdk import ClaudeAgentOptions

        # Extract configuration
        agent_config = context.agent_config or {}
        runtime_config = context.runtime_config or {}

        # Map toolsets to Claude Code tool names
        allowed_tools = self._map_toolsets_to_tools(context.toolsets)

        # Build options
        options = ClaudeAgentOptions(
            system_prompt=context.system_prompt,
            allowed_tools=allowed_tools,
            mcp_servers=context.mcp_servers or {},
            permission_mode=runtime_config.get("permission_mode", "acceptEdits"),
            cwd=agent_config.get("cwd") or runtime_config.get("cwd"),
            model=context.model_id,
            env=runtime_config.get("env", {}),
            max_turns=runtime_config.get("max_turns"),
        )

        return options

    def _map_toolsets_to_tools(self, toolsets: list) -> list:
        """
        Map toolsets to Claude Code tool names.

        This function translates our generic toolset types to the specific
        tool names that Claude Code understands.

        Args:
            toolsets: List of toolset objects

        Returns:
            List of Claude Code tool names
        """
        # Toolset type to Claude Code tool mapping
        tool_mapping = {
            "shell": ["Bash"],
            "file_system": ["Read", "Write", "Edit", "Glob", "Grep"],
            "web": ["WebFetch", "WebSearch"],
            "docker": ["Bash"],  # Docker commands via Bash
            "kubernetes": ["Bash"],  # kubectl via Bash
            "git": ["Bash"],  # git commands via Bash
            "task": ["Task"],  # Subagent tasks
        }

        tools = []
        for toolset in toolsets:
            # Get toolset type
            toolset_type = None
            if hasattr(toolset, "type"):
                toolset_type = toolset.type
            elif isinstance(toolset, dict):
                toolset_type = toolset.get("type")

            # Map to Claude Code tools
            if toolset_type and toolset_type in tool_mapping:
                tools.extend(tool_mapping[toolset_type])

        # Deduplicate
        return list(set(tools)) if tools else ["Read", "Write", "Bash"]  # Default tools

    def _message_to_event(self, message: Any) -> Dict:
        """
        Convert Claude SDK message to Control Plane event format.

        Args:
            message: Claude SDK message

        Returns:
            Event dict for Control Plane
        """
        return {
            "type": "message",
            "content": str(message),
            "timestamp": self._get_timestamp(),
        }

    def _get_timestamp(self) -> str:
        """Get current timestamp in ISO format."""
        from datetime import datetime, timezone

        return datetime.now(timezone.utc).isoformat()
