package rancher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
)

// getClusterProvider returns a human-readable cluster provider name
func getClusterProvider(cluster rancher.Cluster) string {
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
func getClusterCPU(cluster rancher.Cluster) string {
	req := parseResourceString(cluster.Requested["cpu"])
	alloc := parseResourceString(cluster.Allocatable["cpu"])
	return req + "/" + alloc
}

// getClusterRAM returns formatted RAM usage string (requested/allocatable in GB)
func getClusterRAM(cluster rancher.Cluster) string {
	req := parseResourceString(cluster.Requested["memory"])
	alloc := parseResourceString(cluster.Allocatable["memory"])
	return req + "/" + alloc + " GB"
}

// getClusterPods returns formatted pod count string (requested/allocatable)
func getClusterPods(cluster rancher.Cluster) string {
	return cluster.Requested["pods"] + "/" + cluster.Allocatable["pods"]
}

// parseResourceString converts Kubernetes resource strings to human-readable format
// Returns GB for Ki and Mi units, and CPU cores from 'm' (millicores)
func parseResourceString(mem string) string {
	if mem == "" {
		return "-"
	}

	if strings.HasSuffix(mem, "Ki") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "Ki", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1024 / 1024
		return strings.TrimSuffix(fmt.Sprintf("%.2f", num), ".0")
	}
	if strings.HasSuffix(mem, "Mi") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "Mi", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1024
		return strings.TrimSuffix(fmt.Sprintf("%.2f", num), ".0")
	}
	if strings.HasSuffix(mem, "m") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "m", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1000
		return strconv.FormatFloat(num, 'f', 2, 32)
	}
	return mem
}
