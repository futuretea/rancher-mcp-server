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
  - Multi-pod log aggregation via label selector with time-based sorting
  - View rollout history for Deployments
  - Analyze node health and resource usage
  - Inspect pods with parent workload, metrics, and logs
  - Show dependency/dependent trees for any resource (inspired by kube-lineage)
  - **Get all resources** (inspired by [ketall](https://github.com/corneliusweig/ketall)): List all Kubernetes resources including ConfigMaps, Secrets, RBAC, CRDs
  - **Compare resource versions** (kubernetes_diff): Show git-style diffs between two resource versions
  - **Watch resource changes** (kubernetes_watch): Monitor resources and return git-style diffs at regular intervals
  - **Resource capacity overview** (inspired by [kube-capacity](https://github.com/robscott/kube-capacity)): Show cluster resource capacity, requests, limits, and utilization
  - **Resource top ranking** (`kubernetes_top`): Rank pods or nodes by CPU/memory usage, requests, limits, or restart count
  - **Workload health summary** (`kubernetes_workload_health`): Health overview for Deployments, StatefulSets, and DaemonSets with ready/desired ratios and status derivation
  - **Resource summary by group** (`kubernetes_resource_summary`): Aggregate pod resources by namespace or label key with totals for requests/limits
  - **Event pattern analysis** (`kubernetes_event_summary`): Group and rank events by reason, kind, and frequency to identify recurring issues
- **Rancher Resources via Norman API**: List clusters and projects
- **Security Controls**:
  - `read_only`: Disables create, patch, and delete operations
  - `disable_destructive`: Disables delete operations only
  - `show_sensitive_data`: Global administrator control for sensitive data visibility (default: `false`)
    - When disabled (default): All sensitive data is masked with `***`
    - When enabled: Per-tool `showSensitiveData` parameter controls visibility
    - Applies to: Kubernetes Secret `data` and `stringData` fields
    - Affects tools: `kubernetes_get`, `kubernetes_list`, `kubernetes_describe`
  - `enable_container_exec`: Explicit opt-in for pod command execution (default: `false`, also requires `read_only=false`)
  - `enable_container_file_upload` / `enable_container_file_download`: Explicit opt-in for container file transfer tools
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
| `--show-sensitive-data` | Global admin flag to allow sensitive data visibility | `false` |
| `--enable-container-exec` | Enable pod command execution tool; requires `--read-only=false` | `false` |
| `--enable-container-file-upload` | Enable container file upload tool | `false` |
| `--enable-container-file-download` | Enable container file download tool | `false` |
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

# High-risk container operations are disabled by default.
# enable_container_exec requires read_only: false.
enable_container_exec: false
enable_container_file_upload: false
enable_container_file_download: false

# Sensitive Data Control:
# Global administrator setting that controls whether sensitive data can be shown.
# - false (default): All sensitive data is always masked with '***'
# - true: Allows per-tool showSensitiveData parameter to control visibility
# Applies to Kubernetes Secret data and stringData fields.
show_sensitive_data: false

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
RANCHER_MCP_SHOW_SENSITIVE_DATA=false  # Global admin control for sensitive data
RANCHER_MCP_ENABLE_CONTAINER_EXEC=false
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

### Sensitive Data Protection

The server provides a two-tier security control for handling sensitive Kubernetes resources (currently Secrets):

#### Global Administrator Control

The `--show-sensitive-data` flag (default: `false`) is a global administrator setting that determines whether sensitive data can ever be revealed:

- **Disabled (default: `false`)**: All sensitive data is **always masked** with `***`, regardless of per-tool parameters
  - Secret `data` and `stringData` fields are masked
  - Provides maximum security by preventing any accidental data exposure
  - Recommended for production environments

- **Enabled (`true`)**: Allows per-tool `showSensitiveData` parameter to control visibility
  - Each tool call can choose whether to show or mask sensitive data
  - Useful for troubleshooting and administrative tasks
  - Requires explicit per-call parameter to reveal data

#### Per-Tool Parameter Control

When global `--show-sensitive-data` is enabled, tools that access sensitive resources accept a `showSensitiveData` parameter:

- `showSensitiveData: false` (default): Masks sensitive fields with `***`
- `showSensitiveData: true`: Shows actual values

**Affected Tools:**
- `kubernetes_get`: Get individual resources including Secrets
- `kubernetes_list`: List resources including Secrets
- `kubernetes_describe`: Describe resources with events

**Example Behavior:**

```yaml
# Global flag disabled (--show-sensitive-data=false)
# Secret data is ALWAYS masked, regardless of per-tool parameter
apiVersion: v1
kind: Secret
data:
  password: "***"  # Always masked
  token: "***"     # Always masked

# Global flag enabled (--show-sensitive-data=true)
# Per-tool parameter controls visibility:

# With showSensitiveData: false (default)
apiVersion: v1
kind: Secret
data:
  password: "***"  # Masked
  token: "***"     # Masked

# With showSensitiveData: true
apiVersion: v1
kind: Secret
data:
  password: "<base64-encoded-value>"  # Actual base64 value shown
  token: "<base64-encoded-value>"     # Actual base64 value shown
```

**Configuration Examples:**

```shell
# Maximum security (production recommended)
rancher-mcp-server --show-sensitive-data=false  # or omit (default)

# Allow administrators to reveal data when needed
rancher-mcp-server --show-sensitive-data=true
```

```yaml
# config.yaml
show_sensitive_data: false  # Production: always mask
# show_sensitive_data: true  # Development: allow per-tool control
```

```shell
# Environment variable
RANCHER_MCP_SHOW_SENSITIVE_DATA=false
```

Tools are organized into toolsets. Use `--toolsets` to enable specific sets or `--enabled-tools`/`--disabled-tools` for fine-grained control.

### High-Risk Container Operations

Container exec and file mutation tools are disabled by default. To expose `kubernetes_exec`, administrators must set `read_only: false` and enable `enable_container_exec`. The tool accepts only an argv-style command array and does not support stdin or TTY sessions.

```json
{
  "cluster": "c-abc123",
  "namespace": "default",
  "name": "api-7f8d8",
  "container": "api",
  "command": ["printenv", "HOSTNAME"]
}
```

The response is JSON with `exitCode`, `stdout`, and `stderr`. A non-zero command exit is returned as a structured result; validation and transport failures are returned as tool errors.

### Toolsets

| Toolset | API | Description |
|---------|-----|-------------|
| kubernetes | Steve | Kubernetes CRUD operations for any resource type |
| rancher | Norman | Cluster and project listing |

### kubernetes

<details>
<summary>kubernetes_capacity</summary>

Show Kubernetes cluster resource capacity, requests, limits, and utilization. Similar to [kube-capacity](https://github.com/robscott/kube-capacity) CLI tool.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `pods` | boolean | No | Include individual pod resources in the output (default: false) |
| `containers` | boolean | No | Include individual container resources in the output (implies pods=true) (default: false) |
| `util` | boolean | No | Include actual resource utilization from metrics-server (requires metrics-server) (default: false) |
| `available` | boolean | No | Show raw available capacity instead of percentages (default: false) |
| `podCount` | boolean | No | Include pod counts for each node and the whole cluster (default: false) |
| `showLabels` | boolean | No | Include node labels in the output (default: false) |
| `hideRequests` | boolean | No | Hide request columns from output (default: false) |
| `hideLimits` | boolean | No | Hide limit columns from output (default: false) |
| `namespace` | string | No | Filter by namespace (empty for all namespaces) |
| `labelSelector` | string | No | Filter pods by label selector (e.g., "app=nginx,env=prod") |
| `nodeLabelSelector` | string | No | Filter nodes by label selector (e.g., "node-role.kubernetes.io/worker=true") |
| `namespaceLabelSelector` | string | No | Filter namespaces by label selector (e.g., "env=production") |
| `nodeTaints` | string | No | Filter nodes by taints. Use 'key=value:effect' to include, 'key=value:effect-' to exclude. Multiple taints can be separated by comma |
| `noTaint` | boolean | No | Exclude nodes with any taints (default: false) |
| `sortBy` | string | No | Sort by: cpu.util, mem.util, cpu.request, mem.request, cpu.limit, mem.limit, cpu.util.percentage, mem.util.percentage, name |
| `format` | string | No | Output format: table, json, yaml (default: table) |

**Examples:**

```json
// Basic node overview
{
  "cluster": "c-abc123"
}

// Include pods detail
{
  "cluster": "c-abc123",
  "pods": true
}

// Include utilization metrics (requires metrics-server)
{
  "cluster": "c-abc123",
  "util": true
}

// Full detail with utilization
{
  "cluster": "c-abc123",
  "pods": true,
  "util": true
}

// Filter by namespace
{
  "cluster": "c-abc123",
  "namespace": "production"
}

// Sort by CPU utilization
{
  "cluster": "c-abc123",
  "sortBy": "cpu.util"
}

// Filter by node labels (show only worker nodes)
{
  "cluster": "c-abc123",
  "nodeLabelSelector": "node-role.kubernetes.io/worker=true"
}

// Include container-level details
{
  "cluster": "c-abc123",
  "containers": true
}

// Show pod counts per node
{
  "cluster": "c-abc123",
  "podCount": true
}

// Show node labels in output
{
  "cluster": "c-abc123",
  "showLabels": true
}

// Hide request columns (show only limits)
{
  "cluster": "c-abc123",
  "hideRequests": true
}

// Filter by namespace labels
{
  "cluster": "c-abc123",
  "namespaceLabelSelector": "env=production"
}

// Filter by node taints (include nodes with specific taint)
{
  "cluster": "c-abc123",
  "nodeTaints": "dedicated=special:NoSchedule"
}

// Exclude nodes with specific taint
{
  "cluster": "c-abc123",
  "nodeTaints": "dedicated=special:NoSchedule-"
}

// Exclude all tainted nodes
{
  "cluster": "c-abc123",
  "noTaint": true
}

// Sort by CPU utilization percentage
{
  "cluster": "c-abc123",
  "sortBy": "cpu.util.percentage"
}
```

</details>

<details>
<summary>kubernetes_top</summary>

Rank pods or nodes by resource usage, requests, limits, or restart count. Supports metrics-server utilization data with fallback to requests/limits when metrics are unavailable.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | No | Resource kind to rank: `pod` or `node` (default: `pod`) |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `labelSelector` | string | No | Label selector for filtering (e.g., "app=nginx,env=prod") |
| `sortBy` | string | No | Sort by field. Pods: `cpu.util`, `mem.util`, `cpu.request`, `mem.request`, `cpu.limit`, `mem.limit`, `restart.count`. Nodes: `cpu.util`, `mem.util`, `cpu.util.percentage`, `mem.util.percentage`, `pod.count` |
| `limit` | integer | No | Maximum results to return (default: 50, max: 500) |
| `format` | string | No | Output format: `json`, `table`, `yaml` (default: `table`) |

**Examples:**

```json
// Top pods by CPU utilization
{
  "cluster": "c-abc123",
  "kind": "pod",
  "sortBy": "cpu.util",
  "limit": 20
}

// Top pods by restart count across a namespace
{
  "cluster": "c-abc123",
  "kind": "pod",
  "namespace": "production",
  "sortBy": "restart.count",
  "limit": 10
}

// Top nodes by memory utilization percentage
{
  "cluster": "c-abc123",
  "kind": "node",
  "sortBy": "mem.util.percentage",
  "limit": 10
}
```

</details>

<details>
<summary>kubernetes_workload_health</summary>

Get a health summary for Deployments, StatefulSets, and DaemonSets. Shows ready vs desired replicas, unavailable count, update progress, and derived status.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `kind` | string | No | Workload kind: `deployment`, `statefulset`, `daemonset`, or `all` (default: `all`) |
| `labelSelector` | string | No | Label selector for filtering |
| `sortBy` | string | No | Sort by: `unready.count`, `ready.ratio`, `name` |
| `limit` | integer | No | Maximum results (default: 50, max: 500) |
| `format` | string | No | Output format: `json`, `table`, `yaml` (default: `table`) |

**Examples:**

```json
// All workloads sorted by unready count
{
  "cluster": "c-abc123",
  "sortBy": "unready.count"
}

// Deployment health in a namespace
{
  "cluster": "c-abc123",
  "namespace": "production",
  "kind": "deployment",
  "sortBy": "ready.ratio"
}
```

</details>

<details>
<summary>kubernetes_resource_summary</summary>

Aggregate pod/container resources by namespace or label key. Returns total requests, limits, and pod counts per group.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | No | Namespace filter (empty = all namespaces) |
| `labelSelector` | string | No | Label selector for filtering pods |
| `groupBy` | string | No | Group by: `namespace` or `label` (default: `namespace`) |
| `groupByKey` | string | No | Label key to group by (required when `groupBy=label`) |
| `sortBy` | string | No | Sort by: `cpu.request`, `mem.request`, `cpu.limit`, `mem.limit`, `pod.count`, `name` |
| `limit` | integer | No | Maximum results (default: 50, max: 500) |
| `format` | string | No | Output format: `json`, `table`, `yaml` (default: `table`) |

**Examples:**

```json
// Resource summary by namespace
{
  "cluster": "c-abc123",
  "groupBy": "namespace",
  "sortBy": "cpu.request"
}

// Resource summary by app label in production
{
  "cluster": "c-abc123",
  "namespace": "production",
  "groupBy": "label",
  "groupByKey": "app",
  "sortBy": "cpu.request"
}
```

</details>

<details>
<summary>kubernetes_event_summary</summary>

Group and rank Kubernetes events by reason, kind, and frequency. Useful for identifying recurring issues and patterns.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `kind` | string | No | Filter by involved object kind (e.g., Pod, Deployment, Node) |
| `type` | string | No | Filter by event type: `Warning` or `Normal` |
| `since` | string | No | Only include events newer than this duration (e.g., "1h30m", "2h") |
| `sortBy` | string | No | Sort by: `count`, `lastSeen`, `name` |
| `limit` | integer | No | Maximum results (default: 50, max: 500) |
| `format` | string | No | Output format: `json`, `table`, `yaml` (default: `table`) |

**Examples:**

```json
// Top warning events in the last hour
{
  "cluster": "c-abc123",
  "type": "Warning",
  "since": "1h",
  "sortBy": "count",
  "limit": 10
}

// Recent events for a specific kind
{
  "cluster": "c-abc123",
  "kind": "Pod",
  "since": "30m",
  "sortBy": "lastSeen"
}
```

</details>

<details>
<summary>kubernetes_dep</summary>

Show all dependencies or dependents of any Kubernetes resource as a tree. Covers OwnerReference chains, Pod→Node/SA/ConfigMap/Secret/PVC, Service→Pod (label selector), Ingress→IngressClass/Service/TLS Secret, PVC↔PV→StorageClass, RBAC bindings, PDB→Pod, and Events.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., deployment, pod, service, ingress, node, App) |
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
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
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service, App) |
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
| `namespace` | string | No | Namespace (optional for cluster-scoped resources) |
| `name` | string | Yes | Resource name |
| `format` | string | No | Output format: json, yaml (default: json) |
| `showSensitiveData` | boolean | No | Show sensitive data values (e.g., Secret data). Default: false. Only takes effect when global `--show-sensitive-data` is enabled. When global setting is disabled, data is always masked with `***` |

</details>

<details>
<summary>kubernetes_list</summary>

List Kubernetes resources by kind.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service, App) |
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
| `namespace` | string | No | Namespace (empty = all namespaces) |
| `name` | string | No | Filter by name (partial match) |
| `labelSelector` | string | No | Label selector (e.g., "app=nginx,env=prod") |
| `limit` | integer | No | Items per page (default: 100) |
| `page` | integer | No | Page number, starting from 1 (default: 1) |
| `format` | string | No | Output format: json, table, yaml (default: json) |
| `showSensitiveData` | boolean | No | Show sensitive data values (e.g., Secret data). Default: false. Only takes effect when global `--show-sensitive-data` is enabled. When global setting is disabled, data is always masked with `***` |

CRDs can use their manifest identity directly:

```json
{
  "cluster": "c-abc123",
  "apiVersion": "catalog.cattle.io/v1",
  "kind": "App",
  "namespace": "cattle-system"
}
```

</details>

<details>
<summary>kubernetes_logs</summary>

Get logs from a pod container. Supports multi-pod log aggregation via label selector with time-based sorting.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | Yes | Namespace |
| `name` | string | No | Pod name (required if labelSelector not specified) |
| `labelSelector` | string | No | Label selector for multi-pod log aggregation (e.g., "app=nginx") |
| `container` | string | No | Container name (empty = all containers) |
| `tailLines` | integer | No | Lines from end (default: 100) |
| `sinceSeconds` | integer | No | Logs from last N seconds |
| `timestamps` | boolean | No | Include timestamps (default: true) |
| `previous` | boolean | No | Previous container instance (default: false) |
| `keyword` | string | No | Filter log lines containing this keyword (case-insensitive) |

**Notes:**
- When `labelSelector` is specified, logs from all matching pods are aggregated and sorted by timestamp
- Output format for single pod: `[container] timestamp content`
- Output format for multi-pod: `[pod/container] timestamp content`

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
<summary>kubernetes_rollout_history</summary>

View rollout history for Deployments. Shows revision history with change annotations (similar to `kubectl rollout history`).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | Yes | Namespace |
| `name` | string | Yes | Deployment name |

</details>

<details>
<summary>kubernetes_node_analysis</summary>

Analyze node health and resource usage. Shows node capacity, allocatable resources, pod distribution, and identifies potential issues.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `name` | string | No | Node name (if empty, analyzes all nodes) |

</details>

<details>
<summary>kubernetes_describe</summary>

Describe a Kubernetes resource with its related events. Similar to `kubectl describe`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service, node, App) |
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
| `namespace` | string | No | Namespace (optional for cluster-scoped resources) |
| `name` | string | Yes | Resource name |
| `format` | string | No | Output format: json, yaml (default: json) |
| `showSensitiveData` | boolean | No | Show sensitive data values (e.g., Secret data). Default: false. Only takes effect when global `--show-sensitive-data` is enabled. When global setting is disabled, data is always masked with `***` |

</details>

<details>
<summary>kubernetes_diff</summary>

Compare two Kubernetes resource versions and show the differences as a git-style diff. Useful for comparing current vs desired state, or before/after changes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `resource1` | string | Yes | First resource version as JSON string (the 'before' or 'old' version). Use kubernetes_get to retrieve the resource. |
| `resource2` | string | Yes | Second resource version as JSON string (the 'after' or 'new' version). Use kubernetes_get to retrieve the resource. |
| `ignoreStatus` | boolean | No | Ignore changes under the status field when computing diffs (default: false) |
| `ignoreMeta` | boolean | No | Ignore non-essential metadata differences like managedFields, resourceVersion, etc. (default: false) |

**Examples:**

```json
// Compare two versions of the same deployment
// First, get the current resource
{
  "cluster": "c-abc123",
  "kind": "deployment",
  "namespace": "default",
  "name": "nginx"
}
// Then compare with previous version (from rollout history)
{
  "resource1": "<previous-revision-json>",
  "resource2": "<current-revision-json>",
  "ignoreMeta": true
}
```

</details>

<details>
<summary>kubernetes_get_all</summary>

Get really all Kubernetes resources in the cluster (inspired by [ketall](https://github.com/corneliusweig/ketall)). Unlike `kubectl get all`, this shows all resource types including ConfigMaps, Secrets, RBAC resources, CRDs, and other resources that are normally hidden.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `namespace` | string | No | Filter by namespace (optional, empty for all namespaces) |
| `name` | string | No | Filter by resource name (partial match, client-side) |
| `labelSelector` | string | No | Label selector for filtering (e.g., "app=nginx,env=prod") |
| `excludeEvents` | boolean | No | Exclude events from output (default: true, as events are often noisy) |
| `scope` | string | No | Filter by scope: 'namespaced' for namespaced resources only, 'cluster' for cluster-scoped resources only, or empty for all |
| `since` | string | No | Only show resources created since this duration (e.g., '1h30m', '2d', '1w') |
| `limit` | integer | No | Limit number of resources per API call (0 for no limit, default: 0) |
| `format` | string | No | Output format: json, table, yaml (default: table) |

**Examples:**

```json
// Get all resources in the cluster
{
  "cluster": "c-abc123"
}

// Get all resources in a specific namespace
{
  "cluster": "c-abc123",
  "namespace": "production"
}

// Get only cluster-scoped resources
{
  "cluster": "c-abc123",
  "scope": "cluster"
}

// Get resources created in the last 24 hours
{
  "cluster": "c-abc123",
  "since": "24h"
}

// Get all resources with specific labels
{
  "cluster": "c-abc123",
  "labelSelector": "app=nginx,env=prod"
}
```

</details>

<details>
<summary>kubernetes_watch</summary>

Watch Kubernetes resources and return git-style diffs of changes at regular intervals, similar to the Linux `watch` command.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | Cluster ID |
| `kind` | string | Yes | Resource kind (e.g., pod, deployment, service, App) |
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
| `namespace` | string | No | Namespace (empty = all namespaces or cluster-scoped resources) |
| `labelSelector` | string | No | Label selector (e.g., "app=nginx,env=prod") |
| `fieldSelector` | string | No | Field selector for filtering resources |
| `ignoreStatus` | boolean | No | Ignore changes under the `status` field when computing diffs (similar to `--no-status`) |
| `ignoreMeta` | boolean | No | Ignore non-essential metadata differences (similar to `--no-meta`) |
| `intervalSeconds` | integer | No | Interval in seconds between evaluations (default: 10, min: 1, max: 600) |
| `iterations` | integer | No | Number of times to re-evaluate and diff before returning (default: 6, min: 1, max: 100) |

**Notes:**
- Each iteration compares the current resource state with the previous iteration and only emits diffs when there are changes.
- The tool returns the concatenated diffs for all iterations in a single response.

**Examples:**

```json
// Watch pods in a namespace for changes
{
  "cluster": "c-abc123",
  "kind": "pod",
  "namespace": "production",
  "intervalSeconds": 5,
  "iterations": 3
}

// Watch deployments and ignore status changes
{
  "cluster": "c-abc123",
  "kind": "deployment",
  "namespace": "default",
  "ignoreStatus": true,
  "intervalSeconds": 10,
  "iterations": 6
}

// Watch resources by label selector
{
  "cluster": "c-abc123",
  "kind": "pod",
  "labelSelector": "app=nginx",
  "intervalSeconds": 5,
  "iterations": 12
}
```

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
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
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
| `apiVersion` | string | No | API version for CRDs or ambiguous kinds (e.g., catalog.cattle.io/v1) |
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
