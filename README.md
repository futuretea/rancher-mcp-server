# Rancher MCP Server

[![GitHub License](https://img.shields.io/github/license/futuretea/rancher-mcp-server)](https://github.com/futuretea/rancher-mcp-server/blob/main/LICENSE)
[![npm](https://img.shields.io/npm/v/@futuretea/rancher-mcp-server)](https://www.npmjs.com/package/@futuretea/rancher-mcp-server)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/futuretea/rancher-mcp-server?sort=semver)](https://github.com/futuretea/rancher-mcp-server/releases/latest)

[✨ Features](#features) | [🚀 Getting Started](#getting-started) | [⚙️ Configuration](#configuration) | [🛠️ Tools](#tools-and-functionalities) | [🧑‍💻 Development](#development)

## ✨ Features <a id="features"></a>

A powerful and flexible [Model Context Protocol (MCP)](https://blog.marcnuri.com/model-context-protocol-mcp-introduction) server implementation with support for **Rancher multi-cluster management**.

- **✅ Multi-cluster Management**: Access and manage multiple Kubernetes clusters through Rancher API
- **✅ Core Kubernetes Resources**: Perform operations on Kubernetes resources across multiple clusters
  - **List** clusters, nodes, workloads, and namespaces
  - **Get** kubeconfig for any cluster
- **✅ Rancher-specific Resources**: Access Rancher-specific resources
  - **List** Rancher projects and users
  - **Check** cluster health status
  - **View** project access permissions
- **✅ Security Configuration**: Support for read-only mode and disabling destructive operations
- **✅ Multiple Output Formats**: Support for table, YAML, and JSON output formats
- **✅ Cross-platform Support**: Available as native binaries for Linux, macOS, and Windows, as well as an npm package

Unlike other Kubernetes MCP server implementations, this server is **specifically designed for Rancher multi-cluster environments** and provides seamless access to multiple clusters through a single interface.

- **✅ Lightweight**: The server is distributed as a single native binary for Linux, macOS, and Windows
- **✅ High-Performance / Low-Latency**: Directly interacts with Rancher API server without the overhead of calling external commands
- **✅ Cross-Platform**: Available as a native binary for Linux, macOS, and Windows, as well as an npm package
- **✅ Configurable**: Supports [command-line arguments](#configuration) to configure the server behavior
- **✅ Well tested**: The server has an extensive test suite to ensure its reliability and correctness

## 🚀 Getting Started <a id="getting-started"></a>

### Requirements

- Access to a Rancher server
- Rancher API credentials (Token or Access Key/Secret Key)

### Claude Desktop

#### Using npx

If you have npm installed, this is the fastest way to get started with `@futuretea/rancher-mcp-server` on Claude Desktop.

Open your `claude_desktop_config.json` and add the mcp server to the list of `mcpServers`:

```json
{
  "mcpServers": {
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

### VS Code / VS Code Insiders

Install the Rancher MCP server extension in VS Code by running the following command:

```shell
# For VS Code
code --add-mcp '{"name":"rancher","command":"npx","args":["@futuretea/rancher-mcp-server@latest","--rancher-server-url","https://your-rancher-server.com","--rancher-token","your-token"]}'

# For VS Code Insiders
code-insiders --add-mcp '{"name":"rancher","command":"npx","args":["@futuretea/rancher-mcp-server@latest","--rancher-server-url","https://your-rancher-server.com","--rancher-token","your-token"]}'
```

### Cursor

Install the Rancher MCP server extension in Cursor by editing the `mcp.json` file:

```json
{
  "mcpServers": {
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

## ⚙️ Configuration <a id="configuration"></a>

The Rancher MCP server can be configured using command line (CLI) arguments or a configuration file.

You can run the CLI executable either by using `npx` or by downloading the [latest release binary](https://github.com/futuretea/rancher-mcp-server/releases/latest).

```shell
# Run the Rancher MCP server using npx (in case you have npm and node installed)
npx @futuretea/rancher-mcp-server@latest --help
```

```shell
# Run the Rancher MCP server using the latest release binary
./rancher-mcp-server --help
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `--port` | Starts the MCP server in HTTP/SSE mode and listens on the specified port. Use 0 for stdio mode (default for MCP clients) |
| `--sse-base-url` | SSE public base URL to use when sending the endpoint message (e.g. https://example.com) |
| `--log-level` | Sets the logging level (values from 0-9) |
| `--rancher-server-url` | URL of the Rancher server |
| `--rancher-token` | Bearer token for Rancher API authentication |
| `--rancher-access-key` | Access key for Rancher API authentication |
| `--rancher-secret-key` | Secret key for Rancher API authentication |
| `--list-output` | Output format for resource list operations (one of: table, yaml, json) (default "table") |
| `--read-only` | If set, the MCP server will run in read-only mode, meaning it will not allow any write operations on the Rancher cluster |
| `--disable-destructive` | If set, the MCP server will disable all destructive operations on the Rancher cluster |
| `--toolsets` | Comma-separated list of toolsets to enable. Check the [🛠️ Tools and Functionalities](#tools-and-functionalities) section for more information |

### Configuration File

Create a configuration file `config.yaml`:

```yaml
port: 0  # 0 for stdio mode, set to a port number (e.g., 8080) for HTTP/SSE mode
log_level: 0

# SSE (Server-Sent Events) configuration (optional, for HTTP/SSE mode)
# sse_base_url: https://your-domain.com:8080

rancher_server_url: https://your-rancher-server.com
rancher_token: your-bearer-token
# Or use Access Key/Secret Key:
# rancher_access_key: your-access-key
# rancher_secret_key: your-secret-key
list_output: table
read_only: false
disable_destructive: false
toolsets:
  - config
  - core
  - rancher
```

### HTTP/SSE Mode

The Rancher MCP server supports running in HTTP/SSE (Server-Sent Events) mode for network-based access. This is useful when you want to:

- Access the server from remote clients
- Use the server in a containerized environment
- Enable multiple clients to connect to the same server instance

#### Running in HTTP/SSE Mode

```shell
# Start the server on port 8080
rancher-mcp-server --port 8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

The server will expose the following endpoints:

- **`/healthz`** - Health check endpoint (returns 200 OK)
- **`/mcp`** - Streamable HTTP endpoint for MCP protocol
- **`/sse`** - Server-Sent Events endpoint for real-time communication
- **`/message`** - Message endpoint for SSE clients

#### Using SSE Base URL

When deploying behind a reverse proxy or load balancer, you can specify a public base URL:

```shell
rancher-mcp-server --port 8080 \
  --sse-base-url https://your-domain.com:8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

## 🛠️ Tools and Functionalities <a id="tools-and-functionalities"></a>

The Rancher MCP server supports enabling or disabling specific groups of tools and functionalities (toolsets) via the `--toolsets` command-line flag or `toolsets` configuration option.
This allows you to control which Rancher functionalities are available to your AI tools.
Enabling only the toolsets you need can help reduce the context size and improve the LLM's tool selection accuracy.

### Available Toolsets

The following sets of tools are available (all on by default):

| Toolset | Description |
|---------|-------------|
| config | Tools for managing cluster configuration (kubeconfig) |
| core | Core Kubernetes resource management tools (clusters, nodes, workloads, namespaces) |
| rancher | Rancher-specific tools (projects, users, cluster health, access control) |

### Tools

<details>
<summary>config</summary>

- **kubeconfig_get** - Get kubeconfig file for a cluster
  - `cluster_id` (`string`) **(required)** - ID of the cluster to get kubeconfig for

</details>

<details>
<summary>core</summary>

- **cluster_list** - List all available Kubernetes clusters
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **node_list** - List all nodes in a cluster
  - `cluster` (`string`) - Cluster ID to list nodes from (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **workload_list** - List workloads (deployments, statefulsets, daemonsets, jobs) and orphan pods in a cluster
  - `cluster` (`string`) **(required)** - Cluster ID
  - `project` (`string`) - Project ID to filter workloads (optional)
  - `namespace` (`string`) - Namespace name to filter workloads (optional)
  - `node` (`string`) - Node name to filter workloads (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **namespace_list** - List namespaces in a cluster
  - `cluster` (`string`) **(required)** - Cluster ID
  - `project` (`string`) - Project ID to filter namespaces (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

</details>

<details>
<summary>rancher</summary>

- **project_list** - List Rancher projects across clusters
  - `cluster` (`string`) - Filter projects by cluster ID (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **user_list** - List all Rancher users
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **cluster_health** - Get health status of Rancher clusters
  - `cluster` (`string`) - Specific cluster ID to check health (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

- **project_access** - List user access permissions for Rancher projects
  - `project` (`string`) - Project ID to check access (optional)
  - `cluster` (`string`) - Cluster ID (optional)
  - `format` (`string`) - Output format: table, yaml, or json (default: "table")

</details>

## 🧑‍💻 Development <a id="development"></a>

### Running with mcp-inspector

Compile the project and run the Rancher MCP server with [mcp-inspector](https://modelcontextprotocol.io/docs/tools/inspector) to inspect the MCP server.

```shell
# Compile the project
make build
# Run the Rancher MCP server with mcp-inspector
npx @modelcontextprotocol/inspector@latest $(pwd)/rancher-mcp-server
```

For more development information, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details on how to submit issues, feature requests, and pull requests.

### Development

For development setup and guidelines, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Support

If you encounter issues:
- Create a [GitHub Issue](https://github.com/futuretea/rancher-mcp-server/issues)
- Check the [troubleshooting guide](TROUBLESHOOTING.md)
- Review the [documentation](https://github.com/futuretea/rancher-mcp-server/docs)

## Community

- **GitHub**: [futuretea/rancher-mcp-server](https://github.com/futuretea/rancher-mcp-server)
- **Issues**: [GitHub Issues](https://github.com/futuretea/rancher-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/futuretea/rancher-mcp-server/discussions)