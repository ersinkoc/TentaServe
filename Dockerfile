# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies (if needed)
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/tentaserve \
    ./cmd/tentaserve

# Runtime stage
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /bin/tentaserve /tentaserve

# Expose default port
EXPOSE 8080

# Run as non-root (requires Go 1.22+ for user:group support in scratch)
USER 65534:65534

# Default config path (can be overridden)
ENV TENTASERVE_CONFIG=/config/tentaserve.yaml

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/tentaserve", "version"] || exit 1

ENTRYPOINT ["/tentaserve"]
CMD ["serve", "--config", "/config/tentaserve.yaml"]
