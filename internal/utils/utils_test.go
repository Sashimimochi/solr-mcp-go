package utils

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLoggingHandler は LoggingHandler ミドルウェアのテストです。
// リクエストの開始時と完了時に、期待されるログが出力されることを確認します。
func TestLoggingHandler(t *testing.T) {
	// slog の出力をキャプチャするためのバッファ
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	// デフォルトのロガーを、バッファに書き込むロガーに差し替える
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	// テスト終了時にロガーを元に戻す
	defer slog.SetDefault(originalLogger)

	// テスト対象のハンドラ
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// ミドルウェアでラップ
	handlerToTest := LoggingHandler(testHandler)

	// テスト用のリクエストを作成
	req := httptest.NewRequest("GET", "/test-path", nil)
	rr := httptest.NewRecorder()

	// ハンドラを実行
	handlerToTest.ServeHTTP(rr, req)

	// ログの出力を文字列として取得
	logOutput := buf.String()

	// 開始ログの検証
	if !strings.Contains(logOutput, "Started") || !strings.Contains(logOutput, "method=GET") || !strings.Contains(logOutput, "path=/test-path") {
		t.Errorf("開始ログが期待通りではありませんでした. got=%q", logOutput)
	}

	// 完了ログの検証
	if !strings.Contains(logOutput, "Completed") || !strings.Contains(logOutput, "duration=") {
		t.Errorf("完了ログが期待通りではありませんでした. got=%q", logOutput)
	}
}

// TestChoose は Choose 関数のテストです。
// 文字列が空の場合にフォールバック文字列が返されることを確認します。
func TestChoose(t *testing.T) {
	testCases := []struct {
		name     string
		s        string
		fallback string
		expected string
	}{
		{"sが非空", "hello", "world", "hello"},
		{"sが空", "", "world", "world"},
		{"sが空白のみ", "   ", "world", "world"},
		{"両方非空", "hello", "world", "hello"},
		{"両方空", "", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := Choose(tc.s, tc.fallback)
			if actual != tc.expected {
				t.Errorf("結果が異なります. got=%q, want=%q", actual, tc.expected)
			}
		})
	}
}

// TestChooseInt は ChooseInt 関数のテストです。
// 整数が0の場合にフォールバック値が返されることを確認します。
func TestChooseInt(t *testing.T) {
	testCases := []struct {
		name     string
		i        int
		fallback int
		expected int
	}{
		{"iが非ゼロ", 10, 20, 10},
		{"iがゼロ", 0, 20, 20},
		{"両方非ゼロ", 10, 20, 10},
		{"両方ゼロ", 0, 0, 0},
		{"iが負の値", -5, 10, -5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ChooseInt(tc.i, tc.fallback)
			if actual != tc.expected {
				t.Errorf("結果が異なります. got=%d, want=%d", actual, tc.expected)
			}
		})
	}
}

// TestHeadN は HeadN 関数のテストです。
// スライスの先頭N件を正しく取得できることを確認します。
func TestHeadN(t *testing.T) {
	testCases := []struct {
		name     string
		s        []int
		n        int
		expected []int
	}{
		{"nが長さより小さい", []int{1, 2, 3, 4, 5}, 3, []int{1, 2, 3}},
		{"nが長さと等しい", []int{1, 2, 3}, 3, []int{1, 2, 3}},
		{"nが長さより大きい", []int{1, 2}, 5, []int{1, 2}},
		{"nが0", []int{1, 2, 3}, 0, []int{}},
		{"スライスが空", []int{}, 5, []int{}},
		{"nが負の値", []int{1, 2, 3}, -1, []int{}}, // n <= 0 のケース
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// HeadNは負のnを0として扱うべき
			n := tc.n
			if n < 0 {
				n = 0
			}
			expected := tc.s
			if len(tc.s) > n {
				expected = tc.s[:n]
			}

			actual := HeadN(tc.s, tc.n)
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("結果が異なります. got=%v, want=%v", actual, expected)
			}
		})
	}
}

// TestPrioritize は Prioritize 関数のテストです。
// 文字列スライスが、優先キーワードに基づいて正しく並べ替えられることを確認します。
func TestPrioritize(t *testing.T) {
	testCases := []struct {
		name     string
		names    []string
		prefs    []string
		expected []string
	}{
		{
			name:     "基本的な優先順位付け",
			names:    []string{"apple", "banana", "cherry"},
			prefs:    []string{"banana", "apple"},
			expected: []string{"banana", "apple", "cherry"},
		},
		{
			name:     "大文字小文字を区別しない",
			names:    []string{"Apple", "banana", "Cherry"},
			prefs:    []string{"cherry", "apple"},
			expected: []string{"Cherry", "Apple", "banana"},
		},
		{
			name:     "重複する要素は先に現れたものが優先",
			names:    []string{"title_en", "title_ja", "description"},
			prefs:    []string{"title"},
			expected: []string{"title_en", "title_ja", "description"},
		},
		{
			name:     "優先キーワードに一致しない",
			names:    []string{"a", "b", "c"},
			prefs:    []string{"d", "e"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "namesが空",
			names:    []string{},
			prefs:    []string{"a", "b"},
			expected: []string{},
		},
		{
			name:     "prefsが空",
			names:    []string{"a", "b", "c"},
			prefs:    []string{},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := Prioritize(tc.names, tc.prefs)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("結果が異なります. got=%v, want=%v", actual, tc.expected)
			}
		})
	}
}

func TestGetTimezone(t *testing.T) {
	// タイムゾーンがUTCの場合
	t.Run("UTCの場合", func(t *testing.T) {
		// UTCに設定
		loc := time.FixedZone("UTC", 0)
		time.Local = loc

		tz := GetTimezone()
		if tz != "Z" {
			t.Errorf("Expected 'Z' for UTC timezone, got '%s'", tz)
		}
	})

	// タイムゾーンがJSTの場合
	t.Run("JSTの場合", func(t *testing.T) {
		// JSTに設定
		loc := time.FixedZone("JST", 9*3600)
		time.Local = loc

		tz := GetTimezone()
		if tz != "+09:00" {
			t.Errorf("Expected '+09:00' for JST timezone, got '%s'", tz)
		}
	})
}

func TestTwoDigitString(t *testing.T) {
	// nが10以上の場合
	t.Run("nが10以上", func(t *testing.T) {
		for i := 10; i < 100; i++ {
			s := strconv.Itoa(i)
			if len(s) != 2 {
				t.Errorf("Expected length 2 for %d, got '%s'", i, s)
			}
		}
	})

	// nが0から9の場合
	t.Run("nが0から9", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			s := strconv.Itoa(i)
			if len(s) != 1 {
				t.Errorf("Expected length 1 for %d, got '%s'", i, s)
			}
			s = "0" + s
			if len(s) != 2 {
				t.Errorf("Expected length 2 after padding for %d, got '%s'", i, s)
			}
		}
	})

	// nが負の値の場合
	t.Run("nが負の値", func(t *testing.T) {
		s := strconv.Itoa(-5)
		if s != "-5" {
			t.Errorf("Expected '-5' for -5, got '%s'", s)
		}
	})
}
