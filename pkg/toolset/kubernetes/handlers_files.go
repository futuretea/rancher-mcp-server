// Package kubernetes provides file transfer handlers for container file operations.
package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

type downloadFileResponse struct {
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
	Content   string `json:"content"`
}

type uploadFileResponse struct {
	Status       string `json:"status"`
	FilePath     string `json:"filePath"`
	BytesWritten int    `json:"bytesWritten"`
}

// parseMaxFileSize extracts and parses the maxFileSize parameter.
func parseMaxFileSize(params map[string]interface{}) (int64, error) {
	sizeStr := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamMaxFileSize, DefaultMaxFileSize)
	q, err := resource.ParseQuantity(sizeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid maxFileSize %q: %w", sizeStr, err)
	}
	return q.Value(), nil
}

func handleDownloadFile(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
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
	filePath, err := paramutil.ExtractRequiredString(params, paramutil.ParamFilePath)
	if err != nil {
		return "", err
	}

	container := paramutil.ExtractOptionalString(params, paramutil.ParamContainer)

	maxSizeBytes, err := parseMaxFileSize(params)
	if err != nil {
		return "", err
	}

	fileType, sizeBytes, err := steveClient.CheckFileInfo(ctx, cluster, namespace, name, container, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to check file info: %w", err)
	}
	if fileType == "dir" {
		return "", fmt.Errorf("path is a directory, not a file: %s", filePath)
	}
	if sizeBytes > maxSizeBytes {
		return "", fmt.Errorf("file size %d bytes exceeds limit %d bytes", sizeBytes, maxSizeBytes)
	}

	content, err := steveClient.DownloadFile(ctx, cluster, namespace, name, container, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	resp := downloadFileResponse{
		FileName:  path.Base(filePath),
		SizeBytes: int64(len(content)),
		Content:   base64.StdEncoding.EncodeToString(content),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

func handleUploadFile(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
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
	filePath, err := paramutil.ExtractRequiredString(params, paramutil.ParamFilePath)
	if err != nil {
		return "", err
	}
	contentBase64, err := paramutil.ExtractRequiredString(params, paramutil.ParamContent)
	if err != nil {
		return "", err
	}

	container := paramutil.ExtractOptionalString(params, paramutil.ParamContainer)

	maxSizeBytes, err := parseMaxFileSize(params)
	if err != nil {
		return "", err
	}

	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 content: %w", err)
	}
	if int64(len(content)) > maxSizeBytes {
		return "", fmt.Errorf("content size %d bytes exceeds limit %d bytes", len(content), maxSizeBytes)
	}

	if err := steveClient.UploadFile(ctx, cluster, namespace, name, container, filePath, content); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	resp := uploadFileResponse{
		Status:       "ok",
		FilePath:     filePath,
		BytesWritten: len(content),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

