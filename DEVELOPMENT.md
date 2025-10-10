# Development Guide

## Development Environment Setup

### Prerequisites

- Go 1.24.1 or higher
- Git
- Access to Rancher server for testing

### Project Setup

```bash
# Clone the repository
git clone https://github.com/futuretea/rancher-mcp-server.git
cd rancher-mcp-server

# Install dependencies
go mod download
go mod vendor

# Build the project
go build -o rancher-mcp-server ./cmd/rancher-mcp-server
```

## Architecture Overview

### Core Components

1. **MCP Server** (`pkg/mcp/`)
   - Handles MCP protocol communication
   - Manages tool registration and invocation

2. **Rancher Client** (`pkg/rancher/`)
   - Encapsulates Rancher API calls
   - Provides unified client interface

3. **Toolsets** (`pkg/toolsets/`)
   - Implements specific functional tools
   - Organized by functional modules

4. **Configuration Management** (`pkg/config/`)
   - Handles configuration file loading and validation
   - Provides default configuration

### Toolset Architecture

Each toolset must implement the `api.Toolset` interface:

```go
type Toolset interface {
    GetName() string
    GetDescription() string
    GetTools(client interface{}) []ServerTool
}
```

## Adding New Tools

### Step 1: Add Tool to Existing Toolset

In the appropriate toolset package:

```go
// Add new tool in GetTools method
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
    return []api.ServerTool{
        {
            Tool: mcp.Tool{
                Name:        "your_new_tool",
                Description: "Description of your new tool",
                InputSchema: mcp.ToolInputSchema{
                    Type: "object",
                    Properties: map[string]any{
                        "parameter": map[string]any{
                            "type":        "string",
                            "description": "Parameter description",
                        },
                    },
                },
            },
            Annotations: api.ToolAnnotations{
                ReadOnlyHint: boolPtr(true),  // If read-only operation
            },
            Handler: yourToolHandler,
        },
    }
}

// Implement tool handler
func yourToolHandler(client interface{}, params map[string]interface{}) (string, error) {
    // Tool logic implementation
    return "result", nil
}
```

### Step 2: Create New Toolset

If you need to create a new toolset:

1. Create new directory under `pkg/toolsets/`
2. Implement `api.Toolset` interface
3. Register in toolset registry

## Rancher Client Usage

### Basic Usage

```go
import "github.com/futuretea/rancher-mcp-server/pkg/rancher"

// In tool handler
rancherClient, ok := client.(rancher.Client)
if !ok || !rancherClient.IsConfigured() {
    return "", fmt.Errorf("Rancher client not configured")
}

// Use client
clusters, err := rancherClient.ListClusters(ctx)
```

### Available Methods

- `ListClusters(ctx context.Context) ([]Cluster, error)`
- `ListNodes(ctx context.Context, clusterID string) ([]Node, error)`
- `ListPods(ctx context.Context, clusterID, projectID string) ([]Pod, error)`
- `ListNamespaces(ctx context.Context, clusterID string) ([]Namespace, error)`
- `GenerateKubeconfig(ctx context.Context, clusterID string) (string, error)`
- `ListProjects(ctx context.Context, clusterID string) ([]Project, error)`
- `ListUsers(ctx context.Context) ([]User, error)`

## Testing

### Unit Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/config/...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Test Maintenance

- Ensure all tests pass before submitting changes
- Update tests when default configuration values change
- Maintain test coverage for critical components
- Add tests for new tools and features

### Integration Testing

Integration tests require access to a real Rancher server. Create test configuration file:

```yaml
rancher_server_url: https://test-rancher-server.com
rancher_token: test-token
```

Run integration tests:

```bash
go test -tags=integration ./...
```

## Debugging

### Log Levels

Set different log levels to debug issues:

```bash
./rancher-mcp-server --log-level 5
```

### Debug Mode

During development, use HTTP mode for debugging:

```bash
./rancher-mcp-server --port 8080 --log-level 5
```

Then use curl or other HTTP clients to test tools.

## Code Standards

### Go Code Standards

- Use `gofmt` for code formatting
- Follow official Go code style
- Add comments for public functions and methods
- Avoid deprecated functions like `strings.Title` - use `golang.org/x/text/cases` instead

### Error Handling

- Use `fmt.Errorf` to wrap errors
- Provide meaningful error messages
- Return user-friendly error messages in tool handlers

### Security Considerations

- Validate all input parameters
- Disable write operations in read-only mode
- Use appropriate permission checks

### Project Structure

```
├── cmd/rancher-mcp-server/     # Main program entry
├── pkg/
│   ├── config/                 # Configuration management
│   ├── mcp/                   # MCP protocol implementation
│   ├── rancher/               # Rancher client
│   ├── toolsets/              # Toolset implementations
│   │   ├── core/              # Core Kubernetes tools
│   │   ├── config/            # Configuration management tools
│   │   └── rancher/           # Rancher-specific tools
│   └── output/                # Output formatting
├── npm/                       # npm publishing configuration
└── vendor/                    # Dependencies
```

## Building and Publishing

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all-platforms
```

### Publishing to npm

Publishing is automated through GitHub Actions. When you push a tag, the release workflow will:
- Build binaries for all platforms
- Create a GitHub release with binaries
- Publish to npm automatically

Manual publishing (if needed):

```bash
# Prepare for npm publishing
make npm-copy-binaries

# Publish to npm (requires NPM_TOKEN)
export NPM_TOKEN=your-npm-token
make npm-publish
```

## Release Process

Releases are automated through GitHub Actions:

1. Ensure all tests pass: `go test ./...`
2. Update version information if needed
3. Update CHANGELOG.md
4. Create and push a git tag:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
5. GitHub Actions will automatically:
   - Build binaries for all platforms
   - Create a GitHub release
   - Publish to npm (requires NPM_TOKEN secret)

### GitHub Actions Workflows

- **Build Workflow** (`.github/workflows/build.yaml`): Runs on every push to main and PRs, builds and tests on multiple platforms
- **Release Workflow** (`.github/workflows/release.yaml`): Triggers on tag push, builds all platforms and publishes to GitHub and npm

## Common Issues

### Dependency Issues

If you encounter dependency issues:

```bash
go mod tidy
go mod vendor
```

### Rancher API Changes

If Rancher API changes, you need to update:

1. Check `pkg/rancher/client.go` and the generated client packages
2. Update relevant client methods
3. Run tests to ensure compatibility

### MCP Protocol Issues

If you encounter MCP protocol issues:

1. Check MCP client logs
2. Validate tool definitions and input schemas
3. Ensure return format complies with MCP specification