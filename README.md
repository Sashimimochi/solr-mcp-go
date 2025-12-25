# solr-mcp-go

`solr-mcp-go` is a Model Context Protocol (MCP) server that provides Apache Solr access through standardized MCP tools. It enables AI assistants and other MCP clients to query, monitor, and interact with Solr collections.

## Features

*   **Standard Solr Query Tool (`solr.query`)**:
    *   Execute Solr `/select` queries with full parameter support
    *   Support for filter queries, field selection, sorting, and pagination
    *   Echo parameters option for debugging
    *   Raw JSON response format
*   **Health Monitoring Tools**:
    *   `solr.ping`: Check cluster-wide health and live nodes
    *   `solr.collection.health`: Check specific collection health status including shard and replica information
*   **Schema Information (`solr.schema`)**:
    *   Retrieve complete schema information for any collection
    *   Automatic schema caching with configurable TTL (default: 10 minutes)
    *   Field metadata support for enhanced documentation
    *   Support for collections with special characters in names
*   **HTTP Transport**:
    *   Streamable HTTP transport for MCP protocol
    *   Session management support
    *   Basic authentication support for Solr
*   **AI Agent Compatibility**:
    *   Built-in middleware for AI agent HTTP patterns
    *   Automatic GET to POST conversion for initialization
    *   DELETE response transformation (204 → 200 with JSON body)
    *   Compatible with Dify and other AI platforms

## Requirements

*   Go (1.24 or later)
*   A running Apache Solr instance

## Setup

1.  Clone the repository:
    ```sh
    git clone https://github.com/yourusername/solr-mcp-go.git
    cd solr-mcp-go
    ```

2.  Install the required Go modules:
    ```sh
    go mod tidy
    ```

3.  Set the following environment variables:

    | Environment Variable          | Description                                        | Default Value                    |
    | ----------------------------- | -------------------------------------------------- | -------------------------------- |
    | `SOLR_MCP_SOLR_URL`           | The URL of your Solr server                        | `http://localhost:8983`          |
    | `SOLR_MCP_DEFAULT_COLLECTION` | The default Solr collection to use                 | `gettingstarted`                 |
    | `SOLR_BASIC_USER`             | Basic authentication username for Solr (optional)  | ""                               |
    | `SOLR_BASIC_PASS`             | Basic authentication password for Solr (optional)  | ""                               |
    | `LOG_LEVEL`                   | The log level to use (DEBUG, INFO, WARN, ERROR)    | `INFO`                           |

## Running the Server

Run the following command to start the MCP server. It listens on `localhost:9000` by default:

```sh
go run ./cmd/solr-mcp-go/ server
```

You can specify a different host and port using flags:

```sh
go run ./cmd/solr-mcp-go/ -host 0.0.0.0 -port 8000 server
```

### Integration with Dify

This MCP server includes built-in compatibility for Dify. Simply start the server and configure Dify to connect directly:

```
http://host.docker.internal:9000
```

The server automatically handles Dify's specific HTTP request patterns through an internal compatibility middleware:
- Converts initial GET requests to POST for proper MCP initialization
- Transforms DELETE 204 responses to 200 with JSON body for better AI agent compatibility
- Ensures proper Content-Type and Accept headers

For detailed instructions and troubleshooting, see [DIFY.md](DIFY.md).

## Available Tools

### solr.query

Execute a standard Solr select query.

**Input Parameters:**
- `collection` (required): The Solr collection to query
- `query`: The query string (default: `*:*`)
- `fq`: Filter queries (array of strings)
- `fl`: Fields to return (array of strings)
- `sort`: Sort criteria (e.g., `price asc`, `score desc`)
- `start`: Starting offset for pagination
- `rows`: Number of rows to return
- `params`: Additional query parameters (object/map)
- `echoParams`: Echo all parameters in response (boolean)

**Example:**
```json
{
  "collection": "techproducts",
  "query": "electronics",
  "fq": ["inStock:true"],
  "fl": ["id", "name", "price"],
  "sort": "price asc",
  "start": 0,
  "rows": 10,
  "echoParams": true
}
```

**Response:**
Returns the raw Solr JSON response including `responseHeader` and `response` objects.

### solr.ping

Check the health of the Solr cluster.

**Input Parameters:**
None required.

**Output:**
- `status`: Response status code
- `qtime`: Query time in milliseconds
- `live_nodes`: List of live nodes in the cluster
- `num_nodes`: Number of live nodes

**Example Response:**
```json
{
  "status": 0,
  "qtime": 5,
  "live_nodes": ["node1:8983_solr", "node2:8983_solr"],
  "num_nodes": 2
}
```

### solr.collection.health

Check the health status of a specific collection.

**Input Parameters:**
- `collection` (required): The collection name

**Output:**
- `status`: Response status code
- `qtime`: Query time in milliseconds
- `health`: Collection health status (e.g., "GREEN", "YELLOW", "RED")
- `shards`: Detailed shard information including replicas
- `configName`: Configuration set name used by the collection

**Example:**
```json
{
  "collection": "techproducts"
}
```

