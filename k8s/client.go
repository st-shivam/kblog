package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientInfo bundles the Kubernetes client and resolved namespace
type ClientInfo struct {
	Clientset *kubernetes.Clientset
	Namespace string
}

// LoadClient creates a clientset using the specified context and namespace.
// If contextName is empty, the current context in kubeconfig is used.
// If namespace is empty, it attempts to load the namespace from the current context, defaulting to "default".
func LoadClient(contextName string, namespace string) (*ClientInfo, error) {
	// Locate kubeconfig path
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Set up config loading rules
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath

	// Configure overrides (context, namespace)
	configOverrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}
	if namespace != "" {
		configOverrides.Context.Namespace = namespace
	}

	// Create client config loader
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Fetch rest config
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Fetch active namespace
	if namespace == "" {
		namespace, _, err = kubeConfig.Namespace()
		if err != nil || namespace == "" {
			namespace = "default"
		}
	}

	// Create Clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &ClientInfo{
		Clientset: clientset,
		Namespace: namespace,
	}, nil
}
