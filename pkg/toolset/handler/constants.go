package handler

import "errors"

// Format constants
const (
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	FormatTable = "table"
)

// Parameter name constants
const (
	ParamCluster       = "cluster"
	ParamNamespace     = "namespace"
	ParamProject       = "project"
	ParamFormat        = "format"
	ParamName          = "name"
	ParamUser          = "user"
	ParamContainer     = "container"
	ParamTailLines     = "tailLines"
	ParamSinceSeconds  = "sinceSeconds"
	ParamTimestamps    = "timestamps"
	// Kubernetes toolset parameters
	ParamKind          = "kind"
	ParamLabelSelector = "labelSelector"
	ParamLimit         = "limit"
	ParamResource      = "resource"
	ParamPatch         = "patch"
	ParamPrevious      = "previous"
	ParamPage          = "page"
)

// Error definitions
var (
	ErrRancherNotConfigured = errors.New("rancher client not configured, please configure rancher credentials to use this tool")
	ErrSteveNotConfigured   = errors.New("kubernetes client not configured, please configure rancher credentials to use this tool")
	ErrInvalidFormat        = errors.New("invalid output format")
	ErrMissingParameter     = errors.New("missing required parameter")
	ErrReadOnlyMode         = errors.New("operation not allowed: server is running in read-only mode")
	ErrDestructiveDisabled  = errors.New("operation not allowed: destructive operations are disabled")
)
