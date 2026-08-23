# DESIGN — pkg/server

Detail view for `pkg/server/mcp` and `pkg/server/http`. Integration view:
[root DESIGN.md](../../DESIGN.md).

## pkg/server/mcp — MCP assembly

- `NewServer` picks the auth mode and wires a `ClientResolver`:
  static credentials → clients built once at startup (`staticResolver`);
  per-request token → `requestTokenResolver`; OAuth → `oauthTokenResolver`.
- Transports: `ServeStdio`; HTTP handlers mounted by `pkg/server/http`:
  Streamable HTTP at `/mcp` (`WithStateLess(true)`, no session state), SSE at
  `/sse` + `/message`. `contextFunc` copies `Authorization` and `R_token`
  headers into the request context.
- Tool registration pipeline: enabled toolsets → duplicate-name check →
  per-tool gates (cluster-source availability, container-op opt-in,
  enabled/disabled lists) → `configureTool` injects `output`, `readOnly`,
  `disableDestructive`, `outputFilters`, forces `showSensitiveData=false`
  unless globally enabled → `AddTool`.
- Per-request modes resolve a fresh `CombinedClient` per call and `Close()` it
  after. Norman client construction failure degrades — the request is not
  failed; missing Norman surfaces later via `ValidateNormanClient`.
- `/healthz` JSON reports Rancher/Kubernetes capability availability, derived
  from auth mode and cluster sources (`capabilities.go`).
- Metrics are expvar only, curated subset exposed at `/debug/vars`.

## OAuth verification (oauth.go)

- JWKS loaded at startup; fail-closed without an RS256 verification key.
- Hourly refresh; unknown-KID refresh rate-limited to once per 5 min; last
  known-good keyset kept on refresh failure.
- Middleware verifies RS256, configured issuer, 10 s leeway, scopes
  `offline_access` + `rancher:mcp`; failures get `401` +
  `WWW-Authenticate` challenge.
- Serves `/.well-known/oauth-protected-resource`; SSE routes not registered
  in this mode.

## pkg/server/http

- Routes: `/healthz`, `/mcp`, `/sse` + `/message` (SSE modes only),
  `/debug/vars`, `/.well-known/oauth-protected-resource` (OAuth mode).
- Request-logging middleware; graceful shutdown on SIGINT/SIGHUP/SIGTERM.
- Knows nothing about toolsets or clients — MCP handlers come from
  `pkg/server/mcp`.
