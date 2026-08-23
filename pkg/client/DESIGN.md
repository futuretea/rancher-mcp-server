# DESIGN — pkg/client

Detail view for Rancher/Kubernetes access. Integration view:
[root DESIGN.md](../../DESIGN.md).

## Two clients, one facade

- `pkg/client/norman` — Rancher Norman v3 management API: clusters, projects,
  users, `generateKubeconfig`. Wraps generated `rancher/pkg/client` code.
- `pkg/client/steve` — Kubernetes access via standard client-go (`dynamic` +
  `kubernetes`); despite the name, not a Steve REST client.
- `pkg/toolset.CombinedClient` holds both; handlers validate per call
  (`ValidateSteveClient` / `ValidateNormanClient`).

## Cluster sources (steve/source.go)

A cluster reference resolves to exactly one source:

- `rancherSource` — synthesizes a kubeconfig pointing at
  `<rancher>/k8s/clusters/<clusterID>` (`pkg/util/url`); auth = bearer token
  or access/secret basic auth.
- `kubeconfigSource` — contexts from `--kubeconfig-paths`, addressed as
  `kubeconfig:<context>`. First file wins on duplicate context names; empty,
  traversal, and unknown-prefix references are rejected.

Kubeconfig sources require static credentials: excluded from per-request-token
and OAuth modes; HTTP mode warns that kubeconfig requests carry no HTTP auth.

Per-cluster clients are cached; kind→GVR via a static map with discovery
fallback for CRDs.

## Special-purpose access

- Exec: WebSocket primary with SPDY fallback (on upgrade failure); file
  transfer is tar-over-exec.
- Logs: CoreV1 `GetLogs`.
- `WatchResources` exists, but `kubernetes_watch` is synchronous polling —
  see [pkg/toolset/DESIGN.md](../toolset/DESIGN.md).
- `pkg/client/steve/fake` is the test double used across toolset tests.
