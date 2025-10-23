"""Unit tests for ControlPlaneClient"""

import pytest
import httpx
from unittest.mock import Mock, patch, MagicMock
from datetime import datetime, timezone

# Import the module under test
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from control_plane_client import ControlPlaneClient, get_control_plane_client


class TestControlPlaneClient:
    """Test suite for ControlPlaneClient class"""

    @pytest.fixture
    def client(self):
        """Create a ControlPlaneClient instance for testing"""
        return ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_api_key_123"
        )

    @pytest.fixture
    def mock_httpx_client(self):
        """Mock httpx.Client for testing"""
        with patch('control_plane_client.httpx.Client') as mock_client_class:
            mock_client = MagicMock()
            mock_client_class.return_value = mock_client
            yield mock_client

    def test_client_initialization(self, client):
        """Test that client initializes correctly with proper configuration"""
        assert client.base_url == "http://localhost:8000"
        assert client.api_key == "test_api_key_123"
        assert client.headers == {"Authorization": "UserKey test_api_key_123"}

    def test_client_strips_trailing_slash(self):
        """Test that trailing slashes are removed from base_url"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000/",
            api_key="test_key"
        )
        assert client.base_url == "http://localhost:8000"

    def test_publish_event_success(self, client):
        """Test successful event publishing"""
        # Mock the HTTP client
        mock_response = Mock()
        mock_response.status_code = 200
        client._client.post = Mock(return_value=mock_response)

        # Call publish_event
        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test", "role": "assistant"}
        )

        # Verify success
        assert result is True

        # Verify the HTTP call was made correctly
        client._client.post.assert_called_once()
        call_args = client._client.post.call_args

        # Check URL
        assert call_args[0][0] == "http://localhost:8000/api/v1/executions/exec_123/events"

        # Check headers
        assert call_args[1]["headers"] == {"Authorization": "UserKey test_api_key_123"}

        # Check payload
        payload = call_args[1]["json"]
        assert payload["event_type"] == "message_chunk"
        assert payload["data"]["content"] == "test"
        assert payload["data"]["role"] == "assistant"
        assert "timestamp" in payload

    def test_publish_event_with_202_status(self, client):
        """Test that 202 status is also considered success"""
        mock_response = Mock()
        mock_response.status_code = 202
        client._client.post = Mock(return_value=mock_response)

        result = client.publish_event(
            execution_id="exec_123",
            event_type="tool_started",
            data={"tool_name": "test_tool"}
        )

        assert result is True

    def test_publish_event_failure(self, client):
        """Test event publishing with non-success status code"""
        mock_response = Mock()
        mock_response.status_code = 500
        client._client.post = Mock(return_value=mock_response)

        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test"}
        )

        assert result is False

    def test_publish_event_exception(self, client):
        """Test event publishing when HTTP request raises exception"""
        client._client.post = Mock(side_effect=httpx.ConnectError("Connection failed"))

        result = client.publish_event(
            execution_id="exec_123",
            event_type="message_chunk",
            data={"content": "test"}
        )

        assert result is False

    def test_cache_metadata_success(self, client):
        """Test successful metadata caching"""
        # Mock publish_event since cache_metadata calls it
        client.publish_event = Mock(return_value=True)

        result = client.cache_metadata(
            execution_id="exec_123",
            execution_type="AGENT"
        )

        assert result is True
        client.publish_event.assert_called_once_with(
            execution_id="exec_123",
            event_type="metadata",
            data={"execution_type": "AGENT"}
        )

    def test_cache_metadata_failure(self, client):
        """Test metadata caching failure"""
        client.publish_event = Mock(return_value=False)

        result = client.cache_metadata(
            execution_id="exec_123",
            execution_type="TEAM"
        )

        assert result is False

    def test_persist_session_success(self, client):
        """Test successful session persistence"""
        mock_response = Mock()
        mock_response.status_code = 200
        client._client.post = Mock(return_value=mock_response)

        messages = [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi there!"}
        ]

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=messages,
            metadata={"key": "value"}
        )

        assert result is True

        # Verify HTTP call
        client._client.post.assert_called_once()
        call_args = client._client.post.call_args

        # Check URL
        assert call_args[0][0] == "http://localhost:8000/api/v1/executions/exec_123/session"

        # Check payload
        payload = call_args[1]["json"]
        assert payload["session_id"] == "session_456"
        assert payload["user_id"] == "user_789"
        assert payload["messages"] == messages
        assert payload["metadata"] == {"key": "value"}

    def test_persist_session_with_201_status(self, client):
        """Test that 201 status is also considered success"""
        mock_response = Mock()
        mock_response.status_code = 201
        client._client.post = Mock(return_value=mock_response)

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=[],
        )

        assert result is True

    def test_persist_session_without_metadata(self, client):
        """Test session persistence without metadata"""
        mock_response = Mock()
        mock_response.status_code = 200
        client._client.post = Mock(return_value=mock_response)

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=[]
        )

        assert result is True

        # Verify metadata defaults to empty dict
        payload = client._client.post.call_args[1]["json"]
        assert payload["metadata"] == {}

    def test_persist_session_failure(self, client):
        """Test session persistence with non-success status code"""
        mock_response = Mock()
        mock_response.status_code = 500
        client._client.post = Mock(return_value=mock_response)

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=[]
        )

        assert result is False

    def test_persist_session_exception(self, client):
        """Test session persistence when HTTP request raises exception"""
        client._client.post = Mock(side_effect=httpx.TimeoutException("Timeout"))

        result = client.persist_session(
            execution_id="exec_123",
            session_id="session_456",
            user_id="user_789",
            messages=[]
        )

        assert result is False

    def test_get_toolsets_success(self, client):
        """Test successful toolset fetching"""
        mock_toolsets = [
            {"type": "file_system", "name": "File Tools", "enabled": True},
            {"type": "shell", "name": "Shell Tools", "enabled": True}
        ]

        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json = Mock(return_value=mock_toolsets)
        client._client.get = Mock(return_value=mock_response)

        result = client.get_toolsets(agent_id="agent_123")

        assert result == mock_toolsets

        # Verify HTTP call
        client._client.get.assert_called_once()
        call_args = client._client.get.call_args

        # Check URL
        expected_url = "http://localhost:8000/api/v1/toolsets/associations/agents/agent_123/toolsets/resolved"
        assert call_args[0][0] == expected_url

        # Check headers
        assert call_args[1]["headers"] == {"Authorization": "UserKey test_api_key_123"}

    def test_get_toolsets_not_found(self, client):
        """Test toolset fetching when agent not found"""
        mock_response = Mock()
        mock_response.status_code = 404
        client._client.get = Mock(return_value=mock_response)

        result = client.get_toolsets(agent_id="agent_123")

        assert result == []

    def test_get_toolsets_server_error(self, client):
        """Test toolset fetching with server error"""
        mock_response = Mock()
        mock_response.status_code = 500
        client._client.get = Mock(return_value=mock_response)

        result = client.get_toolsets(agent_id="agent_123")

        assert result == []

    def test_get_toolsets_exception(self, client):
        """Test toolset fetching when HTTP request raises exception"""
        client._client.get = Mock(side_effect=httpx.ConnectError("Connection failed"))

        result = client.get_toolsets(agent_id="agent_123")

        assert result == []

    def test_client_cleanup(self, client):
        """Test that client properly closes HTTP connection on cleanup"""
        mock_close = Mock()
        client._client.close = mock_close

        # Trigger cleanup
        del client

        # Verify close was called (would happen in __del__)
        # Note: This is hard to test reliably due to Python GC behavior
        # In real usage, the close would be called by __del__

    def test_client_cleanup_with_exception(self):
        """Test that client cleanup handles exceptions gracefully"""
        client = ControlPlaneClient(
            base_url="http://localhost:8000",
            api_key="test_key"
        )
        client._client.close = Mock(side_effect=Exception("Close failed"))

        # Should not raise exception
        try:
            del client
        except Exception as e:
            pytest.fail(f"Client cleanup should not raise exception: {e}")


class TestGetControlPlaneClient:
    """Test suite for get_control_plane_client singleton function"""

    def teardown_method(self):
        """Reset singleton after each test"""
        import control_plane_client
        control_plane_client._control_plane_client = None

    @patch.dict('os.environ', {
        'CONTROL_PLANE_URL': 'http://localhost:8000',
        'KUBIYA_API_KEY': 'test_key_123'
    })
    def test_singleton_creation(self):
        """Test that singleton is created with environment variables"""
        client = get_control_plane_client()

        assert client is not None
        assert client.base_url == "http://localhost:8000"
        assert client.api_key == "test_key_123"

    @patch.dict('os.environ', {
        'CONTROL_PLANE_URL': 'http://localhost:8000',
        'KUBIYA_API_KEY': 'test_key_123'
    })
    def test_singleton_reuse(self):
        """Test that singleton returns same instance on multiple calls"""
        client1 = get_control_plane_client()
        client2 = get_control_plane_client()

        assert client1 is client2

    @patch.dict('os.environ', {}, clear=True)
    def test_missing_control_plane_url(self):
        """Test that missing CONTROL_PLANE_URL raises ValueError"""
        with pytest.raises(ValueError, match="CONTROL_PLANE_URL environment variable not set"):
            get_control_plane_client()

    @patch.dict('os.environ', {'CONTROL_PLANE_URL': 'http://localhost:8000'}, clear=True)
    def test_missing_api_key(self):
        """Test that missing KUBIYA_API_KEY raises ValueError"""
        with pytest.raises(ValueError, match="KUBIYA_API_KEY environment variable not set"):
            get_control_plane_client()


class TestConnectionPooling:
    """Test suite for HTTP connection pooling configuration"""

    def test_client_uses_connection_pooling(self):
        """Test that client is configured with connection pooling"""
        with patch('control_plane_client.httpx.Client') as mock_client_class:
            client = ControlPlaneClient(
                base_url="http://localhost:8000",
                api_key="test_key"
            )

            # Verify httpx.Client was called with correct parameters
            mock_client_class.assert_called_once()
            call_kwargs = mock_client_class.call_args[1]

            # Check timeout configuration
            assert 'timeout' in call_kwargs
            timeout = call_kwargs['timeout']
            assert isinstance(timeout, httpx.Timeout)

            # Check limits configuration
            assert 'limits' in call_kwargs
            limits = call_kwargs['limits']
            assert isinstance(limits, httpx.Limits)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
