#!/usr/bin/env python3
"""
Simple MCP client to test our Kubiya MCP server functionality
"""
import json
import subprocess
import sys
import os

def send_mcp_request(server_process, request):
    """Send an MCP request and get response"""
    request_json = json.dumps(request) + "\n"
    server_process.stdin.write(request_json.encode())
    server_process.stdin.flush()
    
    # Read response
    response_line = server_process.stdout.readline()
    if response_line:
        return json.loads(response_line.decode())
    return None

def test_mcp_server():
    # Set environment variables
    env = os.environ.copy()
    env['KUBIYA_API_KEY'] = '***REMOVED***'
    env['KUBIYA_MCP_MAX_RESPONSE_SIZE'] = '10240'
    env['KUBIYA_MCP_MAX_TOOLS_IN_RESPONSE'] = '25'
    env['KUBIYA_MCP_DEFAULT_PAGE_SIZE'] = '10'
    
    # Start MCP server
    print("🚀 Starting Kubiya MCP server...")
    server_process = subprocess.Popen(
        ['./kubiya-test', 'mcp', 'serve', '--verbose', '--production'],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=env,
        text=False
    )
    
    try:
        # Test 1: Initialize
        print("\n📡 Test 1: Initialize MCP connection")
        init_request = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {
                    "prompts": {},
                    "tools": {}
                },
                "clientInfo": {
                    "name": "test-client",
                    "version": "1.0.0"
                }
            }
        }
        response = send_mcp_request(server_process, init_request)
        print(f"✅ Initialize response: {json.dumps(response, indent=2) if response else 'No response'}")
        
        # Test 2: List available tools
        print("\n🔧 Test 2: List available tools")
        list_tools_request = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list"
        }
        response = send_mcp_request(server_process, list_tools_request)
        print(f"✅ Tools list response: {json.dumps(response, indent=2) if response else 'No response'}")
        
        # Test 3: List sources with pagination (our main feature)
        print("\n📦 Test 3: List sources with pagination")
        list_sources_request = {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/call",
            "params": {
                "name": "list_sources",
                "arguments": {
                    "page": "1",
                    "page_size": "5"
                }
            }
        }
        response = send_mcp_request(server_process, list_sources_request)
        print(f"✅ List sources response: {json.dumps(response, indent=2) if response else 'No response'}")
        
        # Test 4: Search tools (our new feature)
        print("\n🔍 Test 4: Search tools")
        search_tools_request = {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "tools/call",
            "params": {
                "name": "search_tools",
                "arguments": {
                    "query": "kubectl",
                    "page": "1",
                    "page_size": "5"
                }
            }
        }
        response = send_mcp_request(server_process, search_tools_request)
        print(f"✅ Search tools response: {json.dumps(response, indent=2) if response else 'No response'}")
        
        # Test 5: List available prompts
        print("\n📝 Test 5: List available prompts")
        list_prompts_request = {
            "jsonrpc": "2.0",
            "id": 5,
            "method": "prompts/list"
        }
        response = send_mcp_request(server_process, list_prompts_request)
        print(f"✅ Prompts list response: {json.dumps(response, indent=2) if response else 'No response'}")
        
        # Test 6: Get workflow generation prompt (our comprehensive prompts)
        print("\n🎯 Test 6: Get workflow generation prompt")
        get_prompt_request = {
            "jsonrpc": "2.0",
            "id": 6,
            "method": "prompts/get",
            "params": {
                "name": "workflow_generation",
                "arguments": {
                    "task_description": "Create a Kubernetes pod health check workflow",
                    "complexity": "medium",
                    "environment": "kubernetes"
                }
            }
        }
        response = send_mcp_request(server_process, get_prompt_request)
        print(f"✅ Workflow generation prompt: {json.dumps(response, indent=2) if response else 'No response'}")
        
        print("\n🎉 All MCP tests completed!")
        
    except Exception as e:
        print(f"❌ Error during testing: {e}")
        if server_process.stderr:
            stderr = server_process.stderr.read()
            if stderr:
                print(f"Server stderr: {stderr.decode()}")
    finally:
        server_process.terminate()
        server_process.wait()

if __name__ == "__main__":
    test_mcp_server()