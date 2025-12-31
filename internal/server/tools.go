package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"solr-mcp-go/internal/config"
	"solr-mcp-go/internal/solr"
	"solr-mcp-go/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	solr_sdk "github.com/stevenferrer/solr-go"
)

func AddTools(mcpServer *mcp.Server, st *State) []string {
	var toolNames []string

	// solr.query tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "solr.query",
		Description: "Search documents in Solr /select query",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection": map[string]any{
					"type":        "string",
					"description": "Solr collection name",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Solr query string (default: *:*)",
				},
				"fq": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Filter queries",
				},
				"fl": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Fields to return",
				},
				"sort": map[string]any{
					"type":        "string",
					"description": "Sort criteria (e.g., 'price asc')",
				},
				"start": map[string]any{
					"type":        "integer",
					"description": "Starting offset for pagination",
				},
				"rows": map[string]any{
					"type":        "integer",
					"description": "Number of rows to return",
				},
				"params": map[string]any{
					"type":        "object",
					"description": "Additional query parameters",
				},
				"echoParams": map[string]any{
					"type":        "boolean",
					"description": "Echo all parameters in response",
				},
			},
			"required": []string{"collection"},
		},
	}, st.toolQuery)
	toolNames = append(toolNames, "solr.query")

	// solr.ping tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "solr.ping",
		Description: "Check Solr cluster health (live nodes)",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, st.toolPing)
	toolNames = append(toolNames, "solr.ping")

	// solr.collection.health tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "solr.collection.health",
		Description: "Check specific collection health status",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection": map[string]any{
					"type":        "string",
					"description": "Solr collection name",
				},
			},
			"required": []string{"collection"},
		},
	}, st.toolCollectionHealth)
	toolNames = append(toolNames, "solr.collection.health")

	// solr.schema tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "solr.schema",
		Description: "Get Solr schema information",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection": map[string]any{
					"type":        "string",
					"description": "Solr collection name",
				},
			},
			"required": []string{"collection"},
		},
	}, st.toolSchema)
	toolNames = append(toolNames, "solr.schema")

	return toolNames
}

// Basic Tools
func (st *State) toolQuery(ctx context.Context, _ *mcp.CallToolRequest, in types.QueryIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Collection) == "" {
		return nil, nil, errors.New("input.collection is required")
	}
	qString := in.Query
	if qString == "" {
		qString = "*:*"
	}

	parser := solr_sdk.NewStandardQueryParser().Query(qString).BuildParser()
	query := solr_sdk.NewQuery(parser)
	if len(in.Fields) > 0 {
		query = query.Fields(in.Fields...)
	}
	if len(in.FilterQuery) > 0 {
		query = query.Filters(in.FilterQuery...)
	}
	if in.Sort != "" {
		query = query.Sort(in.Sort)
	}
	if in.Start != nil {
		query = query.Offset(*in.Start)
	}
	if in.Rows != nil {
		query = query.Limit(*in.Rows)
	}

	// Merge params with echoParams if needed
	params := make(map[string]any)
	for k, v := range in.Params {
		params[k] = v
	}
	if in.EchoParams {
		params["echoParams"] = "all"
	}
	if len(params) > 0 {
		query = query.Params(solr_sdk.M(params))
	}

	slog.Debug("Executing Solr query", "collection", in.Collection, "query", query)

	resp, err := solr.QueryWithRawResponse(ctx, st.HttpClient, st.BaseURL, st.BasicUser, st.BasicPass, in.Collection, query)

	return nil, resp, err
}

func (st *State) toolPing(ctx context.Context, _ *mcp.CallToolRequest, in types.PingIn) (*mcp.CallToolResult, any, error) {
	// Use CLUSTERSTATUS API without collection parameter to get cluster-wide status
	// Following solr-go SDK pattern (similar to CreateCollection/DeleteCollection)
	urlStr := fmt.Sprintf("%s/solr/admin/collections?action=CLUSTERSTATUS&wt=json", st.BaseURL)
	slog.Debug("Checking cluster status", "url", urlStr)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %v", err)
	}

	// Add basic auth if configured
	if st.BasicUser != "" && st.BasicPass != "" {
		req.SetBasicAuth(st.BasicUser, st.BasicPass)
	}

	// Send request
	httpResp, err := st.HttpClient.Do(req)
	if err != nil {
		slog.Error("Cluster status request failed", "error", err)
		return nil, nil, fmt.Errorf("cluster status request: %v", err)
	}
	defer httpResp.Body.Close()

	// Decode response
	var clusterResp config.ClusterStatusResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&clusterResp); err != nil {
		slog.Error("Failed to decode cluster status", "error", err)
		return nil, nil, fmt.Errorf("decode response: %v", err)
	}

	// Return cluster-wide health information
	return nil, map[string]any{
		"status":     clusterResp.ResponseHeader.Status,
		"qtime":      clusterResp.ResponseHeader.QTime,
		"live_nodes": clusterResp.Cluster.LiveNodes,
		"num_nodes":  len(clusterResp.Cluster.LiveNodes),
	}, nil
}

func (st *State) toolCollectionHealth(ctx context.Context, _ *mcp.CallToolRequest, in types.CollectionHealthIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Collection) == "" {
		return nil, nil, errors.New("input.collection is required")
	}

	// Use CLUSTERSTATUS API with collection parameter
	// Following solr-go SDK pattern
	urlStr := fmt.Sprintf("%s/solr/admin/collections?action=CLUSTERSTATUS&collection=%s&wt=json", st.BaseURL, in.Collection)
	slog.Debug("Checking collection health", "collection", in.Collection, "url", urlStr)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %v", err)
	}

	// Add basic auth if configured
	if st.BasicUser != "" && st.BasicPass != "" {
		req.SetBasicAuth(st.BasicUser, st.BasicPass)
	}

	// Send request
	httpResp, err := st.HttpClient.Do(req)
	if err != nil {
		slog.Error("Collection health check failed", "error", err)
		return nil, nil, fmt.Errorf("collection health check: %v", err)
	}
	defer httpResp.Body.Close()

	// Decode response
	var clusterResp config.ClusterStatusResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&clusterResp); err != nil {
		slog.Error("Failed to decode collection health", "error", err)
		return nil, nil, fmt.Errorf("decode response: %v", err)
	}

	// Extract collection status
	collStatus, ok := clusterResp.Cluster.Collections[in.Collection]
	if !ok {
		return nil, nil, fmt.Errorf("collection %s not found", in.Collection)
	}

	// Build detailed health response
	return nil, map[string]any{
		"status":     clusterResp.ResponseHeader.Status,
		"qtime":      clusterResp.ResponseHeader.QTime,
		"health":     collStatus.Health,
		"shards":     collStatus.Shards,
		"configName": collStatus.ConfigName,
	}, nil
}

// Smart Search Tool
func (st *State) toolSchema(ctx context.Context, _ *mcp.CallToolRequest, in types.SchemaIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Collection) == "" {
		return nil, nil, errors.New("input.collection is required")
	}

	sCtx := solr.SchemaContext{
		HttpClient: st.HttpClient,
		BaseURL:    st.BaseURL,
		User:       st.BasicUser,
		Pass:       st.BasicPass,
		Cache:      &st.SchemaCache,
	}
	fc, err := solr.GetFieldCatalog(ctx, sCtx, in.Collection)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get schema: %v", err)
	}
	return nil, fc, nil
}
