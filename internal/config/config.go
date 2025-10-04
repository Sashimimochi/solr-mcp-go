package config

import (
	"log/slog"
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
	slog.Info("Using Solr URL", "url", baseURL)
	return client, baseURL, user, pass, &http.Client{Timeout: 30 * time.Second}
}
