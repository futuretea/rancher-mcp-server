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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	request, err := buildWatchRequest(params)
	if err != nil {
		return "", err
	}
	return watchDiffWithReader(ctx, steveClient, request)
}

type watchRequest struct {
	cluster        string
	kind           string
	namespace      string
	labelSelector  string
	fieldSelector  string
	ignoreStatus   bool
	ignoreMeta     bool
	interval       time.Duration
	iterations     int64
	maxItems       int
	maxOutputBytes int
}

func buildWatchRequest(params map[string]interface{}) (*watchRequest, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return nil, err
	}
	kind, err := extractResourceKind(params)
	if err != nil {
		return nil, err
	}

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

	return &watchRequest{
		cluster:        cluster,
		kind:           kind,
		namespace:      paramutil.ExtractOptionalString(params, paramutil.ParamNamespace),
		labelSelector:  paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector),
		fieldSelector:  paramutil.ExtractOptionalString(params, paramutil.ParamFieldSelector),
		ignoreStatus:   paramutil.ExtractBool(params, "ignoreStatus", false),
		ignoreMeta:     paramutil.ExtractBool(params, "ignoreMeta", false),
		interval:       time.Duration(intervalSeconds) * time.Second,
		iterations:     iterations,
		maxItems:       MaxWatchItems,
		maxOutputBytes: MaxWatchOutputBytes,
	}, nil
}

type iterationDiff struct {
	currentObjects map[string]*unstructured.Unstructured
	diffTexts      []string
	changeCount    int
	deleteCount    int
}

func watchDiffWithReader(ctx context.Context, reader steve.ResourceReader, request *watchRequest) (string, error) {
	differ := watchdiff.NewDiffer(true)
	differ.SetIgnoreStatus(request.ignoreStatus)
	differ.SetIgnoreMeta(request.ignoreMeta)

	var resultLines []string
	totalOutputBytes := 0
	previousObjects := make(map[string]*unstructured.Unstructured)

	for i := int64(0); i < request.iterations; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		list, err := reader.ListResources(ctx, request.cluster, request.kind, request.namespace, &steve.ListOptions{
			LabelSelector: request.labelSelector,
			FieldSelector: request.fieldSelector,
			Limit:         int64(request.maxItems + 1),
		})
		if err != nil {
			return "", fmt.Errorf("failed to list resources: %w", err)
		}
		if err := validateWatchIteration(list, request.maxItems); err != nil {
			return "", err
		}

		sortResourceList(list.Items)

		diff, err := diffIteration(differ, previousObjects, list.Items)
		if err != nil {
			return "", err
		}

		iterationOutput := buildIterationOutput(int(i+1), len(list.Items), diff.changeCount, diff.deleteCount, diff.diffTexts)
		if iterationOutput != "" {
			if totalOutputBytes+len(iterationOutput) > request.maxOutputBytes {
				return "", fmt.Errorf(
					"watch response exceeded the %d byte output limit before iteration %d completed; narrow kind, namespace, selectors, or iterations",
					request.maxOutputBytes,
					i+1,
				)
			}
			resultLines = append(resultLines, iterationOutput)
			totalOutputBytes += len(iterationOutput)
		}

		previousObjects = diff.currentObjects

		if i+1 < request.iterations {
			if err := waitForNextIteration(ctx, request.interval); err != nil {
				return "", err
			}
		}
	}

	if len(resultLines) == 0 {
		return fmt.Sprintf("No changes detected across %d iterations", request.iterations), nil
	}

	return strings.Join(resultLines, "\n"), nil
}

func validateWatchIteration(list *unstructured.UnstructuredList, maxItems int) error {
	if list.GetContinue() != "" {
		return fmt.Errorf("watch scope exceeded the per-iteration limit of %d resources; narrow kind, namespace, or selectors", maxItems)
	}
	if len(list.Items) > maxItems {
		return fmt.Errorf("watch scope returned %d resources, exceeding the per-iteration limit of %d; narrow kind, namespace, or selectors", len(list.Items), maxItems)
	}
	return nil
}

func diffIteration(differ *watchdiff.Differ, previousObjects map[string]*unstructured.Unstructured, items []unstructured.Unstructured) (*iterationDiff, error) {
	result := &iterationDiff{
		currentObjects: make(map[string]*unstructured.Unstructured, len(items)),
		diffTexts:      make([]string, 0),
	}

	for idx := range items {
		obj := &items[idx]
		result.currentObjects[watchObjectKey(obj)] = obj.DeepCopy()

		diffText, err := differ.Diff(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to diff resource: %w", err)
		}
		if diffText != "" {
			result.changeCount++
			result.diffTexts = append(result.diffTexts, diffText)
		}
	}

	deletedKeys := diffDeletedKeys(previousObjects, result.currentObjects)
	for _, key := range deletedKeys {
		diffText, err := differ.DiffDelete(previousObjects[key])
		if err != nil {
			return nil, fmt.Errorf("failed to diff deleted resource: %w", err)
		}
		if diffText != "" {
			result.deleteCount++
			result.diffTexts = append(result.diffTexts, diffText)
		}
	}

	return result, nil
}

func buildIterationOutput(iteration, resourceCount, changeCount, deleteCount int, diffTexts []string) string {
	if len(diffTexts) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"# iteration %d resources=%d changes=%d deletions=%d\n\n%s",
		iteration,
		resourceCount,
		changeCount,
		deleteCount,
		strings.Join(diffTexts, "\n"),
	)
}

func waitForNextIteration(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func sortResourceList(items []unstructured.Unstructured) {
	sort.Slice(items, func(i, j int) bool {
		ai := items[i]
		aj := items[j]
		if ai.GetNamespace() != aj.GetNamespace() {
			return ai.GetNamespace() < aj.GetNamespace()
		}
		if ai.GetKind() != aj.GetKind() {
			return ai.GetKind() < aj.GetKind()
		}
		return ai.GetName() < aj.GetName()
	})
}

func diffDeletedKeys(previousObjects, currentObjects map[string]*unstructured.Unstructured) []string {
	var deletedKeys []string
	for key := range previousObjects {
		if _, found := currentObjects[key]; found {
			continue
		}
		deletedKeys = append(deletedKeys, key)
	}
	sort.Strings(deletedKeys)
	return deletedKeys
}

func watchObjectKey(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		obj.GetAPIVersion(),
		obj.GetKind(),
		obj.GetNamespace(),
		obj.GetName(),
	)
}
