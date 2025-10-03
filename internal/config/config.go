package config

import (
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    solr "github.com/stevenferrer/solr-go"
)

const Version = "0.1.0"

func GetEnv(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}

func NewSolrClient() (*solr.JSONClient, string, string, string, *http.Client) {
    baseURL := strings.TrimRight(GetEnv("SOLR_MCP_SOLR_URL", "http://localhost:8983"), "/")
    user := GetEnv("SOLR_BASIC_USER", "")
    pass := GetEnv("SOLR_BASIC_PASS", "")
    rs := solr.NewDefaultRequestSender().WithHTTPClient(&http.Client{Timeout: 30 * time.Second})
    if user != "" {
        rs = rs.WithBasicAuth(user, pass)
    }
    client := solr.NewJSONClient(baseURL).WithRequestSender(rs)
    log.Printf("Using Solr URL: %s", baseURL)
    return client, baseURL, user, pass, &http.Client{Timeout: 30 * time.Second}
}

/*
func NewServerState() *server.State {
    client, baseURL, user, pass, httpClient := NewSolrClient()

    st := &server.State{
        SolrClient:      client,
        BaseURL:         baseURL,
        DefaultCollection: GetEnv("SOLR_MCP_DEFAULT_COLLECTION", "gettingstarted"),
        HttpClient:      httpClient,
        BasicUser:       user,
        BasicPass:       pass,
        SchemaCache: types.SchemaCache{
            LastFetch: make(map[string]time.Time),
            TTL:       10 * time.Minute,
            ByCol:     make(map[string]*types.FieldCatalog),
        },
        LlmBaseURL:       GetEnv("LLM_BASE_URL", "http://localhost:8000/v1"),
        LlmAPIKey:        GetEnv("LLM_API_KEY", ""),
        LlmModel:         GetEnv("LLM_MODEL", "gpt-4o"),
        EmbeddingBaseURL: GetEnv("EMBEDDING_BASE_URL", "http://localhost:8000/v1/embeddings"),
        EmbeddingAPIKey:  GetEnv("EMBEDDING_API_KEY", ""),
        EmbeddingModel:   GetEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
    }
    return st
}
    */