### solr.schema

Retrieve schema information for a collection.

**Input Parameters:**
- `collection` (required): The collection name

**Output:**
- `UniqueKey`: The unique key field name
- `All`: Array of all fields with their properties:
  - `name`: Field name
  - `type`: Field type
  - `indexed`: Whether the field is indexed
  - `stored`: Whether the field is stored
  - `multiValued`: Whether the field supports multiple values
- `Metadata`: Optional field metadata from `field_metadata.json` (if available)

**Features:**
- Automatic caching with 10-minute TTL
- Supports collections with special characters in names
- Gracefully handles missing metadata files

**Example:**
```json
{
  "collection": "techproducts"
}
```

## Usage Examples

### Using the Test Script

A test script is provided to demonstrate the MCP protocol flow:

```sh
./tests/test-mcp-curl.sh http://localhost:9000
```

This script performs a complete MCP session:
1. Initialize the session
2. List available tools
3. Call the `solr.query` tool
4. Terminate the session

### Using curl Directly

Initialize a session:
```sh
curl -X POST http://localhost:9000/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test-client", "version": "1.0.0"}
    }
  }'
```

Call a tool (using the session ID from initialization):
```sh
curl -X POST http://localhost:9000/ \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "solr.query",
      "arguments": {
        "collection": "techproducts",
        "query": "*:*",
        "rows": 5
      }
    }
  }'
```

Check cluster health:
```sh
curl -X POST http://localhost:9000/ \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "solr.ping",
      "arguments": {}
    }
  }'
```

## Project Structure

```
solr-mcp-go/
├── cmd/
│   └── solr-mcp-go/          # Main MCP server application
├── internal/
│   ├── client/               # MCP client implementation
│   ├── config/               # Configuration and Solr client setup
│   ├── server/               # MCP server and tools implementation
│   │   ├── server.go         # Server setup and AI compatibility middleware
│   │   ├── tools.go          # Tool definitions and implementations
│   │   ├── middleware_test.go # Middleware tests
│   │   ├── server_test.go    # Server initialization tests
│   │   └── tools_test.go     # Tool implementation tests
│   ├── solr/                 # Solr-specific logic
│   │   ├── query_builder.go  # Query construction and execution
│   │   ├── schema.go         # Schema retrieval and caching
│   │   ├── query_builder_test.go
│   │   └── schema_test.go
│   ├── types/                # Type definitions
│   └── utils/                # Utility functions
├── tests/                    # Integration test scripts
│   ├── test-mcp-curl.sh      # MCP protocol test script
│   └── test-ai-agent-compatibility.sh # AI agent compatibility tests
└── .github/
    └── workflows/            # CI/CD workflows
        └── go.yml            # GitHub Actions workflow
```

## Testing

Run all tests:
```sh
go test -v ./...
```

Run tests with coverage:
```sh
go test -cover ./...
```

Run tests for a specific package:
```sh
go test -v ./internal/solr/
go test -v ./internal/server/
```

Run integration tests:
```sh
# Test MCP protocol flow
./tests/test-mcp-curl.sh http://localhost:9000

# Test AI agent compatibility
./tests/test-ai-agent-compatibility.sh http://localhost:9000
```

### Test Coverage

The project maintains comprehensive test coverage across all packages:
- **Server package**: Tool implementations, middleware, initialization
- **Solr package**: Query building, schema retrieval, caching
- **Utils package**: Helper functions, middleware
- **Types package**: Data structure definitions

All tests use table-driven approaches and HTTP mock servers (`httptest`) for realistic scenarios.

## Development

The project follows these conventions:
- Code follows Google's Go coding standards
- All tests include descriptive comments explaining their purpose
- Test functions use table-driven tests where appropriate
- HTTP mock servers (`httptest`) are used for integration testing
- Comprehensive error handling with structured logging (`slog`)
- Multi-line log output is formatted as grouped attributes

### Logging

The server uses structured logging with configurable log levels:
- `DEBUG`: Detailed operation logs including query execution
- `INFO`: Normal operation logs (default)
- `WARN`: Warning messages
- `ERROR`: Error conditions

Set the log level via the `LOG_LEVEL` environment variable.

## Architecture

### AI Agent Compatibility Middleware

The server includes a specialized middleware layer ([`AIAgentCompatibilityMiddleware`](internal/server/server.go)) that handles common HTTP patterns from AI agents:

1. **GET to POST Conversion**: Converts initial GET requests without session IDs to POST requests
2. **Response Transformation**: Converts DELETE 204 responses to 200 with JSON body
3. **Header Normalization**: Ensures proper Content-Type and Accept headers

### Schema Caching

The schema system ([`internal/solr/schema.go`](internal/solr/schema.go)) implements intelligent caching:
- Configurable TTL (default: 10 minutes)
- Per-collection cache entries
- Thread-safe cache access
- Automatic cache invalidation after TTL expiration
- Support for TTL=0 (no caching)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please ensure that:
1. All tests pass (`go test ./...`)
2. New features include appropriate tests
3. Code follows the existing style and conventions
4. Documentation is updated for new features
