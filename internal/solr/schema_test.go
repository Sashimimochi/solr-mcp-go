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

// TestGetFieldCatalog は GetFieldCatalog 関数のテストです。
// このテストでは、httptest.Server を使用して Solr の API を模倣し、
// 正常なレスポンス、異常なレスポンス、キャッシュの動作などを検証します。
func TestGetFieldCatalog(t *testing.T) {
	// --- Setup Mock Server ---
	// Solr の API エンドポイントを模倣するハンドラを定義します。
	// リクエストのパスに応じて、異なる JSON レスポンスを返します。
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		// Mock Unique Key Get API from Solr
		case "/solr/testcollection/schema/uniquekey":
			fmt.Fprintln(w, `{"uniqueKey":"id"}`)
		// Mock Field Information Get API from Solr
		case "/solr/testcollection/schema/fields":
			// Mock Invalid URL Path Response
			if r.URL.Query().Get("error") == "true" {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			// Mock Bad JSON Response
			if r.URL.Query().Get("badjson") == "true" {
				fmt.Fprintln(w, `{"fields": [}`)
				return
			}
			// Mock Successful Response
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
			// Mock Invalid URL Path Response
			if r.URL.Query().Get("error") == "true" {
				http.Error(w, "File Not Found", http.StatusNotFound)
				return
			}
			// Mock Successful Response
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
	t.Run("正常系: 全てのAPIが正常にレスポンスを返す場合", func(t *testing.T) {
		// 目的: Solr からユニークキー、フィールド、メタデータを正常に取得し、
		//       FieldCatalog が正しく構築されることを確認します。
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
			t.Fatalf("予期せぬエラーが発生しました: %v", err)
		}

		// 期待される FieldCatalog の構造
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
			Texts:   []string{"id", "title_txt_ja", "category_s"},
			Numbers: []string{"price_i", "stock_l", "weight_f", "score_d"},
			Dates:   []string{"release_dt"},
			Bools:   []string{"in_stock_b"},
			// GuessFields の結果は別途テストが必要だが、ここでは主要な分類を検証
		}

		// 結果の検証
		if fc.UniqueKey != expectedFC.UniqueKey {
			t.Errorf("UniqueKey が異なります. got=%s, want=%s", fc.UniqueKey, expectedFC.UniqueKey)
		}
		if !reflect.DeepEqual(fc.All, expectedFC.All) {
			t.Errorf("All が異なります. got=%v, want=%v", fc.All, expectedFC.All)
		}
		if !reflect.DeepEqual(fc.Metadata, expectedFC.Metadata) {
			t.Errorf("Metadata が異なります. got=%v, want=%v", fc.Metadata, expectedFC.Metadata)
		}
		// スライスの比較は順序を問わないようにするべきだが、ここでは簡略化
		if !reflect.DeepEqual(fc.Texts, expectedFC.Texts) {
			t.Errorf("Texts が異なります. got=%v, want=%v", fc.Texts, expectedFC.Texts)
		}
		if !reflect.DeepEqual(fc.Numbers, expectedFC.Numbers) {
			t.Errorf("Numbers が異なります. got=%v, want=%v", fc.Numbers, expectedFC.Numbers)
		}
		if !reflect.DeepEqual(fc.Dates, expectedFC.Dates) {
			t.Errorf("Dates が異なります. got=%v, want=%v", fc.Dates, expectedFC.Dates)
		}
		if !reflect.DeepEqual(fc.Bools, expectedFC.Bools) {
			t.Errorf("Bools が異なります. got=%v, want=%v", fc.Bools, expectedFC.Bools)
		}
	})

	t.Run("正常系: キャッシュが有効な場合", func(t *testing.T) {
		// 目的: 一度取得したデータがキャッシュされ、TTL 内であれば
		//       再度 HTTP リクエストが発行されないことを確認します。
		requestCount := 0
		// リクエストカウンタ付きのモックサーバー
		countingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			// 正常系と同じレスポンスを返す
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
				TTL:       1 * time.Minute, // 1分間のキャッシュTTL
			},
		}

		// 1回目の呼び出し（キャッシュされる）
		_, err := GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("1回目の呼び出しでエラー: %v", err)
		}
		if requestCount == 0 {
			t.Fatal("1回目の呼び出しでリクエストがありませんでした")
		}
		initialRequestCount := requestCount

		// 2回目の呼び出し（キャッシュから返されるはず）
		_, err = GetFieldCatalog(context.Background(), sCtx, "testcollection")
		if err != nil {
			t.Fatalf("2回目の呼び出しでエラー: %v", err)
		}

		// リクエスト数が増えていないことを確認
		if requestCount != initialRequestCount {
			t.Errorf("キャッシュが効いていません。リクエスト数が増加しました. got=%d, want=%d", requestCount, initialRequestCount)
		}
	})

	t.Run("異常系: フィールド取得APIがエラーを返す場合", func(t *testing.T) {
		// 目的: 依存するAPIの一つがエラーを返した際に、GetFieldCatalog が
		//       正しくエラーを伝播させることを確認します。
		// BaseURLを上書きしてしまっているので、GetFieldCatalog内のURL生成がうまくいかない
		// GetFieldCatalog内のURL生成ロジックを考慮し、テストを修正する必要がある
		// ここでは簡略化のため、モックサーバーのハンドラでパスを判定する方法に戻す
		// このテストケースは意図通りに動作しないため、より良い方法を検討する
		// 代わりに、ハンドラ内で特定のコレクション名に対してエラーを返すようにするなどの工夫が考えられる
		// 今回は、ハンドラは共通とし、GetFieldCatalogに渡すBaseURLで制御するのではなく、
		// GetFieldCatalog内のURL生成ロジックを信頼し、ハンドラ側でパス全体をみて判断する
		// しかし、現在のハンドラはパスのプレフィックスしか見ていないため、このテストは失敗する
		// t.Skip("このテストは現在のモック実装では正しく動作しません")

		// 正しいアプローチ:
		// モックサーバーのハンドラをテストケースごとに設定するか、
		// ハンドラがより詳細なリクエスト情報（クエリパラメータなど）を見て挙動を変えるようにする
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
			t.Fatal("エラーが返されるべきところで、nil が返されました")
		}
	})

	t.Run("異常系: レスポンスが不正なJSONの場合", func(t *testing.T) {
		// 目的: APIからのレスポンスが期待したJSON形式でない場合に、
		//       JSONのデコードエラーとして正しく処理されることを確認します。
		badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/solr/testcollection/schema/fields" {
				fmt.Fprintln(w, `{"fields": [`) // 不正なJSON
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
			t.Fatal("エラーが返されるべきところで、nil が返されました")
		}
	})

	t.Run("正常系: メタデータファイルが存在しない場合", func(t *testing.T) {
		// 目的: field_metadata.json が存在しない（APIがエラーを返す）場合でも
		//       処理が中断せず、FieldCatalog の他の部分が正しく構築されることを確認します。
		noMetaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/solr/testcollection/schema/uniquekey":
				fmt.Fprintln(w, `{"uniqueKey":"id"}`)
			case "/solr/testcollection/schema/fields":
				// 正常系と同じレスポンス
				fields := struct {
					Fields []types.SolrField `json:"fields"`
				}{
					Fields: []types.SolrField{{Name: "id", Type: "string"}},
				}
				json.NewEncoder(w).Encode(fields)
			case "/solr/testcollection/admin/file":
				// メタデータはエラーを返す
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
			t.Fatalf("予期せぬエラーが発生しました: %v", err)
		}

		// メタデータが空であることを確認
		if len(fc.Metadata) != 0 {
			t.Errorf("Metadata は空であるべきです. got=%v", fc.Metadata)
		}
		// 他の情報は取得できていることを確認
		if fc.UniqueKey != "id" {
			t.Errorf("UniqueKey が取得できていません. got=%s", fc.UniqueKey)
		}
	})
}

