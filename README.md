# solr-mcp-go

`solr-mcp-go` is a Model Context Protocol (MCP) server that provides Apache Solr access through standardized MCP tools. It enables AI assistants and other MCP clients to query, monitor, and interact with Solr collections.

## Features

*   **Standard Solr Query Tool (`solr.query`)**:
    *   Execute Solr `/select` queries with full parameter support
    *   Support for filter queries, field selection, sorting, and pagination
    *   Echo parameters option for debugging
*   **Health Monitoring Tools**:
    *   `solr.ping`: Check cluster-wide health and live nodes
    *   `solr.collection.health`: Check specific collection health status including shard and replica information
*   **Schema Information (`solr.schema`)**:
    *   Retrieve complete schema information for any collection
    *   Automatic schema caching with configurable TTL
    *   Field metadata support for enhanced documentation
*   **HTTP Transport**:
    *   Streamable HTTP transport for MCP protocol
    *   Session management support
    *   Basic authentication support for Solr

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

## Available Tools

### solr.query

Execute a standard Solr select query.

**Input Parameters:**
- `collection` (required): The Solr collection to query
- `query`: The query string (default: `*:*`)
- `fq`: Filter queries (array of strings)
- `fl`: Fields to return (array of strings)
- `sort`: Sort criteria
- `start`: Starting offset
- `rows`: Number of rows to return
- `params`: Additional query parameters (map)
- `echoParams`: Echo all parameters in response (boolean)

**Example:**
```json
{
  "collection": "techproducts",
  "query": "electronics",
  "fq": ["inStock:true"],
  "fl": ["id", "name", "price"],
  "sort": "price asc",
  "rows": 10,
  "echoParams": true
}
```

### solr.ping

Check the health of the Solr cluster.

**Output:**
- `status`: Response status
- `qtime`: Query time
- `live_nodes`: List of live nodes
- `num_nodes`: Number of live nodes

### solr.collection.health

Check the health status of a specific collection.

**Input Parameters:**
- `collection` (required): The collection name

**Output:**
- `status`: Response status
- `qtime`: Query time
- `health`: Collection health status
- `shards`: Detailed shard information
- `configName`: Configuration name

### solr.schema

Retrieve schema information for a collection.

**Input Parameters:**
- `collection` (required): The collection name

**Output:**
- `UniqueKey`: The unique key field name
- `All`: Array of all fields with their types and properties
- `Metadata`: Optional field metadata (from `field_metadata.json`)

## Usage Examples

### Using the Test Script

A test script is provided to demonstrate the MCP protocol flow:

```sh
./test-mcp-curl.sh http://localhost:9000
```

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

## Project Structure

```
solr-mcp-go/
├── cmd/
│   └── solr-mcp-go/          # Main application entry point
├── internal/
│   ├── client/               # MCP client implementation
│   ├── config/               # Configuration and Solr client setup
│   ├── server/               # MCP server and tools implementation
│   ├── solr/                 # Solr-specific logic (queries, schema)
│   ├── types/                # Type definitions
│   └── utils/                # Utility functions
└── .github/
    └── workflows/            # CI/CD workflows
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
```

## Development

The project follows these conventions:
- Code follows Google's Go coding standards
- All tests include descriptive comments explaining their purpose
- Test functions use table-driven tests where appropriate
- HTTP mock servers (`httptest`) are used for integration testing

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please ensure that:
1. All tests pass (`go test ./...`)
2. New features include appropriate tests
3. Code follows the existing style and conventions
