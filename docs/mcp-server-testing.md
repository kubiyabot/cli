# Kubiya MCP Server Testing Guide

This guide demonstrates how to test the Kubiya MCP (Model Context Protocol) server end-to-end.

## Overview

The Kubiya MCP server exposes tool execution capabilities through the Model Context Protocol, allowing AI assistants and other MCP clients to execute Kubiya tools with streaming output.

## Prerequisites

1. Build the Kubiya CLI:
   ```bash
   go build -o kubiya main.go
   ```

2. Ensure you have Python 3 installed for running the test client

## Testing the MCP Server

### 1. Using Custom Configuration

The MCP server supports custom configuration via the `--config` flag:

```bash
./kubiya mcp serve --config path/to/config.json
```

### 2. Test Configuration Example

Create a test configuration file (`test/test-mcp-config.json`):

```json
{
  "enable_runners": true,
  "whitelisted_tools": [
    {
      "name": "echo_test",
      "description": "Simple echo test tool",
      "tool_name": "echo_test",
      "integrations": [],
      "parameters": {
        "type": "bash",
        "content": "echo 'MCP Test Output: This is a test tool!'"
      }
    },
    {
      "name": "date_test",
      "description": "Show current date and time",
      "tool_name": "date_test",
      "integrations": [],
      "parameters": {
        "type": "bash",
        "content": "date '+Date: %Y-%m-%d, Time: %H:%M:%S'"
      }
    }
  ],
  "tool_contexts": [
    {
      "type": "test",
      "description": "Test tools for MCP server demonstration",
      "examples": [
        {
          "name": "Echo test",
          "description": "Simple echo command",
          "command": "echo 'hello'"
        }
      ]
    }
  ]
}
```

### 3. Running Integration Tests

We provide several ways to test the MCP server:

#### Python Test Client

Run the Python test client to verify the MCP server functionality:

```bash
python3 test/test_mcp_client.py test/test-mcp-config.json
```

This will:
1. Start the MCP server with your configuration
2. Initialize the connection
3. List available tools
4. Execute a test tool
5. Display the results

Expected output:
```
🧪 Testing Kubiya MCP Server
===========================

Using config file: test/test-mcp-config.json

Starting MCP server...
1️⃣  Initializing connection...
✅ Connected to: Kubiya MCP Server v1.0.0
   Capabilities: ['logging', 'tools']

2️⃣  Listing available tools...
✅ Found 5 tools:
   • kubiya_date_test: Show current date and time
   • kubiya_echo_test: Simple echo test tool
   • kubiya_env_test: Test environment variables
   • kubiya_get_tool_contexts: Get context information
   • kubiya_list_runners: List all available runners

3️⃣  Testing tool execution: kubiya_get_tool_contexts...
✅ Tool executed successfully

✅ All tests completed!
```

#### Go Test Client

Run the Go test client for comprehensive testing:

```bash
# Run with default config
go run test/test_mcp_golang_client.go

# Run with custom config
go run test/test_mcp_golang_client.go test/test-mcp-config.json
```

The Go client provides:
- Full JSON-RPC protocol implementation
- Concurrent stderr logging
- Pretty-printed output
- Comprehensive error handling
- All MCP protocol features

Example output:
```
🚀 Starting MCP Golang Client Test
==================================
Using config: test/test-mcp-config.json

1️⃣  Initializing connection...
✅ Initialized successfully!
Server info: map[name:Kubiya MCP Server version:1.0.0]

2️⃣  Listing available tools...
Found 5 tools:
  1. kubiya_echo_test - Simple echo test tool
  2. kubiya_date_test - Show current date and time
  3. kubiya_get_tool_contexts - Get context information
  4. kubiya_list_runners - List all available runners
  5. kubiya_execute_tool - Execute a Kubiya tool

3️⃣  Getting tool contexts...
4️⃣  Listing runners...
5️⃣  Testing tool execution...

✅ All tests completed!
```

