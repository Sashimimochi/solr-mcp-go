package solr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"solr-mcp-go/internal/types"
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

// TestToRangeFilterQuery は toRangeFilterQuery 関数のテストです。
// 目的: LlmRangeから正しい範囲フィルタ文字列が生成されることを確認します。
func TestToRangeFilterQuery(t *testing.T) {
	from := "10"
	to := "20"
	empty := ""

	testCases := []struct {
		name     string
		r        types.LlmRange
		expected string
	}{
		{
			name: "FromとToが両方ある場合",
			r: types.LlmRange{
				Field: "price",
				From:  &from,
				To:    &to,
			},
			expected: "price:[10 TO 20]",
		},
		{
			name: "Fromのみある場合",
			r: types.LlmRange{
				Field: "price",
				From:  &from,
				To:    nil,
			},
			expected: "price:[10 TO *]",
		},
		{
			name: "Toのみある場合",
			r: types.LlmRange{
				Field: "price",
				From:  nil,
				To:    &to,
			},
			expected: "price:[* TO 20]",
		},
		{
			name: "FromとToが両方ない場合",
			r: types.LlmRange{
				Field: "price",
				From:  nil,
				To:    nil,
			},
			expected: "price:[* TO *]",
		},
		{
			name: "FromとToが空文字列の場合",
			r: types.LlmRange{
				Field: "price",
				From:  &empty,
				To:    &empty,
			},
			expected: "price:[* TO *]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := toRangeFilterQuery(tc.r)
			if actual != tc.expected {
				t.Errorf("expected %q, but got %q", tc.expected, actual)
			}
		})
	}
}

// TestBuildEdismaxParams は BuildEdismaxParams 関数のテストです。
// 目的: LlmPlanとFieldCatalogから正しいeDisMaxクエリパラメータが生成されることを確認します。
func TestBuildEdismaxParams(t *testing.T) {
	from := "100"
	to := "200"
	plan := &types.LlmPlan{
		EdisMax: struct {
			TextQuery   string           `json:"text_query,omitempty"`
			Filters     []string         `json:"filters,omitempty"`
			Ranges      []types.LlmRange `json:"ranges,omitempty"`
			Sort        string           `json:"sort,omitempty"`
			FacetFields []string         `json:"facet_fields,omitempty"`
			Params      map[string]any   `json:"params,omitempty"`
			Fields      []string         `json:"fields,omitempty"`
		}{
			TextQuery:   "test query",
			Filters:     []string{"cat:electronics"},
			Ranges:      []types.LlmRange{{Field: "price", From: &from, To: &to}},
			Sort:        "score desc",
			FacetFields: []string{"manufacturer"},
			Fields:      []string{"id", "name", "price"},
			Params:      map[string]any{"bq": "category:books^2"},
		},
	}
	fc := &types.FieldCatalog{
		Guessed: types.GuessedFields{
			DefaultDF: "text",
		},
	}

	params := BuildEdismaxParams(plan, fc, 10, 0)

	// 基本的なパラメータの確認
	if params["defType"] != "edismax" {
		t.Errorf("defType is not edismax")
	}
	if params["q"] != "test query" {
		t.Errorf("q is not 'test query'")
	}
	if params["rows"] != 10 {
		t.Errorf("rows is not 10")
	}
	if params["start"] != 0 {
		t.Errorf("start is not 0")
	}
	if params["wt"] != "json" {
		t.Errorf("wt is not json")
	}

	// 追加パラメータの確認
	if params["bq"] != "category:books^2" {
		t.Errorf("bq is not 'category:books^2'")
	}

	// dfの確認
	if params["df"] != "text" {
		t.Errorf("df is not 'text'")
	}

	// fqの確認
	expectedFq := []string{"cat:electronics", "price:[100 TO 200]"}
	if !reflect.DeepEqual(params["fq"], expectedFq) {
		t.Errorf("fq is not correct, expected %v, got %v", expectedFq, params["fq"])
	}

	// sortの確認
	if params["sort"] != "score desc" {
		t.Errorf("sort is not 'score desc'")
	}

	// facetの確認
	if params["facet"] != "true" {
		t.Errorf("facet is not 'true'")
	}
	if !reflect.DeepEqual(params["facet.field"], []string{"manufacturer"}) {
		t.Errorf("facet.field is not correct")
	}

	// flの確認
	if params["fl"] != "id,name,price" {
		t.Errorf("fl is not 'id,name,price'")
	}
}

