# Runtime Abstraction Architecture

## Overview

This document describes the architecture for making the worker code runtime-agnostic, enabling support for multiple agent frameworks beyond Agno. The design uses well-established patterns to ensure extensibility, maintainability, and separation of concerns.

## Goals

1. **Framework Agnostic**: Support multiple agent SDK implementations (Agno, Claude Code, future frameworks)
2. **Extensible**: Easy to add new runtime implementations without modifying existing code
3. **Backward Compatible**: Existing Agno-based agents continue to work without changes
4. **Type Safe**: Strong typing and clear contracts between components
5. **Testable**: Clear interfaces enable easy unit and integration testing
6. **Mixed Teams**: Support teams with agents using different runtimes

## Architecture Patterns

### 1. Strategy Pattern (Runtime Selection)

Each runtime implements a common interface, allowing runtime selection at execution time without changing calling code.

### 2. Factory Pattern (Runtime Instantiation)

A `RuntimeFactory` creates runtime instances based on agent configuration, centralizing instantiation logic.

### 3. Adapter Pattern (Framework Integration)

Each runtime adapter wraps a specific framework (Agno, Claude Code SDK) and translates to a common interface.

### 4. Dependency Injection (Service Layer)

Services receive runtime instances rather than creating them, enabling testing and flexibility.

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Control Plane (Database)                     │
│  Agent Model: { id, name, runtime: "default|claude_code", ... } │
└─────────────────────────────────┬───────────────────────────────┘
                                  │ API
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Worker Process                           │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              AgentExecutionWorkflow                        │  │
│  │  (Temporal Workflow - orchestration)                       │  │
│  └───────────────────────┬───────────────────────────────────┘  │
│                          │ calls                                 │
│                          ▼                                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │           execute_agent_llm Activity                       │  │
│  └───────────────────────┬───────────────────────────────────┘  │
│                          │ delegates                             │
│                          ▼                                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │           AgentExecutorService                             │  │
│  │  - Manages session, cancellation, streaming                │  │
│  │  - Delegates execution to RuntimeAdapter                   │  │
│  └───────────────────────┬───────────────────────────────────┘  │
│                          │ uses                                  │
│                          ▼                                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │           RuntimeFactory                                   │  │
│  │  create_runtime(agent_config) -> AgentRuntime             │  │
│  └───────────────────────┬───────────────────────────────────┘  │
│                          │ returns                               │
│                          ▼                                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │     AgentRuntime Protocol (Interface)                      │  │
│  │  - execute()                                               │  │
│  │  - stream_execute()                                        │  │
│  │  - cancel()                                                │  │
│  │  - get_usage()                                             │  │
│  └───────────────┬──────────────────────┬────────────────────┘  │
│                  │                       │                       │
│        ┌─────────▼────────┐    ┌────────▼────────────┐         │
│        │  DefaultRuntime  │    │ ClaudeCodeRuntime   │         │
│        │  (Agno Adapter)  │    │ (Claude SDK Adapter)│         │
│        └─────────┬────────┘    └────────┬────────────┘         │
│                  │                       │                       │
│          ┌───────▼────────┐      ┌──────▼──────────┐           │
│          │  Agno Framework│      │  Claude Code SDK │           │
│          │  (Python lib)  │      │  (Python lib)    │           │
│          └────────────────┘      └──────────────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

## Runtime Type Enumeration

```python
from enum import Enum

class RuntimeType(str, Enum):
    DEFAULT = "default"          # Agno-based (current implementation)
    CLAUDE_CODE = "claude_code"  # Claude Code SDK
    # Future: LANGCHAIN = "langchain", LLAMAINDEX = "llamaindex", etc.
```

## Core Interfaces

### 1. AgentRuntime Protocol

The base protocol that all runtime implementations must satisfy:

