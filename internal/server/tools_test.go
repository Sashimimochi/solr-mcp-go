package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"solr-mcp-go/internal/types"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	solr "github.com/stevenferrer/solr-go"
	"github.com/stretchr/testify/assert"
)

// newTestState creates a test State and HTTP mock server client.
func newTestState(t *testing.T, baseURL string) *State {
	client := solr.NewJSONClient(baseURL)
	return &State{
		SolrClient:        client,
		BaseURL:           baseURL,
		DefaultCollection: "test",
		HttpClient:        &http.Client{},
		SchemaCache: types.SchemaCache{
			LastFetch: make(map[string]time.Time),
			TTL:       10 * time.Minute,
			ByCol:     make(map[string]*types.FieldCatalog),
		},
	}
}

// TestToolQuery tests the toolQuery method.
func TestToolQuery(t *testing.T) {
	t.Run("Success: basic query", func(t *testing.T) {
		// Setup mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/select") {
				t.Errorf("Expected /select in path, got: %s", r.URL.Path)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0, "QTime": 10},
				"response":       map[string]any{"numFound": 1, "docs": []any{map[string]any{"id": "1"}}},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "*:*",
		}

		_, resp, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		respMap, ok := resp.(map[string]any)
		assert.True(t, ok)
		assert.NotNil(t, respMap["response"])
	})

	t.Run("Success: query with parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("rows") != "10" {
				t.Errorf("Expected rows=10, got rows=%s", q.Get("rows"))
			}
			if q.Get("start") != "5" {
				t.Errorf("Expected start=5, got start=%s", q.Get("start"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		rows := 10
		start := 5
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "test",
			Rows:       &rows,
			Start:      &start,
			Fields:     []string{"id", "title"},
			Sort:       "id asc",
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
	})

	t.Run("Success: with filter queries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection:  "testcol",
			Query:       "*:*",
			FilterQuery: []string{"status:active", "type:book"},
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
	})

	t.Run("Success: with echoParams", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "*:*",
			EchoParams: true,
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
	})

	t.Run("Success: custom params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "*:*",
			Params: map[string]any{
				"facet":       "true",
				"facet.field": "category",
			},
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
	})

	t.Run("Success: empty query falls back to *:*", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "",
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.NoError(t, err)
	})

	t.Run("Error: collection not provided", func(t *testing.T) {
		st := newTestState(t, "http://localhost:8983")
		in := types.QueryIn{
			Collection: "",
			Query:      "*:*",
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection is required")
	})

	t.Run("Error: collection only whitespace", func(t *testing.T) {
		st := newTestState(t, "http://localhost:8983")
		in := types.QueryIn{
			Collection: "   ",
			Query:      "*:*",
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection is required")
	})

	t.Run("Error: HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.QueryIn{
			Collection: "testcol",
			Query:      "*:*",
		}

		_, _, err := st.toolQuery(context.Background(), nil, in)

		assert.Error(t, err)
	})
}

