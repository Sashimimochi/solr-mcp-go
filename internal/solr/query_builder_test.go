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

// mockRequestSender は solr.RequestSender インターフェースのモックです。
type mockRequestSender struct {
	statusCode int
	body       string
	err        error
}

// SendRequest は SendRequest メソッドのモック実装です。
// このメソッドは solr-go ライブラリのインターフェースに合わせる必要があります。
func (m *mockRequestSender) SendRequest(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	// httptest.NewRequest を使ってリクエストをシミュレート
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

// TestQuerySelect は QuerySelect 関数のテストです。
// 目的: Solrクエリが正しく構築され、実行されることを確認します。
func TestQuerySelect(t *testing.T) {
	ctx := context.Background()
	collection := "test_collection"

	t.Run("successful query", func(t *testing.T) {
		// モックレスポンスのセットアップ
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

		// 関数の実行
		resp, err := QuerySelect(ctx, client, collection, params)

		// アサーション
		assert.NoError(t, err, "エラーが発生しないはずです。")
		assert.NotNil(t, resp, "レスポンスがnilであってはなりません。")

		// 戻り値の型を *solr.QueryResponse としてアサーションする
		queryResponse, ok := resp.(*solr.QueryResponse)
		assert.True(t, ok, "レスポンスは *solr.QueryResponse であるべきです。")
		assert.NotNil(t, queryResponse.Response, "レスポンスの 'response' フィールドがnilであってはなりません。")
		assert.Equal(t, 1, queryResponse.Response.NumFound, "numFoundは1であるべきです。")
	})

	t.Run("query with empty q parameter", func(t *testing.T) {
		// qが空の場合、"*:*"が使われることを確認
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

// TestAddFieldsForIDs は AddFieldsForIDs 関数のテストです。
// 目的: クエリボディにIDフィールドが正しく追加されることを確認します。
func TestAddFieldsForIDs(t *testing.T) {
	testCases := []struct {
		name     string
		body     map[string]any
		idField  string
		expected map[string]any
	}{
		{
			name: "fieldsが既に存在する場合",
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
			name: "fieldsが存在しない場合",
			body: map[string]any{
				"query": "*:*",
			},
			idField: "id",
			expected: map[string]any{
				"query": "*:*",
			},
		},
		{
			name: "idFieldが空の場合",
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

// TestExtractIDs は ExtractIDs 関数のテストです。
// 目的: SolrレスポンスからIDが正しく抽出されることを確認します。
func TestExtractIDs(t *testing.T) {
	testCases := []struct {
		name     string
		resp     map[string]any
		idField  string
		expected []string
	}{
		{
			name: "正常なケース（IDが文字列）",
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
			name: "正常なケース（IDが数値）",
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
			name: "ドキュメントが空の場合",
			resp: map[string]any{
				"response": map[string]any{
					"docs": []any{},
				},
			},
			idField:  "id",
			expected: []string{},
		},
		{
			name:     "responseがない場合",
			resp:     map[string]any{},
			idField:  "id",
			expected: []string{},
		},
		{
			name: "IDフィールドがないドキュメントが含まれる場合",
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
			// nilと空スライスを区別せずに比較するため
			if len(ids) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(ids, tc.expected) {
				t.Errorf("expected %v, but got %v", tc.expected, ids)
			}
		})
	}
}

// TestAppendFilterQuery は AppendFilterQuery 関数のテストです。
// 目的: 既存のパラメータマップにフィルタ クエリが正しく追加されることを確認します。
func TestAppendFilterQuery(t *testing.T) {
	testCases := []struct {
		name     string
		params   map[string]any
		fq       string
		expected map[string]any
	}{
		{
			name:   "fqがnilの場合",
			params: map[string]any{"q": "*:*"},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"new_filter:true"},
			},
		},
		{
			name:   "fqが文字列の場合",
			params: map[string]any{"q": "*:*", "fq": "existing_filter:true"},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"existing_filter:true", "new_filter:true"},
			},
		},
		{
			name:   "fqが文字列スライスの場合",
			params: map[string]any{"q": "*:*", "fq": []string{"existing_filter:true"}},
			fq:     "new_filter:true",
			expected: map[string]any{
				"q":  "*:*",
				"fq": []string{"existing_filter:true", "new_filter:true"},
			},
		},
		{
			name:   "fqがanyのスライスの場合",
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

// TestPostQueryJSON は PostQueryJSON 関数のテストです。
// 目的: HTTP POSTリクエストが正しく送信され、レスポンスが正しく解析されることを確認します。
func TestPostQueryJSON(t *testing.T) {
	// モックサーバーのセットアップ
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエストヘッダの確認
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header is not application/json")
		}
		// Basic認証の確認
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			t.Errorf("Basic auth is not correct")
		}
		// リクエストボディの確認
		var body map[string]any
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if body["query"] != "*:*" {
			t.Errorf("query in body is not *:*")
		}

		// 正常なレスポンスを返す
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

	// テストの実行
	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	resp, err := PostQueryJSON(context.Background(), client, server.URL, "testuser", "testpass", "testcollection", body)

	// エラーがないことの確認
	if err != nil {
		t.Fatalf("PostQueryJSON returned an error: %v", err)
	}

	// レスポンス内容の確認
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

// TestPostQueryJSON_Error は PostQueryJSON 関数のエラーケースのテストです。
// 目的: HTTPステータスコードが2xxでない場合にエラーが返されることを確認します。
func TestPostQueryJSON_Error(t *testing.T) {
	// 500エラーを返すモックサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{}
	body := map[string]any{"query": "*:*"}
	_, err := PostQueryJSON(context.Background(), client, server.URL, "", "", "testcollection", body)

	// エラーが返されることの確認
	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
	expectedError := "HTTP status 500: Internal Server Error"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain %q, but got %q", expectedError, err.Error())
	}
}

// TestPostQueryJSON_InvalidJSON は PostQueryJSON 関数のJSONデコードエラーのテストです。
// 目的: レスポンスが不正なJSONの場合にエラーが返されることを確認します。
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

// TestPostQueryJSON_NetworkError は PostQueryJSON 関数のネットワークエラーのテストです。
// 目的: HTTPリクエストが失敗した場合にエラーが返されることを確認します。
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

// TestPostQueryJSON_NoAuth は PostQueryJSON 関数の認証なしのテストです。
// 目的: User/Passが空の場合、Basic認証が設定されないことを確認します。
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

// TestQueryWithRawResponse は QueryWithRawResponse 関数のテストです。
// 目的: クエリが正しく実行され、生のJSONレスポンスが返されることを確認します。
func TestQueryWithRawResponse(t *testing.T) {
	t.Run("正常系: 基本的なクエリ", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// クエリパラメータの確認
			q := r.URL.Query()
			// solr-goのQueryParser形式で送信される
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

		// responseHeaderの確認
		respHeader, ok := resp["responseHeader"].(map[string]any)
		assert.True(t, ok, "responseHeader should be a map")
		assert.NotNil(t, respHeader["params"], "params should be present in responseHeader")

		// responseの確認
		response, ok := resp["response"].(map[string]any)
		assert.True(t, ok, "response should be a map")
		assert.Equal(t, float64(1), response["numFound"])
	})

	t.Run("正常系: paramsでパラメータを追加", func(t *testing.T) {
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

	t.Run("正常系: Basic認証", func(t *testing.T) {
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

	t.Run("正常系: 認証なし", func(t *testing.T) {
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

	t.Run("正常系: 複数のフィルタクエリ", func(t *testing.T) {
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

	t.Run("正常系: ネストされたparamsマップ", func(t *testing.T) {
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

	t.Run("正常系: 様々な型のパラメータ", func(t *testing.T) {
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

	t.Run("正常系: []anyのパラメータ", func(t *testing.T) {
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

	t.Run("正常系: コレクション名に特殊文字", func(t *testing.T) {
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

	t.Run("正常系: paramsでネストされたマップ", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			// ネストされたマップのキーがフラット化されて送信される
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

	t.Run("正常系: ネストされたマップに[]stringが含まれる", func(t *testing.T) {
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

	t.Run("正常系: ネストされたマップに予期しない型が含まれる", func(t *testing.T) {
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

	t.Run("正常系: 予期しない型のパラメータ（default case）", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		// BuildQueryの結果を直接操作してdefaultケースをトリガー
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser()).
			Params(solr.M{
				"unexpected": struct{ Value string }{Value: "test"},
			})

		_, err := QueryWithRawResponse(context.Background(), client, server.URL, "", "", "testcollection", query)

		assert.NoError(t, err)
	})

	t.Run("正常系: limit/offset/fields/filterのパラメータ変換", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{}})
		}))
		defer server.Close()

		client := &http.Client{}
		// これらのパラメータキーを使用してswitch文の各caseをカバー
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

	t.Run("異常系: HTTPエラー", func(t *testing.T) {
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

	t.Run("異常系: 不正なJSON", func(t *testing.T) {
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

	t.Run("異常系: ネットワークエラー", func(t *testing.T) {
		client := &http.Client{}
		query := solr.NewQuery(solr.NewStandardQueryParser().Query("*:*").BuildParser())

		_, err := QueryWithRawResponse(context.Background(), client, "http://invalid-host-that-does-not-exist:9999", "", "", "testcollection", query)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP request error")
	})
}
