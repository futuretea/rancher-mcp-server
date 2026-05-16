# Contributing to Rancher MCP Server

## Prerequisites

- [Go](https://go.dev/dl/) 1.26.0 or later
- [Git](https://git-scm.com/)

## Getting Started

```bash
git clone https://github.com/futuretea/rancher-mcp-server.git
cd rancher-mcp-server
```

## Building

```bash
make build
```

This produces the `rancher-mcp-server` binary in the current directory. For cross-platform builds:

```bash
make build-all-platforms
```

## Testing

```bash
make test
```

This runs all unit tests with `-count=1 -v ./...`.

## Linting

```bash
make lint
```

This runs [golangci-lint](https://golangci-lint.run/) with the project's configured linters. Format code with:

```bash
make format
```

## Running Locally

Build the binary, then run against a Rancher server:

```bash
make build
./rancher-mcp-server \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

You can also use the MCP Inspector for interactive debugging:

```bash
npx @modelcontextprotocol/inspector@latest $(pwd)/rancher-mcp-server
```

## Configuration

Copy `config.example.yaml` to `config.yaml` and edit as needed. See [README.md](README.md#configuration) for all options.

## Project Structure

```
cmd/rancher-mcp-server/   # CLI entry point
internal/
  client/                  # Rancher API clients (Steve, Norman)
  core/                    # Core logic and resource operations
  dep/                     # Resource dependency analysis (kube-lineage)
  server/                  # MCP server setup and tool registration
  toolset/                 # Toolset definitions (kubernetes, rancher)
  util/                    # Shared utilities
  watchdiff/               # Watch and diff logic
pkg/                       # Public packages (version info)
```

## Making Changes

1. Fork the repository and create a feature branch.
2. Make your changes. Keep each commit focused on a single logical change.
3. Run `make test` and `make lint` to verify nothing is broken.
4. Open a pull request describing the motivation, approach, and any compatibility notes.

## Pull Request Guidelines

- PRs should include test evidence — either new tests or confirmation that existing tests pass.
- If you're adding a new tool, include the tool documentation in the PR description so it can be added to the README.
- Breaking changes to the MCP tool interface (renamed parameters, removed tools, changed defaults) must be called out explicitly.
