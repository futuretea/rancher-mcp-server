# DESIGN — pkg/toolset

Detail view for the tool framework and toolsets. Integration view:
[root DESIGN.md](../../DESIGN.md).

## Framework (pkg/toolset root)

- `Toolset.GetTools(client)` returns `[]ServerTool`. The client argument is
  currently ignored by both toolsets — the real client reaches handlers per
  request as the resolved `CombinedClient` (see pkg/server/DESIGN.md).
- Handlers receive `(ctx, client, params)`; `paramutil` centralizes parameter
  extraction, format enum, output filters, sensitive-data masking, and error
  sentinels.

## Toolsets

- `kubernetes` (24 tools): resource (get/list/get_all/logs/inspect_pod/
  describe/events/rollout_history), analysis (dep/node_analysis/
  resource_diff/watch/diff/capacity), aggregate (top/workload_health/
  resource_summary/event_summary), file (download_file), write
  (create/patch/exec/upload_file/delete). Subpackages: `aggregate`,
  `capacity`, `internal/formatutil`.
- `rancher` (2 tools): `cluster_list`, `project_list`. `cluster_list` merges
  Rancher rows (`source: rancher`) with `kubeconfig:<context>` rows
  (`source: kubeconfig`); `project_list` is Rancher-only.
- Default selection: `--toolsets kubernetes,rancher`.

## watch + diff semantics

- `kubernetes_watch` is synchronous polling, not push: re-list every
  `intervalSeconds` for `iterations` and return concatenated git-style diffs
  from `watchdiff.Differ` (caches last-seen state), bounded by
  `MaxWatchItems` / `MaxWatchOutputBytes`. No MCP notifications or
  subscriptions.
- One-shot `kubernetes_diff` / `kubernetes_resource_diff` use the stateless
  `watchdiff.Printer`.
- `pkg/dep` resolves kube-lineage-style dependency graphs over
  `steve.ResourceReader` with a fixed kind list.
