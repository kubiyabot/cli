"""Integration tests for Control Plane client integration with activities"""

import pytest
import os
from unittest.mock import Mock, patch, MagicMock
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from control_plane_client import ControlPlaneClient, get_control_plane_client


class TestControlPlaneIntegration:
    """Test integration between Control Plane client and activity modules"""

    @pytest.fixture
    def mock_http_server(self):
        """Mock HTTP responses from Control Plane"""
        with patch('control_plane_client.httpx.Client') as mock_client_class:
            mock_client = MagicMock()
            mock_client_class.return_value = mock_client
            yield mock_client

    def test_client_can_publish_events(self, mock_http_server):
        """Test that client can successfully publish events to Control Plane"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock successful response
        mock_response = Mock()
        mock_response.status_code = 200
        mock_http_server.post = Mock(return_value=mock_response)

        # Publish event
        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test"}
        )

        assert result is True
        assert mock_http_server.post.called

    def test_client_can_cache_metadata(self, mock_http_server):
        """Test that client can cache execution metadata"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock successful response
        mock_response = Mock()
        mock_response.status_code = 202
        mock_http_server.post = Mock(return_value=mock_response)

        # Cache metadata
        result = client.cache_metadata(
            execution_id="exec_123",
            execution_type="AGENT"
        )

        assert result is True

    def test_client_can_persist_session(self, mock_http_server):
        """Test that client can persist session history"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock successful response
        mock_response = Mock()
        mock_response.status_code = 201
        mock_http_server.post = Mock(return_value=mock_response)

        # Persist session
        messages = [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi!"}
        ]

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=messages
        )

        assert result is True

    def test_client_can_fetch_toolsets(self, mock_http_server):
        """Test that client can fetch toolsets from Control Plane"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock successful response with toolsets
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json = Mock(return_value=[
            {"type": "file_system", "name": "File Tools", "enabled": True},
            {"type": "shell", "name": "Shell Tools", "enabled": True}
        ])
        mock_http_server.get = Mock(return_value=mock_response)

        # Fetch toolsets
        toolsets = client.get_toolsets(agent_id="agent_123")

        assert len(toolsets) == 2
        assert toolsets[0]["type"] == "file_system"

    def test_client_handles_connection_errors_gracefully(self, mock_http_server):
        """Test that client handles connection errors without crashing"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock connection error
        import httpx
        mock_http_server.post = Mock(side_effect=httpx.ConnectError("Connection failed"))

        # Should not raise exception
        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test"}
        )

        assert result is False

    def test_client_handles_timeout_errors_gracefully(self, mock_http_server):
        """Test that client handles timeout errors without crashing"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock timeout error
        import httpx
        mock_http_server.post = Mock(side_effect=httpx.TimeoutException("Timeout"))

        # Should not raise exception
        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test"}
        )

        assert result is False

    def test_singleton_integration_with_environment(self):
        """Test that singleton pattern works with environment variables"""
        # Reset singleton
        import control_plane_client
        control_plane_client._control_plane_client = None

        with patch.dict(os.environ, {
            'CONTROL_PLANE_URL': 'http://test.example.com',
            'KUBIYA_API_KEY': 'integration_test_key'
        }):
            client1 = get_control_plane_client()
            client2 = get_control_plane_client()

            # Should return same instance
            assert client1 is client2
            assert client1.base_url == "http://test.example.com"
            assert client1.api_key == "integration_test_key"

        # Reset for other tests
        control_plane_client._control_plane_client = None

    def test_multiple_events_can_be_published_sequentially(self, mock_http_server):
        """Test publishing multiple events in sequence"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        # Mock successful responses
        mock_response = Mock()
        mock_response.status_code = 200
        mock_http_server.post = Mock(return_value=mock_response)

        # Publish multiple events
        results = []
        for i in range(5):
            result = client.publish_event(
                execution_id=f"exec_{i}",
                event_type="message_chunk",
                data={"content": f"chunk {i}"}
            )
            results.append(result)

        # All should succeed
        assert all(results)
        assert mock_http_server.post.call_count == 5

    def test_client_properly_formats_urls(self, mock_http_server):
        """Test that client formats URLs correctly"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        mock_response = Mock()
        mock_response.status_code = 200
        mock_http_server.post = Mock(return_value=mock_response)

        # Publish event
        client.publish_event(
            execution_id="exec_123",
            event_type="test",
            data={}
        )

        # Check URL was properly formatted
        call_args = mock_http_server.post.call_args
        url = call_args[0][0]
        assert url == "http://localhost:8000/api/v1/executions/exec_123/events"

    def test_client_includes_proper_headers(self, mock_http_server):
        """Test that client includes proper authentication headers"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="secret_key_123"
        )

        mock_response = Mock()
        mock_response.status_code = 200
        mock_http_server.post = Mock(return_value=mock_response)

        # Publish event
        client.publish_event(
            execution_id="exec_123",
            event_type="test",
            data={}
        )

        # Check headers
        call_args = mock_http_server.post.call_args
        headers = call_args[1]["headers"]
        assert headers["Authorization"] == "UserKey secret_key_123"

    def test_session_persistence_includes_all_fields(self, mock_http_server):
        """Test that session persistence sends all required fields"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        mock_response = Mock()
        mock_response.status_code = 201
        mock_http_server.post = Mock(return_value=mock_response)

        # Persist session with all fields
        messages = [{"role": "user", "content": "test"}]
        metadata = {"team_id": "team_123", "turn": 1}

        client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=messages,
            metadata=metadata
        )

        # Check payload
        call_args = mock_http_server.post.call_args
        payload = call_args[1]["json"]

        assert payload["session_id"] == "session_456"
        assert payload["user_id"] == "user_789"
        assert payload["messages"] == messages
        assert payload["metadata"] == metadata

    def test_toolsets_request_format(self, mock_http_server):
        """Test that toolsets request is properly formatted"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )

        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json = Mock(return_value=[])
        mock_http_server.get = Mock(return_value=mock_response)

        # Fetch toolsets
        client.get_toolsets(agent_id="agent_abc123")

        # Check URL
        call_args = mock_http_server.get.call_args
        url = call_args[0][0]
        assert "agent_abc123" in url
        assert "toolsets/resolved" in url

        # Check headers
        headers = call_args[1]["headers"]
        assert "Authorization" in headers


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
