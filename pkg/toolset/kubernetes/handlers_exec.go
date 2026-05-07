package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	clientexec "k8s.io/client-go/util/exec"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

type execResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func handleExec(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", paramutil.ErrReadOnlyMode
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := paramutil.ExtractRequiredString(params, paramutil.ParamNamespace)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	command, err := parseExecCommand(params)
	if err != nil {
		return "", err
	}

	container := paramutil.ExtractOptionalString(params, paramutil.ParamContainer)
	stdout, stderr, err := steveClient.ExecInPod(ctx, cluster, namespace, name, container, command, nil)
	resp := execResponse{
		ExitCode: 0,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}
	if err != nil {
		exitCode, ok := execExitCode(err)
		if !ok {
			return "", execTransportError(err, stderr)
		}
		resp.ExitCode = exitCode
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

func parseExecCommand(params map[string]interface{}) ([]string, error) {
	raw, ok := params[paramutil.ParamCommand]
	if !ok {
		return nil, fmt.Errorf("%w: %s", paramutil.ErrMissingParameter, paramutil.ParamCommand)
	}

	var command []string
	switch values := raw.(type) {
	case []string:
		command = append(command, values...)
	case []interface{}:
		command = make([]string, 0, len(values))
		for i, value := range values {
			part, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("invalid command[%d]: expected string", i)
			}
			command = append(command, part)
		}
	default:
		return nil, fmt.Errorf("invalid command: expected array of strings")
	}

	if len(command) == 0 {
		return nil, fmt.Errorf("%w: %s", paramutil.ErrMissingParameter, paramutil.ParamCommand)
	}
	if strings.TrimSpace(command[0]) == "" {
		return nil, fmt.Errorf("invalid command[0]: executable must not be empty")
	}
	return command, nil
}

func execExitCode(err error) (int, bool) {
	var exitErr clientexec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), true
	}
	return 0, false
}

func execTransportError(err error, stderr []byte) error {
	if s := strings.TrimSpace(string(stderr)); s != "" {
		return fmt.Errorf("exec command failed: %w (stderr: %s)", err, s)
	}
	return fmt.Errorf("exec command failed: %w", err)
}
