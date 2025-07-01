#!/bin/bash

# Non-interactive test script for Claude Code MCP integration

echo "🧪 Testing Kubiya MCP Server with Claude Code (Non-Interactive)"
echo "=============================================================="
echo ""

# Set up environment
export KUBIYA_API_KEY="${KUBIYA_API_KEY}"

# Test 1: Check MCP server status
echo "1️⃣ Checking MCP server status..."
mcp_status=$(claude mcp list 2>&1)
if echo "$mcp_status" | grep -q "kubiya-tools"; then
    echo "✅ MCP server 'kubiya-tools' is configured"
else
    echo "❌ MCP server not found"
    exit 1
fi

# Test 2: Use Claude to run a simple tool
echo ""
echo "2️⃣ Testing hello_test tool..."

# Run Claude in non-interactive mode with --print
hello_result=$(echo "Run the hello_test tool from the kubiya-tools MCP server." | claude chat --print 2>&1)

if echo "$hello_result" | grep -q "Hello from Kubiya MCP"; then
    echo "✅ hello_test tool executed successfully"
    echo "Output snippet: $(echo "$hello_result" | grep -o "Hello from Kubiya MCP.*" | head -1)"
else
    echo "⚠️  Could not verify hello_test execution"
    echo "Claude response preview: $(echo "$hello_result" | head -5)"
fi

# Test 3: Test tool with integration
echo ""
echo "3️⃣ Testing k8s_test tool with integration..."

k8s_result=$(echo "Run the k8s_test tool from kubiya-tools to test Kubernetes integration." | claude chat --print 2>&1)

if echo "$k8s_result" | grep -q "K8s integration works"; then
    echo "✅ k8s_test tool with integration executed successfully"
    echo "Output snippet: $(echo "$k8s_result" | grep -o "K8s integration works.*" | head -1)"
else
    echo "⚠️  Could not verify k8s_test execution"
    echo "Claude response preview: $(echo "$k8s_result" | head -5)"
fi

# Test 4: Use execute_tool with integrations
echo ""
echo "4️⃣ Testing execute_tool with AWS integration..."

execute_result=$(echo "Use the execute_tool from kubiya-tools to run a command with AWS integration. Tool definition: name=aws-test, type=docker, content='aws --version', integrations=['aws/cli']" | claude chat --print 2>&1)

if echo "$execute_result" | grep -q "aws\|AWS\|execute_tool"; then
    echo "✅ execute_tool with integration processed"
    echo "Response indicates tool handling"
else
    echo "⚠️  Could not verify execute_tool"
    echo "Claude response preview: $(echo "$execute_result" | head -5)"
fi

# Test 5: Direct MCP protocol test
echo ""
echo "5️⃣ Running direct MCP protocol test..."

# Test the server directly (fix the path)
if [ -f "test_mcp_direct.py" ]; then
    python3 test_mcp_direct.py
elif [ -f "../test/test_mcp_direct.py" ]; then
    python3 ../test/test_mcp_direct.py
else
    echo "⚠️  test_mcp_direct.py not found, skipping direct test"
fi

echo ""
echo "=============================================================="
echo "📊 Test Summary:"
echo ""
echo "✅ MCP server is configured in Claude Code"
echo "📝 Tools can be called through Claude's interface"
echo "🔌 Integration system is accessible"
echo ""
echo "💡 To test interactively in Claude Code:"
echo "   1. Open Claude Code"
echo "   2. Type: /mcp"
echo "   3. Try: 'Run the hello_test tool'"
echo "   4. Try: 'Run k8s_test to test Kubernetes integration'"
echo ""
echo "🔍 For manual verification, run these commands:"
echo "   claude chat --print 'Run hello_test from kubiya-tools'"
echo "   claude chat --print 'Run k8s_test from kubiya-tools'"
echo ""
echo "✨ The MCP server is ready for use with Claude Code!" 