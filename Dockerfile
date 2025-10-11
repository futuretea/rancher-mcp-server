FROM golang:latest AS builder

WORKDIR /app

COPY ./ ./
RUN make build

FROM registry.suse.com/bci/bci-minimal:15.7
WORKDIR /app
COPY --from=builder /app/rancher-mcp-server /app/rancher-mcp-server
USER 65532:65532
ENTRYPOINT ["/app/rancher-mcp-server", "--port", "8080"]

EXPOSE 8080
