package steve

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/util/url"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const kubeconfigPrefix = "kubeconfig:"

// ClusterReferenceError reports an invalid or unavailable cluster reference.
type ClusterReferenceError struct {
	Reference string
	Reason    string
}

func (e *ClusterReferenceError) Error() string {
	return fmt.Sprintf("invalid cluster reference %q: %s", e.Reference, e.Reason)
}

type clusterSource interface {
	restConfig(reference string) (*rest.Config, error)
}

type rancherSource struct {
	client *Client
}

func (s *rancherSource) restConfig(clusterID string) (*rest.Config, error) {
	client := s.client
	clusterURL := url.GetSteveURL(client.serverURL, clusterID)

	kubeconfig := clientcmdapi.NewConfig()
	kubeconfig.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server:                clusterURL,
		InsecureSkipTLSVerify: client.insecure,
	}

	authInfo := &clientcmdapi.AuthInfo{}
	if client.token != "" {
		authInfo.Token = client.token
	} else if client.accessKey != "" && client.secretKey != "" {
		authInfo.Username = client.accessKey
		authInfo.Password = client.secretKey
	}
	kubeconfig.AuthInfos["user"] = authInfo
	kubeconfig.Contexts["context"] = &clientcmdapi.Context{
		Cluster:  "cluster",
		AuthInfo: "user",
	}
	kubeconfig.CurrentContext = "context"

	return clientcmd.NewNonInteractiveClientConfig(
		*kubeconfig,
		kubeconfig.CurrentContext,
		&clientcmd.ConfigOverrides{},
		nil,
	).ClientConfig()
}

type kubeconfigSource struct {
	config       clientcmdapi.Config
	loadingRules *clientcmd.ClientConfigLoadingRules
}

func newKubeconfigSource(paths []string) (*kubeconfigSource, error) {
	for _, kubeconfigPath := range paths {
		if _, err := os.Stat(kubeconfigPath); err != nil {
			return nil, fmt.Errorf("read kubeconfig %q: %w", kubeconfigPath, err)
		}
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{Precedence: paths}
	config, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig paths: %w", err)
	}
	if len(config.Contexts) == 0 {
		return nil, fmt.Errorf("load kubeconfig paths: no contexts configured")
	}

	return &kubeconfigSource{
		config:       *config,
		loadingRules: loadingRules,
	}, nil
}

func (s *kubeconfigSource) restConfig(contextName string) (*rest.Config, error) {
	if _, ok := s.config.Contexts[contextName]; !ok {
		return nil, &ClusterReferenceError{
			Reference: kubeconfigPrefix + contextName,
			Reason:    "kubeconfig context is not configured",
		}
	}

	return clientcmd.NewNonInteractiveClientConfig(
		s.config,
		contextName,
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
		s.loadingRules,
	).ClientConfig()
}

func (s *kubeconfigSource) contextNames() []string {
	contexts := make([]string, 0, len(s.config.Contexts))
	for name := range s.config.Contexts {
		contexts = append(contexts, name)
	}
	sort.Strings(contexts)
	return contexts
}

func parseClusterReference(reference string, rancher clusterSource, kubeconfig *kubeconfigSource) (clusterSource, string, error) {
	if reference == "" || hasPathTraversal(reference) {
		return nil, "", &ClusterReferenceError{Reference: reference, Reason: "reference must not be empty or contain path traversal"}
	}
	if strings.HasPrefix(reference, kubeconfigPrefix) {
		contextName := strings.TrimPrefix(reference, kubeconfigPrefix)
		if contextName == "" {
			return nil, "", &ClusterReferenceError{Reference: reference, Reason: "kubeconfig context is required"}
		}
		if kubeconfig == nil {
			return nil, "", &ClusterReferenceError{Reference: reference, Reason: "kubeconfig source is not configured"}
		}
		return kubeconfig, contextName, nil
	}
	if strings.Contains(reference, ":") {
		return nil, "", &ClusterReferenceError{Reference: reference, Reason: "unknown cluster source prefix"}
	}
	if rancher == nil {
		return nil, "", &ClusterReferenceError{Reference: reference, Reason: "rancher source is not configured"}
	}
	return rancher, reference, nil
}

func hasPathTraversal(reference string) bool {
	for _, segment := range strings.Split(reference, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return path.IsAbs(reference)
}
