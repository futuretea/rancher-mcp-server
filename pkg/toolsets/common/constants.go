package common

import "errors"

// Format constants
const (
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	FormatTable = "table"
)

// Pod state constants
const (
	PodStateRunning = "running"
	PodStateActive  = "active"
)

// Parameter name constants
const (
	ParamCluster       = "cluster"
	ParamNamespace     = "namespace"
	ParamProject       = "project"
	ParamFormat        = "format"
	ParamNode          = "node"
	ParamName          = "name"
	ParamUser          = "user"
	ParamGetPodDetails = "getPodDetails"
)

// Error definitions
var (
	ErrRancherNotConfigured = errors.New("rancher client not configured, please configure rancher credentials to use this tool")
	ErrInvalidFormat        = errors.New("invalid output format")
	ErrMissingParameter     = errors.New("missing required parameter")
)
