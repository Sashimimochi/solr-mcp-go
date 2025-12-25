#!/bin/bash
# MCP Protocol test script using curl

SERVER_URL="${1:-http://localhost:8000}"
TEMP_DIR=$(mktemp -d)
COOKIE_FILE="$TEMP_DIR/cookies.txt"
SESSION_ID_FILE="$TEMP_DIR/session_id.txt"
COLLECTION="unified"

cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

echo "=== Step 1: Initialize ==="
RESPONSE=$(curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -D "$TEMP_DIR/headers1.txt" \
    -d '{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        }
    }')

echo "$RESPONSE"
echo ""

# Extract session ID from response headers
SESSION_ID=$(grep -i "Mcp-Session-Id:" "$TEMP_DIR/headers1.txt" | cut -d' ' -f2 | tr -d '\r\n')
echo "Session ID: $SESSION_ID"
echo "$SESSION_ID" > "$SESSION_ID_FILE"
echo ""

if [ -z "$SESSION_ID" ]; then
    echo "Error: No session ID received"
    exit 1
fi

echo "=== Step 2: Send initialized notification ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "method": "notifications/initialized"
    }'
echo -e "\n"

# Wait a moment for the notification to be processed
sleep 1

echo "=== Step 3: Call solr.ping tool ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": {
            "name": "solr.ping"
        }
    }' | grep '^data:' \
    | sed -E 's/^data: //' \
    | jq .
echo -e "\n"

echo "=== Step 4: List tools ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/list"
    }' | grep '^data:' \
    | sed -E 's/^data: //' \
    | jq .
echo -e "\n"

echo "=== Step 5: Call solr.collection.health tool ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "id": 4,
        "method": "tools/call",
        "params": {
            "name": "solr.collection.health",
            "arguments": {
                "collection": "'$COLLECTION'"
            }
        }
    }' | grep '^data:' \
    | sed -E 's/^data: //' \
    | jq .
echo -e "\n"

echo "=== Step 6: Call solr.schema tool ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "id": 5,
        "method": "tools/call",
        "params": {
            "name": "solr.schema",
            "arguments": {
                "collection": "'$COLLECTION'"
            }
        }
    }' | grep '^data:' \
    | sed -E 's/^data: //' \
    | jq .
echo -e "\n"

echo "=== Step 7: Call solr.query tool ==="
curl -s -X POST "$SERVER_URL/" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $SESSION_ID" \
    -d '{
        "jsonrpc": "2.0",
        "id": 6,
        "method": "tools/call",
        "params": {
            "name": "solr.query",
            "arguments": {
                "collection": "'$COLLECTION'",
                "query": "text:カフェ",
                "rows": 5,
                "start": 0,
                "fl": ["*", "score"],
                "fq": [],
                "sort": "id asc",
                "params": {"hoge": "1"},
                "echoParams": true
            }
        }
    }' | grep '^data:' \
    | sed -E 's/^data: //' \
    | jq .
echo -e "\n"