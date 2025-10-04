package client

import (
	"context"
	"log/slog"

	"solr-mcp-go/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Run(url string) {
	ctx := context.Background()

	slog.Info("Connecting to MCP server", "url", url)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "solr-mcp-go-client",
		Version: config.Version,
	}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		slog.Error("Error connecting to MCP server", "error", err)
	}
	defer session.Close()

	slog.Info("Connected to MCP server", "session_id", session.ID())

	slog.Info("Listing available tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		slog.Error("Error listing tools", "error", err)
	}

	for _, tool := range toolsResult.Tools {
		slog.Info("tool", "name", tool.Name, "description", tool.Description)
	}

	slog.Info("Client completed successfully.")
}
