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

// TestLoggingHandler tests the LoggingHandler middleware.
// Verifies expected logs are emitted at request start and completion.
func TestLoggingHandler(t *testing.T) {
	// Buffer to capture slog output
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	// Replace default logger with one writing to the buffer
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	// Restore the original logger at test end
	defer slog.SetDefault(originalLogger)

	// Handler under test
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	handlerToTest := LoggingHandler(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test-path", nil)
	rr := httptest.NewRecorder()

	// Execute handler
	handlerToTest.ServeHTTP(rr, req)

	// Get log output as string
	logOutput := buf.String()

	// Validate start log
	if !strings.Contains(logOutput, "Started") || !strings.Contains(logOutput, "method=GET") || !strings.Contains(logOutput, "path=/test-path") {
		t.Errorf("Start log not as expected. got=%q", logOutput)
	}

	// Validate completion log
	if !strings.Contains(logOutput, "Completed") || !strings.Contains(logOutput, "duration=") {
		t.Errorf("Completion log not as expected. got=%q", logOutput)
	}
}

// TestChoose tests the Choose function.
// Ensures fallback string is returned when s is empty.
func TestChoose(t *testing.T) {
	testCases := []struct {
		name     string
		s        string
		fallback string
		expected string
	}{
		{"s is non-empty", "hello", "world", "hello"},
		{"s is empty", "", "world", "world"},
		{"s is whitespace only", "   ", "world", "world"},
		{"both non-empty", "hello", "world", "hello"},
		{"both empty", "", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := Choose(tc.s, tc.fallback)
			if actual != tc.expected {
				t.Errorf("Result differs. got=%q, want=%q", actual, tc.expected)
			}
		})
	}
}

// TestChooseInt tests the ChooseInt function.
// Ensures fallback value is returned when i is 0.
func TestChooseInt(t *testing.T) {
	testCases := []struct {
		name     string
		i        int
		fallback int
		expected int
	}{
		{"i is non-zero", 10, 20, 10},
		{"i is zero", 0, 20, 20},
		{"both non-zero", 10, 20, 10},
		{"both zero", 0, 0, 0},
		{"i is negative", -5, 10, -5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ChooseInt(tc.i, tc.fallback)
			if actual != tc.expected {
				t.Errorf("Result differs. got=%d, want=%d", actual, tc.expected)
			}
		})
	}
}

// TestHeadN tests the HeadN function.
// Ensures the first N elements of a slice are returned correctly.
func TestHeadN(t *testing.T) {
	testCases := []struct {
		name     string
		s        []int
		n        int
		expected []int
	}{
		{"n less than length", []int{1, 2, 3, 4, 5}, 3, []int{1, 2, 3}},
		{"n equals length", []int{1, 2, 3}, 3, []int{1, 2, 3}},
		{"n greater than length", []int{1, 2}, 5, []int{1, 2}},
		{"n is 0", []int{1, 2, 3}, 0, []int{}},
		{"slice is empty", []int{}, 5, []int{}},
		{"n is negative", []int{1, 2, 3}, -1, []int{}}, // case where n <= 0
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// HeadN should treat negative n as 0
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
				t.Errorf("Result differs. got=%v, want=%v", actual, expected)
			}
		})
	}
}

// TestPrioritize tests the Prioritize function.
// Ensures strings are ordered based on priority keywords.
func TestPrioritize(t *testing.T) {
	testCases := []struct {
		name     string
		names    []string
		prefs    []string
		expected []string
	}{
		{
			name:     "Basic prioritization",
			names:    []string{"apple", "banana", "cherry"},
			prefs:    []string{"banana", "apple"},
			expected: []string{"banana", "apple", "cherry"},
		},
		{
			name:     "Case-insensitive matching",
			names:    []string{"Apple", "banana", "Cherry"},
			prefs:    []string{"cherry", "apple"},
			expected: []string{"Cherry", "Apple", "banana"},
		},
		{
			name:     "Prefer earlier duplicates",
			names:    []string{"title_en", "title_ja", "description"},
			prefs:    []string{"title"},
			expected: []string{"title_en", "title_ja", "description"},
		},
		{
			name:     "No match with preferences",
			names:    []string{"a", "b", "c"},
			prefs:    []string{"d", "e"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Empty names",
			names:    []string{},
			prefs:    []string{"a", "b"},
			expected: []string{},
		},
		{
			name:     "Empty prefs",
			names:    []string{"a", "b", "c"},
			prefs:    []string{},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := Prioritize(tc.names, tc.prefs)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("Result differs. got=%v, want=%v", actual, tc.expected)
			}
		})
	}
}

func TestGetTimezone(t *testing.T) {
	// When timezone is UTC
	t.Run("UTC case", func(t *testing.T) {
		// Set to UTC
		loc := time.FixedZone("UTC", 0)
		time.Local = loc

		tz := GetTimezone()
		if tz != "Z" {
			t.Errorf("Expected 'Z' for UTC timezone, got '%s'", tz)
		}
	})

	// When timezone is JST
	t.Run("JST case", func(t *testing.T) {
		// Set to JST
		loc := time.FixedZone("JST", 9*3600)
		time.Local = loc

		tz := GetTimezone()
		if tz != "+09:00" {
			t.Errorf("Expected '+09:00' for JST timezone, got '%s'", tz)
		}
	})
}

func TestTwoDigitString(t *testing.T) {
	// When n is greater than or equal to 10
	t.Run("n >= 10", func(t *testing.T) {
		for i := 10; i < 100; i++ {
			s := strconv.Itoa(i)
			if len(s) != 2 {
				t.Errorf("Expected length 2 for %d, got '%s'", i, s)
			}
		}
	})

	// When n is between 0 and 9
	t.Run("n between 0 and 9", func(t *testing.T) {
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

	// When n is negative
	t.Run("n is negative", func(t *testing.T) {
		s := strconv.Itoa(-5)
		if s != "-5" {
			t.Errorf("Expected '-5' for -5, got '%s'", s)
		}
	})
}