```python
from typing import Protocol, AsyncIterator, Dict, Any, Optional, List
from dataclasses import dataclass

@dataclass
class RuntimeExecutionResult:
    """Standardized result structure from any runtime."""
    response: str
    usage: Dict[str, Any]  # Token usage metrics
    success: bool
    finish_reason: Optional[str] = None
    run_id: Optional[str] = None
    model: Optional[str] = None
    tool_execution_messages: Optional[List[Dict]] = None
    tool_messages: Optional[List[Dict]] = None
    error: Optional[str] = None

@dataclass
class RuntimeExecutionContext:
    """Context passed to runtime for execution."""
    execution_id: str
    agent_id: str
    organization_id: str
    prompt: str
    system_prompt: Optional[str]
    conversation_history: List[Dict[str, Any]]
    model_id: Optional[str]
    model_config: Optional[Dict[str, Any]]
    agent_config: Optional[Dict[str, Any]]
    toolsets: List[Any]  # Resolved toolsets
    mcp_servers: Optional[Dict[str, Any]]
    user_metadata: Optional[Dict[str, Any]]

class AgentRuntime(Protocol):
    """Protocol that all agent runtimes must implement."""

    async def execute(
        self,
        context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """Execute agent with the given context synchronously.

        Args:
            context: Execution context with prompt, history, config

        Returns:
            RuntimeExecutionResult with response and metadata
        """
        ...

    async def stream_execute(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable[[Dict], None]] = None
    ) -> AsyncIterator[RuntimeExecutionResult]:
        """Execute agent with streaming responses.

        Args:
            context: Execution context
            event_callback: Optional callback for real-time events

        Yields:
            RuntimeExecutionResult chunks as they arrive
        """
        ...

    async def cancel(self, execution_id: str) -> bool:
        """Cancel an in-progress execution.

        Args:
            execution_id: ID of execution to cancel

        Returns:
            True if cancellation succeeded
        """
        ...

    async def get_usage(self, execution_id: str) -> Dict[str, Any]:
        """Get usage metrics for an execution.

        Args:
            execution_id: ID of execution

        Returns:
            Usage metrics dict
        """
        ...

    def supports_streaming(self) -> bool:
        """Whether this runtime supports streaming."""
        ...

    def supports_tools(self) -> bool:
        """Whether this runtime supports tool calling."""
        ...

    def get_runtime_type(self) -> RuntimeType:
        """Return the runtime type."""
        ...
```

### 2. RuntimeFactory

Centralizes runtime creation logic:

```python
from typing import Optional
import structlog

logger = structlog.get_logger(__name__)

class RuntimeFactory:
    """Factory for creating runtime instances based on agent configuration."""

    @staticmethod
    def create_runtime(
        runtime_type: RuntimeType,
        control_plane_client: 'ControlPlaneClient',
        cancellation_manager: 'CancellationManager',
        **kwargs
    ) -> AgentRuntime:
        """Create a runtime instance.

        Args:
            runtime_type: Type of runtime to create
            control_plane_client: Client for Control Plane API
            cancellation_manager: Manager for execution cancellation
            **kwargs: Additional runtime-specific configuration

        Returns:
            AgentRuntime instance

        Raises:
            ValueError: If runtime_type is not supported
        """
        logger.info("Creating runtime", runtime_type=runtime_type)

        if runtime_type == RuntimeType.DEFAULT:
            return DefaultRuntime(
                control_plane_client=control_plane_client,
                cancellation_manager=cancellation_manager,
                **kwargs
            )
        elif runtime_type == RuntimeType.CLAUDE_CODE:
            return ClaudeCodeRuntime(
                control_plane_client=control_plane_client,
                cancellation_manager=cancellation_manager,
                **kwargs
            )
        else:
            raise ValueError(f"Unsupported runtime type: {runtime_type}")

    @staticmethod
    def get_default_runtime_type() -> RuntimeType:
        """Get the default runtime type."""
        return RuntimeType.DEFAULT

    @staticmethod
    def get_supported_runtimes() -> List[RuntimeType]:
        """Get list of supported runtimes."""
        return [RuntimeType.DEFAULT, RuntimeType.CLAUDE_CODE]
```

## Runtime Implementations

### 1. DefaultRuntime (Agno Adapter)

Wraps the existing Agno-based implementation:

