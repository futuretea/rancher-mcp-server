package rancher

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// userGetHandler handles the user_get tool for single user queries
func userGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	userID := ""
	if userParam, ok := params["user"].(string); ok {
		userID = userParam
	}
	if userID == "" {
		return "", fmt.Errorf("user parameter is required")
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Get the user
	user, err := rancherClient.GetUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"created":   common.FormatTime(user.Created),
		"username":  user.Username,
		"principal": user.PrincipalIDs,
	}

	return formatUserResult(result, format)
}

// formatUserResult formats a single user result
func formatUserResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		data := []map[string]string{}
		row := map[string]string{
			"id":       common.GetStringValue(result["id"]),
			"name":     common.GetStringValue(result["name"]),
			"username": common.GetStringValue(result["username"]),
			"created":  common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "username", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// userListHandler handles the user_list tool
func userListHandler(client interface{}, params map[string]interface{}) (string, error) {
	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	// Try to use real Rancher client if available
	if rancherClient, ok := client.(*rancher.Client); ok && rancherClient != nil && rancherClient.IsConfigured() {
		ctx := context.Background()
		users, err := rancherClient.ListUsers(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list users: %v", err)
		}

		// Convert to map format for consistent output with richer information
		userMaps := make([]map[string]string, len(users))
		for i, user := range users {
			enabled := "false"
			if user.Enabled != nil && *user.Enabled {
				enabled = "true"
			}
			userMaps[i] = map[string]string{
				"id":          user.ID,
				"username":    user.Username,
				"name":        user.Name,
				"enabled":     enabled,
				"description": user.Description,
				"created":     common.FormatTime(user.Created),
			}
		}

		switch format {
		case common.FormatYAML:
			return common.FormatAsYAML(userMaps)
		case common.FormatJSON:
			return common.FormatAsJSON(userMaps)
		case common.FormatTable:
			return common.FormatAsTable(userMaps, []string{"id", "username", "name", "enabled", "description"}), nil
		default:
			return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}
