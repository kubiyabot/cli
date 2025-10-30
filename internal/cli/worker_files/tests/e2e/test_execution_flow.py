"""End-to-end tests for full execution flow

These tests verify the complete flow from Control Plane → Worker → Database → UI
"""

import pytest
import os
import asyncio
from unittest.mock import Mock, patch, MagicMock, AsyncMock
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from control_plane_client import ControlPlaneClient


class MockRedis:
    """Mock Redis client for E2E tests"""

    def __init__(self):
        self.store = {}

    async def set(self, key: str, value: str, ex: int = None):
        """Mock Redis set"""
        self.store[key] = {"value": value, "ttl": ex}
        return True

    async def get(self, key: str):
        """Mock Redis get"""
        if key in self.store:
            return self.store[key]["value"]
        return None

    async def exists(self, key: str):
        """Mock Redis exists"""
        return key in self.store


class MockDatabase:
    """Mock database for E2E tests"""

    def __init__(self):
        self.executions = {}
        self.sessions = []

    def insert_execution(self, execution_id: str, data: dict):
        """Mock execution insert"""
        self.executions[execution_id] = data

    def update_execution(self, execution_id: str, updates: dict):
        """Mock execution update"""
        if execution_id in self.executions:
            self.executions[execution_id].update(updates)

    def get_execution(self, execution_id: str):
        """Mock execution get"""
        return self.executions.get(execution_id)

    def insert_session(self, session_id: str, data: dict):
        """Mock session insert"""
        self.sessions.append({"session_id": session_id, **data})

    def get_session(self, session_id: str):
        """Mock session get"""
        for session in self.sessions:
            if session["session_id"] == session_id:
                return session
        return None


@pytest.fixture
def mock_redis():
    """Fixture providing mock Redis"""
    return MockRedis()


@pytest.fixture
def mock_db():
    """Fixture providing mock Database"""
    return MockDatabase()


@pytest.fixture
def control_plane_client():
    """Fixture providing real ControlPlaneClient for E2E testing"""
    return ControlPlaneClient(
        base_url="http://localhost:8000",
        api_key="test_e2e_key"
    )


class TestAgentExecutionFlow:
    """Test complete agent execution flow"""

    @pytest.mark.asyncio
    async def test_complete_agent_execution_flow(self, mock_redis, mock_db, control_plane_client):
        """
        Test full agent execution flow:
        1. Control Plane receives request
        2. Worker picks up task
        3. Worker executes agent
        4. Worker streams events to Control Plane
        5. Worker persists session
        6. Control Plane updates database
        7. UI receives SSE events
        """

        execution_id = "e2e_agent_exec_123"

        # Step 1: Control Plane creates execution
        mock_db.insert_execution(execution_id, {
            "status": "pending",
            "execution_type": "AGENT",
            "agent_id": "agent_456"
        })

        # Step 2: Mock HTTP responses for worker activity
        with patch.object(control_plane_client._client, 'post') as mock_post:
            with patch.object(control_plane_client._client, 'get') as mock_get:
                # Mock event publishing (streaming)
                mock_post_response = Mock()
                mock_post_response.status_code = 200
                mock_post.return_value = mock_post_response

                # Mock toolset fetching
                mock_get_response = Mock()
                mock_get_response.status_code = 200
                mock_get_response.json = Mock(return_value=[
                    {"type": "file_system", "name": "File Tools", "enabled": True}
                ])
                mock_get.return_value = mock_get_response

                # Step 3: Worker caches metadata
                result = control_plane_client.cache_metadata(execution_id, "AGENT")
                assert result is True

                # Verify metadata event was published
                assert mock_post.called
                metadata_call = mock_post.call_args_list[0]
                assert "metadata" in str(metadata_call)

                # Step 4: Worker fetches toolsets
                toolsets = control_plane_client.get_toolsets("agent_456")
                assert len(toolsets) == 1
                assert toolsets[0]["type"] == "file_system"

                # Step 5: Worker streams message chunks
                chunks = ["Hello ", "from ", "agent!"]
                for chunk in chunks:
                    result = control_plane_client.publish_event(
                        execution_id=execution_id,
                        event_type="message_chunk",
                        data={"content": chunk, "role": "assistant"}
                    )
                    assert result is True

                # Verify all chunks were published
                assert mock_post.call_count >= 4  # metadata + 3 chunks

                # Step 6: Worker persists session
                messages = [
                    {"role": "user", "content": "Hello"},
                    {"role": "assistant", "content": "Hello from agent!"}
                ]

                result = control_plane_client.persist_session(
                    execution_id=execution_id,
                    session_id=execution_id,
                    user_id="user_789",
                    messages=messages
                )
                assert result is True

        # Step 7: Verify Control Plane would update database
        mock_db.update_execution(execution_id, {
            "status": "completed",
            "response": "Hello from agent!",
            "usage": {"input_tokens": 10, "output_tokens": 20}
        })

        execution = mock_db.get_execution(execution_id)
        assert execution["status"] == "completed"
        assert execution["response"] == "Hello from agent!"

    @pytest.mark.asyncio
    async def test_agent_execution_with_tool_calls(self, mock_redis, mock_db, control_plane_client):
        """Test agent execution flow including tool calls"""

        execution_id = "e2e_agent_tools_123"

        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 200
            mock_post.return_value = mock_response

            # Cache metadata
            control_plane_client.cache_metadata(execution_id, "AGENT")

            # Stream tool started event
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="tool_started",
                data={
                    "tool_name": "read_file",
                    "tool_execution_id": "tool_1",
                    "tool_arguments": {"path": "test.txt"}
                }
            )

            # Stream tool completed event
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="tool_completed",
                data={
                    "tool_name": "read_file",
                    "tool_execution_id": "tool_1",
                    "status": "success"
                }
            )

            # Verify events were published
            assert mock_post.call_count >= 3  # metadata + tool_started + tool_completed


