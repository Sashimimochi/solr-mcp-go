package server

import (
	"errors"
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
	LlmBaseURL        string
	LlmAPIKey         string
	LlmModel          string
	EmbeddingBaseURL  string
	EmbeddingAPIKey   string
	EmbeddingModel    string
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
		LlmBaseURL:       config.GetEnv("LLM_BASE_URL", "http://localhost:8000/v1"),
		LlmAPIKey:        config.GetEnv("LLM_API_KEY", ""),
		LlmModel:         config.GetEnv("LLM_MODEL", "gpt-4o"),
		EmbeddingBaseURL: config.GetEnv("EMBEDDING_BASE_URL", "http://localhost:8000/v1"),
		EmbeddingAPIKey:  config.GetEnv("EMBEDDING_API_KEY", ""),
		EmbeddingModel:   config.GetEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
	}
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

func (st *State) ColOrDefault(c string) (string, error) {
	if c != "" {
		return c, nil
	}
	if st.DefaultCollection != "" {
		return st.DefaultCollection, nil
	}
	return "", errors.New("collection is required. set input.collection or SOLR_MCP_DEFAULT_COLLECTION")
}
