# Kubernetes namespace policy

Each MCP server owns a snapshot of its configured namespace allowlist. Constructing
or closing another server does not change that snapshot. The MCP handler replaces
the internal `namespaceAllowlist` parameter with this trusted configuration before
calling a Kubernetes tool, including when the configuration is empty. It is not a
client-selectable tool parameter.

The toolset wrapper places the policy in a private request context value. All
namespace authorization and query planning helpers consume that context, including
nested dependency, aggregation and watch paths. Resource readers receive only
queries allowed by this policy. The context carries immutable policy for the
entire tool invocation; watch iterations use the same policy.

`SetNamespaceAllowlist` remains the startup default for direct toolset callers
without a server-supplied policy. It is not runtime reconfiguration and must not
be called while those direct callers are active. MCP server construction does not
write this default.

Restricted Namespace queries read only allowed names. A positive fetch limit
counts matching objects after selectors and missing-name filtering. Reaching the
limit with unread names returns an incompleteness signal; completing the name
list returns a complete result. Watch rejects incomplete snapshots before diffing.
