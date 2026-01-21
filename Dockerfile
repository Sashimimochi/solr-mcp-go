# Build stage
FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o solr-mcp-go ./cmd/solr-mcp-go

# Runtime stage
FROM alpine:latest

# Metadata labels
LABEL org.opencontainers.image.title="solr-mcp-go" \
  org.opencontainers.image.description="Model Context Protocol (MCP) server for Apache Solr - enables AI assistants to query and interact with Solr collections" \
  org.opencontainers.image.authors="Your Name" \
  org.opencontainers.image.url="https://github.com/Sashimimochi/solr-mcp-go" \
  org.opencontainers.image.source="https://github.com/Sashimimochi/solr-mcp-go" \
  org.opencontainers.image.documentation="https://github.com/Sashimimochi/solr-mcp-go#readme" \
  org.opencontainers.image.version="1.0.0" \
  org.opencontainers.image.licenses="MIT"

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/solr-mcp-go .

# Create a non-root user
RUN addgroup -g 1000 appuser && \
  adduser -D -u 1000 -G appuser appuser && \
  chown -R appuser:appuser /app

USER appuser

# Expose default port
EXPOSE 9000

# Environment variables (with defaults shown in documentation)
# SOLR_MCP_SOLR_URL: Solr server URL (default: http://localhost:8983)
# SOLR_MCP_DEFAULT_COLLECTION: Default collection name (default: gettingstarted)
# SOLR_BASIC_USER: Solr basic auth username (optional)
# SOLR_BASIC_PASS: Solr basic auth password (optional)
# LOG_LEVEL: Logging level - DEBUG, INFO, WARN, ERROR (default: INFO)

# Run the application
ENTRYPOINT ["/app/solr-mcp-go"]
# Default command arguments: -host 0.0.0.0 -port 9000
# Override with: docker run image-name -host 0.0.0.0 -port 8000