```python
class DefaultRuntime:
    """Runtime implementation using Agno framework."""

    def __init__(
        self,
        control_plane_client: 'ControlPlaneClient',
        cancellation_manager: 'CancellationManager',
        **kwargs
    ):
        self.control_plane_client = control_plane_client
        self.cancellation_manager = cancellation_manager
        self.logger = structlog.get_logger(__name__)

    async def execute(
        self,
        context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """Execute using Agno framework (current implementation)."""
        try:
            # Import Agno
            from agno import Agent

            # Build conversation context
            messages = self._build_messages(context)

            # Create Agno agent
            agent = self._create_agno_agent(context)

            # Execute
            response = agent.run(messages)

            # Extract usage
            usage = self._extract_usage(response)

            return RuntimeExecutionResult(
                response=response.content,
                usage=usage,
                success=True,
                finish_reason=response.stop_reason,
                run_id=response.run_id,
                model=context.model_id,
                tool_execution_messages=self._extract_tool_messages(response),
                tool_messages=self._extract_detailed_tool_messages(response)
            )
        except Exception as e:
            self.logger.error("Agno execution failed", error=str(e))
            return RuntimeExecutionResult(
                response="",
                usage={},
                success=False,
                error=str(e)
            )

    async def stream_execute(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable[[Dict], None]] = None
    ) -> AsyncIterator[RuntimeExecutionResult]:
        """Execute with streaming using Agno."""
        # Current streaming implementation
        pass

    async def cancel(self, execution_id: str) -> bool:
        """Cancel via CancellationManager."""
        return self.cancellation_manager.cancel(execution_id)

    def supports_streaming(self) -> bool:
        return True

    def supports_tools(self) -> bool:
        return True

    def get_runtime_type(self) -> RuntimeType:
        return RuntimeType.DEFAULT

    # Private helper methods
    def _create_agno_agent(self, context: RuntimeExecutionContext) -> Any:
        """Create Agno agent instance."""
        pass

    def _build_messages(self, context: RuntimeExecutionContext) -> List[Dict]:
        """Build message history for Agno."""
        pass
```

### 2. ClaudeCodeRuntime Adapter

Integrates Claude Code SDK:

```python
class ClaudeCodeRuntime:
    """Runtime implementation using Claude Code SDK."""

    def __init__(
        self,
        control_plane_client: 'ControlPlaneClient',
        cancellation_manager: 'CancellationManager',
        **kwargs
    ):
        self.control_plane_client = control_plane_client
        self.cancellation_manager = cancellation_manager
        self.logger = structlog.get_logger(__name__)
        self._active_clients: Dict[str, 'ClaudeSDKClient'] = {}

    async def execute(
        self,
        context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """Execute using Claude Code SDK."""
        try:
            from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions

            # Build options
            options = self._build_claude_options(context)

            # Create client
            client = ClaudeSDKClient(options=options)
            self._active_clients[context.execution_id] = client

            # Connect and query
            await client.connect()
            await client.query(context.prompt)

            # Collect response
            response_text = ""
            usage = {}
            tool_messages = []

            async for message in client.receive_response():
                if hasattr(message, 'content'):
                    for block in message.content:
                        if hasattr(block, 'text'):
                            response_text += block.text
                        elif hasattr(block, 'name'):  # Tool use
                            tool_messages.append({
                                "tool": block.name,
                                "input": block.input
                            })

                if hasattr(message, 'usage'):
                    usage = message.usage

            # Disconnect
            await client.disconnect()
            del self._active_clients[context.execution_id]

            return RuntimeExecutionResult(
                response=response_text,
                usage=usage,
                success=True,
                tool_messages=tool_messages,
                model=context.model_id
            )

        except Exception as e:
            self.logger.error("Claude Code execution failed", error=str(e))
            return RuntimeExecutionResult(
                response="",
                usage={},
                success=False,
                error=str(e)
            )

    async def stream_execute(
        self,
        context: RuntimeExecutionContext,
        event_callback: Optional[Callable[[Dict], None]] = None
    ) -> AsyncIterator[RuntimeExecutionResult]:
        """Execute with streaming using Claude Code SDK."""
        from claude_agent_sdk import ClaudeSDKClient

        client = ClaudeSDKClient(options=self._build_claude_options(context))
        self._active_clients[context.execution_id] = client

        await client.connect()
        await client.query(context.prompt)

        async for message in client.receive_messages():
            # Yield incremental results
            if hasattr(message, 'content'):
                for block in message.content:
                    if hasattr(block, 'text'):
                        yield RuntimeExecutionResult(
                            response=block.text,
                            usage={},
                            success=True
                        )

            # Publish event if callback provided
            if event_callback:
                event_callback(self._message_to_event(message))

        await client.disconnect()
        del self._active_clients[context.execution_id]

    async def cancel(self, execution_id: str) -> bool:
        """Cancel via ClaudeSDKClient interrupt."""
        if execution_id in self._active_clients:
            client = self._active_clients[execution_id]
            await client.interrupt()
            return True
        return False

    def supports_streaming(self) -> bool:
        return True

    def supports_tools(self) -> bool:
        return True  # Claude Code supports tools

    def get_runtime_type(self) -> RuntimeType:
        return RuntimeType.CLAUDE_CODE

    # Private helper methods
    def _build_claude_options(self, context: RuntimeExecutionContext) -> Any:
        """Build ClaudeAgentOptions from context."""
        from claude_agent_sdk import ClaudeAgentOptions

        # Convert toolsets to allowed_tools
        allowed_tools = self._extract_tool_names(context.toolsets)

        return ClaudeAgentOptions(
            system_prompt=context.system_prompt,
            allowed_tools=allowed_tools,
            mcp_servers=context.mcp_servers or {},
            permission_mode="acceptEdits",  # Or from config
            cwd=context.agent_config.get("cwd") if context.agent_config else None,
            model=context.model_id
        )

    def _extract_tool_names(self, toolsets: List[Any]) -> List[str]:
        """Extract tool names from toolsets."""
        # Map toolsets to Claude Code tool names
        tool_mapping = {
            "shell": ["Bash"],
            "file_system": ["Read", "Write", "Edit", "Glob"],
            "web": ["WebFetch", "WebSearch"],
            # Add more mappings
        }

        tools = []
        for toolset in toolsets:
            toolset_type = getattr(toolset, 'type', None)
            if toolset_type in tool_mapping:
                tools.extend(tool_mapping[toolset_type])

        return tools

    def _message_to_event(self, message) -> Dict:
        """Convert Claude SDK message to Control Plane event."""
        return {
            "type": "message",
            "content": str(message)
        }
```

