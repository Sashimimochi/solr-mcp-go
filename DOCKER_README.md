# solr-mcp-go

Model Context Protocol (MCP) server for Apache Solr - enables AI assistants and other MCP clients to query, monitor, and interact with Solr collections.

## Quick Start

```bash
docker run -d \
  --name solr-mcp-go \
  -p 9000:9000 \
  -e SOLR_MCP_SOLR_URL=http://your-solr-host:8983/solr \
  -e SOLR_MCP_DEFAULT_COLLECTION=your_collection \
  343mochi/solr-mcp-go:latest \
  -host 0.0.0.0 -port 9000
```

## Features

- **Standard Solr Query Tool** (`solr.query`): Execute Solr queries with full parameter support
- **Health Monitoring**: Check cluster and collection health
- **Schema Information**: Retrieve complete schema with automatic caching
- **AI Agent Compatible**: Built-in middleware for Dify and other AI platforms

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SOLR_MCP_SOLR_URL` | Solr server URL | `http://localhost:8983/solr` |
| `SOLR_MCP_DEFAULT_COLLECTION` | Default collection name | `gettingstarted` |
| `SOLR_BASIC_USER` | Solr basic auth username (optional) | - |
| `SOLR_BASIC_PASS` | Solr basic auth password (optional) | - |
| `LOG_LEVEL` | Log level: DEBUG, INFO, WARN, ERROR | `INFO` |

## Command Line Arguments

```bash
docker run 343mochi/solr-mcp-go:latest [OPTIONS]

Options:
  -host string    Host to listen on (default "localhost")
  -port int       Port number to listen on (default 9000)
  -proto string   Protocol prefix (ignored for server mode) (default "http")
```

## Examples

### Basic Usage

Connect to a Solr instance:

```bash
docker run -d \
  --name solr-mcp-go \
  -p 9000:9000 \
  -e SOLR_MCP_SOLR_URL=http://solr.example.com:8983/solr \
  343mochi/solr-mcp-go:latest
```

### With Authentication

Connect to Solr with basic authentication:

```bash
docker run -d \
  --name solr-mcp-go \
  -p 9000:9000 \
  -e SOLR_MCP_SOLR_URL=http://solr.example.com:8983/solr \
  -e SOLR_BASIC_USER=admin \
  -e SOLR_BASIC_PASS=secret \
  343mochi/solr-mcp-go:latest
```

### Custom Port

Run on a different port:

```bash
docker run -d \
  --name solr-mcp-go \
  -p 8080:8080 \
  -e SOLR_MCP_SOLR_URL=http://solr.example.com:8983/solr \
  343mochi/solr-mcp-go:latest \
  -host 0.0.0.0 -port 8080
```

### Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  solr:
    image: solr:9
    ports:
      - "8983:8983"
    command: solr-precreate mycollection

  solr-mcp-go:
    image: 343mochi/solr-mcp-go:latest
    ports:
      - "9000:9000"
    environment:
      - SOLR_MCP_SOLR_URL=http://solr:8983/solr
      - SOLR_MCP_DEFAULT_COLLECTION=mycollection
      - LOG_LEVEL=INFO
    depends_on:
      - solr
    command: ["-host", "0.0.0.0", "-port", "9000"]
```

Run with:
```bash
docker-compose up -d
```

## Integration with AI Platforms

### Dify

Configure Dify to connect to the MCP server:

```
http://host.docker.internal:9000
```

The server automatically handles Dify's HTTP request patterns for seamless integration.

## Available MCP Tools

- `solr.query` - Execute Solr select queries
- `solr.ping` - Check cluster-wide health
- `solr.collection.health` - Check collection health status
- `solr.schema` - Retrieve collection schema information

## Health Check

Check if the server is running:

```bash
curl http://localhost:9000
```

## Documentation

- [GitHub Repository](https://github.com/Sashimimochi/solr-mcp-go)
- [Full Documentation](https://github.com/Sashimimochi/solr-mcp-go#readme)
- [MCP Protocol](https://modelcontextprotocol.io)

## Support

- Report issues: [GitHub Issues](https://github.com/Sashimimochi/solr-mcp-go/issues)
- Source code: [GitHub](https://github.com/Sashimimochi/solr-mcp-go)

## License

MIT License - See [LICENSE](https://github.com/Sashimimochi/solr-mcp-go/blob/main/LICENSE) for details.
