# Rancher MCP Server

[English](README.md) | [中文](README.zh-CN.md)

[![GitHub License](https://img.shields.io/github/license/futuretea/rancher-mcp-server)](https://github.com/futuretea/rancher-mcp-server/blob/main/LICENSE)
[![npm](https://img.shields.io/npm/v/@futuretea/rancher-mcp-server)](https://www.npmjs.com/package/@futuretea/rancher-mcp-server)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/futuretea/rancher-mcp-server?sort=semver)](https://github.com/futuretea/rancher-mcp-server/releases/latest)

[功能特性](#features) | [快速开始](#getting-started) | [配置](#configuration) | [工具](#tools-and-functionalities) | [开发](#development)

## 功能特性 <a id="features"></a>

面向 Rancher 多集群管理的 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) 服务器。

- **多集群管理**：通过 Rancher API 或已配置的 kubeconfig 路径访问 Kubernetes 集群
- **通过 Steve API 操作 Kubernetes 资源**：对任意资源类型执行 CRUD
  - 获取/列出任意资源（Pod、Deployment、Service、ConfigMap、Secret、CRD 等）
  - 通过 JSON 清单创建资源
  - 使用 JSON Patch（RFC 6902）修补资源
  - 删除资源
  - 描述资源及其关联事件（类似 `kubectl describe`）
  - 按命名空间、对象名称和对象类型列出并筛选 Kubernetes 事件
  - 查询容器日志并支持过滤（尾部行数、时间范围、时间戳、关键词搜索）
  - 通过标签选择器聚合多 Pod 日志并按时间排序
  - 查看 Deployment 的滚动更新历史
  - 分析节点健康状态与资源使用情况
  - 检查 Pod，包含父级工作负载、指标和日志
  - 展示任意资源的依赖/被依赖树（灵感来自 kube-lineage）
  - **获取全部资源**（灵感来自 [ketall](https://github.com/corneliusweig/ketall)）：列出所有 Kubernetes 资源，包括 ConfigMap、Secret、RBAC、CRD
  - **比较资源版本**（kubernetes_diff）：以 git 风格 diff 展示两个资源版本之间的差异
  - **监视资源变更**（kubernetes_watch）：定期监视资源并返回 git 风格 diff
  - **资源容量概览**（灵感来自 [kube-capacity](https://github.com/robscott/kube-capacity)）：展示集群资源容量、requests、limits 及利用率
  - **资源 Top 排行**（`kubernetes_top`）：按 CPU/内存使用量、requests、limits 或重启次数对 Pod 或节点排序
  - **工作负载健康摘要**（`kubernetes_workload_health`）：Deployment、StatefulSet、DaemonSet 的健康概览，含就绪/期望副本比及状态推导
  - **按组汇总资源**（`kubernetes_resource_summary`）：按命名空间或标签键聚合 Pod 资源，汇总 requests/limits 总量
  - **事件模式分析**（`kubernetes_event_summary`）：按 reason、kind 和频率分组排序事件，识别重复出现的问题
- **通过 Norman API 操作 Rancher 资源**：列出集群和项目
- **安全控制**：
  - `read_only`：禁用创建、修补和删除操作
  - `disable_destructive`：仅禁用删除操作
  - `show_sensitive_data`：敏感数据可见性的全局管理员控制（默认：`false`）
    - 禁用时（默认）：所有敏感数据以 `***` 遮蔽
    - 启用时：由各工具的 `showSensitiveData` 参数控制可见性
    - 适用范围：Kubernetes Secret 的 `data` 和 `stringData` 字段
    - 影响的工具：`kubernetes_get`、`kubernetes_list`、`kubernetes_describe`
  - `enable_container_exec`：显式启用 Pod 命令执行（默认：`false`，且需要 `read_only=false`）
  - `enable_container_file_upload` / `enable_container_file_download`：显式启用容器文件传输工具
- **输出格式**：Table、YAML、JSON
- **输出过滤**：从响应中移除 `managedFields` 等冗长字段
- **分页**：列表操作支持 limit 和 page 参数
- **跨平台**：提供 Linux、macOS、Windows 原生二进制及 npm 包

## 快速开始 <a id="getting-started"></a>

### 前置要求

至少配置一种集群访问方式：

- Rancher 访问：可访问的 Rancher 服务器和 Rancher API 凭据（Token 或 Access Key/Secret Key）
- 直连 Kubernetes：一个或多个 kubeconfig 路径

配置了 kubeconfig 路径时，Rancher 访问和凭据均为可选项。

### Claude Code

```shell
claude mcp add rancher -- npx -y @futuretea/rancher-mcp-server@latest \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

`--` 之后的参数会原样写入 MCP 配置。`-y` 标志告知 `npx` 在不提示的情况下安装依赖，非交互式 MCP 启动需要此标志。等效的 JSON 配置见 [VS Code / Cursor](#vs-code--cursor) 章节。

### VS Code / Cursor <a id="vs-code--cursor"></a>

添加到 `.vscode/mcp.json` 或 `~/.cursor/mcp.json`：

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

## 配置 <a id="configuration"></a>

可通过 CLI 标志、环境变量或配置文件进行配置。

**优先级（从高到低）：**
1. 命令行标志
2. 环境变量（前缀：`RANCHER_MCP_`）
3. 配置文件
4. 默认值

### CLI 选项

```shell
npx @futuretea/rancher-mcp-server@latest --help
```

| Option | Description | Default |
|--------|-------------|---------|
| `--config` | 配置文件路径（YAML） | |
| `--port` | HTTP/SSE 模式端口（0 = stdio 模式） | `0` |
| `--sse-base-url` | SSE 端点的公开基础 URL | |
| `--log-level` | 日志级别（0-9） | `5` |
| `--rancher-server-url` | Rancher 服务器 URL | |
| `--rancher-token` | Rancher bearer token | |
| `--rancher-access-key` | Rancher access key | |
| `--rancher-secret-key` | Rancher secret key | |
| `--rancher-tls-insecure` | 跳过 TLS 验证 | `false` |
| `--kubeconfig-paths` | 用于直连 Kubernetes 集群的 kubeconfig 文件 | |
| `--rancher-request-token-auth` | 使用每个 HTTP/SSE 请求的 `Authorization: Bearer <token>` 头代替静态凭据；仅当 Authorization 缺失时才使用非空的原始 `R_token` 头 | `false` |
| `--rancher-oauth-token-auth` | 在 `/mcp` 调用 Rancher 前校验 Rancher OAuth Bearer token | `false` |
| `--rancher-oauth-authorization-server-url` | Rancher OAuth 授权服务器根 URL 与 JWT issuer；不要追加 `/authorize` | |
| `--rancher-oauth-jwks-url` | Rancher OAuth JWKS URL，启动时加载并自动刷新 | |
| `--rancher-oauth-resource-url` | 用于发现元数据与 401 挑战的公开资源根 URL | |
| `--read-only` | 禁用写操作 | `true` |
| `--disable-destructive` | 禁用删除操作 | `false` |
| `--show-sensitive-data` | 全局管理员标志，允许显示敏感数据 | `false` |
| `--enable-container-exec` | 启用 Pod 命令执行工具；需要 `--read-only=false` | `false` |
| `--enable-container-file-upload` | 启用容器文件上传工具 | `false` |
| `--enable-container-file-download` | 启用容器文件下载工具 | `false` |
| `--max-file-size` | 容器文件操作的最大文件大小 | `10Mi` |
| `--list-output` | 保留的兼容设置；当前工具处理逻辑忽略该值 | `json` |
| `--output-filters` | 从输出中移除的字段 | `metadata.managedFields` |
| `--toolsets` | 要启用的工具集 | `kubernetes,rancher` |
| `--enabled-tools` | 要启用的特定工具 | |
| `--disabled-tools` | 要禁用的特定工具 | |

### 配置文件

创建 `config.yaml`：

```yaml
port: 0  # 0 for stdio, or set a port like 8080 for HTTP/SSE

log_level: 5

rancher_server_url: https://your-rancher-server.com

# 认证方式：选择以下其中一种。

# 方式 1：静态 Bearer Token
rancher_token: your-bearer-token
# 或使用 Access Key/Secret Key：
# rancher_access_key: your-access-key
# rancher_secret_key: your-secret-key

# 方式 2：每请求 token（仅 HTTP/SSE 模式）
# 启用后，服务端会从每个 HTTP/SSE 请求的 Authorization: Bearer <token> 头中读取 token
# 并转发给 Rancher；仅当 Authorization 缺失时，才使用非空的原始 R_token 头。
# 存在但为空或格式错误的 Authorization 头不会回退到 R_token。
# 此时上方静态凭据必须为空。
# rancher_request_token_auth: true

# 方式 3：Rancher OAuth token 透传（仅 Streamable HTTP /mcp）
# 客户端在本服务之外获取 Rancher access token；服务端在调用 Rancher 前校验该
# Bearer JWT，再把校验通过的 token 透传给 Rancher API。不要与方式 1 或方式 2 组合使用。
# rancher_oauth_token_auth: true
# rancher_oauth_authorization_server_url: https://rancher.example.com/oidc
# 服务端在启动时加载 JWKS，并自动刷新。
# rancher_oauth_jwks_url: https://rancher.example.com/oidc/.well-known/jwks.json
# rancher_oauth_resource_url: https://mcp.example.com

# rancher_tls_insecure: false

# 可选：直连 Kubernetes 集群来源。每个 context 会暴露为
# kubeconfig:<context>；使用 cluster_list 查看可用 ID。
# 多个文件定义同名 context 时，排在最前的文件优先。
# kubeconfig_paths:
#   - /etc/rancher-mcp/kubeconfig
#   - /etc/rancher-mcp/extra-kubeconfig

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

必须显式指定该文件启动服务，它不会被自动发现：

```shell
rancher-mcp-server --config ./config.yaml
```

### 环境变量

使用 `RANCHER_MCP_` 前缀，单词间用下划线连接：

```shell
RANCHER_MCP_PORT=8080
RANCHER_MCP_RANCHER_SERVER_URL=https://rancher.example.com
RANCHER_MCP_RANCHER_TOKEN=your-token
RANCHER_MCP_KUBECONFIG_PATHS=/etc/rancher-mcp/kubeconfig,/etc/rancher-mcp/extra-kubeconfig
RANCHER_MCP_READ_ONLY=true
RANCHER_MCP_SHOW_SENSITIVE_DATA=false  # Global admin control for sensitive data
RANCHER_MCP_ENABLE_CONTAINER_EXEC=false
```

`RANCHER_MCP_KUBECONFIG_PATHS` 使用逗号分隔的有序列表，对应
`kubeconfig_paths`。多个文件定义相同 context 时，排在前面的路径优先。

Rancher OAuth token 透传使用同一前缀配置 OAuth 设置：

```shell
RANCHER_MCP_RANCHER_OAUTH_TOKEN_AUTH=true
RANCHER_MCP_RANCHER_OAUTH_AUTHORIZATION_SERVER_URL=https://rancher.example.com/oidc
RANCHER_MCP_RANCHER_OAUTH_JWKS_URL=https://rancher.example.com/oidc/.well-known/jwks.json
RANCHER_MCP_RANCHER_OAUTH_RESOURCE_URL=https://mcp.example.com
```

### Kubeconfig 集群来源

`kubeconfig_paths` 可直接使用 Kubernetes context，不经过 Rancher。文件按配置
顺序加载；若多个文件定义相同 context，以第一个文件为准。运行 `cluster_list`
可获取 `kubeconfig:production` 这类 ID，并将其传给 Kubernetes 工具。

`cluster_list` 的结果包含 `source` 列：Rancher 集群为 `rancher`，直连 context
为 `kubeconfig`，且直连行的 Rancher 专属字段会保持为空。`project_list` 仍仅支持
Rancher 集群，并会拒绝 `kubeconfig:` ID。

不要将 `kubeconfig_paths` 与 `rancher_request_token_auth` 或
`rancher_oauth_token_auth` 组合使用：前者使用服务进程的 kubeconfig 凭据，后两者
使用调用者范围的 Rancher token。在 HTTP/SSE 模式（`port > 0`）下，本服务不会为
kubeconfig 请求提供 HTTP 鉴权。请优先使用 stdio，或仅向可信调用者开放网络。

对于使用 `npx` 的 MCP 客户端，把同样的参数放在包名之后：
`npx -y @futuretea/rancher-mcp-server@latest --config ./config.yaml`。

### HTTP/SSE 模式

指定端口号以启用网络访问：

```shell
rancher-mcp-server --port 8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

端点：
- `/healthz` - 健康检查
- `/mcp` - 可流式 HTTP 端点
- `/sse` - Server-Sent Events 端点
- `/message` - SSE 客户端消息端点

在 Rancher OAuth token 透传模式下，MCP 传输路由为 `/mcp` 与
`/.well-known/oauth-protected-resource`；该模式下 `/sse` 与 `/message` 返回
`404`，`/healthz` 与 `/debug/vars` 仍可用。

在代理后使用公开 URL：

```shell
rancher-mcp-server --port 8080 \
  --sse-base-url https://your-domain.com:8080 \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

### 每请求 Rancher Token 认证

在 HTTP/SSE 模式下，如果上游网关已对用户完成认证，可启用 `--rancher-request-token-auth`，使服务端不再存储静态 Rancher 凭据。存在 `Authorization: Bearer <token>` 头时使用该头；否则使用非空的原始 `R_token` 头访问 Rancher API。

要求：
- 仅限 HTTP/SSE 模式（`--port` 必须大于 `0`；与 stdio 模式互斥）
- 上游网关或代理必须在每次请求 `/mcp`、`/sse`、`/message` 时转发 `Authorization` 或 `R_token`；存在但为空或格式错误的 Authorization 头会阻止 `R_token` 回退
- 不能与 `--rancher-token`、`--rancher-access-key`、`--rancher-secret-key` 同时使用

示例：

```shell
rancher-mcp-server --port 8080 \
  --rancher-request-token-auth \
  --rancher-server-url https://your-rancher-server.com
```

#### 网关示例

上游网关必须转发所选的请求 token 头。以下最小示例使用 Authorization：

**nginx：**

```nginx
location ~ ^/(mcp|sse|message)$ {
    proxy_pass http://rancher-mcp-server:8080;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

该路由保留原始请求路径，并覆盖每请求 token 认证所需的全部传输路由。

**Traefik ForwardAuth：**

```yaml
http:
  middlewares:
    rancher-mcp-auth:
      forwardAuth:
        address: "https://auth.example.com/verify"
        authResponseHeaders:
          - Authorization
```

仅当认证服务返回合法的 `Authorization: Bearer <Rancher token>` 响应头时才使用该配置；
Traefik 会把该响应头复制到上游请求。如果原始请求已携带 Rancher Bearer token，
不要用静态头覆盖它。

### Rancher OAuth Token 透传认证

Rancher OAuth token 透传是静态凭据与每请求 token 之外的另一种可选方式。MCP 客户端
在本服务之外获取 Rancher access token，并以 `Authorization: Bearer <token>` 发送到
`/mcp`；服务端在构造 Rancher 客户端之前，会校验它的 RS256 签名、issuer、带十秒
容差的时间声明，以及 `offline_access` 与 `rancher:mcp` scope。audience 不校验，
expiration 也不是必需。无效或缺失的 token 会收到 `401 Unauthorized` 与
`WWW-Authenticate` 资源元数据挑战，且不会发起任何 Rancher API 调用。

仅在 Streamable HTTP 模式下使用：

```shell
rancher-mcp-server --port 8080 \
  --rancher-server-url https://rancher.example.com \
  --rancher-oauth-token-auth \
  --rancher-oauth-authorization-server-url https://rancher.example.com/oidc \
  --rancher-oauth-jwks-url https://rancher.example.com/oidc/.well-known/jwks.json \
  --rancher-oauth-resource-url https://mcp.example.com
```

要求与限制：

- 必须提供 `--rancher-oauth-token-auth` 与三个 OAuth URL，且必须提供 Rancher
  服务器 URL。OAuth 模式不能与静态凭据或 `--rancher-request-token-auth` 组合，
  在 stdio 模式下会被拒绝。
- 已移除的 audience YAML 键及其派生环境变量不会被新增拒绝；当前配置加载会忽略
  未知的历史输入。
- 资源 URL 必须是公开根 URL，不支持挂载在子路径下的部署。服务端不校验 URL 格式、
  URL 路径或 HTTPS，这些由部署方自行保证。
- OAuth 模式只注册 `/mcp` 与 `/.well-known/oauth-protected-resource` 作为 MCP
  传输路由。它不提供 OAuth SSE 或会话主体状态，因此 `/sse` 与 `/message` 返回
  `404`；`/healthz` 与 `/debug/vars` 仍可用。
- 本服务从不读取、转发、存储或记录 `R_SESS` 等 Rancher 浏览器 Cookie，也不实现
  授权码交换、token 交换、refresh token 或动态客户端注册。
- 服务端在启动时加载 JWKS，若其中没有可用的 RS256 签名校验密钥则启动失败。JWKS
  每小时刷新一次；遇到未知 key ID 可触发一次额外刷新，频率限制为每五分钟一次。
  刷新失败或结果不可用时保留最后一次可用的密钥集；刷新成功后，授权服务器不再
  发布的密钥会被立即移除。
- 这是一个明确接受的 Rancher token 透传模式：它提供与参考实现兼容的元数据与挑战，
  但不声称兼容通用 MCP OAuth 或 RFC 9728 互操作。

### Rancher 版本支持

各认证模式与各 Rancher 版本的对应关系：

| Rancher | 静态凭据 | `--rancher-request-token-auth` | `--rancher-oauth-token-auth` |
|---|---|---|---|
| 2.11 及更早 | 支持 | 支持 | 不可用：Rancher 没有 OIDC provider |
| 2.12 - 2.13 | 支持 | 支持 | 不可用：Rancher API 拒绝 OIDC access token（JWT 认证由 [rancher/rancher#53016](https://github.com/rancher/rancher/pull/53016) 在 2.14 引入） |
| 2.14 - 2.15 | 支持 | 支持 | 支持，需满足下面的前置条件 |

2.14 及以后版本的 OAuth 透传前置条件：

- 必须启用 `oidc-provider` feature。它仅在 Rancher Prime 上默认开启；社区版需要
  手动开启，开启后 Rancher 会重启一次。
- `OIDCClient` 必须允许本服务要求的 scope。在默认的必需 scope 集合下，客户端需要
  `offline_access` 与 `rancher:mcp`，而 Rancher 只有当 `rancher:mcp` 出现在
  `OIDCClient.spec.scopes` 中时才会签发它。默认配置的客户端只签发 `openid`、
  `profile` 和 `offline_access`，会被本服务拒绝。
- 必须配置 `server-url` 设置。否则签发的 token 携带相对路径 `/oidc` 作为 issuer，
  任何外部校验方都无法匹配。

2.11 及更早与 2.12 两行基于 Rancher 源码分析；2.13.3、2.14.3、2.15.1 的行为由
[测试](#测试) 章节所述的集成测试覆盖。

## 工具与功能 <a id="tools-and-functionalities"></a>

### 敏感数据保护

服务器对 Secret 资源采用两级安全模型：

1. **全局标志** `--show-sensitive-data`（默认：`false`）：禁用时，无论各工具参数如何，所有 Secret 的 `data` 和 `stringData` 字段**始终**以 `***` 遮蔽。启用时，允许按工具控制。
2. **工具级参数** `showSensitiveData`（默认：`false`）：仅在全局标志启用时生效，控制单次调用的可见性。

**受影响的工具：** `kubernetes_get`、`kubernetes_list`、`kubernetes_describe`。

配置示例见[配置](#configuration)章节。

```yaml
# --show-sensitive-data=false (default): always masked
apiVersion: v1
kind: Secret
data:
  password: "***"

# --show-sensitive-data=true + showSensitiveData: true
apiVersion: v1
kind: Secret
data:
  password: "<base64-encoded-value>"
```

工具按工具集组织。使用 `--toolsets` 启用特定集合，或使用 `--enabled-tools`/`--disabled-tools` 进行细粒度控制。

### 高风险容器操作

容器执行与文件传输工具默认禁用，必须显式启用：

| Tool | Gate | Requires `read_only=false` |
|------|------|---------------------------|
| `kubernetes_exec` | `--enable-container-exec` | Yes |
| `kubernetes_upload_file` | `--enable-container-file-upload` | No |
| `kubernetes_download_file` | `--enable-container-file-download` | No |

`kubernetes_exec` 接受 argv 风格的命令数组（不支持 stdin 和 TTY），返回 `exitCode`、`stdout` 和 `stderr`。文件传输工具要求容器内存在 `tar`，并受 `--max-file-size` 限制（默认：10Mi）。

完整参数文档见 [kubernetes 工具章节](#kubernetes)。

### 工具集

| Toolset | API | Description |
|---------|-----|-------------|
| kubernetes | Steve | 对任意资源类型执行 Kubernetes CRUD 操作 |
| rancher | Norman | 列出集群和项目 |

### kubernetes

<details>
<summary>kubernetes_capacity</summary>

展示 Kubernetes 集群资源容量、requests、limits 及利用率。类似 [kube-capacity](https://github.com/robscott/kube-capacity) CLI 工具。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `pods` | boolean | No | 在输出中包含各 Pod 资源（默认：false） |
| `containers` | boolean | No | 在输出中包含各容器资源（隐含 pods=true）（默认：false） |
| `util` | boolean | No | 包含 metrics-server 的实际资源利用率（需要 metrics-server）（默认：false） |
| `available` | boolean | No | 显示原始可用容量而非百分比（默认：false） |
| `podCount` | boolean | No | 包含各节点及整个集群的 Pod 数量（默认：false） |
| `showLabels` | boolean | No | 在输出中包含节点标签（默认：false） |
| `hideRequests` | boolean | No | 从输出中隐藏 request 列（默认：false） |
| `hideLimits` | boolean | No | 从输出中隐藏 limit 列（默认：false） |
| `namespace` | string | No | 按命名空间过滤（空表示所有命名空间） |
| `labelSelector` | string | No | 按标签选择器过滤 Pod（例如："app=nginx,env=prod"） |
| `nodeLabelSelector` | string | No | 按标签选择器过滤节点（例如："node-role.kubernetes.io/worker=true"） |
| `namespaceLabelSelector` | string | No | 按标签选择器过滤命名空间（例如："env=production"） |
| `nodeTaints` | string | No | 按污点过滤节点。使用 'key=value:effect' 包含，'key=value:effect-' 排除。多个污点可用逗号分隔 |
| `noTaint` | boolean | No | 排除带有任何污点的节点（默认：false） |
| `sortBy` | string | No | 排序字段：cpu.util、mem.util、cpu.request、mem.request、cpu.limit、mem.limit、cpu.util.percentage、mem.util.percentage、name |
| `format` | string | No | 输出格式：table、json、yaml（默认：table） |

**示例：**

```json
// Basic node overview with utilization
{
  "cluster": "c-abc123",
  "util": true
}

// Include pod and container detail, sorted by CPU utilization
{
  "cluster": "c-abc123",
  "pods": true,
  "containers": true,
  "util": true,
  "sortBy": "cpu.util"
}

// Filter by namespace and show pod counts
{
  "cluster": "c-abc123",
  "namespace": "production",
  "podCount": true
}

// Filter by node and namespace labels
{
  "cluster": "c-abc123",
  "nodeLabelSelector": "node-role.kubernetes.io/worker=true",
  "namespaceLabelSelector": "env=production"
}

// Filter by taints (include or exclude)
{
  "cluster": "c-abc123",
  "nodeTaints": "dedicated=special:NoSchedule",
  "noTaint": false
}
```

</details>

<details>
<summary>kubernetes_top</summary>

按资源使用量、requests、limits 或重启次数对 Pod 或节点排序。支持 metrics-server 利用率数据，在指标不可用时回退到 requests/limits。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | No | 要排序的资源类型：`pod` 或 `node`（默认：`pod`） |
| `namespace` | string | No | 命名空间（空 = 所有命名空间） |
| `labelSelector` | string | No | 标签选择器过滤（例如："app=nginx,env=prod"） |
| `sortBy` | string | No | 排序字段。Pod：`cpu.util`、`mem.util`、`cpu.request`、`mem.request`、`cpu.limit`、`mem.limit`、`restart.count`。Node：`cpu.util`、`mem.util`、`cpu.util.percentage`、`mem.util.percentage`、`pod.count` |
| `limit` | integer | No | 最大返回结果数（默认：50，最大：500） |
| `format` | string | No | 输出格式：`json`、`table`、`yaml`（默认：`table`） |

**示例：**

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

获取 Deployment、StatefulSet、DaemonSet 的健康摘要。展示就绪与期望副本数、不可用数量、更新进度及推导状态。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | No | 命名空间（空 = 所有命名空间） |
| `kind` | string | No | 工作负载类型：`deployment`、`statefulset`、`daemonset` 或 `all`（默认：`all`） |
| `labelSelector` | string | No | 标签选择器过滤 |
| `sortBy` | string | No | 排序字段：`unready.count`、`ready.ratio`、`name` |
| `limit` | integer | No | 最大结果数（默认：50，最大：500） |
| `format` | string | No | 输出格式：`json`、`table`、`yaml`（默认：`table`） |

**示例：**

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

按命名空间或标签键聚合 Pod/容器资源。返回各组的 requests、limits 总量及 Pod 数量。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | No | 命名空间过滤（空 = 所有命名空间） |
| `labelSelector` | string | No | 标签选择器过滤 Pod |
| `groupBy` | string | No | 分组方式：`namespace` 或 `label`（默认：`namespace`） |
| `groupByKey` | string | No | 按标签键分组（`groupBy=label` 时必填） |
| `sortBy` | string | No | 排序字段：`cpu.request`、`mem.request`、`cpu.limit`、`mem.limit`、`pod.count`、`name` |
| `limit` | integer | No | 最大结果数（默认：50，最大：500） |
| `format` | string | No | 输出格式：`json`、`table`、`yaml`（默认：`table`） |

**示例：**

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

按 reason、kind 和频率分组排序 Kubernetes 事件。用于识别重复出现的问题和模式。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | No | 命名空间（空 = 所有命名空间） |
| `kind` | string | No | 按关联对象 kind 过滤（例如：Pod、Deployment、Node） |
| `type` | string | No | 按事件类型过滤：`Warning` 或 `Normal` |
| `since` | string | No | 仅包含比此时间跨度更新的事件（例如："1h30m"、"2h"） |
| `sortBy` | string | No | 排序字段：`count`、`lastSeen`、`name` |
| `limit` | integer | No | 最大结果数（默认：50，最大：500） |
| `format` | string | No | 输出格式：`json`、`table`、`yaml`（默认：`table`） |

**示例：**

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

以树形结构展示任意 Kubernetes 资源的所有依赖或被依赖关系。覆盖 OwnerReference 链、Pod→Node/SA/ConfigMap/Secret/PVC、Service→Pod（标签选择器）、Ingress→IngressClass/Service/TLS Secret、PVC↔PV→StorageClass、RBAC 绑定、PDB→Pod 及 Events。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind（例如：deployment、pod、service、ingress、node、App） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（集群级资源可选） |
| `name` | string | Yes | 资源名称 |
| `direction` | string | No | 遍历方向：`dependents`（默认）或 `dependencies` |
| `depth` | integer | No | 最大遍历深度，1-20（默认：10） |
| `format` | string | No | 输出格式：tree、json（默认：tree） |

</details>

<details>
<summary>kubernetes_get</summary>

按 kind、命名空间和名称获取 Kubernetes 资源。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind（例如：pod、deployment、service、App） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（集群级资源可选） |
| `name` | string | Yes | 资源名称 |
| `format` | string | No | 输出格式：json、yaml（默认：json） |
| `showSensitiveData` | boolean | No | 显示敏感数据值（例如 Secret data）。默认：false。仅在全局 `--show-sensitive-data` 启用时生效。全局设置禁用时，数据始终以 `***` 遮蔽 |

</details>

<details>
<summary>kubernetes_list</summary>

按 kind 列出 Kubernetes 资源。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind（例如：pod、deployment、service、App） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（空 = 所有命名空间） |
| `name` | string | No | 按名称过滤（部分匹配） |
| `labelSelector` | string | No | 标签选择器（例如："app=nginx,env=prod"） |
| `limit` | integer | No | 每页条目数（默认：100） |
| `page` | integer | No | 页码，从 1 开始（默认：1） |
| `format` | string | No | 输出格式：json、table、yaml（默认：json） |
| `showSensitiveData` | boolean | No | 显示敏感数据值（例如 Secret data）。默认：false。仅在全局 `--show-sensitive-data` 启用时生效。全局设置禁用时，数据始终以 `***` 遮蔽 |

CRD 可直接使用其清单标识：

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

获取 Pod 容器日志。支持通过标签选择器聚合多 Pod 日志并按时间排序。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | No | Pod 名称（未指定 labelSelector 时必填） |
| `labelSelector` | string | No | 多 Pod 日志聚合的标签选择器（例如："app=nginx"） |
| `container` | string | No | 容器名称（空 = 所有容器） |
| `tailLines` | integer | No | 从末尾获取的行数（默认：100） |
| `sinceSeconds` | integer | No | 最近 N 秒内的日志 |
| `timestamps` | boolean | No | 包含时间戳（默认：true） |
| `previous` | boolean | No | 上一个容器实例（默认：false） |
| `keyword` | string | No | 过滤包含此关键词的日志行（不区分大小写） |

**说明：**
- 指定 `labelSelector` 时，所有匹配 Pod 的日志会聚合并按时间戳排序
- 单 Pod 输出格式：`[container] timestamp content`
- 多 Pod 输出格式：`[pod/container] timestamp content`

</details>

<details>
<summary>kubernetes_inspect_pod</summary>

获取 Pod 诊断信息：详情、父级工作负载、指标和日志。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | Yes | Pod 名称 |

</details>

<details>
<summary>kubernetes_rollout_history</summary>

查看 Deployment 的滚动更新历史。展示修订历史及变更注解（类似 `kubectl rollout history`）。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | Yes | Deployment 名称 |
| `format` | string | No | 输出格式：json、table（默认：table） |

</details>

<details>
<summary>kubernetes_node_analysis</summary>

分析节点健康状态与资源使用情况。展示节点容量、可分配资源、Pod 分布，并识别潜在问题。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `name` | string | No | 节点名称（为空时分析所有节点） |
| `format` | string | No | 输出格式：json、yaml（默认：json） |

</details>

<details>
<summary>kubernetes_describe</summary>

描述 Kubernetes 资源及其关联事件。类似 `kubectl describe`。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind（例如：pod、deployment、service、node、App） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（集群级资源可选） |
| `name` | string | Yes | 资源名称 |
| `format` | string | No | 输出格式：json、yaml（默认：json） |
| `showSensitiveData` | boolean | No | 显示敏感数据值（例如 Secret data）。默认：false。仅在全局 `--show-sensitive-data` 启用时生效。全局设置禁用时，数据始终以 `***` 遮蔽 |

</details>

<details>
<summary>kubernetes_diff</summary>

比较两个 Kubernetes 资源版本，以 git 风格 diff 展示差异。适用于比较当前与期望状态，或变更前后。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `resource1` | string | Yes | 第一个资源版本的 JSON 字符串（"before" 或 "old" 版本）。使用 kubernetes_get 获取资源。 |
| `resource2` | string | Yes | 第二个资源版本的 JSON 字符串（"after" 或 "new" 版本）。使用 kubernetes_get 获取资源。 |
| `ignoreStatus` | boolean | No | 计算 diff 时忽略 status 字段下的变更（默认：false） |
| `ignoreMeta` | boolean | No | 忽略 managedFields、resourceVersion 等非必要元数据差异（默认：false） |

**示例：**

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
<summary>kubernetes_resource_diff</summary>

比较同一 kind 的两个资源，包括不同集群或命名空间中的资源。返回 git 风格 diff。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `kind` | string | Yes | 资源 kind（例如：deployment、daemonset、statefulset） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：`apps/v1`） |
| `left` | object | Yes | 左侧资源：`cluster` 是集群引用（Rancher ID 或 `kubeconfig:<context>`）；`name` 必填，`namespace` 可选 |
| `right` | object | Yes | 右侧资源：`cluster` 是集群引用（Rancher ID 或 `kubeconfig:<context>`）；`name` 必填，`namespace` 可选 |
| `ignoreStatus` | boolean | No | 忽略 `status` 下的变更（默认：false） |
| `ignoreMeta` | boolean | No | 忽略非必要元数据差异（默认：true） |

</details>

<details>
<summary>kubernetes_get_all</summary>

获取集群中真正的全部 Kubernetes 资源（灵感来自 [ketall](https://github.com/corneliusweig/ketall)）。与 `kubectl get all` 不同，此工具展示所有资源类型，包括 ConfigMap、Secret、RBAC 资源、CRD 等通常被隐藏的资源。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | No | 按命名空间过滤（可选，空表示所有命名空间） |
| `name` | string | No | 按资源名称过滤（部分匹配，客户端侧） |
| `labelSelector` | string | No | 标签选择器过滤（例如："app=nginx,env=prod"） |
| `excludeEvents` | boolean | No | 从输出中排除事件（默认：true，因事件通常较嘈杂） |
| `scope` | string | No | 按作用域过滤：'namespaced' 仅命名空间级资源，'cluster' 仅集群级资源，空表示全部 |
| `since` | string | No | 仅显示此时间跨度内创建的资源（例如：'1h30m'、'2d'、'1w'） |
| `limit` | integer | No | 每次 API 调用的资源数量限制（0 表示无限制，默认：0） |
| `format` | string | No | 输出格式：json、table、yaml（默认：table） |

**示例：**

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

监视 Kubernetes 资源，定期返回变更的 git 风格 diff，类似 Linux `watch` 命令。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind（例如：pod、deployment、service、App） |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（空 = 所有命名空间或集群级资源） |
| `labelSelector` | string | No | 标签选择器（例如："app=nginx,env=prod"） |
| `fieldSelector` | string | No | 字段选择器过滤资源 |
| `ignoreStatus` | boolean | No | 计算 diff 时忽略 `status` 字段下的变更（类似 `--no-status`） |
| `ignoreMeta` | boolean | No | 忽略非必要元数据差异（类似 `--no-meta`） |
| `intervalSeconds` | integer | No | 每次评估之间的间隔秒数（默认：10，最小：1，最大：600） |
| `iterations` | integer | No | 重新评估并 diff 的次数后返回（默认：6，最小：1，最大：100） |

**说明：**
- 每次迭代将当前资源状态与上一次迭代比较，仅在有变更时输出 diff。
- 工具在单次响应中返回所有迭代的拼接 diff。

**示例：**

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

列出 Kubernetes 事件。支持按命名空间、关联对象名称和 kind 过滤。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | No | 命名空间（空 = 所有命名空间） |
| `name` | string | No | 按关联对象名称过滤 |
| `kind` | string | No | 按关联对象 kind 过滤（例如：Pod、Deployment、Node） |
| `limit` | integer | No | 每页事件数（默认：50） |
| `page` | integer | No | 页码，从 1 开始（默认：1） |
| `format` | string | No | 输出格式：json、table、yaml（默认：table） |

</details>

<details>
<summary>kubernetes_create</summary>

创建 Kubernetes 资源。`read_only=true` 时禁用。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `resource` | string | Yes | JSON 清单（必须包含 apiVersion、kind、metadata、spec） |

</details>

<details>
<summary>kubernetes_patch</summary>

使用 JSON Patch（RFC 6902）修补资源。`read_only=true` 时禁用。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（集群级资源可选） |
| `name` | string | Yes | 资源名称 |
| `patch` | string | Yes | JSON Patch 数组，例如：`[{"op":"replace","path":"/spec/replicas","value":3}]` |

</details>

<details>
<summary>kubernetes_delete</summary>

删除 Kubernetes 资源。`read_only=true` 或 `disable_destructive=true` 时禁用。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `kind` | string | Yes | 资源 kind |
| `apiVersion` | string | No | CRD 或歧义 kind 的 API 版本（例如：catalog.cattle.io/v1） |
| `namespace` | string | No | 命名空间（集群级资源可选） |
| `name` | string | Yes | 资源名称 |

</details>

<details>
<summary>kubernetes_exec</summary>

在 Pod 容器中执行非交互式命令。默认禁用（需要 `--enable-container-exec`，且需要 `--read-only=false`）。命令必须是 argv 风格数组；不支持 stdin 和 TTY。返回 `exitCode`、`stdout` 和 `stderr`。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | Yes | Pod 名称 |
| `container` | string | No | 容器名称（默认为第一个容器） |
| `command` | array | Yes | 命令及参数，例如：`["printenv", "HOSTNAME"]` |

**示例：**

```json
{
  "cluster": "c-abc123",
  "namespace": "default",
  "name": "nginx-7d8b8f9c4-x7k2q",
  "command": ["cat", "/etc/hostname"]
}
```

</details>

<details>
<summary>kubernetes_upload_file</summary>

上传文件到 Pod 容器。默认禁用（需要 `--enable-container-file-upload`）。接受 base64 编码内容并写入指定路径。要求容器内存在 `tar`。文件大小受 `--max-file-size` 限制（默认：10Mi）。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | Yes | Pod 名称 |
| `container` | string | No | 容器名称（默认为第一个容器） |
| `filePath` | string | Yes | 容器内的绝对目标路径 |
| `content` | string | Yes | Base64 编码的文件内容 |

</details>

<details>
<summary>kubernetes_download_file</summary>

从 Pod 容器下载文件。默认启用，需要 `--enable-container-file-download` 才能激活。返回 base64 编码的文件内容及元数据。要求容器内存在 `tar`。文件大小受 `--max-file-size` 限制（默认：10Mi）。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | Yes | 集群引用：Rancher ID 或 `kubeconfig:<context>` |
| `namespace` | string | Yes | 命名空间 |
| `name` | string | Yes | Pod 名称 |
| `container` | string | No | 容器名称（默认为第一个容器） |
| `filePath` | string | Yes | 要下载文件的绝对路径 |

</details>

### rancher

<details>
<summary>cluster_list</summary>

列出 Rancher 和已配置 kubeconfig context 中的所有可用集群。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | No | 按集群名称过滤（部分匹配） |
| `limit` | integer | No | 每页条目数（默认：100） |
| `page` | integer | No | 页码（默认：1） |
| `format` | string | No | 输出格式：json、table、yaml（默认：json） |

</details>

<details>
<summary>project_list</summary>

列出 Rancher 项目。

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster` | string | No | 仅按 Rancher 集群 ID 过滤；拒绝 `kubeconfig:` 引用 |
| `name` | string | No | 按项目名称过滤（部分匹配） |
| `limit` | integer | No | 每页条目数（默认：100） |
| `page` | integer | No | 页码（默认：1） |
| `format` | string | No | 输出格式：json、table、yaml（默认：json） |

</details>

## 开发 <a id="development"></a>

### 前置要求

- Go 1.26.0+
- 可访问的 Rancher 服务器（仅用于 Rancher 集成测试；仅使用 kubeconfig 的运行不需要 Rancher 访问）

### 构建

```shell
make build
```

### 测试

```shell
make test        # 仅单元测试，无需 Docker
```

集成测试用 Docker 启动真实 Rancher 容器，并驱动构建出的服务端访问它们。它需要
Docker、每个版本数分钟时间，以及足以同时只运行一个 Rancher 容器的内存：

```shell
go test -tags=integration -timeout 45m ./test/integration/...                       # 2.13.3、2.14.3、2.15.1
RANCHER_TEST_VERSIONS=2.14.3 go test -tags=integration -timeout 30m ./test/integration/...
```

设置 `RANCHER_TEST_KEEP=1` 可保留 Rancher 容器以便排查。同一套测试也由
`Integration` GitHub Actions workflow 运行。

### Lint

```shell
make lint        # Run golangci-lint
make format      # Auto-format code
```

### 本地运行

```shell
make build
./rancher-mcp-server \
  --rancher-server-url https://your-rancher-server.com \
  --rancher-token your-token
```

### 使用 MCP Inspector 调试

```shell
npx @modelcontextprotocol/inspector@latest $(pwd)/rancher-mcp-server
```

## 贡献

开发环境搭建、项目结构和 Pull Request 指南见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 支持

- [GitHub Issues](https://github.com/futuretea/rancher-mcp-server/issues)
- 运行 `rancher-mcp-server --help` 查看支持的配置标志。

## 许可证

[Apache-2.0](LICENSE)
