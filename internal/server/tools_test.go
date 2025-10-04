package server

import (
	"net/http"
	"solr-mcp-go/internal/types"
	"testing"
	"time"

	solr "github.com/stevenferrer/solr-go"
)

// newTestState はテスト用のStateとモックサーバーを生成します。
func newTestState(t *testing.T) *State {
	client := solr.NewJSONClient("http://localhost:8983/solr")
	return &State{
		SolrClient:        client,
		BaseURL:           "http://localhost:8983",
		DefaultCollection: "test",
		HttpClient:        &http.Client{},
		SchemaCache: types.SchemaCache{
			LastFetch: make(map[string]time.Time),
			TTL:       10 * time.Minute,
			ByCol:     make(map[string]*types.FieldCatalog),
		},
	}
}

// TestToolQuery は toolQuery メソッドのテストです。
func TestToolQuery(t *testing.T) {
	// 目的: toolQueryがSolrクエリを正しく構築し、実行できることを確認する。

	// 目的: コレクションが指定されていない場合にエラーが発生することを確認する。

}

// TestToolCommit は toolCommit メソッドのテストです。
func TestToolCommit(t *testing.T) {
	// 目的: toolCommitがSolrへのcommitリクエストを正しく送信できることを確認する。

}

// TestToolPing は toolPing メソッドのテストです。
func TestToolPing(t *testing.T) {
	// 目的: toolPingがSolrへのping（実体はlimit=0のクエリ）を正しく実行できることを確認する。

}

// TestToolSearchSmart は toolSearchSmart メソッドのテストです。
func TestToolSearchSmart(t *testing.T) {
	// 目的: toolSearchSmartがLLMと連携し、キーワード検索を正しく実行できることを確認する。

	// 目的: クエリが空の場合にエラーが発生することを確認する。

	// 目的: LLMのAPIキーが設定されていない場合にエラーが発生することを確認する。
}

// TestAddTools は AddTools 関数のテストです。
func TestAddTools(t *testing.T) {
	// 目的: すべてのツールが正しくmcp.Serverに登録されることを確認する。
}