## Service Layer Integration

### Updated AgentExecutorService

```python
class AgentExecutorService:
    """Service for executing individual agents using runtime abstraction."""

    def __init__(
        self,
        control_plane_client: ControlPlaneClient,
        session_service: SessionService,
        cancellation_manager: CancellationManager
    ):
        self.control_plane_client = control_plane_client
        self.session_service = session_service
        self.cancellation_manager = cancellation_manager
        self.runtime_factory = RuntimeFactory()
        self.logger = structlog.get_logger(__name__)

    async def execute(
        self,
        input_data: AgentExecutionInput
    ) -> Dict[str, Any]:
        """Execute agent using configured runtime.

        This method now delegates to the appropriate runtime adapter
        based on the agent's runtime configuration.
        """
        try:
            # Load session history
            session = await self.session_service.load_session(
                input_data.execution_id,
                input_data.agent_id
            )

            # Get agent configuration (includes runtime type)
            agent_config = input_data.agent_config or {}
            runtime_type_str = agent_config.get("runtime", "default")
            runtime_type = RuntimeType(runtime_type_str)

            self.logger.info(
                "Executing agent with runtime",
                runtime=runtime_type,
                agent_id=input_data.agent_id
            )

            # Create runtime
            runtime = self.runtime_factory.create_runtime(
                runtime_type=runtime_type,
                control_plane_client=self.control_plane_client,
                cancellation_manager=self.cancellation_manager
            )

            # Get toolsets (if runtime supports tools)
            toolsets = []
            if runtime.supports_tools():
                toolsets = await self._get_toolsets(input_data.agent_id)

            # Build execution context
            context = RuntimeExecutionContext(
                execution_id=input_data.execution_id,
                agent_id=input_data.agent_id,
                organization_id=input_data.organization_id,
                prompt=input_data.prompt,
                system_prompt=input_data.system_prompt,
                conversation_history=session.messages,
                model_id=input_data.model_id,
                model_config=input_data.model_config,
                agent_config=agent_config,
                toolsets=toolsets,
                mcp_servers=input_data.mcp_servers,
                user_metadata=input_data.user_metadata
            )

            # Execute via runtime (with streaming if supported)
            if runtime.supports_streaming():
                result = await self._execute_streaming(runtime, context)
            else:
                result = await runtime.execute(context)

            # Persist session
            if result.success:
                await self.session_service.persist_session(
                    execution_id=input_data.execution_id,
                    messages=[
                        {"role": "user", "content": input_data.prompt},
                        {"role": "assistant", "content": result.response}
                    ]
                )

            # Return standardized result
            return {
                "response": result.response,
                "usage": result.usage,
                "success": result.success,
                "finish_reason": result.finish_reason,
                "run_id": result.run_id,
                "model": result.model,
                "tool_execution_messages": result.tool_execution_messages or [],
                "tool_messages": result.tool_messages or [],
                "runtime_type": runtime.get_runtime_type().value
            }

        except Exception as e:
            self.logger.error("Agent execution failed", error=str(e))
            return {
                "response": "",
                "usage": {},
                "success": False,
                "error": str(e)
            }

    async def _execute_streaming(
        self,
        runtime: AgentRuntime,
        context: RuntimeExecutionContext
    ) -> RuntimeExecutionResult:
        """Execute with streaming and publish events."""
        accumulated_response = ""
        accumulated_usage = {}
        tool_messages = []

        async for chunk in runtime.stream_execute(
            context,
            event_callback=self._publish_event
        ):
            accumulated_response += chunk.response
            if chunk.usage:
                accumulated_usage.update(chunk.usage)
            if chunk.tool_messages:
                tool_messages.extend(chunk.tool_messages)

        return RuntimeExecutionResult(
            response=accumulated_response,
            usage=accumulated_usage,
            success=True,
            tool_messages=tool_messages
        )

    def _publish_event(self, event: Dict):
        """Publish event to Control Plane for UI updates."""
        self.control_plane_client.publish_event(event)

    async def _get_toolsets(self, agent_id: str) -> List[Any]:
        """Get resolved toolsets for agent."""
        # Call Control Plane API
        pass
```

