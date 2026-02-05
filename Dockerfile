# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 go build -o rancher-mcp-server ./cmd/rancher-mcp-server

# Final stage
FROM cgr.dev/chainguard/wolfi-base:latest AS runtime

# Create non-root user
RUN adduser -D -s /bin/sh rancher

USER rancher

ENTRYPOINT ["/usr/local/bin/rancher-mcp-server"]

# Release image
FROM runtime AS release

COPY rancher-mcp-server /usr/local/bin/rancher-mcp-server

# Dev image
FROM runtime AS dev

# Copy the binary from builder
COPY --from=builder /build/rancher-mcp-server /usr/local/bin/rancher-mcp-server
