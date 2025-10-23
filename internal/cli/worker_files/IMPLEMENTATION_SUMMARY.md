# Runtime Abstraction Implementation Summary

## Overview

This document summarizes the implementation of the runtime abstraction layer that makes the worker code agnostic to specific agent frameworks. The system now supports multiple runtimes:

- **Default (Agno)**: Current implementation using Agno framework
- **Claude Code**: New implementation using Claude Code SDK

## What Was Implemented

### 1. Database Schema Changes

**File**: `/db/migrations/010_add_runtime_to_agents.sql`

- Added `runtime` column to `agents` table
- Default value: `"default"` (Agno)
- Supported values: `"default"`, `"claude_code"`
- Created indexes for efficient queries
- Added check constraint for validation

**Migration Status**: Ready to run

```bash
# Apply migration
psql <database_url> < db/migrations/010_add_runtime_to_agents.sql
```

### 2. Control Plane Model Updates

**File**: `/app/models/agent.py`

- Added `RuntimeType` enum with `DEFAULT` and `CLAUDE_CODE` values
- Added `runtime` column to `Agent` model
- Backward compatible: defaults to `"default"`

### 3. Runtime Abstraction Layer

#### Base Protocol (`runtimes/base.py`)

Defines core interfaces and types:

- `RuntimeType`: Enum of supported runtimes
- `RuntimeExecutionResult`: Standardized result structure
- `RuntimeExecutionContext`: Input context for execution
- `AgentRuntime`: Protocol that all runtimes must implement

**Key Methods**:
```python
async def execute(context) -> RuntimeExecutionResult
async def stream_execute(context, callback) -> AsyncIterator[RuntimeExecutionResult]
async def cancel(execution_id) -> bool
def supports_streaming() -> bool
def supports_tools() -> bool
def get_runtime_type() -> RuntimeType
```

#### Runtime Factory (`runtimes/factory.py`)

Centralizes runtime creation:

```python
runtime = RuntimeFactory.create_runtime(
    runtime_type=RuntimeType.CLAUDE_CODE,
    control_plane_client=client,
    cancellation_manager=manager
)
```

**Features**:
- Type-safe runtime creation
- Default runtime selection
- Runtime validation
- Discovery of supported runtimes

### 4. Runtime Implementations

#### DefaultRuntime (`runtimes/default_runtime.py`)

Wraps existing Agno framework:

**Capabilities**:
- ✅ Streaming execution
- ✅ Tool calling via toolsets
- ✅ Cancellation via CancellationManager
- ✅ Session history integration
- ✅ Real-time event publishing
- ❌ No native MCP support

**Usage**:
```python
runtime = DefaultRuntime(control_plane_client, cancellation_manager)
result = await runtime.execute(context)
```

#### ClaudeCodeRuntime (`runtimes/claude_code_runtime.py`)

Integrates Claude Code SDK:

**Capabilities**:
- ✅ Streaming execution
- ✅ Tool calling (Read, Write, Bash, etc.)
- ✅ Cancellation via SDK interrupt
- ✅ Session history integration
- ✅ Real-time event publishing
- ✅ Native MCP support

**Toolset Mapping**:
```python
{
    "shell": ["Bash"],
    "file_system": ["Read", "Write", "Edit", "Glob", "Grep"],
    "web": ["WebFetch", "WebSearch"],
    "task": ["Task"],
}
```

**Usage**:
```python
runtime = ClaudeCodeRuntime(control_plane_client, cancellation_manager)
result = await runtime.execute(context)
```

### 5. Refactored Service Layer

#### AgentExecutorServiceV2 (`services/agent_executor_v2.py`)

New service implementation using runtime abstraction:

**Flow**:
1. Load session history
2. Determine runtime type from agent config
3. Create runtime instance via factory
4. Fetch toolsets if runtime supports tools
5. Build execution context
6. Execute via runtime (streaming or non-streaming)
7. Persist session history
8. Return standardized result

