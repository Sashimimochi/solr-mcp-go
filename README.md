# solr-mcp-go

`solr-mcp-go` is a Model Context Protocol (MCP) server that uses Apache Solr as a backend. It leverages Large Language Models (LLMs) to interpret natural language queries from users and translates them into advanced Solr search queries, including keyword, vector, and hybrid searches.

## Features

*   **Smart Search with Natural Language**:
    *   An LLM interprets ambiguous natural language queries (e.g., "Show me expensive video cards") to generate an optimal Solr query plan.
    *   It automatically executes a keyword search (`edismax`), a vector search (`knn`), or a hybrid search combining both, depending on the query's content.
*   **Automatic Schema Analysis**:
    *   Automatically fetches and caches schema information from your Solr collection.
    *   Infers the semantic meaning of fields (e.g., price, date, brand) from their names and types to improve query generation accuracy (see `GuessFields` in [`internal/solr/schema.go`](internal/solr/schema.go)).
*   **MCP Tools**:
    *   `solr.searchSmart`: The main tool for executing natural language queries.
    *   `solr.query`: A tool for executing standard Solr select queries.
    *   `solr.commit`: A tool for committing changes to Solr.
    *   `solr.ping`: A tool for health-checking the Solr server.

## Requirements

*   Go (1.24 or later)
*   A running Apache Solr instance
*   An OpenAI-compatible API endpoint for chat completions (LLM)
*   An OpenAI-compatible API endpoint for embedding models

## Setup

1.  Clone the repository.
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
    | `LLM_BASE_URL`                | The base URL for the chat completion API           | `http://localhost:8000/v1`       |
    | `LLM_API_KEY`                 | The API key for the LLM API                        | ""                               |
    | `LLM_MODEL`                   | The LLM model name to use                          | `gpt-4o`                         |
    | `EMBEDDING_BASE_URL`          | The base URL for the embedding API                 | `http://localhost:8000/v1`       |
    | `EMBEDDING_API_KEY`           | The API key for the embedding API                  | ""                               |
    | `EMBEDDING_MODEL`             | The embedding model name to use                    | `text-embedding-3-small`         |

## Running the Server

Run the following command to start the MCP server. It listens on `localhost:8080` by default.

```sh
go run ./cmd/solr-mcp-go/ server
```

You can specify a different host and port using flags:

```sh
go run ./cmd/solr-mcp-go/ -host 0.0.0.0 -port 8000 server
```

## Usage

Once the server is running, you can call its tools from an MCP client.
Here is an example of calling the `solr.searchSmart` tool using `curl`:

```sh
curl -X POST http://localhost:8080/ \
-H "Content-Type: application/json" \
-d '{
  "toolName": "solr.searchSmart",
  "input": {
    "collection": "techproducts",
    "query": "show me expensive video cards"
  }
}'
```

The server will communicate with the LLM to generate a query plan, execute the search on Solr, and return the results.

## Testing

To run all tests for the project, use the following command:

```sh
go test -v ./...
```

To run tests with a coverage report:

```sh
go test -cover ./...
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
