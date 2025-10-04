package server

import (
	"testing"
)

// TestNewServerState は NewServerState 関数のテストです。
func TestNewServerState(t *testing.T) {
	// 目的: 環境変数が設定されている場合に、NewServerStateが正しくStateを初期化することを確認する。
	t.Run("With environment variables", func(t *testing.T) {
		t.Setenv("SOLR_MCP_DEFAULT_COLLECTION", "test_collection")
		t.Setenv("LLM_BASE_URL", "http://test-llm-url")
		t.Setenv("LLM_API_KEY", "test-llm-key")
		t.Setenv("LLM_MODEL", "test-llm-model")
		t.Setenv("EMBEDDING_BASE_URL", "http://test-embedding-url")
		t.Setenv("EMBEDDING_API_KEY", "test-embedding-key")
		t.Setenv("EMBEDDING_MODEL", "test-embedding-model")

		state := NewServerState()

		if state.DefaultCollection != "test_collection" {
			t.Errorf("Expected DefaultCollection to be 'test_collection', got '%s'", state.DefaultCollection)
		}
		if state.LlmBaseURL != "http://test-llm-url" {
			t.Errorf("Expected LlmBaseURL to be 'http://test-llm-url', got '%s'", state.LlmBaseURL)
		}
		if state.LlmAPIKey != "test-llm-key" {
			t.Errorf("Expected LlmAPIKey to be 'test-llm-key', got '%s'", state.LlmAPIKey)
		}
		if state.LlmModel != "test-llm-model" {
			t.Errorf("Expected LlmModel to be 'test-llm-model', got '%s'", state.LlmModel)
		}
		if state.EmbeddingBaseURL != "http://test-embedding-url" {
			t.Errorf("Expected EmbeddingBaseURL to be 'http://test-embedding-url', got '%s'", state.EmbeddingBaseURL)
		}
		if state.EmbeddingAPIKey != "test-embedding-key" {
			t.Errorf("Expected EmbeddingAPIKey to be 'test-embedding-key', got '%s'", state.EmbeddingAPIKey)
		}
		if state.EmbeddingModel != "test-embedding-model" {
			t.Errorf("Expected EmbeddingModel to be 'test-embedding-model', got '%s'", state.EmbeddingModel)
		}
	})

	// 目的: 環境変数が設定されていない場合に、デフォルト値でStateが初期化されることを確認する。
	t.Run("Without environment variables (defaults)", func(t *testing.T) {
		// t.Setenv() はこのサブテスト内でのみ有効なため、ここでは環境変数は未設定の状態
		state := NewServerState()

		if state.DefaultCollection != "gettingstarted" {
			t.Errorf("Expected DefaultCollection to be 'gettingstarted', got '%s'", state.DefaultCollection)
		}
		if state.LlmBaseURL != "http://localhost:8000/v1" {
			t.Errorf("Expected LlmBaseURL to be 'http://localhost:8000/v1', got '%s'", state.LlmBaseURL)
		}
		if state.LlmAPIKey != "" {
			t.Errorf("Expected LlmAPIKey to be '', got '%s'", state.LlmAPIKey)
		}
		if state.LlmModel != "gpt-4o" {
			t.Errorf("Expected LlmModel to be 'gpt-4o', got '%s'", state.LlmModel)
		}
		if state.EmbeddingBaseURL != "http://localhost:8000/v1" {
			t.Errorf("Expected EmbeddingBaseURL to be 'http://localhost:8000/v1', got '%s'", state.EmbeddingBaseURL)
		}
		if state.EmbeddingAPIKey != "" {
			t.Errorf("Expected EmbeddingAPIKey to be '', got '%s'", state.EmbeddingAPIKey)
		}
		if state.EmbeddingModel != "text-embedding-3-small" {
			t.Errorf("Expected EmbeddingModel to be 'text-embedding-3-small', got '%s'", state.EmbeddingModel)
		}
	})
}

// TestColOrDefault は ColOrDefault メソッドのテストです。
func TestColOrDefault(t *testing.T) {
	st := &State{DefaultCollection: "default"}

	// 目的: 入力でコレクションが指定された場合に、そのコレクションが返されることを確認する。
	t.Run("Collection provided in input", func(t *testing.T) {
		col, err := st.ColOrDefault("input_col")
		if err != nil {
			t.Fatalf("Expected no error, but got %v", err)
		}
		if col != "input_col" {
			t.Errorf("Expected 'input_col', but got '%s'", col)
		}
	})

	// 目的: 入力でコレクションが指定されず、デフォルトコレクションが設定されている場合に、デフォルトコレクションが返されることを確認する。
	t.Run("No input collection, uses default", func(t *testing.T) {
		col, err := st.ColOrDefault("")
		if err != nil {
			t.Fatalf("Expected no error, but got %v", err)
		}
		if col != "default" {
			t.Errorf("Expected 'default', but got '%s'", col)
		}
	})

	// 目的: 入力でもデフォルトでもコレクションが指定されていない場合に、エラーが返されることを確認する。
	t.Run("No collection provided or defaulted", func(t *testing.T) {
		st.DefaultCollection = ""
		_, err := st.ColOrDefault("")
		if err == nil {
			t.Fatal("Expected an error, but got nil")
		}
		expectedErrMsg := "collection is required. set input.collection or SOLR_MCP_DEFAULT_COLLECTION"
		if err.Error() != expectedErrMsg {
			t.Errorf("Expected error message '%s', but got '%s'", expectedErrMsg, err.Error())
		}
	})
}