**Benefits**:
- Cleaner code: ~300 lines vs ~360 lines
- Runtime-agnostic: works with any runtime
- Easier testing: mock runtimes for tests
- Better separation of concerns

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                   Control Plane API                          │
│  Agent: { id, name, runtime: "default|claude_code", ... }   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│               Worker Process (Temporal)                      │
│                                                              │
│  AgentExecutionWorkflow                                      │
│         ↓                                                    │
│  execute_agent_llm Activity                                  │
│         ↓                                                    │
│  AgentExecutorServiceV2                                      │
│         ↓                                                    │
│  RuntimeFactory.create_runtime(type)                         │
│         ↓                                                    │
│  ┌──────────────────┬──────────────────┐                    │
│  │                  │                  │                    │
│  ▼                  ▼                  ▼                    │
│  DefaultRuntime     ClaudeCodeRuntime  [Future Runtimes]    │
│  (Agno)            (Claude SDK)                             │
│         ↓                  ↓                                 │
│  Agno Framework    Claude Code SDK                          │
└─────────────────────────────────────────────────────────────┘
```

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

**API Request**:
```http
POST /api/v1/agents
{
  "name": "Data Analyst",
  "runtime": "default",
  "model_id": "gpt-4"
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
    "runtime_config": {
      "permission_mode": "acceptEdits",
      "cwd": "/workspace/project",
      "max_turns": 10
    }
  }
}
```

**API Request**:
```http
POST /api/v1/agents
{
  "name": "Code Reviewer",
  "runtime": "claude_code",
  "model_id": "claude-sonnet-4",
  "configuration": {
    "runtime_config": {
      "permission_mode": "acceptEdits",
      "cwd": "/workspace/project"
    }
  }
}
```

## Integration Steps

### Phase 1: Database Migration (Zero Downtime)

1. **Apply Migration**:
   ```bash
   psql $DATABASE_URL < db/migrations/010_add_runtime_to_agents.sql
   ```

2. **Verify**:
   ```sql
   SELECT runtime, COUNT(*) FROM agents GROUP BY runtime;
   -- All existing agents should show "default"
   ```

### Phase 2: Install Dependencies

1. **Add Claude Code SDK**:
   ```bash
   cd cli/internal/cli/worker_files
   pip install claude-agent-sdk
   ```

2. **Update requirements.txt**:
   ```txt
   claude-agent-sdk>=0.1.0
   ```

### Phase 3: Deploy Worker Code

1. **Update worker imports** in `activities/agent_activities.py`:
   ```python
   # Change:
   from services.agent_executor import AgentExecutorService

   # To:
   from services.agent_executor_v2 import AgentExecutorServiceV2 as AgentExecutorService
   ```

2. **Deploy worker**:
   ```bash
   # Build new worker image
   docker build -t agent-worker:v2-runtime .

   # Deploy
   kubectl apply -f worker-deployment.yaml
   ```

### Phase 4: Update API Models

**File**: `app/routers/agents.py`

```python
from app.models.agent import RuntimeType

class AgentCreateInput(BaseModel):
    name: str
    description: Optional[str] = None
    runtime: RuntimeType = RuntimeType.DEFAULT  # Add this
    # ... other fields

class AgentResponse(BaseModel):
    id: str
    name: str
    runtime: RuntimeType  # Add this
    # ... other fields
```

### Phase 5: UI Updates

Add runtime selector to agent creation/edit forms:

```tsx
<select name="runtime">
  <option value="default">Default (Agno)</option>
  <option value="claude_code">Claude Code SDK</option>
</select>
```

## Testing

### Unit Tests

**File**: `tests/unit/test_runtimes.py`

```python
import pytest
from runtimes import RuntimeFactory, RuntimeType, RuntimeExecutionContext

