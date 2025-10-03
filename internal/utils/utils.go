package utils

import (
    "log"
    "net/http"
    "strings"
    "time"
)

// LoggingHandler is a middleware that logs requests.
func LoggingHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("Started %s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        log.Printf("Completed %s in %v", r.URL.Path, time.Since(start))
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
    if len(s) <= n {
        return s
    }
    return s[:n]
}

func Prioritize(names []string, prefs []string) []string {
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