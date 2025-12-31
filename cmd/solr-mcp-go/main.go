package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"solr-mcp-go/internal/client"
	"solr-mcp-go/internal/server"
)

var (
	host  = flag.String("host", "localhost", "host to connect to/listen on")
	port  = flag.Int("port", 9000, "port number to connect to/listen on")
	proto = flag.String("proto", "http", "if set, use as proto:// part of URL (ignored for server)")
)

func main() {
	// --- Setup Logger ---
	logLevel := new(slog.LevelVar)
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "INFO":
		logLevel.Set(slog.LevelInfo)
	case "WARN":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo) // Default LogLevel is INFO
	}

	// Custom attribute replacement function
	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		// If the value is a string and contains newlines
		if a.Value.Kind() == slog.KindString && strings.Contains(a.Value.String(), "\n") {
			lines := strings.Split(a.Value.String(), "\n")
			// Format multi-line strings as a slog.Group
			var groupAttrs []slog.Attr
			for i, line := range lines {
				// Skip empty lines in the log
				if strings.TrimSpace(line) != "" {
					groupAttrs = append(groupAttrs, slog.String(fmt.Sprintf("line%02d", i+1), line))
				}
			}

			// Convert []slog.Attr to []any
			anyAttrs := make([]any, len(groupAttrs))
			for i, attr := range groupAttrs {
				anyAttrs[i] = attr
			}

			return slog.Group(a.Key, anyAttrs...)
		}
		return a
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: replaceAttr,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, handlerOpts))
	slog.SetDefault(logger)
	// --- Setup complete ---

	out := flag.CommandLine.Output()
	flag.Usage = func() {
		fmt.Fprintf(out, "Usage: %s <client|server> [-proto <http|https>] [-port <port>] [-host <host>]\n\n", os.Args[0])
		fmt.Fprintf(out, "This program demonstrates MCP over HTTP using the streamable transport.\n")
		fmt.Fprintf(out, "It can run as either a server or client.\n\n")
		fmt.Fprintf(out, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(out, "\nExamples:\n")
		fmt.Fprintf(out, " Run as server: %s server\n", os.Args[0])
		fmt.Fprintf(out, " Run as client: %s client\n", os.Args[0])
		fmt.Fprintf(out, " Custom host/port: %s -port 9000 -host 0.0.0.0 server\n", os.Args[0])
		os.Exit(1)
	}
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(out, "Error: Must specify 'client' or 'server' as first argument\n")
		flag.Usage()
	}
	mode := flag.Arg(0)

	switch mode {
	case "server":
		addr := fmt.Sprintf("%s:%d", *host, *port)
		server.Run(addr)
	case "client":
		url := fmt.Sprintf("%s://%s:%d", *proto, *host, *port)
		client.Run(url)
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid mode '%s'. Must be 'client' or 'server'\n\n", mode)
		flag.Usage()
	}
}
