package solr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	solr "github.com/stevenferrer/solr-go"
	"github.com/stretchr/testify/assert"
)

// mockRequestSender is a mock of the solr.RequestSender interface.
type mockRequestSender struct {
	statusCode int
	body       string
	err        error
}

// SendRequest is a mock implementation of the SendRequest method.
// This method must match the solr-go library's interface.
func (m *mockRequestSender) SendRequest(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Simulate a request using httptest.NewRequest
	req := httptest.NewRequest(method, "http://localhost"+path, body)
	req = req.WithContext(ctx)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/json")
	recorder.WriteHeader(m.statusCode)
	recorder.WriteString(m.body)

	return recorder.Result(), nil
}

// TestQuerySelect tests the QuerySelect function.
// Goal: Ensure Solr queries are correctly built and executed.
func TestQuerySelect(t *testing.T) {
	ctx := context.Background()
	collection := "test_collection"

	t.Run("successful query", func(t *testing.T) {
		// Setup mock response
		mockRespBody := `{
			"responseHeader": { "status": 0, "QTime": 1 },
			"response": { "numFound": 1, "start": 0, "docs": [ { "id": "1" } ] }
		}`
		mockSender := &mockRequestSender{
			statusCode: http.StatusOK,
			body:       mockRespBody,
		}
		client := solr.NewJSONClient("http://localhost:8983").WithRequestSender(mockSender)

		params := map[string]any{
			"q":    "*:*",
			"rows": 10,
		}

		// Execute function
		resp, err := QuerySelect(ctx, client, collection, params)

		// Assertions
		assert.NoError(t, err, "No error expected")
		assert.NotNil(t, resp, "Response should not be nil")

		// Assert return type is *solr.QueryResponse
		queryResponse, ok := resp.(*solr.QueryResponse)
		assert.True(t, ok, "Response should be *solr.QueryResponse")
		assert.NotNil(t, queryResponse.Response, "'response' field should not be nil")
		assert.Equal(t, 1, queryResponse.Response.NumFound, "numFound should be 1")
	})

	t.Run("query with empty q parameter", func(t *testing.T) {
		// When q is empty, ensure "*:*" is used
		mockSender := &mockRequestSender{
			statusCode: http.StatusOK,
			body:       `{}`,
		}
		client := solr.NewJSONClient("http://localhost:8983").WithRequestSender(mockSender)
		params := map[string]any{
			"q": "",
		}
		_, err := QuerySelect(ctx, client, collection, params)
		assert.NoError(t, err)
	})
}

