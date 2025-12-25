package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAIAgentCompatibilityMiddleware_GETtoPOST tests GET to POST conversion
func TestAIAgentCompatibilityMiddleware_GETtoPOST(t *testing.T) {
	// Mock handler: verify request method and headers
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Should convert GET to POST")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json, text/event-stream", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	// GET request without session ID
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAIAgentCompatibilityMiddleware_GETwithSessionID tests GET with session ID (should not convert)
func TestAIAgentCompatibilityMiddleware_GETwithSessionID(t *testing.T) {
	// Mock handler: ensure method remains GET
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should NOT convert GET with session ID")
		w.WriteHeader(http.StatusOK)
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	// GET request with session ID
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Mcp-Session-Id", "test-session-123")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAIAgentCompatibilityMiddleware_POST tests POST requests (should pass through)
func TestAIAgentCompatibilityMiddleware_POST(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "POST should pass through")
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"test":"data"}`, string(body))
		w.WriteHeader(http.StatusOK)
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	body := bytes.NewBufferString(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestResponseWrapper_DELETE204to200 tests DELETE 204 to 200 conversion
func TestResponseWrapper_DELETE204to200(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.Header.Set("Mcp-Session-Id", "test-session")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Should convert 204 to 200")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	body, _ := io.ReadAll(w.Body)
	assert.Contains(t, string(body), "status", "Should include JSON body")
	assert.Contains(t, string(body), "ok", "Should include status ok")
}

// TestResponseWrapper_DELETE200 tests DELETE with 200 response (should pass through)
func TestResponseWrapper_DELETE200(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"custom":"response"}`))
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body, _ := io.ReadAll(w.Body)
	assert.Equal(t, `{"custom":"response"}`, string(body), "Should pass through 200 response")
}

// TestResponseWrapper_NonDELETE tests non-DELETE methods (should pass through)
func TestResponseWrapper_NonDELETE(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := &AIAgentCompatibilityMiddleware{
		mcpHandler: mockHandler,
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code, "Non-DELETE 204 should pass through")
}

// TestAIAgentCompatibilityMiddleware_Integration tests complete integration
func TestAIAgentCompatibilityMiddleware_Integration(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		sessionID      string
		handlerStatus  int
		expectedMethod string
		expectedStatus int
	}{
		{
			name:           "GET without session -> POST",
			method:         http.MethodGet,
			sessionID:      "",
			handlerStatus:  http.StatusOK,
			expectedMethod: http.MethodPost,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET with session -> GET",
			method:         http.MethodGet,
			sessionID:      "test-123",
			handlerStatus:  http.StatusOK,
			expectedMethod: http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST -> POST",
			method:         http.MethodPost,
			sessionID:      "",
			handlerStatus:  http.StatusOK,
			expectedMethod: http.MethodPost,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE 204 -> 200",
			method:         http.MethodDelete,
			sessionID:      "test-123",
			handlerStatus:  http.StatusNoContent,
			expectedMethod: http.MethodDelete,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedMethod string
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				w.WriteHeader(tt.handlerStatus)
			})

			middleware := &AIAgentCompatibilityMiddleware{
				mcpHandler: mockHandler,
			}

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.sessionID != "" {
				req.Header.Set("Mcp-Session-Id", tt.sessionID)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedMethod, capturedMethod, "Method should match expected")
			assert.Equal(t, tt.expectedStatus, w.Code, "Status code should match expected")
		})
	}
}
