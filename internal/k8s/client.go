package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Clients holds all Kubernetes client interfaces needed by checkers.
type Clients struct {
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
	RestConfig    *rest.Config
	ContextName   string
}

// NewClients builds Kubernetes clients from kubeconfig path and context.
// Resolution order: explicit path > KUBECONFIG env > ~/.kube/config.
// Also supports in-cluster config when running inside a pod.
func NewClients(kubeconfigPath, context string) (*Clients, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("getting raw config: %w", err)
	}

	contextName := rawConfig.CurrentContext
	if context != "" {
		contextName = context
	}

	return &Clients{
		Clientset:     clientset,
		DynamicClient: dynClient,
		RestConfig:    restConfig,
		ContextName:   contextName,
	}, nil
}