// TestGuessFields は GuessFields 関数のテストです。
// 様々なフィールド名から、価格、日付、ブランドなどの役割を正しく推測できるか検証します。
func TestGuessFields(t *testing.T) {
	// --- テストケースの定義 ---
	testCases := []struct {
		name     string
		fc       *types.FieldCatalog
		expected types.GuessedFields
	}{
		{
			name: "英語のフィールド名から推測",
			fc: &types.FieldCatalog{
				Numbers: []string{"item_price", "age"},
				Dates:   []string{"created_date"},
				Texts:   []string{"product_brand", "item_category", "title"},
				Bools:   []string{"is_in_stock"},
			},
			expected: types.GuessedFields{
				Price:     "item_price",
				Date:      "created_date",
				Brand:     "product_brand",
				Category:  "item_category",
				InStock:   "is_in_stock",
				DefaultDF: "title",
				TextTopN:  []string{"title", "product_brand", "item_category"},
			},
		},
		{
			name: "日本語のフィールド名から推測",
			fc: &types.FieldCatalog{
				Numbers: []string{"商品価格"},
				Dates:   []string{"登録日時"},
				Texts:   []string{"メーカー名", "商品分類", "商品名"},
				Bools:   []string{"在庫有無"},
			},
			expected: types.GuessedFields{
				Price:     "商品価格",
				Date:      "登録日時",
				Brand:     "メーカー名",
				Category:  "商品分類",
				InStock:   "在庫有無",
				DefaultDF: "商品名",
				TextTopN:  []string{"商品名", "メーカー名", "商品分類"},
			},
		},
		{
			name: "該当するフィールドがない場合",
			fc: &types.FieldCatalog{
				Numbers: []string{"field1"},
				Dates:   []string{"field2"},
				Texts:   []string{"field3"},
				Bools:   []string{"field4"},
			},
			expected: types.GuessedFields{
				Price:     "",
				Date:      "",
				Brand:     "",
				Category:  "",
				InStock:   "",
				DefaultDF: "field3", // Textsの最初のフィールドが使われる
				TextTopN:  []string{"field3"},
			},
		},
		{
			name: "テキストフィールドの優先順位付け",
			fc: &types.FieldCatalog{
				Texts: []string{"description", "product_name", "text"},
			},
			expected: types.GuessedFields{
				Price:     "",
				Date:      "",
				Brand:     "",
				Category:  "",
				InStock:   "",
				DefaultDF: "product_name", // "name" が "description" や "text" より優先される
				TextTopN:  []string{"product_name", "description", "text"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// テスト対象の関数を実行
			actual := GuessFields(tc.fc)
			// 結果を比較
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("結果が異なります。\n got: %+v\nwant: %+v", actual, tc.expected)
			}
		})
	}
}