class TestRuntimeFactory:
    def test_create_default_runtime(self):
        runtime = RuntimeFactory.create_runtime(
            RuntimeType.DEFAULT,
            mock_control_plane,
            mock_cancellation_manager
        )
        assert runtime.get_runtime_type() == RuntimeType.DEFAULT
        assert runtime.supports_streaming()

    def test_create_claude_code_runtime(self):
        runtime = RuntimeFactory.create_runtime(
            RuntimeType.CLAUDE_CODE,
            mock_control_plane,
            mock_cancellation_manager
        )
        assert runtime.get_runtime_type() == RuntimeType.CLAUDE_CODE
        assert runtime.supports_mcp()

class TestDefaultRuntime:
    @pytest.mark.asyncio
    async def test_execute_success(self):
        runtime = DefaultRuntime(mock_control_plane, mock_cancellation_manager)
        context = RuntimeExecutionContext(
            execution_id="test-123",
            agent_id="agent-123",
            organization_id="org-123",
            prompt="Hello, world!",
            # ... other fields
        )

        result = await runtime.execute(context)

        assert result.success
        assert result.response
        assert result.usage

class TestClaudeCodeRuntime:
    @pytest.mark.asyncio
    async def test_execute_with_tools(self):
        runtime = ClaudeCodeRuntime(mock_control_plane, mock_cancellation_manager)
        context = RuntimeExecutionContext(
            execution_id="test-456",
            agent_id="agent-456",
            organization_id="org-123",
            prompt="List files in current directory",
            toolsets=[{"type": "file_system"}],
        )

        result = await runtime.execute(context)

        assert result.success
        assert result.tool_messages  # Should have used Bash or Read tool
```

### Integration Tests

**File**: `tests/integration/test_agent_execution_runtimes.py`

```python
@pytest.mark.asyncio
async def test_execute_default_runtime_agent(control_plane, temporal_client):
    # Create agent with default runtime
    agent = await control_plane.create_agent(
        name="Test Agent",
        runtime="default"
    )

    # Start execution workflow
    result = await temporal_client.execute_workflow(
        "AgentExecutionWorkflow",
        input={
            "agent_id": agent.id,
            "prompt": "What is 2+2?",
            "runtime": "default"
        }
    )

    assert result["success"]
    assert "4" in result["response"]

@pytest.mark.asyncio
async def test_execute_claude_code_runtime_agent(control_plane, temporal_client):
    # Create agent with Claude Code runtime
    agent = await control_plane.create_agent(
        name="Code Agent",
        runtime="claude_code"
    )

    # Start execution workflow
    result = await temporal_client.execute_workflow(
        "AgentExecutionWorkflow",
        input={
            "agent_id": agent.id,
            "prompt": "List files in /tmp",
            "runtime": "claude_code"
        }
    )

    assert result["success"]
    assert result["tool_messages"]  # Should have tool usage
