package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"solr-mcp-go/internal/types"
)

// TestCallLLMForPlan は CallLLMForPlan 関数のテストです。
func TestCallLLMForPlan(t *testing.T) {
	// 目的: LLMへのプランニングリクエストが成功し、期待されるレスポンスを正しく解析できることを確認する。
	t.Run("Successful plan generation", func(t *testing.T) {
		expectedPlan := types.LlmPlan{
			Mode: "keyword",
			EdisMax: types.LlmEdisMax{
				TextQuery: "test query",
			},
			Vector: types.Vector{
				Field:     "",
				K:         5,
				QueryText: "test query",
			},
		}
		planBytes, _ := json.Marshal(expectedPlan)
		planString := string(planBytes)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": planString,
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		cfg := LLMConfig{
			HttpClient: server.Client(),
			BaseURL:    server.URL,
			APIKey:     "test-key",
			Model:      "test-model",
		}

		plan, _, err := CallLLMForPlan(context.Background(), cfg, "test query", "en", "schema", false, false)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !reflect.DeepEqual(*plan, expectedPlan) {
			t.Errorf("Expected plan %+v, got %+v", expectedPlan, *plan)
		}
	})

	// 目的: LLMが不正なJSONを返した場合にエラーとなることを確認する。
	t.Run("Malformed JSON response from LLM", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": "this is not json",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		cfg := LLMConfig{
			HttpClient: server.Client(),
			BaseURL:    server.URL,
			APIKey:     "test-key",
			Model:      "test-model",
		}

		_, _, err := CallLLMForPlan(context.Background(), cfg, "test query", "en", "schema", false, false)
		if err == nil {
			t.Fatal("Expected an error for malformed JSON, but got nil")
		}
	})
}

