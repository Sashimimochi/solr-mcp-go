package config

import (
	"os"
	"testing"
)

// TestGetEnv は GetEnv 関数のテストです。
func TestGetEnv(t *testing.T) {
	// ケース1: 環境変数が設定されている場合
	t.Run("環境変数が設定されている場合", func(t *testing.T) {
		// テスト用の環境変数を設定
		testKey := "TEST_ENV_VAR"
		expectedValue := "test_value"
		os.Setenv(testKey, expectedValue)
		// テスト後に環境変数をクリア
		defer os.Unsetenv(testKey)

		// GetEnv を呼び出し
		actualValue := GetEnv(testKey, "default")

		// 結果を検証
		if actualValue != expectedValue {
			t.Errorf("Expected %s, Actual %s", expectedValue, actualValue)
		}
	})

	// ケース2: 環境変数が設定されていない場合
	t.Run("環境変数が設定されていない場合", func(t *testing.T) {
		// 存在しない環境変数のキー
		testKey := "NON_EXISTENT_VAR"
		defaultValue := "default_value"

		// GetEnv を呼び出し
		actualValue := GetEnv(testKey, defaultValue)

		// 結果を検証
		if actualValue != defaultValue {
			t.Errorf("Expected %s, Actual %s", defaultValue, actualValue)
		}
	})
}

// TestNewSolrClient は NewSolrClient 関数のテストです。
func TestNewSolrClient(t *testing.T) {
	// 事前に設定されている可能性のある環境変数をクリア
	os.Unsetenv("SOLR_MCP_SOLR_URL")
	os.Unsetenv("SOLR_BASIC_USER")
	os.Unsetenv("SOLR_BASIC_PASS")

	// ケース1: 環境変数が何も設定されていない場合
	t.Run("環境変数が何も設定されていない場合", func(t *testing.T) {
		_, baseURL, user, pass, _ := NewSolrClient()
		expectedURL := "http://localhost:8983"
		if baseURL != expectedURL {
			t.Errorf("Expected URL %s, Actual %s", expectedURL, baseURL)
		}
		if user != "" {
			t.Errorf("Expected empty username, Actual %s", user)
		}
		if pass != "" {
			t.Errorf("Expected empty password, Actual %s", pass)
		}
	})

	// ケース2: SolrのURLが設定されている場合
	t.Run("SolrのURLが設定されている場合", func(t *testing.T) {
		expectedURL := "http://solr.example.com:8983"
		os.Setenv("SOLR_MCP_SOLR_URL", expectedURL)
		defer os.Unsetenv("SOLR_MCP_SOLR_URL")

		_, baseURL, _, _, _ := NewSolrClient()
		if baseURL != expectedURL {
			t.Errorf("期待するURLは %s, しかし実際は %s", expectedURL, baseURL)
		}
	})

	// ケース3: Basic認証の情報が設定されている場合
	t.Run("Basic認証の情報が設定されている場合", func(t *testing.T) {
		expectedUser := "testuser"
		expectedPass := "testpass"
		os.Setenv("SOLR_BASIC_USER", expectedUser)
		os.Setenv("SOLR_BASIC_PASS", expectedPass)
		defer os.Unsetenv("SOLR_BASIC_USER")
		defer os.Unsetenv("SOLR_BASIC_PASS")

		_, _, user, pass, _ := NewSolrClient()
		if user != expectedUser {
			t.Errorf("期待するユーザー名は %s, しかし実際は %s", expectedUser, user)
		}
		if pass != expectedPass {
			t.Errorf("期待するパスワードは %s, しかし実際は %s", expectedPass, pass)
		}
	})
}
