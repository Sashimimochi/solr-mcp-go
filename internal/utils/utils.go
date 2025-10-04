package utils

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LoggingHandler is a middleware that logs requests.
func LoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		slog.Info("Started", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
		slog.Info("Completed", "path", r.URL.Path, "duration", time.Since(start))
	})
}

func Choose(s, fallback string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func ChooseInt(i, fallback int) int {
	if i != 0 {
		return i
	}
	return fallback
}

func HeadN[T any](s []T, n int) []T {
	if n < 0 {
		n = 0
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func Prioritize(names []string, prefs []string) []string {
	if len(names) == 0 {
		return []string{} // namesが空なら、nilではなく空のスライスを返す
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range prefs {
		for _, n := range names {
			if !seen[n] && strings.Contains(strings.ToLower(n), p) {
				out = append(out, n)
				seen[n] = true
			}
		}
	}
	for _, n := range names {
		if !seen[n] {
			out = append(out, n)
		}
	}
	return out
}

func GetTimezone() string {
	// 2025-10-02T18:00:00%sの%sにあたる部分を生成する
	// 例: +09:00, -05:00, Z
	_, offset := time.Now().Zone()
	if offset == 0 {
		return "Z"
	}
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return sign + twoDigitString(hours) + ":" + twoDigitString(minutes)
}

func twoDigitString(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
