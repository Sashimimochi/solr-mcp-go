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

// AIAgentCompatibilityMiddleware wraps the MCP handler to handle AI agent-specific HTTP patterns
type AIAgentCompatibilityMiddleware struct {
	mcpHandler http.Handler
}

func (m *AIAgentCompatibilityMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Some AI agents may send an initial GET without a session ID.
	// Convert it to POST so the MCP server can handle initialization properly.
	if r.Method == http.MethodGet && r.Header.Get("Mcp-Session-Id") == "" {
		slog.Debug("Converting initial GET to POST for AI agent compatibility",
			"path", r.URL.Path,
			"remote", r.RemoteAddr)

		// Convert GET to POST
		r.Method = http.MethodPost
		r.ContentLength = 0
		r.Body = http.NoBody

		// Ensure headers meet MCP spec expectations
		if r.Header.Get("Content-Type") == "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if r.Header.Get("Accept") == "" {
			r.Header.Set("Accept", "application/json, text/event-stream")
		}
	}

	// Wrap ResponseWriter to mutate DELETE responses if needed
	wrappedWriter := &responseWrapper{
		ResponseWriter: w,
		request:        r,
	}

	// Delegate to the MCP handler
	m.mcpHandler.ServeHTTP(wrappedWriter, r)
}

// responseWrapper wraps http.ResponseWriter to transform specific responses
type responseWrapper struct {
	http.ResponseWriter
	request     *http.Request
	statusCode  int
	wroteHeader bool
}

func (rw *responseWrapper) WriteHeader(statusCode int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.statusCode = statusCode

	// Convert 204 No Content for DELETE requests to 200 OK.
	// Some clients (e.g., certain AI agents) may not handle 204 responses correctly.
	if rw.request.Method == http.MethodDelete && statusCode == http.StatusNoContent {
		slog.Debug("Converting DELETE 204 to 200 for AI agent compatibility")
		rw.ResponseWriter.Header().Set("Content-Type", "application/json")
		rw.ResponseWriter.WriteHeader(http.StatusOK)
		// 204 typically has no body; write a minimal JSON body here
		jsonBody := []byte(`{"status":"ok","message":"session terminated"}`)
		rw.ResponseWriter.Write(jsonBody)
		return
	}

	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWrapper) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	// For all other responses, write as-is
	return rw.ResponseWriter.Write(data)
}

func Run(url string) {
	st := NewServerState()

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "solr-mcp-go",
		Version: config.Version,
	}, nil)

	toolNames := AddTools(mcpServer, st)

	// Create MCP Streamable HTTP handler
	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	// Wrap with AI agent compatibility middleware
	aiAgentCompatHandler := &AIAgentCompatibilityMiddleware{
		mcpHandler: mcpHandler,
	}

	// Add logging middleware
	handlerWithLogging := utils.LoggingHandler(aiAgentCompatHandler)

	slog.Info("MCP server listening", "address", url)
	slog.Info("Available tools", "tools", strings.Join(toolNames, ", "))
	slog.Info("AI agent compatibility mode enabled")

	if err := http.ListenAndServe(url, handlerWithLogging); err != nil {
		slog.Error("Error running MCP server", "error", err)
		os.Exit(1)
	}
}
