# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

RUN apk add --no-cache bash make

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Reuse the shared build contract instead of duplicating ldflags in Docker.
RUN make build BUILD_DATE="${BUILD_DATE}" GIT_VERSION="${VERSION}" GIT_COMMIT="${GIT_COMMIT}"

# Final stage
FROM cgr.dev/chainguard/wolfi-base:latest AS runtime

# Create non-root user
RUN adduser -D -s /bin/sh rancher

USER rancher

ENTRYPOINT ["/usr/local/bin/rancher-mcp-server"]

# Release image
FROM runtime AS goreleaser-release

# This target expects a prebuilt rancher-mcp-server binary in the Docker context.
COPY rancher-mcp-server /usr/local/bin/rancher-mcp-server

# Local image
FROM runtime AS local

# This target is the supported direct docker build path from the repository root.
COPY --from=builder /build/rancher-mcp-server /usr/local/bin/rancher-mcp-server

# Dev image
FROM local AS dev
