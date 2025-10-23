"""
Refactored Agent executor service using runtime abstraction.

This version delegates execution to pluggable runtime implementations,
making the code more maintainable and extensible.
"""

from typing import Dict, Any, Optional
import structlog

from control_plane_client import ControlPlaneClient
from services.session_service import SessionService
from services.cancellation_manager import CancellationManager
from runtimes import (
    RuntimeFactory,
    RuntimeType,
    RuntimeExecutionContext,
)
from utils.streaming_utils import StreamingHelper

logger = structlog.get_logger()


class AgentExecutorServiceV2:
    """
    Service for executing agents using runtime abstraction.

    This service orchestrates agent execution by:
    1. Loading session history
    2. Selecting appropriate runtime based on agent config
    3. Delegating execution to the runtime
    4. Persisting session after execution
    """

    def __init__(
        self,
        control_plane: ControlPlaneClient,
        session_service: SessionService,
        cancellation_manager: CancellationManager,
    ):
        """
        Initialize the agent executor service.

        Args:
            control_plane: Control Plane API client
            session_service: Session management service
            cancellation_manager: Execution cancellation manager
        """
        self.control_plane = control_plane
        self.session_service = session_service
        self.cancellation_manager = cancellation_manager
        self.runtime_factory = RuntimeFactory()

    async def execute(self, input: Any) -> Dict[str, Any]:
        """
        Execute an agent using the configured runtime.

        This method:
        1. Loads session history
        2. Determines runtime type from agent config
        3. Creates runtime instance
        4. Executes agent via runtime
        5. Persists session
        6. Returns standardized result

        Args:
            input: AgentExecutionInput with execution details

        Returns:
            Dict with response, usage, success flag, runtime_type, etc.
        """
        execution_id = input.execution_id

        print("\n" + "=" * 80)
        print("🤖 AGENT EXECUTION START (Runtime-Abstracted)")
        print("=" * 80)
        print(f"Execution ID: {execution_id}")
        print(f"Agent ID: {input.agent_id}")
        print(f"Organization: {input.organization_id}")
        print(f"Model: {input.model_id or 'default'}")
        print(f"Session ID: {input.session_id}")
        print(
            f"Prompt: {input.prompt[:100]}..."
            if len(input.prompt) > 100
            else f"Prompt: {input.prompt}"
        )
        print("=" * 80 + "\n")

        logger.info(
            "agent_execution_start",
            execution_id=execution_id[:8],
            agent_id=input.agent_id,
            session_id=input.session_id,
        )

        try:
            # STEP 1: Load session history
            print("📚 Loading session history...")
            session_history = self.session_service.load_session(
                execution_id=execution_id, session_id=input.session_id
            )

            if session_history:
                print(
                    f"✅ Loaded {len(session_history)} messages from previous session\n"
                )
            else:
                print("ℹ️  Starting new conversation\n")

            # STEP 2: Determine runtime type
            agent_config = input.agent_config or {}
            runtime_type_str = agent_config.get("runtime", "default")
            runtime_type = self.runtime_factory.parse_runtime_type(runtime_type_str)

            print(f"🔌 Runtime Type: {runtime_type.value}")
            print(f"   Framework: {self._get_framework_name(runtime_type)}\n")

            logger.info(
                "runtime_selected",
                execution_id=execution_id[:8],
                runtime=runtime_type.value,
            )

            # STEP 3: Create runtime instance
            print(f"⚙️  Creating runtime instance...")
            runtime = self.runtime_factory.create_runtime(
                runtime_type=runtime_type,
                control_plane_client=self.control_plane,
                cancellation_manager=self.cancellation_manager,
            )
            print(f"✅ Runtime created: {runtime.get_runtime_info()}\n")

            # STEP 4: Get toolsets (if runtime supports tools)
            toolsets = []
            if runtime.supports_tools():
                print(f"🔧 Fetching toolsets from Control Plane...")
                try:
                    toolset_configs = self.control_plane.get_toolsets(input.agent_id)
                    if toolset_configs:
                        print(f"✅ Resolved {len(toolset_configs)} toolsets")
                        print(f"   Types: {[t.get('type') for t in toolset_configs]}")
                        print(
                            f"   Names: {[t.get('name') for t in toolset_configs]}\n"
                        )

                        # Import here to avoid circular dependency
                        from services.toolset_factory import ToolsetFactory

                        toolsets = ToolsetFactory.create_toolsets_from_list(
                            toolset_configs
                        )

                        if toolsets:
                            print(f"✅ Instantiated {len(toolsets)} toolset(s)\n")
                    else:
                        print(f"⚠️  No toolsets found\n")
                except Exception as e:
                    print(f"❌ Error fetching toolsets: {str(e)}\n")
                    logger.error("toolset_fetch_error", error=str(e))

            # STEP 5: Build execution context
            print("📦 Building execution context...")
            context = RuntimeExecutionContext(
                execution_id=execution_id,
                agent_id=input.agent_id,
                organization_id=input.organization_id,
                prompt=input.prompt,
                system_prompt=input.system_prompt,
                conversation_history=session_history,
                model_id=input.model_id,
                model_config=input.model_config,
                agent_config=agent_config,
                toolsets=toolsets,
                mcp_servers=input.mcp_servers,
                user_metadata=input.user_metadata,
                runtime_config=agent_config.get("runtime_config"),
            )
            print("✅ Context ready\n")

            # STEP 6: Execute via runtime (with streaming if supported)
            print("⚡ Executing via runtime...\n")

            if runtime.supports_streaming():
                result = await self._execute_streaming(runtime, context, input)
            else:
                result = await runtime.execute(context)

            print("\n✅ Runtime execution completed!")
            print(f"   Response Length: {len(result.response)} chars")
            print(f"   Success: {result.success}\n")

            logger.info(
                "agent_execution_completed",
                execution_id=execution_id[:8],
                success=result.success,
                response_length=len(result.response),
            )

            # STEP 7: Persist session
            if result.success and result.response:
                print("💾 Persisting session history...")

                # Build new messages
                new_messages = [
                    {"role": "user", "content": input.prompt},
                    {"role": "assistant", "content": result.response},
                ]

                # Combine with previous history
                complete_session = session_history + new_messages

                success = self.session_service.persist_session(
                    execution_id=execution_id,
                    session_id=input.session_id or execution_id,
                    user_id=input.user_id,
                    messages=complete_session,
                    metadata={
                        "agent_id": input.agent_id,
                        "organization_id": input.organization_id,
                        "runtime_type": runtime_type.value,
                        "turn_count": len(complete_session),
                    },
                )

                if success:
                    print(
                        f"✅ Session persisted ({len(complete_session)} total messages)\n"
                    )
                else:
                    print(f"⚠️  Session persistence failed\n")

            # STEP 8: Print usage metrics
            if result.usage:
                print(f"📊 Token Usage:")
                print(f"   Input: {result.usage.get('prompt_tokens', 0)}")
                print(f"   Output: {result.usage.get('completion_tokens', 0)}")
                print(f"   Total: {result.usage.get('total_tokens', 0)}\n")

            print("=" * 80)
            print("🏁 AGENT EXECUTION END")
            print("=" * 80 + "\n")

            # Return standardized result
            return {
                "success": result.success,
                "response": result.response,
                "usage": result.usage,
                "model": result.model or input.model_id,
                "finish_reason": result.finish_reason or "stop",
                "run_id": result.run_id,
                "tool_messages": result.tool_messages or [],
                "runtime_type": runtime_type.value,
                "error": result.error,
            }

        except Exception as e:
            print("\n" + "=" * 80)
            print("❌ AGENT EXECUTION FAILED")
            print("=" * 80)
            print(f"Error: {str(e)}")
            print("=" * 80 + "\n")

            logger.error(
                "agent_execution_failed", execution_id=execution_id[:8], error=str(e)
            )

            return {
                "success": False,
                "error": str(e),
                "model": input.model_id,
                "usage": {},
                "finish_reason": "error",
                "runtime_type": runtime_type_str if "runtime_type_str" in locals() else "unknown",
            }

    async def _execute_streaming(
        self, runtime, context: RuntimeExecutionContext, input: Any
    ) -> Any:
        """
        Execute with streaming and publish events to Control Plane.

        Args:
            runtime: Runtime instance
            context: Execution context
            input: Original input for additional metadata

        Returns:
            Final RuntimeExecutionResult
        """
        # Create streaming helper for publishing events
        streaming_helper = StreamingHelper(
            control_plane_client=self.control_plane, execution_id=context.execution_id
        )

        accumulated_response = ""
        final_result = None

        # Define event callback for publishing to Control Plane
        def event_callback(event: Dict):
            """Callback to publish events to Control Plane SSE"""
            event_type = event.get("type")

            if event_type == "content_chunk":
                # Publish content chunk
                streaming_helper.publish_content_chunk(
                    content=event.get("content", ""),
                    message_id=event.get("message_id", context.execution_id),
                )
            elif event_type == "tool_start":
                # Publish tool start
                streaming_helper.publish_tool_start(
                    tool_name=event.get("tool_name"),
                    tool_execution_id=event.get("tool_execution_id"),
                    tool_args=event.get("tool_args", {}),
                    source="agent",
                )
            elif event_type == "tool_complete":
                # Publish tool completion
                streaming_helper.publish_tool_complete(
                    tool_name=event.get("tool_name"),
                    tool_execution_id=event.get("tool_execution_id"),
                    status=event.get("status", "success"),
                    output=event.get("output"),
                    error=event.get("error"),
                    source="agent",
                )

        # Stream execution
        async for chunk in runtime.stream_execute(context, event_callback):
            if chunk.response:
                accumulated_response += chunk.response
                print(chunk.response, end="", flush=True)

            # Keep final result for metadata
            if chunk.usage or chunk.finish_reason:
                final_result = chunk

        print()  # New line after streaming

        # Return final result with accumulated response
        if final_result:
            # Update response with accumulated content
            final_result.response = accumulated_response
            return final_result
        else:
            # Create final result if not provided
            from runtimes.base import RuntimeExecutionResult

            return RuntimeExecutionResult(
                response=accumulated_response,
                usage={},
                success=True,
                finish_reason="stop",
            )

    def _get_framework_name(self, runtime_type: RuntimeType) -> str:
        """
        Get friendly framework name for runtime type.

        Args:
            runtime_type: Runtime type enum

        Returns:
            Framework name string
        """
        mapping = {
            RuntimeType.DEFAULT: "Agno",
            RuntimeType.CLAUDE_CODE: "Claude Code SDK",
        }
        return mapping.get(runtime_type, "Unknown")