// TestSummarizeSchema は SummarizeSchema 関数のテストです。
// FieldCatalog の内容が、期待通りに文字列として要約されるか検証します。
func TestSummarizeSchema(t *testing.T) {
	// --- テストケースの定義 ---
	testCases := []struct {
		name     string
		fc       *types.FieldCatalog
		expected string
	}{
		{
			name: "全ての情報を持つカタログ",
			fc: &types.FieldCatalog{
				UniqueKey: "product_id",
				Texts:     []string{"title", "description"},
				Numbers:   []string{"price"},
				Dates:     []string{"release_date"},
				Bools:     []string{"in_stock"},
				Metadata: map[string]types.FieldMetadata{
					"title": {Description: "商品名"},
					"price": {Description: "価格"},
				},
				Guessed: types.GuessedFields{
					Price:     "price",
					Date:      "release_date",
					DefaultDF: "title",
				},
			},
			expected: `uniqueKey=product_id
text_fields: title(商品名), description
number_fields: price(価格)
date_fields: release_date
bool_fields: in_stock
guess.price=price
guess.date=release_date
guess.defaultDF=title
`,
		},
		{
			name: "一部の情報が欠けているカタログ",
			fc: &types.FieldCatalog{
				UniqueKey: "id",
				Texts:     []string{"name"},
				Numbers:   []string{}, // 数字フィールドなし
				Dates:     []string{}, // 日付フィールドなし
				Bools:     []string{"available"},
				Metadata:  map[string]types.FieldMetadata{}, // メタデータなし
				Guessed:   types.GuessedFields{},            // 推測結果なし
			},
			expected: `uniqueKey=id
text_fields: name
bool_fields: available
`,
		},
		{
			name:     "空のカタログ",
			fc:       &types.FieldCatalog{},
			expected: "uniqueKey=\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// テスト対象の関数を実行
			actual := SummarizeSchema(tc.fc)
			// 結果を比較
			if actual != tc.expected {
				t.Errorf("結果が異なります。\n--- got ---\n%s\n--- want ---\n%s", actual, tc.expected)
			}
		})
	}
}
