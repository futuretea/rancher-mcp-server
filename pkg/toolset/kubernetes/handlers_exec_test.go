package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	clientexec "k8s.io/client-go/util/exec"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

func TestParseExecCommand(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		want    []string
		wantErr string
	}{
		{
			name:   "string slice",
			params: map[string]interface{}{"command": []string{"printenv", "HOSTNAME"}},
			want:   []string{"printenv", "HOSTNAME"},
		},
		{
			name:   "interface slice",
			params: map[string]interface{}{"command": []interface{}{"sh", "-c", "echo ok"}},
			want:   []string{"sh", "-c", "echo ok"},
		},
		{
			name:    "missing",
			params:  map[string]interface{}{},
			wantErr: "command",
		},
		{
			name:    "empty array",
			params:  map[string]interface{}{"command": []interface{}{}},
			wantErr: "command",
		},
		{
			name:    "wrong type",
			params:  map[string]interface{}{"command": "printenv HOSTNAME"},
			wantErr: "array of strings",
		},
		{
			name:    "non string element",
			params:  map[string]interface{}{"command": []interface{}{"echo", 1}},
			wantErr: "command[1]",
		},
		{
			name:    "empty executable",
			params:  map[string]interface{}{"command": []interface{}{" ", "arg"}},
			wantErr: "command[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExecCommand(tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseExecCommand() expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseExecCommand() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExecCommand() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseExecCommand() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestHandleExec_ReadOnlyMode(t *testing.T) {
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"command":   []interface{}{"true"},
		"readOnly":  true,
	}

	_, err := handleExec(context.Background(), nil, params)
	if err != paramutil.ErrReadOnlyMode {
		t.Fatalf("handleExec() error = %v, want %v", err, paramutil.ErrReadOnlyMode)
	}
}

func TestHandleExec_InvalidClientType(t *testing.T) {
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"command":   []interface{}{"true"},
	}

	_, err := handleExec(context.Background(), nil, params)
	if err != paramutil.ErrSteveNotConfigured {
		t.Fatalf("handleExec() error = %v, want %v", err, paramutil.ErrSteveNotConfigured)
	}
}

func TestHandleExec_MissingRequiredParams(t *testing.T) {
	mockClient := steve.NewClient("https://example.com", "token", "", "", false)
	tests := []struct {
		name        string
		params      map[string]interface{}
		wantErrPart string
	}{
		{
			name:        "missing cluster",
			params:      map[string]interface{}{"namespace": "ns", "name": "pod", "command": []interface{}{"true"}},
			wantErrPart: "cluster",
		},
		{
			name:        "missing namespace",
			params:      map[string]interface{}{"cluster": "c1", "name": "pod", "command": []interface{}{"true"}},
			wantErrPart: "namespace",
		},
		{
			name:        "missing name",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "command": []interface{}{"true"}},
			wantErrPart: "name",
		},
		{
			name:        "missing command",
			params:      map[string]interface{}{"cluster": "c1", "namespace": "ns", "name": "pod"},
			wantErrPart: "command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleExec(context.Background(), mockClient, tt.params)
			if err == nil {
				t.Fatalf("handleExec() expected error containing %q, got nil", tt.wantErrPart)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("handleExec() error = %v, want containing %q", err, tt.wantErrPart)
			}
		})
	}
}

func TestHandleExec_CombinedClientNilSteve(t *testing.T) {
	combinedClient := &toolset.CombinedClient{}
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "ns",
		"name":      "pod",
		"command":   []interface{}{"true"},
	}

	_, err := handleExec(context.Background(), combinedClient, params)
	if err != paramutil.ErrSteveNotConfigured {
		t.Fatalf("handleExec() error = %v, want %v", err, paramutil.ErrSteveNotConfigured)
	}
}

func TestExecExitCode(t *testing.T) {
	wrapped := fmt.Errorf("exec command: %w", clientexec.CodeExitError{
		Err:  errors.New("command terminated with exit code 7"),
		Code: 7,
	})

	code, ok := execExitCode(wrapped)
	if !ok {
		t.Fatal("execExitCode() ok = false, want true")
	}
	if code != 7 {
		t.Fatalf("execExitCode() code = %d, want 7", code)
	}
}

func TestExecTransportErrorIncludesStderr(t *testing.T) {
	err := execTransportError(errors.New("transport failed"), []byte("permission denied\n"))
	if err == nil {
		t.Fatal("execTransportError() returned nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("execTransportError() = %v, want stderr context", err)
	}
}

func TestExecResponse_JSONMarshal(t *testing.T) {
	response := execResponse{
		ExitCode: 3,
		Stdout:   "out\n",
		Stderr:   "err\n",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if int(decoded["exitCode"].(float64)) != 3 {
		t.Fatalf("exitCode = %v, want 3", decoded["exitCode"])
	}
	if decoded["stdout"] != "out\n" {
		t.Fatalf("stdout = %v, want out", decoded["stdout"])
	}
	if decoded["stderr"] != "err\n" {
		t.Fatalf("stderr = %v, want err", decoded["stderr"])
	}
}
