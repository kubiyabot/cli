"""
Runtime abstraction layer for agent execution.

This package provides a pluggable runtime system that allows agents to be
powered by different frameworks (Agno, Claude Code SDK, etc.) without changing
the core workflow and activity logic.
"""

from .base import (
    RuntimeType,
    RuntimeExecutionResult,
    RuntimeExecutionContext,
    AgentRuntime,
)
from .factory import RuntimeFactory
from .default_runtime import DefaultRuntime
from .claude_code_runtime import ClaudeCodeRuntime

__all__ = [
    "RuntimeType",
    "RuntimeExecutionResult",
    "RuntimeExecutionContext",
    "AgentRuntime",
    "RuntimeFactory",
    "DefaultRuntime",
    "ClaudeCodeRuntime",
]