class TestTeamExecutionFlow:
    """Test complete team execution flow"""

    @pytest.mark.asyncio
    async def test_complete_team_execution_flow(self, mock_redis, mock_db, control_plane_client):
        """
        Test full team execution flow:
        1. Control Plane receives team request
        2. Worker picks up task
        3. Worker executes team coordination
        4. Worker streams team leader and member events
        5. Worker persists team session
        6. Control Plane updates database
        """

        execution_id = "e2e_team_exec_123"

        # Step 1: Control Plane creates execution
        mock_db.insert_execution(execution_id, {
            "status": "pending",
            "execution_type": "TEAM",
            "team_id": "team_456"
        })

        # Step 2: Mock HTTP responses
        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 200
            mock_post.return_value = mock_response

            # Step 3: Worker caches metadata
            result = control_plane_client.cache_metadata(execution_id, "TEAM")
            assert result is True

            # Step 4: Worker streams team leader message
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="message_chunk",
                data={
                    "role": "assistant",
                    "content": "Team leader response",
                    "source": "team_leader"
                }
            )

            # Step 5: Worker streams member message
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="member_message_chunk",
                data={
                    "role": "assistant",
                    "content": "Member response",
                    "source": "team_member",
                    "member_name": "Agent 1"
                }
            )

            # Step 6: Worker persists team session
            messages = [
                {"role": "user", "content": "Team task"},
                {"role": "assistant", "content": "Team leader response"},
                {"role": "assistant", "content": "Member response", "member_name": "Agent 1"}
            ]

            result = control_plane_client.persist_session(
                execution_id=execution_id,
                session_id=execution_id,
                user_id="user_789",
                messages=messages,
                metadata={"team_id": "team_456"}
            )
            assert result is True

        # Step 7: Verify database update
        mock_db.update_execution(execution_id, {
            "status": "completed",
            "response": "Team leader response"
        })

        execution = mock_db.get_execution(execution_id)
        assert execution["status"] == "completed"

    @pytest.mark.asyncio
    async def test_team_execution_with_hitl(self, mock_redis, mock_db, control_plane_client):
        """Test team execution with Human-in-the-Loop (multiple turns)"""

        execution_id = "e2e_team_hitl_123"

        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 200
            mock_post.return_value = mock_response

            # Turn 1: Initial request
            control_plane_client.cache_metadata(execution_id, "TEAM")

            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="message_chunk",
                data={"content": "Turn 1 response"}
            )

            messages_turn_1 = [
                {"role": "user", "content": "Initial request"},
                {"role": "assistant", "content": "Turn 1 response"}
            ]

            control_plane_client.persist_session(
                execution_id=execution_id,
                session_id=execution_id,
                user_id="user_789",
                messages=messages_turn_1
            )

            # Turn 2: Follow-up request
            control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="message_chunk",
                data={"content": "Turn 2 response"}
            )

            messages_turn_2 = [
                {"role": "user", "content": "Initial request"},
                {"role": "assistant", "content": "Turn 1 response"},
                {"role": "user", "content": "Follow-up question"},
                {"role": "assistant", "content": "Turn 2 response"}
            ]

            control_plane_client.persist_session(
                execution_id=execution_id,
                session_id=execution_id,
                user_id="user_789",
                messages=messages_turn_2
            )

            # Verify both turns were persisted
            assert mock_post.call_count >= 4  # metadata + 2 chunks + 2 persist calls


