FROM golang:latest AS builder

WORKDIR /app

COPY ./ ./
RUN make build

FROM registry.suse.com/bci/bci-minimal:15.7
WORKDIR /app
COPY --from=builder /app/rancher-mcp-server /app/rancher-mcp-server
ENTRYPOINT ["/app/rancher-mcp-server"]
