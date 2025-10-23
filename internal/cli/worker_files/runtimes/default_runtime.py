"""
Default runtime implementation using Agno framework.

This runtime adapter wraps the existing Agno-based agent execution logic,
providing a clean interface that conforms to the AgentRuntime protocol.
"""

from typing import Dict, Any, Optional, AsyncIterator, Callable, TYPE_CHECKING
import structlog
import asyncio
import os

from agno.agent import Agent
from agno.models.litellm import LiteLLM

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


class DefaultRuntime:
    """
    Runtime implementation using Agno framework.

    This is the default runtime that wraps the existing Agno-based
    agent execution logic. It maintains backward compatibility while
    conforming to the new AgentRuntime interface.
    """

    def __init__(
        self,
        control_plane_client: "ControlPlaneClient",
        cancellation_manager: "CancellationManager",
        **kwargs,
    ):
        """
        Initialize the Agno runtime.

        Args:
            control_plane_client: Client for Control Plane API
            cancellation_manager: Manager for execution cancellation
            **kwargs: Additional configuration options
        """
        self.control_plane_client = control_plane_client
        self.cancellation_manager = cancellation_manager
        self.logger = structlog.get_logger(__name__)
        self.config = kwargs

    async def execute(
        self, context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """
        Execute agent using Agno framework without streaming.

        Args:
            context: Execution context with prompt, history, config

        Returns:
            RuntimeExecutionResult with response and metadata
        """
        try:
            # Create Agno agent
            agent = self._create_agno_agent(context)

            # Register for cancellation
            self.cancellation_manager.register(
                execution_id=context.execution_id,
                instance=agent,
                instance_type="agent",
            )

            # Build conversation context
            messages = self._build_conversation_messages(context)

            # Execute without streaming
            def run_agent():
                if messages:
                    return agent.run(context.prompt, stream=False, messages=messages)
                else:
                    return agent.run(context.prompt, stream=False)

            # Run in thread pool to avoid blocking
            result = await asyncio.to_thread(run_agent)

            # Cleanup
            self.cancellation_manager.unregister(context.execution_id)

            # Extract response and metadata
            response_content = (
                result.content if hasattr(result, "content") else str(result)
            )
            usage = self._extract_usage(result)
            tool_messages = self._extract_tool_messages(result)

            return RuntimeExecutionResult(
                response=response_content,
                usage=usage,
                success=True,
                finish_reason="stop",
                run_id=getattr(result, "run_id", None),
                model=context.model_id,
                tool_messages=tool_messages,
            )

        except asyncio.CancelledError:
            # Handle cancellation
            self.cancellation_manager.cancel(context.execution_id)
            self.cancellation_manager.unregister(context.execution_id)
            raise

        except Exception as e:
            self.logger.error(
                "Agno execution failed",
                execution_id=context.execution_id,
                error=str(e),
            )
            self.cancellation_manager.unregister(context.execution_id)

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
        Execute agent with streaming using Agno framework.

        Args:
            context: Execution context
            event_callback: Optional callback for real-time events

        Yields:
            RuntimeExecutionResult chunks as they arrive
        """
        try:
            # Create Agno agent with tool hooks for events
            agent = self._create_agno_agent(context, event_callback)

            # Register for cancellation
            self.cancellation_manager.register(
                execution_id=context.execution_id,
                instance=agent,
                instance_type="agent",
            )

            # Cache execution metadata
            self.control_plane_client.cache_metadata(context.execution_id, "AGENT")

            # Build conversation context
            messages = self._build_conversation_messages(context)

            # Stream execution
            accumulated_response = ""
            run_result = None

            def stream_agent_run():
                """Run agent with streaming and collect response"""
                try:
                    if messages:
                        return agent.run(
                            context.prompt,
                            stream=True,
                            messages=messages,
                        )
                    else:
                        return agent.run(context.prompt, stream=True)
                except Exception as e:
                    self.logger.error("Streaming error", error=str(e))
                    # Fallback to non-streaming
                    if messages:
                        return agent.run(
                            context.prompt, stream=False, messages=messages
                        )
                    else:
                        return agent.run(context.prompt, stream=False)

            # Execute in thread pool
            try:
                run_response = await asyncio.to_thread(stream_agent_run)

                # Process streaming chunks
                for chunk in run_response:
                    # Capture run_id for cancellation
                    if hasattr(chunk, "run_id") and chunk.run_id:
                        self.cancellation_manager.set_run_id(
                            context.execution_id, chunk.run_id
                        )

                    # Extract content
                    chunk_content = ""
                    if hasattr(chunk, "content") and chunk.content:
                        if isinstance(chunk.content, str):
                            chunk_content = chunk.content
                        elif hasattr(chunk.content, "text"):
                            chunk_content = chunk.content.text

                    if chunk_content:
                        accumulated_response += chunk_content

                        # Publish event if callback provided
                        if event_callback:
                            event_callback(
                                {
                                    "type": "content_chunk",
                                    "content": chunk_content,
                                    "execution_id": context.execution_id,
                                }
                            )

                        # Yield incremental result
                        yield RuntimeExecutionResult(
                            response=chunk_content,
                            usage={},
                            success=True,
                        )

                run_result = run_response

            except asyncio.CancelledError:
                # Handle cancellation
                self.cancellation_manager.cancel(context.execution_id)
                raise

            # Final result with complete metadata
            usage = self._extract_usage(run_result) if run_result else {}
            tool_messages = (
                self._extract_tool_messages(run_result) if run_result else []
            )

            yield RuntimeExecutionResult(
                response="",  # Already streamed
                usage=usage,
                success=True,
                finish_reason="stop",
                run_id=getattr(run_result, "run_id", None) if run_result else None,
                model=context.model_id,
                tool_messages=tool_messages,
                metadata={"accumulated_response": accumulated_response},
            )

        finally:
            # Cleanup
            self.cancellation_manager.unregister(context.execution_id)

    async def cancel(self, execution_id: str) -> bool:
        """
        Cancel an in-progress execution via CancellationManager.

        Args:
            execution_id: ID of execution to cancel

        Returns:
            True if cancellation succeeded
        """
        result = self.cancellation_manager.cancel(execution_id)
        return result.get("success", False)

    async def get_usage(self, execution_id: str) -> Dict[str, Any]:
        """
        Get usage metrics for an execution.

        Note: Agno doesn't maintain execution history, so this
        returns empty dict unless called immediately after execution.

        Args:
            execution_id: ID of execution

        Returns:
            Usage metrics dict
        """
        return {}

    def supports_streaming(self) -> bool:
        """Agno supports streaming."""
        return True

    def supports_tools(self) -> bool:
        """Agno supports tool calling."""
        return True

    def supports_mcp(self) -> bool:
        """Agno doesn't have native MCP support."""
        return False

    def get_runtime_type(self) -> RuntimeType:
        """Return RuntimeType.DEFAULT."""
        return RuntimeType.DEFAULT

    def get_runtime_info(self) -> Dict[str, Any]:
        """Get information about Agno runtime."""
        return {
            "runtime_type": "default",
            "framework": "agno",
            "supports_streaming": True,
            "supports_tools": True,
            "supports_mcp": False,
        }

    # Private helper methods

    def _create_agno_agent(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable] = None,
    ) -> Agent:
        """
        Create Agno Agent instance.

        Args:
            context: Execution context
            event_callback: Optional callback for tool execution events

        Returns:
            Configured Agno Agent instance
        """
        # Get LiteLLM configuration
        litellm_api_base = os.getenv(
            "LITELLM_API_BASE", "https://llm-proxy.kubiya.ai"
        )
        litellm_api_key = os.getenv("LITELLM_API_KEY")

        if not litellm_api_key:
            raise ValueError("LITELLM_API_KEY environment variable not set")

        model = context.model_id or os.environ.get(
            "LITELLM_DEFAULT_MODEL", "kubiya/claude-sonnet-4"
        )

        # Create tool hooks if event_callback provided
        tool_hooks = []
        if event_callback:
            tool_hooks.append(self._create_tool_hook(context.execution_id, event_callback))

        # Create agent
        agent = Agent(
            name=f"Agent {context.agent_id}",
            role=context.system_prompt or "You are a helpful AI assistant",
            model=LiteLLM(
                id=f"openai/{model}",
                api_base=litellm_api_base,
                api_key=litellm_api_key,
            ),
            tools=context.toolsets if context.toolsets else None,
            tool_hooks=tool_hooks if tool_hooks else None,
        )

        return agent

    def _create_tool_hook(
        self, execution_id: str, event_callback: Callable
    ) -> Callable:
        """
        Create a tool hook for capturing tool execution events.

        Args:
            execution_id: Execution ID
            event_callback: Callback to publish events

        Returns:
            Tool hook function
        """

        def tool_hook(
            name: str = None,
            function_name: str = None,
            function=None,
            arguments: dict = None,
            **kwargs,
        ):
            """Hook to capture tool execution for real-time streaming"""
            import time

            tool_name = name or function_name or "unknown"
            tool_args = arguments or {}
            tool_execution_id = f"{tool_name}_{int(time.time() * 1000000)}"

            # Publish tool start event
            event_callback(
                {
                    "type": "tool_start",
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,
                    "tool_args": tool_args,
                    "execution_id": execution_id,
                }
            )

            # Execute tool
            result = None
            error = None
            try:
                if function and callable(function):
                    result = function(**tool_args) if tool_args else function()
                else:
                    raise ValueError(f"Function not callable: {function}")

                status = "success"

            except Exception as e:
                error = e
                status = "failed"

            # Publish tool completion event
            event_callback(
                {
                    "type": "tool_complete",
                    "tool_name": tool_name,
                    "tool_execution_id": tool_execution_id,
                    "status": status,
                    "output": str(result)[:1000] if result else None,
                    "error": str(error) if error else None,
                    "execution_id": execution_id,
                }
            )

            if error:
                raise error

            return result

        return tool_hook

    def _build_conversation_messages(
        self, context: RuntimeExecutionContext
    ) -> list:
        """
        Build conversation messages for Agno from context history.

        Args:
            context: Execution context with history

        Returns:
            List of message dicts for Agno
        """
        if not context.conversation_history:
            return []

        # Convert to Agno message format
        messages = []
        for msg in context.conversation_history:
            role = msg.get("role", "user")
            content = msg.get("content", "")

            # Agno uses 'user' and 'assistant' roles
            if role in ["user", "assistant"]:
                messages.append({"role": role, "content": content})

        return messages

    def _extract_usage(self, result: Any) -> Dict[str, Any]:
        """
        Extract usage metrics from Agno result.

        Args:
            result: Agno run result

        Returns:
            Usage metrics dict
        """
        usage = {}
        if hasattr(result, "metrics") and result.metrics:
            metrics = result.metrics
            usage = {
                "prompt_tokens": getattr(metrics, "input_tokens", 0),
                "completion_tokens": getattr(metrics, "output_tokens", 0),
                "total_tokens": getattr(metrics, "total_tokens", 0),
            }
        return usage

    def _extract_tool_messages(self, result: Any) -> list:
        """
        Extract tool messages from Agno result.

        Args:
            result: Agno run result

        Returns:
            List of tool message dicts
        """
        tool_messages = []

        # Check if result has messages attribute
        if hasattr(result, "messages") and result.messages:
            for msg in result.messages:
                if hasattr(msg, "role") and msg.role == "tool":
                    tool_messages.append(
                        {
                            "role": "tool",
                            "content": getattr(msg, "content", ""),
                            "tool_use_id": getattr(msg, "tool_use_id", None),
                        }
                    )

        return tool_messages