### Updated TeamExecutorService

```python
class TeamExecutorService:
    """Service for executing teams with mixed runtime agents."""

    async def execute(
        self,
        input_data: TeamExecutionInput,
        agents: List[Dict[str, Any]]
    ) -> Dict[str, Any]:
        """Execute team with support for mixed runtimes.

        Teams can now have agents using different runtimes.
        The team coordination logic remains the same, but individual
        agent executions are delegated to their respective runtimes.
        """
        # Check if all agents use same runtime
        runtime_types = set(
            agent.get("runtime", "default") for agent in agents
        )

        if len(runtime_types) == 1 and "default" in runtime_types:
            # All agents use Agno - use existing team coordination
            return await self._execute_agno_team(input_data, agents)
        else:
            # Mixed runtimes - use sequential orchestration
            return await self._execute_mixed_team(input_data, agents)

    async def _execute_mixed_team(
        self,
        input_data: TeamExecutionInput,
        agents: List[Dict[str, Any]]
    ) -> Dict[str, Any]:
        """Execute team with mixed runtimes using orchestration pattern."""
        # Delegate to a "team leader" agent that coordinates
        # by invoking individual agents via RuntimeFactory
        pass
```

## Database Schema Changes

### Control Plane: Agent Model Update

```python
# In /app/models/agent.py

class Agent(Base):
    __tablename__ = "agents"

    # ... existing fields ...

    runtime = Column(
        String,
        nullable=False,
        default="default",
        server_default="default"
    )
    """Runtime type for agent execution.

    Values:
        - "default": Agno-based runtime (current implementation)
        - "claude_code": Claude Code SDK runtime
    """
```

### Migration Script

```python
"""Add runtime field to agents

Revision ID: add_runtime_field
"""
from alembic import op
import sqlalchemy as sa

def upgrade():
    op.add_column(
        'agents',
        sa.Column(
            'runtime',
            sa.String(),
            nullable=False,
            server_default='default'
        )
    )

def downgrade():
    op.drop_column('agents', 'runtime')
```

## API Changes

### Agent Creation/Update Endpoints

