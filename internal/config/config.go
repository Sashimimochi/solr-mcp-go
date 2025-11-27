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

// ClusterStatusResponse represents the response from CLUSTERSTATUS API
type ClusterStatusResponse struct {
	ResponseHeader solr.ResponseHeader `json:"responseHeader"`
	Cluster        ClusterInfo         `json:"cluster"`
}

type ClusterInfo struct {
	Collections map[string]CollectionStatus `json:"collections"`
	LiveNodes   []string                    `json:"live_nodes"`
}

type CollectionStatus struct {
	ConfigName        string               `json:"configName"`
	ReplicationFactor interface{}          `json:"replicationFactor"` // can be int or string
	Router            map[string]string    `json:"router"`
	Shards            map[string]ShardInfo `json:"shards"`
	Health            string               `json:"health"`
	ZnodeVersion      int                  `json:"znodeVersion"`
}

type ShardInfo struct {
	Range    string                 `json:"range"`
	State    string                 `json:"state"`
	Replicas map[string]ReplicaInfo `json:"replicas"`
	Health   string                 `json:"health"`
}

type ReplicaInfo struct {
	Core     string `json:"core"`
	NodeName string `json:"node_name"`
	Type     string `json:"type"`
	State    string `json:"state"`
	BaseURL  string `json:"base_url"`
	Leader   string `json:"leader,omitempty"`
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
