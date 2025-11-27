package server

import (
	"testing"
)

// TestNewServerState は NewServerState 関数のテストです。
func TestNewServerState(t *testing.T) {
	// 目的: 環境変数が設定されている場合に、NewServerStateが正しくStateを初期化することを確認する。
	t.Run("With environment variables", func(t *testing.T) {
		t.Setenv("SOLR_MCP_DEFAULT_COLLECTION", "test_collection")

		state := NewServerState()

		if state.DefaultCollection != "test_collection" {
			t.Errorf("Expected DefaultCollection to be 'test_collection', got '%s'", state.DefaultCollection)
		}
	})

	// 目的: 環境変数が設定されていない場合に、デフォルト値でStateが初期化されることを確認する。
	t.Run("Without environment variables (defaults)", func(t *testing.T) {
		// t.Setenv() はこのサブテスト内でのみ有効なため、ここでは環境変数は未設定の状態
		state := NewServerState()

		if state.DefaultCollection != "gettingstarted" {
			t.Errorf("Expected DefaultCollection to be 'gettingstarted', got '%s'", state.DefaultCollection)
		}
	})
}