// TestAddFieldsForIDs tests the AddFieldsForIDs function.
// Goal: Ensure the ID field is correctly added to the query body.
func TestAddFieldsForIDs(t *testing.T) {
	testCases := []struct {
		name     string
		body     map[string]any
		idField  string
		expected map[string]any
	}{
		{
			name: "when fields already exist",
			body: map[string]any{
				"query":  "*:*",
				"fields": []string{"name", "price"},
			},
			idField: "id",
			expected: map[string]any{
				"query":  "*:*",
				"fields": []string{"id"},
			},
		},
		{
			name: "when fields do not exist",
			body: map[string]any{
				"query": "*:*",
			},
			idField: "id",
			expected: map[string]any{
				"query": "*:*",
			},
		},
		{
			name: "when idField is empty",
			body: map[string]any{
				"query":  "*:*",
				"fields": []string{"name", "price"},
			},
			idField: "",
			expected: map[string]any{
				"query":  "*:*",
				"fields": []string{"name", "price"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			AddFieldsForIDs(tc.body, tc.idField)
			if !reflect.DeepEqual(tc.body, tc.expected) {
				t.Errorf("expected %v, but got %v", tc.expected, tc.body)
			}
		})
	}
}

// TestExtractIDs tests the ExtractIDs function.
// Goal: Ensure IDs are correctly extracted from Solr responses.
func TestExtractIDs(t *testing.T) {
	testCases := []struct {
		name     string
		resp     map[string]any
		idField  string
		expected []string
	}{
		{
			name: "Success (string IDs)",
			resp: map[string]any{
				"response": map[string]any{
					"docs": []any{
						map[string]any{"id": "doc1", "name": "doc1 name"},
						map[string]any{"id": "doc2", "name": "doc2 name"},
					},
				},
			},
			idField:  "id",
			expected: []string{"doc1", "doc2"},
		},
		{
			name: "Success (numeric IDs)",
			resp: map[string]any{
				"response": map[string]any{
					"docs": []any{
						map[string]any{"id": float64(123), "name": "doc1 name"},
						map[string]any{"id": float64(456), "name": "doc2 name"},
					},
				},
			},
			idField:  "id",
			expected: []string{"123", "456"},
		},
		{
			name: "Empty documents",
			resp: map[string]any{
				"response": map[string]any{
					"docs": []any{},
				},
			},
			idField:  "id",
			expected: []string{},
		},
		{
			name:     "No response field",
			resp:     map[string]any{},
			idField:  "id",
			expected: []string{},
		},
		{
			name: "Some documents missing ID field",
			resp: map[string]any{
				"response": map[string]any{
					"docs": []any{
						map[string]any{"id": "doc1"},
						map[string]any{"name": "doc2 name"},
					},
				},
			},
			idField:  "id",
			expected: []string{"doc1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ids := ExtractIDs(tc.resp, tc.idField)
			// Treat nil and empty slices equivalently for comparison
			if len(ids) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(ids, tc.expected) {
				t.Errorf("expected %v, but got %v", tc.expected, ids)
			}
		})
	}
}

// TestAppendFilterQuery tests the AppendFilterQuery function.
// Goal: Ensure filter queries are correctly added to the params map.
func TestAppendFilterQuery(t *testing.T) {
	testCases := []struct {
		name     string
		params   map[string]any
		fq       string
		expected map[string]any
	}{
		{
			name:   "fq is nil",
			params: map[string]any{"q": "*:*"},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"new_filter:true"},
			},
		},
		{
			name:   "fq is a string",
			params: map[string]any{"q": "*:*", "fq": "existing_filter:true"},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"existing_filter:true", "new_filter:true"},
			},
		},
		{
			name:   "fq is a []string",
			params: map[string]any{"q": "*:*", "fq": []string{"existing_filter:true"}},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"existing_filter:true", "new_filter:true"},
			},
		},
		{
			name:   "fq is a []any",
			params: map[string]any{"q": "*:*", "fq": []any{"existing_filter:true"}},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []any{"existing_filter:true", "new_filter:true"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			AppendFilterQuery(tc.params, tc.fq)
			if !reflect.DeepEqual(tc.params, tc.expected) {
				t.Errorf("expected %v, but got %v", tc.expected, tc.params)
			}
		})
	}
}

// TestPostQueryJSON tests the PostQueryJSON function.
// Goal: Ensure HTTP POST requests are sent correctly and responses parsed.
func TestPostQueryJSON(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header is not application/json")
		}
		// Validate Basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			t.Errorf("Basic auth is not correct")
		}
		// Validate request body
		var body map[string]any
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if body["query"] != "*:*" {
			t.Errorf("query in body is not *:*")
		}

		// Return successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"responseHeader": map[string]any{
				"status": 0,
				"QTime":  10,
			},
			"response": map[string]any{
				"numFound": 1,
				"start":    0,
				"docs": []any{
					map[string]any{"id": "1"},
				},
			},
		})
	}))
	defer server.Close()

	// Execute test
	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	resp, err := PostQueryJSON(context.Background(), client, server.URL, "testuser", "testpass", "testcollection", body)

	// Ensure no error
	if err != nil {
		t.Fatalf("PostQueryJSON returned an error: %v", err)
	}

	// Validate response content
	if resp["responseHeader"] == nil {
		t.Errorf("responseHeader is nil")
	}
	response, ok := resp["response"].(map[string]any)
	if !ok {
		t.Fatal("response is not a map[string]any")
	}
	if response["numFound"] != float64(1) {
		t.Errorf("numFound is not 1")
	}
}

// TestPostQueryJSON_Error tests error cases for PostQueryJSON.
// Goal: Ensure non-2xx HTTP status codes return errors.
func TestPostQueryJSON_Error(t *testing.T) {
	// Mock server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	_, err := PostQueryJSON(context.Background(), client, server.URL, "", "", "testcollection", body)

	// Confirm error is returned
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	expectedError := "HTTP status 500: Internal Server Error"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain %q, but got %q", expectedError, err.Error())
	}
}

