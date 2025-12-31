#!/bin/bash

# Test script to simulate an AI Agent connecting and listing MCP tools

set -e

MCP_URL="${1:-http://localhost:9000}"

echo "=== Testing MCP Server at $MCP_URL ==="
echo ""

# 1. Initialize (POST request)
echo "1. Initializing session..."
# Get both response headers and body
INIT_FULL=$(curl -si -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "ai-agent-test", "version": "1.0.0"}
    }
  }')

# Extract and print the SSE data body
INIT_BODY=$(echo "$INIT_FULL" | sed -n '/^data:/p' | sed -E 's/^data: //')
echo "$INIT_BODY" | jq .
echo ""

# Get session ID from Mcp-Session-Id response header
SESSION_ID=$(echo "$INIT_FULL" | grep -i '^Mcp-Session-Id:' | sed -E 's/^[^:]+: *//;s/\r$//')
if [ -z "$SESSION_ID" ]; then
  echo "❌ Failed to get session ID from Mcp-Session-Id header"
  exit 1
fi

echo "✅ Session ID: $SESSION_ID"
echo ""

# 2. List tools
echo "2. Listing tools..."
TOOLS_RESPONSE=$(curl -s -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }')

# Extract SSE data line and print as JSON
TOOLS_DATA=$(echo "$TOOLS_RESPONSE" | grep '^data:' | sed -E 's/^data: //')
echo "$TOOLS_DATA" | jq .
echo ""

# Check tools' InputSchema
TOOL_COUNT=$(echo "$TOOLS_DATA" | jq '.result.tools | length')
echo "✅ Found $TOOL_COUNT tools"
echo ""

# Inspect each tool's InputSchema
echo "3. Checking InputSchemas..."
for i in $(seq 0 $((TOOL_COUNT - 1))); do
  TOOL_NAME=$(echo "$TOOLS_DATA" | jq -r ".result.tools[$i].name")
  INPUT_SCHEMA=$(echo "$TOOLS_DATA" | jq ".result.tools[$i].inputSchema")

  echo "Tool: $TOOL_NAME"
  echo "  InputSchema type: $(echo "$INPUT_SCHEMA" | jq -r '.type // "MISSING"')"

  # Print number of properties
  PROP_COUNT=$(echo "$INPUT_SCHEMA" | jq '.properties | length // 0')
  echo "  Properties count: $PROP_COUNT"

  # Check for null values in schema
  if echo "$INPUT_SCHEMA" | jq -e 'recurse | select(type == "null")' > /dev/null 2>&1; then
    echo "  ⚠️  WARNING: Contains null values"
  else
    echo "  ✅ No null values"
  fi

  echo ""
done

# 4. Delete session (AI agent compatibility test)
echo "4. Testing session deletion..."
DELETE_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X DELETE "$MCP_URL" \
  -H "Mcp-Session-Id: $SESSION_ID")

HTTP_CODE=$(echo "$DELETE_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$DELETE_RESPONSE" | grep -v "HTTP_CODE:")

echo "HTTP Status: $HTTP_CODE"
echo "Response Body: $BODY"

if [ "$HTTP_CODE" = "200" ]; then
  echo "✅ DELETE returns 200 OK (AI agent compatible)"
elif [ "$HTTP_CODE" = "204" ]; then
  echo "⚠️  DELETE returns 204 No Content (may cause issues with some AI agents)"
else
  echo "❌ Unexpected status code: $HTTP_CODE"
  exit 1
fi

echo ""
echo "=== All tests completed ==="
