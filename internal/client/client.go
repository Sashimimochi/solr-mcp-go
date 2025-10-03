package client

import (
    "context"
    "log"

    "solr-mcp-go/internal/config"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func Run(url string) {
    ctx := context.Background()

    log.Printf("Connecting to MCP server at %s", url)

    client := mcp.NewClient(&mcp.Implementation{
        Name:    "solr-mcp-go-client",
        Version: config.Version,
    }, nil)

    session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
    if err != nil {
        log.Fatalf("Error connecting to MCP server: %v", err)
    }
    defer session.Close()

    log.Println("Connected to MCP server", session.ID())

    log.Println("Listing available tools...")
    toolsResult, err := session.ListTools(ctx, nil)
    if err != nil {
        log.Fatalf("Error listing tools: %v", err)
    }

    for _, tool := range toolsResult.Tools {
        log.Printf(" - %s: %s", tool.Name, tool.Description)
    }

    log.Println("Client completed successfully.")
}