#### Go Integration Tests

Run the Go integration tests:

```bash
go test -v ./test/... -run TestMCPServer
```

Note: Use `-short` flag to skip integration tests in CI environments.

### 4. Manual Testing with MCP Inspector

For interactive testing, you can use the MCP Inspector tool:

1. Install MCP Inspector:
   ```bash
   npm install -g @modelcontextprotocol/inspector
   ```

2. Run the inspector with your server:
   ```bash
   mcp-inspector ./kubiya mcp serve --config test/test-mcp-config.json
   ```

3. Open the provided URL in your browser to interact with the server

### 5. Testing with Real MCP Clients

#### Using mcp-cli

1. Install mcp-cli:
   ```bash
   npm install -g @modelcontextprotocol/cli
   ```

2. Connect to the server:
   ```bash
   mcp-cli ./kubiya mcp serve --config test/test-mcp-config.json
   ```

3. Execute commands:
   - List tools: `tools/list`
   - Call a tool: `tools/call kubiya_echo_test {"runner": "auto"}`

#### Using Claude Desktop

1. Configure Claude Desktop to use your MCP server:
   ```json
   {
     "mcpServers": {
       "kubiya": {
         "command": "/path/to/kubiya",
         "args": ["mcp", "serve", "--config", "/path/to/config.json"]
       }
     }
   }
   ```

2. Restart Claude Desktop and your tools will be available

## Available Test Tools

When using the test configuration, these tools are available:

1. **kubiya_echo_test** - Simple echo test
2. **kubiya_date_test** - Show current date/time
3. **kubiya_env_test** - Test environment variables
4. **kubiya_list_runners** - List available runners (if configured)
5. **kubiya_get_tool_contexts** - Get tool context information

## Troubleshooting

### Server doesn't start

1. Check that the Kubiya binary is built:
   ```bash
   ./kubiya version
   ```

2. Verify your configuration file is valid JSON:
   ```bash
   jq '.' test/test-mcp-config.json
   ```

3. Check server logs (written to stderr):
   ```bash
   ./kubiya mcp serve --config test/test-mcp-config.json 2>&1
   ```

### Connection issues

1. Ensure the server is running before connecting
2. Check that the protocol version matches (currently "2024-11-05")
3. Verify stdio communication is working properly

### Tool execution fails

1. Check that you have a valid API key set:
   ```bash
   export KUBIYA_API_KEY=your-api-key
   ```

2. Verify runners are available:
   ```bash
   ./kubiya runner list
   ```

3. Check tool definition in your configuration

## Advanced Testing

### Testing with Integrations

Create tools with integrations:

```json
{
  "name": "k8s_test",
  "description": "Kubernetes test tool",
  "tool_name": "k8s_pods",
  "integrations": ["k8s/kubeconfig"],
  "parameters": {
    "type": "bash",
    "image": "bitnami/kubectl:latest",
    "content": "kubectl get pods -A"
  }
}
```

### Testing with Custom Runners

Specify a custom runner in tool execution:

```python
call_tool_request = {
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
        "name": "kubiya_echo_test",
        "arguments": {
            "runner": "my-custom-runner",
            "timeout": 60
        }
    }
}
```

### Performance Testing

For performance testing, create tools with longer execution times:

```json
{
  "name": "perf_test",
  "description": "Performance test tool",
  "tool_name": "perf_test",
  "parameters": {
    "type": "bash",
    "content": "for i in {1..10}; do echo \"Iteration $i\"; sleep 1; done"
  }
}
```

## Next Steps

1. Create custom tool configurations for your use case
2. Integrate with your preferred MCP client
3. Set up monitoring for production deployments
4. Explore advanced features like tool contexts and integrations

For more information, see:
- [MCP Server Configuration](./mcp-server-config-example.json)
- [Tool Execution Examples](./tool-exec-examples.md)
- [Workflow Commands](./workflow-commands.md) 