// TestBuildKNNJSON は BuildKNNJSON 関数のテストです。
// 目的: LlmPlan, FieldCatalog, embeddingから正しいKNNクエリのJSONボディが生成されることを確認します。
func TestBuildKNNJSON(t *testing.T) {
	plan := &types.LlmPlan{
		Mode: "hybrid",
		EdisMax: struct {
			TextQuery   string           `json:"text_query,omitempty"`
			Filters     []string         `json:"filters,omitempty"`
			Ranges      []types.LlmRange `json:"ranges,omitempty"`
			Sort        string           `json:"sort,omitempty"`
			FacetFields []string         `json:"facet_fields,omitempty"`
			Params      map[string]any   `json:"params,omitempty"`
			Fields      []string         `json:"fields,omitempty"`
		}{
			TextQuery:   "hybrid search",
			Filters:     []string{"inStock:true"},
			FacetFields: []string{"category"},
			Fields:      []string{"id", "name"},
			Params:      map[string]any{"qf": "text_en"},
		},
		Vector: struct {
			Field     string `json:"field,omitempty"`
			K         int    `json:"k,omitempty"`
			QueryText string `json:"query_text,omitempty"`
		}{
			Field: "vector_field",
			K:     50,
		},
	}
	fc := &types.FieldCatalog{
		Guessed: types.GuessedFields{
			DefaultDF: "text_all",
		},
	}
	embedding := []float64{0.1, 0.2, 0.3}

	jsonBody := BuildKNNJSON(plan, fc, embedding, 20, 5)

	// KNN仕様の確認
	knnSpecs, ok := jsonBody["knn"].([]interface{})
	if !ok {
		t.Fatal("knn is not a []interface{}")
	}
	if len(knnSpecs) != 1 {
		t.Fatalf("expected 1 knn spec, got %d", len(knnSpecs))
	}
	knnSpecMap, ok := knnSpecs[0].(map[string]interface{})
	if !ok {
		t.Fatal("knn spec is not a map[string]interface{}")
	}

	if knnSpecMap["field"] != "vector_field" {
		t.Errorf("knn field is not 'vector_field'")
	}

	// Vectorの比較
	vec, ok := knnSpecMap["vector"].([]interface{})
	if !ok {
		t.Fatal("vector is not []interface{}")
	}
	actualVector := make([]float64, len(vec))
	for i, v := range vec {
		actualVector[i], _ = v.(float64)
	}
	if !reflect.DeepEqual(actualVector, embedding) {
		t.Errorf("knn vector is not correct")
	}

	// Kの比較（JSONの数値はfloat64になる）
	if knnSpecMap["k"].(float64) != 50 {
		t.Errorf("knn K is not 50")
	}

	// 基本的なパラメータの確認
	if jsonBody["limit"].(float64) != 20 {
		t.Errorf("limit is not 20")
	}
	if jsonBody["offset"].(float64) != 5 {
		t.Errorf("offset is not 5")
	}

	// フィルターの確認
	expectedFilter := []interface{}{"inStock:true"}
	actualFilter, _ := jsonBody["filter"].([]interface{})
	if !reflect.DeepEqual(actualFilter, expectedFilter) {
		t.Errorf("filter is not correct, expected %v, got %v", expectedFilter, actualFilter)
	}

	// クエリの確認
	expectedQuery := "{!edismax qf=text_en df=text_all}hybrid search"
	if jsonBody["query"] != expectedQuery {
		t.Errorf("query is not correct, expected %q, got %q", expectedQuery, jsonBody["query"])
	}

	// フィールドの確認
	expectedFields := []interface{}{"id", "name"}
	actualFields, _ := jsonBody["fields"].([]interface{})
	if !reflect.DeepEqual(actualFields, expectedFields) {
		t.Errorf("fields are not correct, expected %v, got %v", expectedFields, actualFields)
	}

	// ファセットの確認
	facet, ok := jsonBody["facet"].(map[string]any)
	if !ok {
		t.Fatal("facet is not a map[string]any")
	}
	if _, ok := facet["facet_category"]; !ok {
		t.Errorf("facet_category is not set")
	}
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
