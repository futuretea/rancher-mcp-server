package steve

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var parameterCodec = runtime.NewParameterCodec(scheme.Scheme)

// ExecInPod executes a command inside a container and returns stdout, stderr, and any error.
func (c *Client) ExecInPod(ctx context.Context, clusterID, namespace, podName, container string, command []string, stdin io.Reader) ([]byte, []byte, error) {
	restConfig, err := c.createRestConfig(clusterID)
	if err != nil {
		return nil, nil, fmt.Errorf("create REST config: %w", err)
	}

	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return nil, nil, fmt.Errorf("create clientset: %w", err)
	}

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, parameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, fmt.Errorf("create SPDY executor: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	}
	if stdin != nil {
		streamOpts.Stdin = stdin
	}

	if err := executor.StreamWithContext(ctx, streamOpts); err != nil {
		// Return buffers even on error - they may contain useful diagnostics
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("exec command: %w", err)
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}

// CheckFileInfo checks if a path exists in a container and returns its type ("dir", "file") and size.
func (c *Client) CheckFileInfo(ctx context.Context, clusterID, namespace, podName, container, filePath string) (string, int64, error) {
	// Uses positional parameters ($1) to prevent shell injection.
	// stat -c %s for files, du -sb for directories (fallback to ls for non-GNU stat).
	script := `if [ -d "$1" ]; then echo dir; du -sb "$1" | awk '{print $1}'
elif [ -f "$1" ]; then echo file; stat -c %s "$1" 2>/dev/null || ls -l "$1" | awk '{print $5}'
else echo notfound; echo 0; fi`

	command := []string{"sh", "-c", script, "checkfile", filePath}
	stdout, stderr, err := c.ExecInPod(ctx, clusterID, namespace, podName, container, command, nil)
	if err != nil {
		return "", 0, wrapExecError("check file info", err, stderr)
	}

	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) < 2 {
		return "", 0, fmt.Errorf("unexpected output format: %s", string(stdout))
	}

	fileType := strings.TrimSpace(lines[0])
	if fileType == "notfound" {
		return "", 0, fmt.Errorf("path does not exist: %s", filePath)
	}

	sizeBytes, err := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return fileType, 0, fmt.Errorf("parse size: %w", err)
	}

	return fileType, sizeBytes, nil
}

// DownloadFile downloads a file from a container and returns its content.
// Symlinks are followed up to a maximum depth of 10.
func (c *Client) DownloadFile(ctx context.Context, clusterID, namespace, podName, container, filePath string) ([]byte, error) {
	return c.downloadFileWithDepth(ctx, clusterID, namespace, podName, container, filePath, 10)
}

func (c *Client) downloadFileWithDepth(ctx context.Context, clusterID, namespace, podName, container, filePath string, maxDepth int) ([]byte, error) {
	if maxDepth <= 0 {
		return nil, fmt.Errorf("maximum symlink depth exceeded: %s", filePath)
	}

	dir := path.Dir(filePath)
	filename := path.Base(filePath)
	command := []string{"tar", "cf", "-", "-C", dir, filename}

	stdout, stderr, err := c.ExecInPod(ctx, clusterID, namespace, podName, container, command, nil)
	if err != nil {
		return nil, wrapExecError("download file", err, stderr)
	}

	tr := tar.NewReader(bytes.NewReader(stdout))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar header: %w", err)
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read file from tar: %w", err)
			}
			return content, nil

		case tar.TypeSymlink:
			target := hdr.Linkname
			if !path.IsAbs(target) {
				target = path.Join(dir, target)
			}
			return c.downloadFileWithDepth(ctx, clusterID, namespace, podName, container, target, maxDepth-1)

		case tar.TypeDir:
			return nil, fmt.Errorf("path is a directory: %s", filePath)
		}
	}

	// Empty file case
	return []byte{}, nil
}

// UploadFile uploads content to a file in a container using tar.
func (c *Client) UploadFile(ctx context.Context, clusterID, namespace, podName, container, filePath string, content []byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := tw.WriteHeader(&tar.Header{
		Name: filePath,
		Mode: 0644,
		Size: int64(len(content)),
	})
	if err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	command := []string{"tar", "xf", "-", "-C", "/"}
	_, stderr, err := c.ExecInPod(ctx, clusterID, namespace, podName, container, command, &buf)
	if err != nil {
		return wrapExecError("upload file", err, stderr)
	}

	return nil
}

// wrapExecError formats an exec error with optional stderr context.
func wrapExecError(op string, err error, stderr []byte) error {
	if s := strings.TrimSpace(string(stderr)); s != "" {
		return fmt.Errorf("%s: %w (stderr: %s)", op, err, s)
	}
	return fmt.Errorf("%s: %w", op, err)
}
