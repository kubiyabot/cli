"""
Base runtime protocol and supporting types.

This module defines the core interface that all runtime implementations must satisfy.
"""

from typing import Protocol, AsyncIterator, Dict, Any, Optional, List, Callable
from dataclasses import dataclass, field
from enum import Enum


class RuntimeType(str, Enum):
    """Enumeration of supported runtime types."""

    DEFAULT = "default"  # Agno-based runtime (current implementation)
    CLAUDE_CODE = "claude_code"  # Claude Code SDK runtime


@dataclass
class RuntimeExecutionResult:
    """
    Standardized result structure from any runtime.

    This ensures all runtimes return consistent data structures that can
    be consumed by the workflow and activity layers.
    """

    response: str
    """The main response text from the agent."""

    usage: Dict[str, Any]
    """Token usage metrics (input_tokens, output_tokens, total_tokens, etc.)."""

    success: bool
    """Whether the execution succeeded."""

    finish_reason: Optional[str] = None
    """Reason the execution finished (e.g., 'stop', 'length', 'tool_use')."""

    run_id: Optional[str] = None
    """Unique identifier for this execution run."""

    model: Optional[str] = None
    """Model identifier used for this execution."""

    tool_execution_messages: Optional[List[Dict]] = None
    """
    Tool execution messages for UI display.
    Format: [{"tool": "Bash", "input": {...}, "output": {...}}, ...]
    """

    tool_messages: Optional[List[Dict]] = None
    """
    Detailed tool messages with execution metadata.
    Format: [{"role": "tool", "content": "...", "tool_use_id": "..."}, ...]
    """

    error: Optional[str] = None
    """Error message if execution failed."""

    metadata: Dict[str, Any] = field(default_factory=dict)
    """Additional runtime-specific metadata."""


@dataclass
class RuntimeExecutionContext:
    """
    Context passed to runtime for execution.

    This contains all the information needed for an agent to execute,
    regardless of which runtime implementation is used.
    """

    execution_id: str
    """Unique identifier for this execution."""

    agent_id: str
    """Unique identifier for the agent being executed."""

    organization_id: str
    """Organization context for this execution."""

    prompt: str
    """User's input prompt/message."""

    system_prompt: Optional[str] = None
    """System-level instructions for the agent."""

    conversation_history: List[Dict[str, Any]] = field(default_factory=list)
    """
    Previous conversation messages.
    Format: [{"role": "user|assistant|system", "content": "..."}, ...]
    """

    model_id: Optional[str] = None
    """LiteLLM model identifier (e.g., 'gpt-4', 'claude-3-opus')."""

    model_config: Optional[Dict[str, Any]] = None
    """Model-specific configuration (temperature, top_p, etc.)."""

    agent_config: Optional[Dict[str, Any]] = None
    """Agent-specific configuration."""

    toolsets: List[Any] = field(default_factory=list)
    """Resolved toolsets available to the agent."""

    mcp_servers: Optional[Dict[str, Any]] = None
    """MCP server configurations."""

    user_metadata: Optional[Dict[str, Any]] = None
    """User-provided metadata for this execution."""

    runtime_config: Optional[Dict[str, Any]] = None
    """Runtime-specific configuration options."""


class AgentRuntime(Protocol):
    """
    Protocol that all agent runtimes must implement.

    This defines the contract between the workflow/activity layer and
    the runtime implementation. Any class implementing this protocol
    can be used as an agent runtime.
    """

    async def execute(
        self, context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """
        Execute agent with the given context synchronously.

        Args:
            context: Execution context with prompt, history, config

        Returns:
            RuntimeExecutionResult with response and metadata

        Raises:
            Exception: If execution fails critically
        """
        ...

    async def stream_execute(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable[[Dict], None]] = None,
    ) -> AsyncIterator[RuntimeExecutionResult]:
        """
        Execute agent with streaming responses.

        This method yields results incrementally as they become available,
        enabling real-time UI updates and better user experience.

        Args:
            context: Execution context
            event_callback: Optional callback for real-time events

        Yields:
            RuntimeExecutionResult chunks as they arrive

        Raises:
            Exception: If execution fails critically
        """
        ...

    async def cancel(self, execution_id: str) -> bool:
        """
        Cancel an in-progress execution.

        Args:
            execution_id: ID of execution to cancel

        Returns:
            True if cancellation succeeded, False otherwise
        """
        ...

    async def get_usage(self, execution_id: str) -> Dict[str, Any]:
        """
        Get usage metrics for an execution.

        Args:
            execution_id: ID of execution

        Returns:
            Usage metrics dict (tokens, cost, etc.)
        """
        ...

    def supports_streaming(self) -> bool:
        """
        Whether this runtime supports streaming execution.

        Returns:
            True if streaming is supported
        """
        ...

    def supports_tools(self) -> bool:
        """
        Whether this runtime supports tool calling.

        Returns:
            True if tools are supported
        """
        ...

    def supports_mcp(self) -> bool:
        """
        Whether this runtime supports MCP servers.

        Returns:
            True if MCP is supported
        """
        ...

    def get_runtime_type(self) -> RuntimeType:
        """
        Return the runtime type.

        Returns:
            RuntimeType enum value
        """
        ...

    def get_runtime_info(self) -> Dict[str, Any]:
        """
        Get information about this runtime implementation.

        Returns:
            Dict with version, capabilities, etc.
        """
        ...