```python
# In /app/routers/agents.py

class AgentCreateInput(BaseModel):
    name: str
    description: Optional[str]
    runtime: RuntimeType = RuntimeType.DEFAULT  # New field
    # ... other fields ...

class AgentResponse(BaseModel):
    id: str
    name: str
    runtime: RuntimeType  # New field
    # ... other fields ...
```

## Testing Strategy

### 1. Unit Tests

```python
# tests/unit/test_runtimes.py

class TestDefaultRuntime:
    async def test_execute_success(self):
        """Test successful execution with Agno runtime."""
        pass

    async def test_execute_with_tools(self):
        """Test execution with toolset integration."""
        pass

    async def test_cancel_execution(self):
        """Test cancellation via CancellationManager."""
        pass

class TestClaudeCodeRuntime:
    async def test_execute_success(self):
        """Test successful execution with Claude Code SDK."""
        pass

    async def test_streaming_execution(self):
        """Test streaming execution."""
        pass

    async def test_interrupt(self):
        """Test interrupt via SDK client."""
        pass

class TestRuntimeFactory:
    def test_create_default_runtime(self):
        """Test factory creates DefaultRuntime correctly."""
        pass

    def test_create_claude_code_runtime(self):
        """Test factory creates ClaudeCodeRuntime correctly."""
        pass

    def test_unsupported_runtime_raises(self):
        """Test factory raises on unsupported runtime."""
        pass
```

### 2. Integration Tests

```python
# tests/integration/test_agent_execution_runtimes.py

class TestAgentExecutionWithRuntimes:
    async def test_execute_default_runtime_agent(self):
        """End-to-end test with Agno runtime."""
        pass

    async def test_execute_claude_code_runtime_agent(self):
        """End-to-end test with Claude Code runtime."""
        pass

    async def test_team_with_mixed_runtimes(self):
        """Test team with both runtime types."""
        pass
```

## Migration Path

### Phase 1: Infrastructure (Non-Breaking)
1. Add `runtime` field to Agent model with default value
2. Create runtime protocol and factory
3. Implement DefaultRuntime (wrapper around existing code)
4. Update AgentExecutorService to use runtime abstraction
5. Deploy - all existing agents continue working

### Phase 2: Claude Code Runtime
1. Implement ClaudeCodeRuntime adapter
2. Add Claude Code SDK dependency
3. Test with dedicated test agents
4. Deploy behind feature flag

### Phase 3: UI & Production
1. Add runtime selector to UI
2. Update API documentation
3. Enable for production agents
4. Monitor usage and performance

## Configuration Examples

### Agent with Default Runtime (Agno)

```json
{
  "id": "agent-123",
  "name": "Data Analyst",
  "runtime": "default",
  "model_id": "gpt-4",
  "configuration": {
    "temperature": 0.7
  }
}
```

### Agent with Claude Code Runtime

```json
{
  "id": "agent-456",
  "name": "Code Reviewer",
  "runtime": "claude_code",
  "model_id": "claude-sonnet-4",
  "configuration": {
    "permission_mode": "acceptEdits",
    "allowed_tools": ["Read", "Grep", "Glob"],
    "cwd": "/workspace/project"
  }
}
```

### Team with Mixed Runtimes

```json
{
  "id": "team-789",
  "name": "Full Stack Team",
  "agents": [
    {
      "id": "agent-123",
      "name": "Backend Developer",
      "runtime": "default"
    },
    {
      "id": "agent-456",
      "name": "Frontend Developer",
      "runtime": "claude_code"
    }
  ]
}
```

## Benefits

1. **Flexibility**: Choose best runtime for each agent's use case
2. **Future-Proof**: Easy to add new runtimes (LangChain, LlamaIndex, etc.)
3. **Performance**: Use specialized runtimes for specific tasks
4. **Testing**: Mock runtimes for testing without external dependencies
5. **Isolation**: Runtime failures don't affect other components
6. **Gradual Migration**: Migrate agents incrementally without downtime

## Next Steps

1. Review and approve architecture
2. Create database migration for `runtime` field
3. Implement runtime protocol and factory
4. Implement DefaultRuntime wrapper
5. Test with existing agents
6. Implement ClaudeCodeRuntime
7. Add UI support for runtime selection
8. Update documentation and examples