```

## Migration Path for Existing Agents

### Gradual Migration

1. **All existing agents automatically use "default" runtime** (no changes needed)

2. **Test new agents with Claude Code**:
   ```bash
   curl -X POST https://api.example.com/agents \
     -H "Content-Type: application/json" \
     -d '{
       "name": "Test Claude Code Agent",
       "runtime": "claude_code",
       "model_id": "claude-sonnet-4"
     }'
   ```

3. **Monitor performance** and compare metrics

4. **Migrate agents selectively**:
   ```bash
   curl -X PATCH https://api.example.com/agents/{agent_id} \
     -H "Content-Type: application/json" \
     -d '{"runtime": "claude_code"}'
   ```

### Rollback Plan

If issues arise:

1. **Change agent runtime back to "default"**:
   ```sql
   UPDATE agents SET runtime = 'default' WHERE runtime = 'claude_code';
   ```

2. **Deploy previous worker version** (no database changes needed)

## Benefits

### For Development

- **Easier Testing**: Mock runtimes for unit tests
- **Cleaner Code**: Separation of concerns
- **Better Debugging**: Clear boundaries between layers
- **Type Safety**: Protocol ensures runtime compatibility

### For Operations

- **Gradual Migration**: Test new runtimes without risk
- **Runtime Selection**: Choose best runtime per agent
- **Performance Optimization**: Specialized runtimes for specific tasks
- **Future-Proof**: Easy to add new runtimes (LangChain, LlamaIndex, etc.)

### For Users

- **More Choices**: Select runtime based on needs
- **Better Performance**: Specialized runtimes for specific tasks
- **Advanced Features**: Claude Code SDK for software engineering

## Performance Comparison

| Feature | Default (Agno) | Claude Code |
|---------|----------------|-------------|
| **Streaming** | ✅ Yes | ✅ Yes |
| **Tool Calling** | ✅ Yes (via toolsets) | ✅ Yes (native tools) |
| **MCP Support** | ❌ No | ✅ Yes |
| **Cancellation** | ✅ Yes (custom) | ✅ Yes (SDK interrupt) |
| **Session Restore** | ✅ Yes | ✅ Yes |
| **File Operations** | ⚠️ Via toolsets | ✅ Native |
| **Code Editing** | ⚠️ Via toolsets | ✅ Native |
| **Multi-Turn** | ✅ Yes | ✅ Yes |

## Future Enhancements

### Additional Runtimes

```python
class RuntimeType(str, Enum):
    DEFAULT = "default"
    CLAUDE_CODE = "claude_code"
    LANGCHAIN = "langchain"        # Future
    LLAMAINDEX = "llamaindex"      # Future
    AUTOGEN = "autogen"            # Future
    CREWAI = "crewai"              # Future
```

### Runtime Capabilities

Add capability queries:

```python
capabilities = runtime.get_capabilities()
# {
#   "streaming": True,
#   "tools": ["Bash", "Read", "Write"],
#   "mcp": True,
#   "max_context_tokens": 100000,
#   "supports_vision": True,
# }
```

### Runtime Metrics

Track runtime performance:

```python
metrics = await runtime.get_metrics(execution_id)
# {
#   "latency_ms": 1234,
#   "token_throughput": 45.2,
#   "tool_execution_count": 3,
#   "cache_hits": 5,
# }
```

## Troubleshooting

### Agent not using Claude Code runtime

**Check**:
```sql
SELECT id, name, runtime FROM agents WHERE id = 'agent-id';
```

**Fix**:
```sql
UPDATE agents SET runtime = 'claude_code' WHERE id = 'agent-id';
```

### Claude Code SDK not installed

**Error**: `ModuleNotFoundError: No module named 'claude_agent_sdk'`

**Fix**:
```bash
pip install claude-agent-sdk
```

### Runtime creation fails

**Error**: `ValueError: Unsupported runtime type: xyz`

**Check**:
- Verify runtime value in database is valid
- Check `RuntimeType` enum for supported values
- Look at worker logs for detailed error

## Documentation

### Architecture Documents

- [RUNTIME_ARCHITECTURE.md](./RUNTIME_ARCHITECTURE.md): Detailed architecture design
- [IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md): This document

### API Documentation

Update API docs to include runtime field:

```yaml
Agent:
  type: object
  properties:
    id:
      type: string
    name:
      type: string
    runtime:
      type: string
      enum: [default, claude_code]
      default: default
      description: Runtime type for agent execution
```

## Next Steps

1. **Review and approve** architecture design
2. **Apply database migration** to add runtime field
3. **Install Claude Code SDK** in worker environment
4. **Deploy updated worker** with runtime abstraction
5. **Test with sample agents** using both runtimes
6. **Update UI** to support runtime selection
7. **Create monitoring dashboard** for runtime metrics
8. **Write user documentation** on runtime selection
9. **Gradually migrate agents** to optimal runtimes

## Questions?

For questions or issues:
- Check architecture docs
- Review test examples
- Examine runtime implementations
- Look at agent executor service flow

---

**Implementation Date**: 2025-10-22
**Version**: 1.0
**Status**: Ready for Review
