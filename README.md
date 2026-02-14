# Rancher MCP Server

[![GitHub License](https://img.shields.io/github/license/futuretea/rancher-mcp-server)](https://github.com/futuretea/rancher-mcp-server/blob/main/LICENSE)
[![npm](https://img.shields.io/npm/v/@futuretea/rancher-mcp-server)](https://www.npmjs.com/package/@futuretea/rancher-mcp-server)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/futuretea/rancher-mcp-server?sort=semver)](https://github.com/futuretea/rancher-mcp-server/releases/latest)

[Features](#features) | [Getting Started](#getting-started) | [Configuration](#configuration) | [Tools](#tools-and-functionalities) | [Development](#development)

## Features <a id="features"></a>

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for Rancher multi-cluster management.

- **Multi-cluster Management**: Access multiple Kubernetes clusters through Rancher API
- **Kubernetes Resources via Steve API**: CRUD operations on any resource type
  - Get/List any resource (Pod, Deployment, Service, ConfigMap, Secret, CRD, etc.)
  - Create resources from JSON manifests
  - Patch resources using JSON Patch (RFC 6902)
  - Delete resources
  - Describe resources with related events (similar to `kubectl describe`)
  - List and filter Kubernetes events by namespace, object name, and object kind
  - Query container logs with filtering (tail lines, time range, timestamps, keyword search)
  - Inspect pods with parent workload, metrics, and logs
  - Show dependency/dependent trees for any resource (inspired by kube-lineage)
- **Rancher Resources via Norman API**: List clusters and projects
- **Security Controls**:
  - `read_only`: Disables create, patch, and delete operations
  - `disable_destructive`: Disables delete operations only
  - Secret data is never exposed, only metadata
- **Output Formats**: Table, YAML, and JSON
- **Output Filters**: Remove verbose fields like `managedFields` from responses
- **Pagination**: Limit and page parameters for list operations
- **Cross-platform**: Native binaries for Linux, macOS, Windows, and npm package

## Getting Started <a id="getting-started"></a>

### Requirements

- Access to a Rancher server
- Rancher API credentials (Token or Access Key/Secret Key)

### Claude Code

```shell
claude mcp add rancher -- npx @futuretea/rancher-mcp-server@latest \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

### VS Code / Cursor

Add to `.vscode/mcp.json` or `~/.cursor/mcp.json`:

```json
{
  "servers": {
    "rancher": {
      "command": "npx",
      "args": [
        "-y",
        "@futuretea/rancher-mcp-server@latest",
        "--rancher-server-url",
        "https://your-rancher-server.com",
        "--rancher-token",
        "your-token"
      ]
    }
  }
}
```

## Configuration <a id="configuration"></a>

Configuration can be set via CLI flags, environment variables, or a config file.

**Priority (highest to lowest):**
1. Command-line flags
2. Environment variables (prefix: `RANCHER_MCP_`)
3. Configuration file
4. Default values

### CLI Options

```shell
npx @futuretea/rancher-mcp-server@latest --help
```

| Option | Description | Default |
|--------|-------------|---------|
| `--config` | Config file path (YAML) | |
| `--port` | Port for HTTP/SSE mode (0 = stdio mode) | `0` |
| `--sse-base-url` | Public base URL for SSE endpoint | |
| `--log-level` | Log level (0-9) | `5` |
| `--rancher-server-url` | Rancher server URL | |
| `--rancher-token` | Rancher bearer token | |
| `--rancher-access-key` | Rancher access key | |
| `--rancher-secret-key` | Rancher secret key | |
| `--rancher-tls-insecure` | Skip TLS verification | `false` |
| `--read-only` | Disable write operations | `true` |
| `--disable-destructive` | Disable delete operations | `false` |
| `--list-output` | Output format (json, table, yaml) | `json` |
| `--output-filters` | Fields to remove from output | `metadata.managedFields` |
| `--toolsets` | Toolsets to enable | `kubernetes,rancher` |
| `--enabled-tools` | Specific tools to enable | |
| `--disabled-tools` | Specific tools to disable | |

### Configuration File

Create `config.yaml`:

```yaml
port: 0  # 0 for stdio, or set a port like 8080 for HTTP/SSE

log_level: 5

rancher_server_url: https://your-rancher-server.com
rancher_token: your-bearer-token
# Or use Access Key/Secret Key:
# rancher_access_key: your-access-key
# rancher_secret_key: your-secret-key
# rancher_tls_insecure: false

read_only: true  # default: true
disable_destructive: false

list_output: json

# Remove verbose fields from output
output_filters:
  - metadata.managedFields
  - metadata.annotations.kubectl.kubernetes.io/last-applied-configuration

toolsets:
  - kubernetes
  - rancher

# enabled_tools: []
# disabled_tools: []
```

### Environment Variables

Use `RANCHER_MCP_` prefix with underscores:

```shell
RANCHER_MCP_PORT=8080
RANCHER_MCP_RANCHER_SERVER_URL=https://rancher.example.com
RANCHER_MCP_RANCHER_TOKEN=your-token
RANCHER_MCP_READ_ONLY=true
```

### HTTP/SSE Mode

Run with a port number for network access:

```shell
rancher-mcp-server --port 8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

Endpoints:
- `/healthz` - Health check
- `/mcp` - Streamable HTTP endpoint
- `/sse` - Server-Sent Events endpoint
- `/message` - Message endpoint for SSE clients

With a public URL behind a proxy:

```shell
rancher-mcp-server --port 8080 \
  --sse-base-url https://your-domain.com:8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

## Tools and Functionalities <a id="tools-and-functionalities"></a>

Tools are organized into toolsets. Use `--toolsets` to enable specific sets or `--enabled-tools`/`--disabled-tools` for fine-grained control.

### Toolsets

| Toolset | API | Description |
|---------|-----|-------------|
| kubernetes | Steve | Kubernetes CRUD operations for any resource type |
| rancher | Norman | Cluster and project listing |

### kubernetes

<details>
<summary>kubernetes_dep</summary>

Show all dependencies or dependents of any Kubernetes resource as a tree. Covers OwnerReference chains, Pod→Node/SA/ConfigMap/Secret/PVC, Service→Pod (label selector), Ingress→IngressClass/Service/TLS Secret, PVC↔PV→StorageClass, RBAC bindings, PDB→Pod, and Events.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., deployment, pod, service, ingress, node) |
| `namespace` | string | No | Namespace (optional for cluster-scoped resources) |
| `name` | string | Yes | Resource name |
| `direction` | string | No | Traversal direction: `dependents` (default) or `dependencies` |
| `depth` | integer | No | Maximum traversal depth, 1-20 (default: 10) |
| `format` | string | No | Output format: tree, json (default: tree) |

</details>

<details>
<summary>kubernetes_get</summary>

Get a Kubernetes resource by kind, namespace, and name.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service) |
| `namespace` | string | No | Namespace (optional for cluster-scoped resources) |
| `name` | string | Yes | Resource name |
| `format` | string | No | Output format: json, yaml (default: json) |

</details>

<details>
<summary>kubernetes_list</summary>

List Kubernetes resources by kind.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `name` | string | No | Filter by name (partial match) |
| `labelSelector` | string | No | Label selector (e.g., "app=nginx,env=prod") |
| `limit` | integer | No | Items per page (default: 100) |
| `page` | integer | No | Page number, starting from 1 (default: 1) |
| `format` | string | No | Output format: json, table, yaml (default: json) |

</details>

<details>
<summary>kubernetes_logs</summary>

Get logs from a pod container.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | Yes | Namespace |
| `name` | string | Yes | Pod name |
| `container` | string | No | Container name (empty = all containers) |
| `tailLines` | integer | No | Lines from end (default: 100) |
| `sinceSeconds` | integer | No | Logs from last N seconds |
| `timestamps` | boolean | No | Include timestamps (default: false) |
| `previous` | boolean | No | Previous container instance (default: false) |
| `keyword` | string | No | Filter log lines containing this keyword (case-insensitive) |

</details>

<details>
<summary>kubernetes_inspect_pod</summary>

Get pod diagnostics: details, parent workload, metrics, and logs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | Yes | Namespace |
| `name` | string | Yes | Pod name |

</details>

<details>
<summary>kubernetes_describe</summary>

Describe a Kubernetes resource with its related events. Similar to `kubectl describe`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service, node) |
| `namespace` | string | No | Namespace (optional for cluster-scoped resources) |
| `name` | string | Yes | Resource name |
| `format` | string | No | Output format: json, yaml (default: json) |

</details>

<details>
<summary>kubernetes_events</summary>

List Kubernetes events. Supports filtering by namespace, involved object name, and kind.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `name` | string | No | Filter by involved object name |
| `kind` | string | No | Filter by involved object kind (e.g., Pod, Deployment, Node) |
| `limit` | integer | No | Events per page (default: 50) |
| `page` | integer | No | Page number, starting from 1 (default: 1) |
| `format` | string | No | Output format: json, table, yaml (default: table) |

</details>

<details>
<summary>kubernetes_create</summary>

Create a Kubernetes resource. Disabled when `read_only=true`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `resource` | string | Yes | JSON manifest (must include apiVersion, kind, metadata, spec) |

</details>

<details>
<summary>kubernetes_patch</summary>

Patch a resource using JSON Patch (RFC 6902). Disabled when `read_only=true`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind |
| `namespace` | string | No | Namespace (optional for cluster-scoped) |
| `name` | string | Yes | Resource name |
| `patch` | string | Yes | JSON Patch array, e.g., `[{"op":"replace","path":"/spec/replicas","value":3}]` |

</details>

<details>
<summary>kubernetes_delete</summary>

Delete a Kubernetes resource. Disabled when `read_only=true` or `disable_destructive=true`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind |
| `namespace` | string | No | Namespace (optional for cluster-scoped) |
| `name` | string | Yes | Resource name |

</details>

### rancher

<details>
<summary>cluster_list</summary>

List available Rancher clusters.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | No | Filter by cluster name (partial match) |
| `limit` | integer | No | Items per page (default: 100) |
| `page` | integer | No | Page number (default: 1) |
| `format` | string | No | Output format: json, table, yaml (default: json) |

</details>

<details>
<summary>project_list</summary>

List Rancher projects.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | No | Filter by cluster ID |
| `name` | string | No | Filter by project name (partial match) |
| `limit` | integer | No | Items per page (default: 100) |
| `page` | integer | No | Page number (default: 1) |
| `format` | string | No | Output format: json, table, yaml (default: json) |

</details>

## Development <a id="development"></a>

### Build

```shell
make build
```

### Run with mcp-inspector

```shell
npx @modelcontextprotocol/inspector@latest $(pwd)/rancher-mcp-server
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for more details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- [GitHub Issues](https://github.com/futuretea/rancher-mcp-server/issues)
- [Troubleshooting Guide](TROUBLESHOOTING.md)

## License

[Apache-2.0](LICENSE)
