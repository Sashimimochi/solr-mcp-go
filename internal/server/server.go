package server

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"solr-mcp-go/internal/config"
	"solr-mcp-go/internal/types"
	"solr-mcp-go/internal/utils"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	solr "github.com/stevenferrer/solr-go"
)

type State struct {
	SolrClient        *solr.JSONClient
	BaseURL           string
	DefaultCollection string
	HttpClient        *http.Client
	BasicUser         string
	BasicPass         string
	SchemaCache       types.SchemaCache
}

func NewServerState() *State {
	client, baseURL, user, pass, httpClient := config.NewSolrClient()

	st := &State{
		SolrClient:        client,
		BaseURL:           baseURL,
		DefaultCollection: config.GetEnv("SOLR_MCP_DEFAULT_COLLECTION", "gettingstarted"),
		HttpClient:        httpClient,
		BasicUser:         user,
		BasicPass:         pass,
		SchemaCache: types.SchemaCache{
			LastFetch: make(map[string]time.Time),
			TTL:       10 * time.Minute,
			ByCol:     make(map[string]*types.FieldCatalog),
		},
	}

	slog.Info("Configured Solr client", "base_url", baseURL, "default_collection", st.DefaultCollection)
	return st
}

func Run(url string) {
	st := NewServerState()

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "solr-mcp-go",
		Version: config.Version,
	}, nil)

	toolNames := AddTools(mcpServer, st)

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	handlerWithLogging := utils.LoggingHandler(handler)

	slog.Info("MCP server listening", "address", url)
	slog.Info("Available tools", "tools", strings.Join(toolNames, ", "))

	if err := http.ListenAndServe(url, handlerWithLogging); err != nil {
		slog.Error("Error running MCP server", "error", err)
		os.Exit(1)
	}
}
