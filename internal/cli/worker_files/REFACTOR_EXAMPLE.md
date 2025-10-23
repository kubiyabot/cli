# Worker Refactor Example

## ❌ BEFORE: Monolithic Activity (500+ lines)

```python
# agent_activities.py - OLD
@activity.defn(name="execute_agent")
async def execute_agent(input: ActivityExecuteAgentInput) -> dict:
    """
    500+ lines of:
    - Session loading with retry logic
    - Agent creation
    - Toolset instantiation
    - Streaming logic
    - Session persistence with retry
    - Cancellation registration
    - Error handling
    - Logging
    """
    # Massive function with all logic mixed together
    ...
```

## ✅ AFTER: Thin Activity Wrapper (~50 lines)

```python
# activities/agent_activities.py - NEW
from temporalio import activity
from models.inputs import AgentExecutionInput
from services.agent_executor import AgentExecutorService
from services.session_service import SessionService
from services.cancellation_manager import cancellation_manager
from control_plane_client import get_control_plane_client


@activity.defn(name="execute_agent")
async def execute_agent(input: AgentExecutionInput) -> dict:
    """
    Execute an agent with full session management and cancellation support.

    This is a thin wrapper around AgentExecutorService which contains
    the actual business logic.
    """
    try:
        # Initialize services
        control_plane = get_control_plane_client()
        session_service = SessionService(control_plane)
        executor = AgentExecutorService(
            control_plane=control_plane,
            session_service=session_service,
            cancellation_manager=cancellation_manager
        )

        # Execute (all business logic in service)
        result = await executor.execute(input)

        return result

    except Exception as e:
        activity.logger.error(
            "agent_execution_failed",
            extra={"execution_id": input.execution_id, "error": str(e)}
        )
        raise


@activity.defn(name="cancel_agent_run")
async def cancel_agent_run(input: CancelExecutionInput) -> dict:
    """Cancel an active agent run."""
    return cancellation_manager.cancel(input.execution_id)
```

## 📊 Comparison

| Aspect | Before | After |
|--------|--------|-------|
| **Activity Size** | 500+ lines | ~50 lines |
| **Testable** | ❌ Hard (Temporal required) | ✅ Easy (test services) |
| **Reusable** | ❌ No | ✅ Yes (services) |
| **Maintainable** | ❌ Complex | ✅ Clean |
| **Separation** | ❌ Mixed | ✅ Clear layers |

## 🏗️ New Structure

```
activities/
  agent_activities.py         # 50 lines - just Temporal wrappers
  team_activities.py          # 50 lines - just Temporal wrappers
  shared_activities.py        # 30 lines - status updates

services/
  agent_executor.py           # 200 lines - agent execution logic
  team_executor.py            # 250 lines - team execution logic
  session_service.py          # 150 lines - session management
  cancellation_manager.py     # 120 lines - cancellation logic
  toolset_factory.py          # 100 lines - toolset creation

models/
  inputs.py                   # 80 lines - all dataclasses

utils/
  retry_utils.py              # 60 lines - retry decorator
  streaming_utils.py          # 80 lines - streaming helpers
```

## 🎯 Benefits

1. **Activities are Temporal boilerplate only** - Just wire services together
2. **Services are pure business logic** - Easily testable, no Temporal dependency
3. **Single Responsibility** - Each service does ONE thing well
4. **Reusable** - Services can be used by multiple activities
5. **Maintainable** - Changes are isolated to specific services
6. **Clean Architecture** - Clear separation of concerns

## 🔄 Migration Path

1. ✅ Create new structure (services/, models/, utils/)
2. ✅ Extract SessionService
3. ✅ Extract CancellationManager
4. 🔄 Extract AgentExecutorService (next)
5. 🔄 Extract TeamExecutorService (next)
6. 🔄 Extract ToolsetFactory (next)
7. 🔄 Refactor activities to use services
8. 🔄 Remove old monolithic code
9. ✅ Add tests for services