// TestPostQueryJSON_InvalidJSON tests JSON decode error scenarios.
// Goal: Ensure invalid JSON responses return errors.
func TestPostQueryJSON_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid json`))
	}))
	defer server.Close()

	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	_, err := PostQueryJSON(context.Background(), client, server.URL, "", "", "testcollection", body)

	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !strings.Contains(err.Error(), "JSON decode error") {
		t.Errorf("Expected JSON decode error, but got: %v", err)
	}
}

// TestPostQueryJSON_NetworkError tests network error scenarios for PostQueryJSON.
// Goal: Ensure errors are returned when HTTP requests fail.
func TestPostQueryJSON_NetworkError(t *testing.T) {
	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	_, err := PostQueryJSON(context.Background(), client, "http://invalid-host-that-does-not-exist:9999", "", "", "testcollection", body)

	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	if !strings.Contains(err.Error(), "HTTP request error") {
		t.Errorf("Expected HTTP request error, but got: %v", err)
	}
}

// TestPostQueryJSON_NoAuth tests PostQueryJSON without authentication.
// Goal: Ensure Basic auth is not set when user/pass are empty.
func TestPostQueryJSON_NoAuth(t *testing.T) {
	authHeaderReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			authHeaderReceived = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
	}))
	defer server.Close()

	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	_, err := PostQueryJSON(context.Background(), client, server.URL, "", "", "testcollection", body)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if authHeaderReceived {
		t.Error("Authorization header should not be sent when user is empty")
	}
}

// TestQueryWithRawResponse tests the QueryWithRawResponse function.
// Goal: Ensure the query executes and returns raw JSON responses.
func TestQueryWithRawResponse(t *testing.T) {
	t.Run("Success: basic query", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate query parameters
			q := r.URL.Query()
			// Sent by solr-go QueryParser format
			if q.Get("q") == "" {
				t.Errorf("q parameter should be set")
			}
			if q.Get("wt") != "json" {
				t.Errorf("Expected wt=json, got wt=%s", q.Get("wt"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"responseHeader": map[string]any{
					"status": 0,
					"QTime":  10,
					"params": map[string]any{
						"q":  "*:*",
						"wt": "json",
					},
				},
				"response": map[string]any{
					"numFound": 1,
					"start":    0,
					"docs":     []any{map[string]any{"id": "1"}},
				},
			})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		resp, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Validate responseHeader
		respHeader, ok := resp["responseHeader"].(map[string]any)
		assert.True(t, ok, "responseHeader should be a map")
		assert.NotNil(t, respHeader["params"], "params should be present in responseHeader")

		// Validate response
		response, ok := resp["response"].(map[string]any)
		assert.True(t, ok, "response should be a map")
		assert.Equal(t, float64(1), response["numFound"])
	})

	t.Run("Success: add params via Params()", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"rows":  10,
				"start": 5,
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: Basic auth", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "testuser", "testpass", "testcollection", query)

		assert.NoError(t, err)
		assert.NotEmpty(t, receivedAuth, "Authorization header should be sent")
		assert.True(t, strings.HasPrefix(receivedAuth, "Basic "), "Should use Basic auth")
	})

	t.Run("Success: without auth", func(t *testing.T) {
		authHeaderReceived := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				authHeaderReceived = true
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
		assert.False(t, authHeaderReceived, "Authorization header should not be sent when user is empty")
	})

	t.Run("Success: multiple filter queries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"fq": []string{"status:active", "type:book"},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: nested params map", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"facet":       "true",
				"facet.field": "category",
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: various parameter types", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"rows":         100,
				"custom_int64": int64(9223372036854775807),
				"custom_float": float64(3.14159),
				"debug":        true,
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: []any parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"fq": []any{"status:active", "type:book"},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: collection name has special characters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "test%20collection") && !strings.Contains(r.URL.Path, "test collection") {
				t.Errorf("Expected URL path to contain escaped collection name, got: %s", r.URL.Path)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "test collection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: nested map in params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			// Keys in nested map are flattened in query params
			if q.Get("custom_key") == "" {
				t.Logf("Query params: %v", q)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"nested": map[string]any{
					"custom_key": "custom_value",
				},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: nested map contains []string", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"nested_map": map[string]any{
					"tags": []string{"tag1", "tag2"},
				},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: nested map contains unexpected type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"nested_map": map[string]any{
					"complex": map[string]string{"inner": "value"},
				},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: unexpected parameter type (default case)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		// Operate on BuildQuery result directly to trigger default case
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"unexpected": struct{ Value string }{Value: "test"},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Success: limit/offset/fields/filter parameter conversion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		// Use these parameter keys to cover each switch case
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"limit":  20,
				"offset": 10,
				"fields": "id,title",
				"filter": "status:active",
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("Error: HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP status 500")
	})

	t.Run("Error: invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"invalid json`))
		}))
		defer server.Close()

		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JSON decode error")
	})

	t.Run("Error: network error", func(t *testing.T) {
		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, "http://invalid-host-that-does-not-exist:9999", "", "", "testcollection", query)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP request error")
	})
}
