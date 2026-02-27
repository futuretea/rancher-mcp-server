package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"github.com/futuretea/rancher-mcp-server/pkg/watchdiff"
)

// watchDiffHandler handles the kubernetes_watch tool.
// It behaves similarly to the Linux `watch` command: it repeatedly
// evaluates the current state of matching resources at a configurable
// interval and returns the concatenated diffs from all iterations.
func watchDiffHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	fieldSelector := paramutil.ExtractOptionalString(params, paramutil.ParamFieldSelector)

	ignoreStatus := paramutil.ExtractBool(params, "ignoreStatus", false)
	ignoreMeta := paramutil.ExtractBool(params, "ignoreMeta", false)

	intervalSeconds := paramutil.ExtractInt64(params, paramutil.ParamIntervalSeconds, DefaultIntervalSeconds)
	if intervalSeconds < MinIntervalSeconds {
		intervalSeconds = MinIntervalSeconds
	}
	if intervalSeconds > MaxIntervalSeconds {
		intervalSeconds = MaxIntervalSeconds
	}

	iterations := paramutil.ExtractInt64(params, paramutil.ParamIterations, DefaultIterations)
	if iterations < MinIterations {
		iterations = MinIterations
	}
	if iterations > MaxIterations {
		iterations = MaxIterations
	}

	differ := watchdiff.NewDiffer(true)
	differ.SetIgnoreStatus(ignoreStatus)
	differ.SetIgnoreMeta(ignoreMeta)

	var resultLines []string

	for i := int64(0); i < iterations; i++ {
		// List current resources for this iteration
		listOpts := &steve.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		}
		list, err := steveClient.ListResources(ctx, cluster, kind, namespace, listOpts)
		if err != nil {
			return "", fmt.Errorf("failed to list resources: %w", err)
		}

		// Sort for deterministic output
		sort.Slice(list.Items, func(i, j int) bool {
			ai := list.Items[i]
			aj := list.Items[j]
			if ai.GetNamespace() != aj.GetNamespace() {
				return ai.GetNamespace() < aj.GetNamespace()
			}
			if ai.GetKind() != aj.GetKind() {
				return ai.GetKind() < aj.GetKind()
			}
			return ai.GetName() < aj.GetName()
		})

		iterationHeader := fmt.Sprintf("# iteration %d\n", i+1)
		iterationLines := []string{iterationHeader}

		for idx := range list.Items {
			obj := &list.Items[idx]
			diffText, err := differ.Diff(obj)
			if err != nil {
				return "", fmt.Errorf("failed to diff resource: %w", err)
			}
			if diffText != "" {
				iterationLines = append(iterationLines, diffText)
			}
		}

		// Only append iteration output if there was any diff beyond the header.
		if len(iterationLines) > 1 {
			resultLines = append(resultLines, strings.Join(iterationLines, "\n"))
		}

		// Sleep between iterations, except after the last one.
		if i+1 < iterations {
			time.Sleep(time.Duration(intervalSeconds) * time.Second)
		}
	}

	if len(resultLines) == 0 {
		return "No changes detected across iterations", nil
	}

	return strings.Join(resultLines, "\n"), nil
}
