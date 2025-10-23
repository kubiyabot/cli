"""Agent executor service - handles agent execution business logic"""

from typing import Dict, Any, Optional, List
from datetime import datetime, timezone
import structlog
import asyncio
import os

from agno.agent import Agent
from agno.models.litellm import LiteLLM

from control_plane_client import ControlPlaneClient
from services.session_service import SessionService
from services.cancellation_manager import CancellationManager
from services.toolset_factory import ToolsetFactory
from utils.streaming_utils import StreamingHelper

logger = structlog.get_logger()


class AgentExecutorService:
    """
    Service for executing agents with full session management and cancellation support.

    This service orchestrates:
    - Session loading and restoration
    - Agent creation with LiteLLM configuration
    - Toolset instantiation
    - Streaming execution with real-time updates
    - Session persistence
    - Cancellation support via CancellationManager
    """

    def __init__(
        self,
        control_plane: ControlPlaneClient,
        session_service: SessionService,
        cancellation_manager: CancellationManager
    ):
        self.control_plane = control_plane
        self.session_service = session_service
        self.cancellation_manager = cancellation_manager

    async def execute(self, input: Any) -> Dict[str, Any]:
        """
        Execute an agent with full session management and streaming.

        Args:
            input: AgentExecutionInput with execution details

        Returns:
            Dict with response, usage, success flag, etc.
        """
        execution_id = input.execution_id

        print("\n" + "="*80)
        print("🤖 AGENT EXECUTION START")
        print("="*80)
        print(f"Execution ID: {execution_id}")
        print(f"Agent ID: {input.agent_id}")
        print(f"Organization: {input.organization_id}")
        print(f"Model: {input.model_id or 'default'}")
        print(f"Session ID: {input.session_id}")
        print(f"Prompt: {input.prompt[:100]}..." if len(input.prompt) > 100 else f"Prompt: {input.prompt}")
        print("="*80 + "\n")

        logger.info(
            "agent_execution_start",
            execution_id=execution_id[:8],
            agent_id=input.agent_id,
            session_id=input.session_id
        )

        try:
            # STEP 1: Load session history
            session_history = self.session_service.load_session(
                execution_id=execution_id,
                session_id=input.session_id
            )

            if session_history:
                print(f"✅ Loaded {len(session_history)} messages from previous session\n")
            else:
                print("ℹ️  Starting new conversation\n")

            # STEP 2: Build conversation context for Agno
            conversation_context = self.session_service.build_conversation_context(session_history)

            # STEP 3: Get LiteLLM configuration
            litellm_api_base = os.getenv("LITELLM_API_BASE", "https://llm-proxy.kubiya.ai")
            litellm_api_key = os.getenv("LITELLM_API_KEY")

            if not litellm_api_key:
                raise ValueError("LITELLM_API_KEY environment variable not set")

            model = input.model_id or os.environ.get("LITELLM_DEFAULT_MODEL", "kubiya/claude-sonnet-4")

            # STEP 4: Fetch and instantiate toolsets
            toolsets = []
            if input.agent_id:
                print(f"🔧 Fetching toolsets from Control Plane...")
                try:
                    toolset_configs = self.control_plane.get_toolsets(input.agent_id)
                    if toolset_configs:
                        print(f"✅ Resolved {len(toolset_configs)} toolsets")
                        print(f"   Types: {[t.get('type') for t in toolset_configs]}")
                        print(f"   Names: {[t.get('name') for t in toolset_configs]}\n")

                        toolsets = ToolsetFactory.create_toolsets_from_list(toolset_configs)

                        if toolsets:
                            print(f"✅ Instantiated {len(toolsets)} toolset(s)\n")
                    else:
                        print(f"⚠️  No toolsets found\n")
                except Exception as e:
                    print(f"❌ Error fetching toolsets: {str(e)}\n")
                    logger.error("toolset_fetch_error", error=str(e))

            # STEP 5: Create agent with streaming helper
            print(f"\n🤖 Creating Agno Agent:")
            print(f"   Model: {model}")
            print(f"   Toolsets: {len(toolsets)}")

            # Create streaming helper for this execution
            streaming_helper = StreamingHelper(
                control_plane_client=self.control_plane,
                execution_id=execution_id
            )

            # Create tool hook for real-time updates
            def tool_hook(name: str = None, function_name: str = None, function=None, arguments: dict = None, **kwargs):
                """Hook to capture tool execution for real-time streaming"""
                tool_name = name or function_name or "unknown"
                tool_args = arguments or {}

                # Generate unique tool execution ID
                import time
                tool_execution_id = f"{tool_name}_{int(time.time() * 1000000)}"

                print(f"   🔧 Tool Starting: {tool_name} (ID: {tool_execution_id})")

                # Publish tool start event
                streaming_helper.publish_tool_start(
                    tool_name=tool_name,
                    tool_execution_id=tool_execution_id,
                    tool_args=tool_args,
                    source="agent"
                )

                # Execute the tool
                result = None
                error = None
                try:
                    if function and callable(function):
                        result = function(**tool_args) if tool_args else function()
                    else:
                        raise ValueError(f"Function not callable: {function}")

                    status = "success"
                    print(f"   ✅ Tool Success: {tool_name}")

                except Exception as e:
                    error = e
                    status = "failed"
                    print(f"   ❌ Tool Failed: {tool_name} - {str(e)}")

                # Publish tool completion event
                streaming_helper.publish_tool_complete(
                    tool_name=tool_name,
                    tool_execution_id=tool_execution_id,
                    status=status,
                    output=str(result)[:1000] if result else None,
                    error=str(error) if error else None,
                    source="agent"
                )

                if error:
                    raise error

                return result

            # Create Agno Agent
            agent = Agent(
                name=f"Agent {input.agent_id}",
                role=input.system_prompt or "You are a helpful AI assistant",
                model=LiteLLM(
                    id=f"openai/{model}",
                    api_base=litellm_api_base,
                    api_key=litellm_api_key,
                ),
                tools=toolsets if toolsets else None,
                tool_hooks=[tool_hook],
            )

            # STEP 6: Register for cancellation
            self.cancellation_manager.register(
                execution_id=execution_id,
                instance=agent,
                instance_type="agent"
            )
            print(f"✅ Agent registered for cancellation support\n")

            # Cache execution metadata in Redis
            self.control_plane.cache_metadata(execution_id, "AGENT")

            # STEP 7: Execute with streaming
            print("⚡ Executing Agent Run with Streaming...\n")

            # Generate unique message ID for this turn
            import time
            message_id = f"{execution_id}_{int(time.time() * 1000000)}"

            def stream_agent_run():
                """Run agent with streaming and collect response"""
                try:
                    # Execute with conversation context
                    if conversation_context:
                        run_response = agent.run(
                            input.prompt,
                            stream=True,
                            messages=conversation_context,
                        )
                    else:
                        run_response = agent.run(input.prompt, stream=True)

                    # Process streaming chunks
                    for chunk in run_response:
                        # Handle run_id capture
                        streaming_helper.handle_run_id(
                            chunk=chunk,
                            on_run_id=lambda run_id: self.cancellation_manager.set_run_id(execution_id, run_id)
                        )

                        # Handle content chunk
                        streaming_helper.handle_content_chunk(
                            chunk=chunk,
                            message_id=message_id,
                            print_to_console=True
                        )

                    print()  # New line after streaming
                    return run_response

                except Exception as e:
                    print(f"\n❌ Streaming error: {str(e)}")
                    # Fall back to non-streaming
                    if conversation_context:
                        return agent.run(input.prompt, stream=False, messages=conversation_context)
                    else:
                        return agent.run(input.prompt, stream=False)

            # STEP 7.5: Create periodic session persistence task
            # This ensures conversation history is saved even if worker crashes
            execution_running = True
            last_persisted_length = 0

            async def periodic_persistence_task():
                """Persist session history every 60 seconds during execution"""
                nonlocal last_persisted_length
                while execution_running:
                    await asyncio.sleep(60)  # Persist every minute
                    if not execution_running:
                        break

                    # Build current snapshot from streamed content
                    current_response = streaming_helper.get_full_response()
                    if current_response and len(current_response) > last_persisted_length:
                        print(f"\n💾 Periodic persistence ({len(current_response)} chars)...")

                        # Create snapshot message
                        snapshot_messages = session_history + [{
                            "role": "assistant",
                            "content": current_response,
                            "timestamp": datetime.now(timezone.utc).isoformat(),
                        }]

                        # Persist (best effort - don't fail execution if this fails)
                        try:
                            success = self.session_service.persist_session(
                                execution_id=execution_id,
                                session_id=input.session_id or execution_id,
                                user_id=input.user_id,
                                messages=snapshot_messages,
                                metadata={
                                    "agent_id": input.agent_id,
                                    "organization_id": input.organization_id,
                                    "snapshot": True,  # Mark as intermediate snapshot
                                }
                            )
                            if success:
                                last_persisted_length = len(current_response)
                                print(f"   ✅ Session snapshot persisted")
                            else:
                                print(f"   ⚠️  Session snapshot persistence failed (non-fatal)")
                        except Exception as e:
                            print(f"   ⚠️  Session snapshot error: {str(e)} (non-fatal)")

            # Start periodic persistence in background
            persistence_task = asyncio.create_task(periodic_persistence_task())

            # Execute in thread pool (no timeout - user controls via STOP button)
            # Wrap in try-except to handle Temporal cancellation
            try:
                result = await asyncio.to_thread(stream_agent_run)
            except asyncio.CancelledError:
                # Temporal cancelled the activity - cancel the running agent
                print("\n🛑 Cancellation signal received - stopping agent execution...")
                cancel_result = self.cancellation_manager.cancel(execution_id)
                if cancel_result["success"]:
                    print(f"✅ Agent execution cancelled successfully")
                else:
                    print(f"⚠️  Cancellation completed with warning: {cancel_result.get('error', 'Unknown')}")
                # Re-raise to let Temporal know we're cancelled
                raise
            finally:
                # Always stop persistence task
                execution_running = False
                try:
                    await asyncio.wait_for(persistence_task, timeout=2.0)
                except asyncio.TimeoutError:
                    persistence_task.cancel()

            print("✅ Agent Execution Completed!")
            full_response = streaming_helper.get_full_response()
            print(f"   Response Length: {len(full_response)} chars\n")

            logger.info(
                "agent_execution_completed",
                execution_id=execution_id[:8],
                response_length=len(full_response)
            )

            # Use the streamed response content
            response_content = full_response if full_response else (result.content if hasattr(result, "content") else str(result))

            # STEP 8: Extract usage metrics
            usage = {}
            if hasattr(result, "metrics") and result.metrics:
                metrics = result.metrics
                usage = {
                    "prompt_tokens": getattr(metrics, "input_tokens", 0),
                    "completion_tokens": getattr(metrics, "output_tokens", 0),
                    "total_tokens": getattr(metrics, "total_tokens", 0),
                }
                print(f"📊 Token Usage:")
                print(f"   Input: {usage.get('prompt_tokens', 0)}")
                print(f"   Output: {usage.get('completion_tokens', 0)}")
                print(f"   Total: {usage.get('total_tokens', 0)}\n")

            # STEP 9: Persist complete session history
            print("\n💾 Persisting session history to Control Plane...")

            # Extract messages from result
            new_messages = self.session_service.extract_messages_from_result(
                result=result,
                user_id=input.user_id
            )

            # Combine with previous history
            complete_session = session_history + new_messages

            if complete_session:
                success = self.session_service.persist_session(
                    execution_id=execution_id,
                    session_id=input.session_id or execution_id,
                    user_id=input.user_id,
                    messages=complete_session,
                    metadata={
                        "agent_id": input.agent_id,
                        "organization_id": input.organization_id,
                        "turn_count": len(complete_session),
                    }
                )

                if success:
                    print(f"   ✅ Session persisted ({len(complete_session)} total messages)")
                else:
                    print(f"   ⚠️  Session persistence failed")
            else:
                print("   ℹ️  No messages to persist")

            print("\n" + "="*80)
            print("🏁 AGENT EXECUTION END")
            print("="*80 + "\n")

            # STEP 10: Cleanup
            self.cancellation_manager.unregister(execution_id)

            return {
                "success": True,
                "response": response_content,
                "usage": usage,
                "model": model,
                "finish_reason": "stop",
            }

        except Exception as e:
            # Cleanup on error
            self.cancellation_manager.unregister(execution_id)

            print("\n" + "="*80)
            print("❌ AGENT EXECUTION FAILED")
            print("="*80)
            print(f"Error: {str(e)}")
            print("="*80 + "\n")

            logger.error(
                "agent_execution_failed",
                execution_id=execution_id[:8],
                error=str(e)
            )

            return {
                "success": False,
                "error": str(e),
                "model": input.model_id,
                "usage": None,
                "finish_reason": "error",
            }
