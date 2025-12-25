package solr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"solr-mcp-go/internal/types"
	"testing"
	"time"
)

// TestGetFieldCatalog tests the GetFieldCatalog function.
// It uses httptest.Server to mock Solr APIs and verifies
// normal responses, error responses, and cache behavior.
func TestGetFieldCatalog(t *testing.T) {
	// --- Setup Mock Server ---
	// Define a handler that mimics Solr API endpoints.
	// It returns different JSON responses based on the request path.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		// Mock Unique Key Get API from Solr
		case "/solr/testcollection/schema/uniquekey":
			fmt.Fprintln(w, `{"uniqueKey":"id"}`)
		// Mock Field Information Get API from Solr
		case "/solr/testcollection/schema/fields":
			// Mock invalid URL path response
			if r.URL.Query().Get("error") == "true" {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			// Mock bad JSON response
			if r.URL.Query().Get("badjson") == "true" {
				fmt.Fprintln(w, `{"fields": [}`)
				return
			}
			// Mock successful response
			fields := struct {
				Fields []types.SolrField `json:"fields"`
			}{
				Fields: []types.SolrField{
					{Name: "id", Type: "string"},
					{Name: "title_txt_ja", Type: "text_ja"},
					{Name: "price_i", Type: "pint"},
					{Name: "stock_l", Type: "plong"},
					{Name: "weight_f", Type: "pfloat"},
					{Name: "score_d", Type: "pdouble"},
					{Name: "release_dt", Type: "pdate"},
					{Name: "in_stock_b", Type: "boolean"},
					{Name: "category_s", Type: "string"},
				},
			}
			json.NewEncoder(w).Encode(fields)
		// Mock Field Metadata Get API from Solr
		case "/solr/testcollection/admin/file":
			// Mock invalid URL path response
			if r.URL.Query().Get("error") == "true" {
				http.Error(w, "File Not Found", http.StatusNotFound)
				return
			}
			// Mock successful response
			metadata := map[string]types.FieldMetadata{
				"title_txt_ja": {Description: "タイトル"},
				"price_i":      {Description: "価格"},
			}
			json.NewEncoder(w).Encode(metadata)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// --- Define Test Cases ---
	t.Run("Success: all APIs return valid responses", func(t *testing.T) {
		// Goal: Successfully fetch uniqueKey, fields, and metadata from Solr
		// and confirm FieldCatalog is constructed correctly.
		sCtx := SchemaContext{
			HttpClient: mockServer.Client(),
			BaseURL:    mockServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		fc, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Expected FieldCatalog structure
		expectedFC := &types.FieldCatalog{
			UniqueKey: "id",
			All: []types.SolrField{
				{Name: "id", Type: "string"},
				{Name: "title_txt_ja", Type: "text_ja"},
				{Name: "price_i", Type: "pint"},
				{Name: "stock_l", Type: "plong"},
				{Name: "weight_f", Type: "pfloat"},
				{Name: "score_d", Type: "pdouble"},
				{Name: "release_dt", Type: "pdate"},
				{Name: "in_stock_b", Type: "boolean"},
				{Name: "category_s", Type: "string"},
			},
			Metadata: map[string]types.FieldMetadata{
				"title_txt_ja": {Description: "タイトル"},
				"price_i":      {Description: "価格"},
			},
		}

		// Validate results
		if fc.UniqueKey != expectedFC.UniqueKey {
			t.Errorf("UniqueKey mismatch. got=%s, want=%s", fc.UniqueKey, expectedFC.UniqueKey)
		}
		if !reflect.DeepEqual(fc.All, expectedFC.All) {
			t.Errorf("All mismatch. got=%v, want=%v", fc.All, expectedFC.All)
		}
		if !reflect.DeepEqual(fc.Metadata, expectedFC.Metadata) {
			t.Errorf("Metadata mismatch. got=%v, want=%v", fc.Metadata, expectedFC.Metadata)
		}
	})

	t.Run("Success: cache works within TTL", func(t *testing.T) {
		// Goal: Verify data is cached and no new HTTP requests
		// are made within TTL after the first fetch.
		requestCount := 0
		// Mock server with request counter
		countingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			// Return the same responses as the success case
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/solr/testcollection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case "/solr/testcollection/schema/fields":
				fmt.Fprintln(w, `{"fields":[]}`)
			case "/solr/testcollection/admin/file":
				fmt.Fprintln(w, `{}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer countingServer.Close()

		sCtx := SchemaContext{
			HttpClient: countingServer.Client(),
			BaseURL:    countingServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute, // cache TTL of 1 minute
			},
		}

		// First call (will be cached)
		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("First call error: %v", err)
		}
		if requestCount == 0 {
			t.Fatal("No request made on first call")
		}
		initialRequestCount := requestCount

		// Second call (should return from cache)
		_, err = GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("Second call error: %v", err)
		}

		// Confirm request count did not increase
		if requestCount != initialRequestCount {
			t.Errorf("Cache ineffective; request count increased. got=%d, want=%d", requestCount, initialRequestCount)
		}
	})

	t.Run("Error: field API returns error", func(t *testing.T) {
		// Goal: Ensure GetFieldCatalog propagates errors when
		// a dependent API returns an error.
		// Correct approach: configure mock server per test case
		// or vary behavior based on request details (e.g., query params).
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/solr/testcollection/schema/fields" {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			} else if r.URL.Path == "/solr/testcollection/schema/uniquekey" {
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer errorServer.Close()

		sCtx_err := SchemaContext{
			HttpClient: errorServer.Client(),
			BaseURL:    errorServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		_, err := GetFieldCatalog(context.Background(), sCtx_err, "testcollection")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})

	t.Run("Error: invalid JSON response", func(t *testing.T) {
		// Goal: Verify invalid JSON responses are handled
		// as JSON decode errors.
		badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/solr/testcollection/schema/fields" {
				fmt.Fprintln(w, `{"fields": [`)
			} else if r.URL.Path == "/solr/testcollection/schema/uniquekey" {
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer badJSONServer.Close()

		sCtx := SchemaContext{
			HttpClient: badJSONServer.Client(),
			BaseURL:    badJSONServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})

	t.Run("Success: metadata file missing", func(t *testing.T) {
		// Goal: Even if field_metadata.json is missing (API returns error),
		// ensure processing continues and other parts of FieldCatalog are built.
		noMetaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/solr/testcollection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case "/solr/testcollection/schema/fields":
				// Same response as the success case
				fields := struct {
					Fields []types.SolrField `json:"fields"`
				}{
					Fields: []types.SolrField{{Name: "id", Type: "string"}},
				}
				json.NewEncoder(w).Encode(fields)
			case "/solr/testcollection/admin/file":
				// Return error for metadata
				http.Error(w, "File Not Found", http.StatusNotFound)
			default:
				http.NotFound(w, r)
			}
		}))
		defer noMetaServer.Close()

		sCtx := SchemaContext{
			HttpClient: noMetaServer.Client(),
			BaseURL:    noMetaServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		fc, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Metadata should be empty
		if len(fc.Metadata) != 0 {
			t.Errorf("Metadata should be empty. got=%v", fc.Metadata)
		}
		// Other information should still be available
		if fc.UniqueKey != "id" {
			t.Errorf("UniqueKey not obtained. got=%s", fc.UniqueKey)
		}
	})

	t.Run("Success: with Basic auth", func(t *testing.T) {
		// Goal: When User/Pass are set, ensure Basic auth header is sent.
		var receivedAuth string
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/solr/testcollection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case "/solr/testcollection/schema/fields":
				fmt.Fprintln(w, `{"fields":[]}`)
			case "/solr/testcollection/admin/file":
				fmt.Fprintln(w, `{}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer authServer.Close()

		sCtx := SchemaContext{
			HttpClient: authServer.Client(),
			BaseURL:    authServer.URL,
			User:       "testuser",
			Pass:       "testpass",
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Confirm Basic auth header was sent
		if receivedAuth == "" {
			t.Error("Authorization header was not sent")
		}
		if len(receivedAuth) < 6 || receivedAuth[:5] != "Basic" {
			t.Errorf("Invalid Basic auth header format. got=%s", receivedAuth)
		}
	})

	t.Run("Error: uniqueKey API returns error", func(t *testing.T) {
		// Goal: Ensure GetFieldCatalog returns error when uniqueKey retrieval fails.
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/solr/testcollection/schema/uniquekey" {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer errorServer.Close()

		sCtx := SchemaContext{
			HttpClient: errorServer.Client(),
			BaseURL:    errorServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})

	t.Run("Error: HTTP request fails", func(t *testing.T) {
		// Goal: Verify appropriate errors are returned on network failures.
		sCtx := SchemaContext{
			HttpClient: &http.Client{},
			BaseURL:    "http://invalid-host-that-does-not-exist:9999",
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})

	t.Run("Success: collection name with special characters", func(t *testing.T) {
		// Goal: Ensure collection names requiring URL escaping are handled correctly.
		specialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Verify URL-escaped collection names (support multiple patterns)
			switch {
			case r.URL.Path == "/solr/test%20collection/schema/uniquekey" ||
				r.URL.Path == "/solr/test collection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case r.URL.Path == "/solr/test%20collection/schema/fields" ||
				r.URL.Path == "/solr/test collection/schema/fields":
				fmt.Fprintln(w, `{"fields":[]}`)
			case r.URL.Path == "/solr/test%20collection/admin/file" ||
				r.URL.Path == "/solr/test collection/admin/file":
				fmt.Fprintln(w, `{}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer specialServer.Close()

		sCtx := SchemaContext{
			HttpClient: specialServer.Client(),
			BaseURL:    specialServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       1 * time.Minute,
			},
		}

		fc, err := GetFieldCatalog(context.Background(), sCtx, "test collection")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if fc.UniqueKey != "id" {
			t.Errorf("UniqueKey not obtained. got=%s", fc.UniqueKey)
		}
	})

	t.Run("Success: cache TTL is 0", func(t *testing.T) {
		// Goal: When TTL is 0, verify each call triggers API requests.
		requestCount := 0
		zeroTTLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/solr/testcollection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case "/solr/testcollection/schema/fields":
				fmt.Fprintln(w, `{"fields":[]}`)
			case "/solr/testcollection/admin/file":
				fmt.Fprintln(w, `{}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer zeroTTLServer.Close()

		sCtx := SchemaContext{
			HttpClient: zeroTTLServer.Client(),
			BaseURL:    zeroTTLServer.URL,
			Cache: &types.SchemaCache{
				ByCol:     make(map[string]*types.FieldCatalog),
				LastFetch: make(map[string]time.Time),
				TTL:       0, // set TTL to 0
			},
		}

		// First call
		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("First call error: %v", err)
		}
		firstRequestCount := requestCount

		// Second call (TTL is 0, so another request should be made)
		_, err = GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("Second call error: %v", err)
		}

		// Confirm request count increased
		if requestCount <= firstRequestCount {
			t.Errorf("With TTL=0, requests should be reissued. got=%d, want>%d", requestCount, firstRequestCount)
		}
	})
}
