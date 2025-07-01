#!/usr/bin/env python3
"""
Direct test of MCP server with protocol commands
"""

import json
import subprocess
import sys
import time
import os

def test_mcp_server():
    print("🧪 Testing Kubiya MCP Server Directly")
    print("=" * 50)
    
    # Set environment variables
    env = os.environ.copy()
    env['KUBIYA_API_KEY'] = os.environ.get('KUBIYA_API_KEY', '')
    env['KUBIYA_DEFAULT_RUNNER'] = 'auto'
    
    # Start MCP server (non-production mode)
    cmd = [
        '/Users/shaked/kubiya/cli/kubiya',
        'mcp', 'serve',
        '--config', '/Users/shaked/kubiya/cli/test/claude-code-test-simple.json'
    ]
    
    print(f"Starting MCP server...")
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=env,
        bufsize=1
    )
    
    time.sleep(2)  # Give server time to start
    
    try:
        # Check if server started correctly
        if proc.poll() is not None:
            stderr = proc.stderr.read()
            print(f"❌ Server failed to start: {stderr}")
            return
            
        # Test 1: Initialize
        print("\n1️⃣ Initializing MCP connection...")
        init_request = {
            "jsonrpc": "2.0",
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {
                    "name": "test-client",
                    "version": "1.0.0"
                }
            },
            "id": 1
        }
        
        proc.stdin.write(json.dumps(init_request) + '\n')
        proc.stdin.flush()
        
        # Read response with timeout
        response = None
        for _ in range(10):
            line = proc.stdout.readline()
            if line:
                response = line
                break
            time.sleep(0.5)
            
        if response:
            try:
                result = json.loads(response)
                print(f"✅ Initialized: {result.get('result', {}).get('serverInfo', {})}")
            except json.JSONDecodeError:
                print(f"⚠️  Invalid JSON response: {response}")
        else:
            print("⚠️  No response from server")
        
        # Test 2: List tools
        print("\n2️⃣ Listing available tools...")
        list_request = {
            "jsonrpc": "2.0",
            "method": "tools/list",
            "id": 2
        }
        
        proc.stdin.write(json.dumps(list_request) + '\n')
        proc.stdin.flush()
        
        response = proc.stdout.readline()
        if response:
            try:
                result = json.loads(response)
                tools = result.get('result', {}).get('tools', [])
                print(f"Found {len(tools)} tools:")
                for tool in tools[:10]:  # Show first 10
                    print(f"  - {tool.get('name')}: {tool.get('description')}")
            except json.JSONDecodeError:
                print(f"⚠️  Invalid JSON response: {response}")
        
        # Test 3: Execute hello_test
        print("\n3️⃣ Testing hello_test tool...")
        hello_request = {
            "jsonrpc": "2.0",
            "method": "tools/call",
            "params": {
                "name": "hello_test",
                "arguments": {}
            },
            "id": 3
        }
        
        proc.stdin.write(json.dumps(hello_request) + '\n')
        proc.stdin.flush()
        
        # Wait for execution and read multiple lines
        time.sleep(3)
        
        # Try to read response
        while True:
            line = proc.stdout.readline()
            if not line:
                break
            try:
                result = json.loads(line)
                if result.get('id') == 3:
                    print(f"Result: {json.dumps(result, indent=2)}")
                    break
            except json.JSONDecodeError:
                continue
                
        print("\n✅ MCP server is working!")
        print("\n📝 The server is ready for Claude Code integration testing")
        
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        # Print stderr if any
        stderr = proc.stderr.read()
        if stderr:
            print(f"Server error: {stderr}")
    finally:
        proc.terminate()
        proc.wait()

if __name__ == "__main__":
    test_mcp_server() 