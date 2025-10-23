# Worker Files Test Suite

Comprehensive test suite for the agent control plane worker implementation.

## Test Structure

```
tests/
├── unit/                          # Unit tests (24 tests)
│   └── test_control_plane_client.py    - Tests for ControlPlaneClient class
├── integration/                   # Integration tests (12 tests)
│   └── test_control_plane_integration.py - Tests for client integration with Control Plane API
└── e2e/                          # End-to-end tests (10 tests)
    └── test_execution_flow.py     - Tests for complete execution flows
```

## Test Coverage

### Unit Tests (24 tests)
Tests for the `ControlPlaneClient` class in isolation:

- **Initialization**: Client configuration, URL formatting
- **Event Publishing**: Success/failure cases, status codes (200/202)
- **Metadata Caching**: Redis caching via events API
- **Session Persistence**: Success/failure, optional metadata
- **Toolset Fetching**: Success/failure, error handling
- **Error Handling**: Network errors, timeouts, exceptions
- **Singleton Pattern**: Creation, reuse, environment variables
- **Connection Pooling**: HTTP client configuration

### Integration Tests (12 tests)
Tests for integration between ControlPlaneClient and Control Plane API:

- **Event Publishing**: Single and multiple events, proper formatting
- **Session Persistence**: Complete workflow, all fields included
- **Toolset Resolution**: Request format, authentication headers
- **Error Handling**: Connection errors, timeouts, graceful degradation
- **URL Formatting**: Proper endpoint construction
- **Authentication**: Header formatting, API key usage
- **Singleton Integration**: Environment variable configuration

### End-to-End Tests (10 tests)
Tests for complete execution flows from Control Plane → Worker → Database → UI:

#### Agent Execution Flow
- Complete agent execution with streaming
- Agent execution with tool calls
- Tool execution events (started/completed)

#### Team Execution Flow
- Complete team coordination with member events
- Team execution with HITL (multi-turn conversation)
- Team leader and member event streaming

#### Session Persistence
- Session history persistence when worker offline
- Multi-user session isolation
- Large session handling (50+ messages)

#### Error Handling
- Execution failure recording
- Network failure during streaming

#### Performance
- High-frequency event streaming (100 events)
- Large session persistence

## Running Tests

### Run All Tests
```bash
cd /Users/shaked/projects/orchestrator/agent-control-plane/cli/internal/cli/worker_files
python3 -m pytest tests/ -v
```

### Run Specific Test Suites
```bash
# Unit tests only
python3 -m pytest tests/unit/ -v

# Integration tests only
python3 -m pytest tests/integration/ -v

# E2E tests only
python3 -m pytest tests/e2e/ -v
```

### Run Specific Test Files
```bash
# Test ControlPlaneClient
python3 -m pytest tests/unit/test_control_plane_client.py -v

# Test Control Plane integration
python3 -m pytest tests/integration/test_control_plane_integration.py -v

# Test execution flows
python3 -m pytest tests/e2e/test_execution_flow.py -v
```

### Run With Coverage
```bash
# Install coverage tool
pip install pytest-cov

# Run with coverage report
python3 -m pytest tests/ --cov=. --cov-report=html --cov-report=term
```

### Run Specific Tests
```bash
# Run single test
python3 -m pytest tests/unit/test_control_plane_client.py::TestControlPlaneClient::test_publish_event_success -v

# Run test class
python3 -m pytest tests/e2e/test_execution_flow.py::TestAgentExecutionFlow -v
```

## Test Results Summary

**Total Tests: 46**
- ✅ Unit Tests: 24/24 passing
- ✅ Integration Tests: 12/12 passing
- ✅ E2E Tests: 10/10 passing

All tests execute in **~0.23 seconds**.

## Key Files Tested

### Production Code
- `control_plane_client.py` - Central client for Control Plane communication
- `activities/agent_activities.py` - Agent execution activities
- `activities/team_activities.py` - Team coordination activities

### Test Coverage Areas
1. **Event Streaming**: Real-time UI updates via SSE
2. **Session Persistence**: History storage for offline access
3. **Metadata Caching**: Fast SSE lookups via Redis
4. **Toolset Resolution**: Dynamic tool loading per agent
5. **Error Handling**: Network failures, timeouts, validation errors
6. **Multi-User Support**: User identity from JWT, session isolation
7. **HITL Support**: Multi-turn conversations with user input
8. **Performance**: High-frequency streaming, large sessions

## Test Dependencies

Tests use mocking to avoid external dependencies:
- `unittest.mock` - Mocking HTTP clients, databases, Redis
- `pytest-asyncio` - Testing async/await code
- `pytest` - Test framework

No external services required to run tests.

## Continuous Integration

To add to CI/CD pipeline:

```yaml
# Example GitHub Actions
- name: Run Worker Tests
  run: |
    cd cli/internal/cli/worker_files
    python3 -m pytest tests/ -v --tb=short
```

## Test Maintenance

### Adding New Tests

1. **Unit Tests**: Add to `tests/unit/` for isolated component testing
2. **Integration Tests**: Add to `tests/integration/` for API integration testing
3. **E2E Tests**: Add to `tests/e2e/` for complete workflow testing

### Test Naming Convention

- Test files: `test_*.py`
- Test classes: `Test*` (e.g., `TestControlPlaneClient`)
- Test methods: `test_*` (e.g., `test_publish_event_success`)

### Mocking Strategy

- **Unit Tests**: Mock all external dependencies (HTTP, Redis, DB)
- **Integration Tests**: Mock HTTP responses, test client integration
- **E2E Tests**: Mock full stack (Control Plane, Worker, DB, Redis)

## Troubleshooting

### Import Errors
If you see import errors, ensure you're running from the correct directory:
```bash
cd /Users/shaked/projects/orchestrator/agent-control-plane/cli/internal/cli/worker_files
python3 -m pytest tests/
```

### Async Test Failures
Ensure `pytest-asyncio` is installed:
```bash
pip install pytest-asyncio
```

### Environment Variables
Tests mock environment variables, but if needed:
```bash
export CONTROL_PLANE_URL=http://localhost:8000
export KUBIYA_API_KEY=test_key
python3 -m pytest tests/
```

## Related Documentation

- [ControlPlaneClient API](/cli/internal/cli/worker_files/control_plane_client.py)
- [Agent Activities](/cli/internal/cli/worker_files/activities/agent_activities.py)
- [Team Activities](/cli/internal/cli/worker_files/activities/team_activities.py)
