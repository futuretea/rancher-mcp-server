package rancher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
)

// getClusterProvider returns a human-readable cluster provider name
func getClusterProvider(cluster norman.Cluster) string {
	switch cluster.Driver {
	case "imported":
		switch cluster.Provider {
		case "rke2":
			return "RKE2"
		case "k3s":
			return "K3S"
		default:
			return "Imported"
		}
	case "k3s":
		return "K3S"
	case "rke2":
		return "RKE2"
	case "rancherKubernetesEngine":
		return "Rancher Kubernetes Engine"
	case "azureKubernetesService", "AKS":
		return "Azure Kubernetes Service"
	case "googleKubernetesEngine", "GKE":
		return "Google Kubernetes Engine"
	case "EKS":
		return "Elastic Kubernetes Service"
	default:
		return "Unknown"
	}
}

// getClusterCPU returns formatted CPU usage string (requested/allocatable)
func getClusterCPU(cluster norman.Cluster) string {
	req := parseResourceString(cluster.Requested["cpu"])
	alloc := parseResourceString(cluster.Allocatable["cpu"])
	return req + "/" + alloc
}

// getClusterRAM returns formatted RAM usage string (requested/allocatable in GB)
func getClusterRAM(cluster norman.Cluster) string {
	req := parseResourceString(cluster.Requested["memory"])
	alloc := parseResourceString(cluster.Allocatable["memory"])
	return req + "/" + alloc + " GB"
}

// getClusterPods returns formatted pod count string (requested/allocatable)
func getClusterPods(cluster norman.Cluster) string {
	return cluster.Requested["pods"] + "/" + cluster.Allocatable["pods"]
}

// parseResourceString converts Kubernetes resource strings to human-readable format
// Returns GB for Ki and Mi units, and CPU cores from 'm' (millicores)
func parseResourceString(s string) string {
	if s == "" {
		return "-"
	}

	switch {
	case strings.HasSuffix(s, "Ki"):
		num, err := strconv.ParseFloat(strings.TrimSuffix(s, "Ki"), 64)
		if err != nil {
			return s
		}
		return formatFloat(num / 1024 / 1024)
	case strings.HasSuffix(s, "Mi"):
		num, err := strconv.ParseFloat(strings.TrimSuffix(s, "Mi"), 64)
		if err != nil {
			return s
		}
		return formatFloat(num / 1024)
	case strings.HasSuffix(s, "m"):
		num, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil {
			return s
		}
		return strconv.FormatFloat(num/1000, 'f', 2, 64)
	default:
		return s
	}
}

// formatFloat formats a float with up to 2 decimal places, trimming trailing zeros
func formatFloat(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