// TestToolPing tests the toolPing method.
func TestToolPing(t *testing.T) {
	t.Run("Success: cluster status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/admin/collections") {
				t.Errorf("Expected /admin/collections in path, got: %s", r.URL.Path)
			}
			if r.URL.Query().Get("action") != "CLUSTERSTATUS" {
				t.Errorf("Expected action=CLUSTERSTATUS, got: %s", r.URL.Query().Get("action"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0, "QTime": 5},
				"cluster": map[string]any{
					"live_nodes": []string{"node1:8983_solr", "node2:8983_solr"},
				},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.PingIn{}

		_, resp, err := st.toolPing(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		respMap, ok := resp.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, 0, respMap["status"])
		assert.Equal(t, 2, respMap["num_nodes"])
	})

	t.Run("Success: Basic auth", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0},
				"cluster":        map[string]any{"live_nodes": []string{}},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		st.BasicUser = "testuser"
		st.BasicPass = "testpass"
		in := types.PingIn{}

		_, _, err := st.toolPing(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotEmpty(t, receivedAuth)
		assert.True(t, strings.HasPrefix(receivedAuth, "Basic "))
	})

	t.Run("Error: HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.PingIn{}

		_, _, err := st.toolPing(context.Background(), nil, in)

		assert.Error(t, err)
		// JSON decode error is expected on HTTP error
		assert.Contains(t, err.Error(), "decode response")
	})

	t.Run("Error: invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"invalid json`))
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.PingIn{}

		_, _, err := st.toolPing(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "decode response")
	})

	t.Run("Error: network error", func(t *testing.T) {
		st := newTestState(t, "http://invalid-host-that-does-not-exist:9999")
		in := types.PingIn{}

		_, _, err := st.toolPing(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cluster status request")
	})

	t.Run("Success: without auth", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0},
				"cluster":        map[string]any{"live_nodes": []string{}},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		// Clear BasicUser and BasicPass
		st.BasicUser = ""
		st.BasicPass = ""
		in := types.PingIn{}

		_, _, err := st.toolPing(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.Empty(t, receivedAuth, "Authorization header should not be sent")
	})
}

// TestToolCollectionHealth tests the toolCollectionHealth method.
func TestToolCollectionHealth(t *testing.T) {
	t.Run("Success: collection health", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("collection") != "testcol" {
				t.Errorf("Expected collection=testcol, got: %s", r.URL.Query().Get("collection"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0, "QTime": 5},
				"cluster": map[string]any{
					"collections": map[string]any{
						"testcol": map[string]any{
							"health":     "GREEN",
							"configName": "testconf",
							"shards": map[string]any{
								"shard1": map[string]any{"state": "active"},
							},
						},
					},
				},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, resp, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		respMap, ok := resp.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "GREEN", respMap["health"])
		assert.Equal(t, "testconf", respMap["configName"])
	})

	t.Run("Success: Basic auth", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0},
				"cluster": map[string]any{
					"collections": map[string]any{
						"testcol": map[string]any{"health": "GREEN"},
					},
				},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		st.BasicUser = "testuser"
		st.BasicPass = "testpass"
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotEmpty(t, receivedAuth)
	})

	t.Run("Error: collection not provided", func(t *testing.T) {
		st := newTestState(t, "http://localhost:8983")
		in := types.CollectionHealthIn{Collection: ""}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection is required")
	})

	t.Run("Error: collection not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0},
				"cluster": map[string]any{
					"collections": map[string]any{},
				},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.CollectionHealthIn{Collection: "notfound"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Error: HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.Error(t, err)
	})

	t.Run("Error: invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"invalid json`))
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "decode response")
	})

	t.Run("Error: network error", func(t *testing.T) {
		st := newTestState(t, "http://invalid-host-that-does-not-exist:9999")
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection health check")
	})

	t.Run("Success: without auth", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{"status": 0},
				"cluster": map[string]any{
					"collections": map[string]any{
						"testcol": map[string]any{"health": "GREEN"},
					},
				},
			})
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		st.BasicUser = ""
		st.BasicPass = ""
		in := types.CollectionHealthIn{Collection: "testcol"}

		_, _, err := st.toolCollectionHealth(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.Empty(t, receivedAuth, "Authorization header should not be sent")
	})
}

// TestToolSchema tests the toolSchema method.
func TestToolSchema(t *testing.T) {
	t.Run("Success: schema retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			switch {
			case strings.Contains(r.URL.Path, "/schema/uniquekey"):
				json.NewEncoder(w).Encode(map[string]any{"uniqueKey": "id"})
			case strings.Contains(r.URL.Path, "/schema/fields"):
				json.NewEncoder(w).Encode(map[string]any{
					"fields": []map[string]any{
						{"name": "id", "type": "string"},
						{"name": "title", "type": "text_general"},
					},
				})
			case strings.Contains(r.URL.Path, "/admin/file"):
				json.NewEncoder(w).Encode(map[string]any{
					"title": map[string]any{"description": "Title field"},
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.SchemaIn{Collection: "testcol"}

		_, resp, err := st.toolSchema(context.Background(), nil, in)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		fc, ok := resp.(*types.FieldCatalog)
		assert.True(t, ok)
		assert.Equal(t, "id", fc.UniqueKey)
		assert.Len(t, fc.All, 2)
	})

	t.Run("Error: collection not provided", func(t *testing.T) {
		st := newTestState(t, "http://localhost:8983")
		in := types.SchemaIn{Collection: ""}

		_, _, err := st.toolSchema(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection is required")
	})

	t.Run("Error: schema retrieval failed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}))
		defer server.Close()

		st := newTestState(t, server.URL)
		in := types.SchemaIn{Collection: "testcol"}

		_, _, err := st.toolSchema(context.Background(), nil, in)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get schema")
	})
}

// TestAddTools tests the AddTools function.
func TestAddTools(t *testing.T) {
	t.Run("Success: all tools are registered", func(t *testing.T) {
		impl := &mcp.Implementation{}
		mcpServer := mcp.NewServer(impl, nil)
		st := newTestState(t, "http://localhost:8983")

		toolNames := AddTools(mcpServer, st)

		assert.Len(t, toolNames, 4)
		assert.Contains(t, toolNames, "solr.query")
		assert.Contains(t, toolNames, "solr.ping")
		assert.Contains(t, toolNames, "solr.collection.health")
		assert.Contains(t, toolNames, "solr.schema")
	})

	t.Run("Success: tool order is correct", func(t *testing.T) {
		impl := &mcp.Implementation{}
		mcpServer := mcp.NewServer(impl, nil)
		st := newTestState(t, "http://localhost:8983")

		toolNames := AddTools(mcpServer, st)

		assert.Equal(t, "solr.query", toolNames[0])
		assert.Equal(t, "solr.ping", toolNames[1])
		assert.Equal(t, "solr.collection.health", toolNames[2])
		assert.Equal(t, "solr.schema", toolNames[3])
	})
}