// TestEnsureEmbedding は EnsureEmbedding 関数のテストです。
func TestEnsureEmbedding(t *testing.T) {
	// 目的: 埋め込みAPIへのリクエストが成功し、期待されるベクトルを正しく解析できることを確認する。
	t.Run("Successful embedding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{
				"data": []any{
					map[string]any{
						"embedding": []float64{0.1, 0.2, 0.3},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		cfg := EmbeddingConfig{
			HttpClient: server.Client(),
			BaseURL:    server.URL,
			APIKey:     "test-key",
			Model:      "test-model",
		}

		vec, err := EnsureEmbedding(context.Background(), cfg, "test text")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(vec) != 3 || vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
			t.Errorf("Expected vector [0.1 0.2 0.3], got %v", vec)
		}
	})

	// 目的: APIキーが設定されていない場合にエラーとなることを確認する。
	t.Run("Missing API key or BaseURL", func(t *testing.T) {
		cfg := EmbeddingConfig{
			HttpClient: &http.Client{},
			BaseURL:    "",
			APIKey:     "",
		}
		_, err := EnsureEmbedding(context.Background(), cfg, "test text")
		if err == nil {
			t.Fatal("Expected an error for missing API key or BaseURL, but got nil")
		}
		expectedError := "EMBEDDING_BASE_URL and EMBEDDING_API_KEY must be set for vector search"
		if err.Error() != expectedError {
			t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
		}
	})

	// 目的: 埋め込みAPIが空のデータを返した場合にエラーとなることを確認する。
	t.Run("Empty data from embedding API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{
				"data": []any{},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		cfg := EmbeddingConfig{
			HttpClient: server.Client(),
			BaseURL:    server.URL,
			APIKey:     "test-key",
		}
		_, err := EnsureEmbedding(context.Background(), cfg, "test text")
		if err == nil {
			t.Fatal("Expected an error for empty data, but got nil")
		}
		expectedError := "embedding API returned no data"
		if err.Error() != expectedError {
			t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
		}
	})

	// 目的: 埋め込みAPIが空のベクトルを返した場合にエラーとなることを確認する。
	t.Run("Empty embedding vector", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{
				"data": []any{
					map[string]any{
						"embedding": []float64{},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		cfg := EmbeddingConfig{
			HttpClient: server.Client(),
			BaseURL:    server.URL,
			APIKey:     "test-key",
		}
		_, err := EnsureEmbedding(context.Background(), cfg, "test text")
		if err == nil {
			t.Fatal("Expected an error for empty vector, but got nil")
		}
		expectedError := "empty embedding vector"
		if err.Error() != expectedError {
			t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
		}
	})
}

func TestPost(t *testing.T) {
	ctx := context.Background()
	httpClient := &http.Client{}
	body := map[string]any{"key": "value"}
	apiKey := "test-api-key"

	// 目的: HTTP POSTリクエストが成功し、期待されるレスポンスを正しく解析できることを確認する。
	t.Run("Successful POST request", func(t *testing.T) {
		url := "https://httpbin.org/post" // テスト用のURL
		_, err := post(ctx, httpClient, url, body, apiKey)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	// 目的: 無効なURLに対してリクエストを送信した場合にエラーとなることを確認する。
	t.Run("Invalid URL", func(t *testing.T) {
		url := "http://invalid-url" // 無効なURL
		_, err := post(ctx, httpClient, url, body, apiKey)
		if err == nil {
			t.Errorf("Expected error for invalid URL, got nil")
		}
	})

	// 目的: 認証エラーが発生した場合に適切にエラーを返すことを確認する。
	t.Run("Authentication Error", func(t *testing.T) {
		url := "https://httpbin.org/status/401" // 認証エラーを返すURL
		_, err := post(ctx, httpClient, url, body, "invalid-api-key")
		if err == nil {
			t.Errorf("Expected authentication error, got nil")
		}
	})
}

// TestGetFirstChoiceContent は getFirstChoiceContent 関数のテストです。
func TestGetFirstChoiceContent(t *testing.T) {
	// 目的: 正常なレスポンスからコンテンツを正しく抽出できることを確認する。
	t.Run("Extract content from valid response", func(t *testing.T) {
		response := map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": "test content",
					},
				},
			},
		}
		content := getFirstChoiceContent(response)
		if content != "test content" {
			t.Errorf("Expected 'test content', got '%s'", content)
		}
	})

	// 目的: APIエラーが含まれている場合に空文字列を返すことを確認する。
	t.Run("Handle error in response", func(t *testing.T) {
		responseWithError := map[string]any{
			"error": "Some error occurred",
		}
		content := getFirstChoiceContent(responseWithError)
		if content != "" {
			t.Errorf("Expected empty string for error response, got '%s'", content)
		}
	})

	// 目的: 'choices'フィールドがない場合に空文字列を返すことを確認する。
	t.Run("Handle missing choices field", func(t *testing.T) {
		responseNoChoices := map[string]any{}
		content := getFirstChoiceContent(responseNoChoices)
		if content != "" {
			t.Errorf("Expected empty string for no choices, got '%s'", content)
		}
	})

	// 目的: 'choices'が空のスライスの場合に空文字列を返すことを確認する。
	t.Run("Handle empty choices array", func(t *testing.T) {
		responseEmptyChoices := map[string]any{
			"choices": []any{},
		}
		content := getFirstChoiceContent(responseEmptyChoices)
		if content != "" {
			t.Errorf("Expected empty string for empty choices, got '%s'", content)
		}
	})

	// 目的: 'message'フィールドがない場合に空文字列を返すことを確認する。
	t.Run("Handle missing message field", func(t *testing.T) {
		responseNoMessage := map[string]any{
			"choices": []any{
				map[string]any{},
			},
		}
		content := getFirstChoiceContent(responseNoMessage)
		if content != "" {
			t.Errorf("Expected empty string for no message, got '%s'", content)
		}
	})

	// 目的: 'content'フィールドがない場合に空文字列を返すことを確認する。
	t.Run("Handle missing content field", func(t *testing.T) {
		responseNoContent := map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{},
				},
			},
		}
		content := getFirstChoiceContent(responseNoContent)
		if content != "" {
			t.Errorf("Expected empty string for no content, got '%s'", content)
		}
	})
}