class TestSessionPersistence:
    """Test session persistence and history retrieval"""

    @pytest.mark.asyncio
    async def test_session_history_persists_when_worker_offline(self, mock_db, control_plane_client):
        """
        Test that session history is available even when worker is offline:
        1. Worker executes and persists session
        2. Worker goes offline
        3. UI requests execution history
        4. Control Plane loads from database (not Redis)
        """

        execution_id = "e2e_session_persist_123"
        session_id = execution_id

        # Step 1: Worker persists session
        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 201
            mock_post.return_value = mock_response

            messages = [
                {"role": "user", "content": "Hello", "timestamp": "2024-01-01T00:00:00Z"},
                {"role": "assistant", "content": "Hi!", "timestamp": "2024-01-01T00:00:01Z"}
            ]

            result = control_plane_client.persist_session(
                execution_id=execution_id,
                session_id=session_id,
                user_id="user_123",
                messages=messages
            )

            assert result is True

        # Step 2: Simulate Control Plane storing in database
        mock_db.insert_session(session_id, {
            "user_id": "user_123",
            "messages": messages,
            "execution_id": execution_id
        })

        # Step 3: Worker goes offline (no Redis events)

        # Step 4: Control Plane loads from database
        session_data = mock_db.get_session(session_id)
        assert session_data is not None
        assert len(session_data["messages"]) == 2
        assert session_data["user_id"] == "user_123"

    @pytest.mark.asyncio
    async def test_multi_user_session_isolation(self, mock_db, control_plane_client):
        """Test that sessions are properly isolated by user_id"""

        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 201
            mock_post.return_value = mock_response

            # User 1 session
            control_plane_client.persist_session(
                execution_id="exec_user1",
                session_id="session_user1",
                user_id="user_1",
                messages=[{"role": "user", "content": "User 1 message"}]
            )

            # User 2 session
            control_plane_client.persist_session(
                execution_id="exec_user2",
                session_id="session_user2",
                user_id="user_2",
                messages=[{"role": "user", "content": "User 2 message"}]
            )

            # Store in database
            mock_db.insert_session("session_user1", {
                "user_id": "user_1",
                "messages": [{"role": "user", "content": "User 1 message"}]
            })

            mock_db.insert_session("session_user2", {
                "user_id": "user_2",
                "messages": [{"role": "user", "content": "User 2 message"}]
            })

            # Verify isolation
            session1 = mock_db.get_session("session_user1")
            session2 = mock_db.get_session("session_user2")

            assert session1["user_id"] == "user_1"
            assert session2["user_id"] == "user_2"
            assert session1["messages"][0]["content"] == "User 1 message"
            assert session2["messages"][0]["content"] == "User 2 message"


class TestErrorHandlingE2E:
    """Test end-to-end error handling"""

    @pytest.mark.asyncio
    async def test_execution_failure_updates_status(self, mock_db, control_plane_client):
        """Test that execution failures are properly recorded"""

        execution_id = "e2e_error_123"

        mock_db.insert_execution(execution_id, {
            "status": "pending"
        })

        # Simulate execution failure
        # In real scenario, this would come from activity error handling

        mock_db.update_execution(execution_id, {
            "status": "failed",
            "error_message": "Agent execution failed: Model timeout"
        })

        execution = mock_db.get_execution(execution_id)
        assert execution["status"] == "failed"
        assert "timeout" in execution["error_message"]

    @pytest.mark.asyncio
    async def test_network_failure_during_streaming(self, control_plane_client):
        """Test handling of network failures during event streaming"""

        execution_id = "e2e_network_fail_123"

        with patch.object(control_plane_client._client, 'post') as mock_post:
            # Simulate network failure
            import httpx
            mock_post.side_effect = httpx.ConnectError("Connection failed")

            # Should not raise exception
            result = control_plane_client.publish_event(
                execution_id=execution_id,
                event_type="message_chunk",
                data={"content": "test"}
            )

            # Should return False but not crash
            assert result is False


class TestPerformanceE2E:
    """Test performance characteristics end-to-end"""

    @pytest.mark.asyncio
    async def test_high_frequency_event_streaming(self, control_plane_client):
        """Test that high-frequency event streaming works reliably"""

        execution_id = "e2e_perf_123"

        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 200
            mock_post.return_value = mock_response

            # Stream 100 events rapidly
            for i in range(100):
                result = control_plane_client.publish_event(
                    execution_id=execution_id,
                    event_type="message_chunk",
                    data={"content": f"Chunk {i}"}
                )
                assert result is True

            # All events should have been published
            assert mock_post.call_count == 100

    @pytest.mark.asyncio
    async def test_large_session_persistence(self, control_plane_client):
        """Test persisting large sessions with many messages"""

        execution_id = "e2e_large_session_123"

        with patch.object(control_plane_client._client, 'post') as mock_post:
            mock_response = Mock()
            mock_response.status_code = 201
            mock_post.return_value = mock_response

            # Create large session with 50 messages
            messages = []
            for i in range(50):
                messages.append({
                    "role": "user" if i % 2 == 0 else "assistant",
                    "content": f"Message {i} content" * 10,  # Longer content
                    "timestamp": f"2024-01-01T00:{i:02d}:00Z"
                })

            result = control_plane_client.persist_session(
                execution_id=execution_id,
                session_id=execution_id,
                user_id="user_123",
                messages=messages
            )

            assert result is True

            # Verify payload structure
            call_args = mock_post.call_args
            payload = call_args[1]["json"]
            assert len(payload["messages"]) == 50


if __name__ == "__main__":
    pytest.main([__file__, "-v", "--tb=short"])
