// Package kube builds Kubernetes clients and resolves the runtime
// environment (in-cluster vs out-of-cluster, namespace scope) that checks
// run against.
//
// Checks should not import client-go directly — they receive a *kube.Env and
// use its preconstructed clients. This keeps the auth/discovery code in one
// place and makes checks trivially testable with the fake client.
package kube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Mode is the resolved launch mode for this khealth run.
type Mode string

// Defined launch modes. ModeAuto is the default; resolveConfig picks
// in-cluster or out-of-cluster from the environment.
const (
	ModeInCluster    Mode = "in-cluster"
	ModeOutOfCluster Mode = "out-of-cluster"
	ModeAuto         Mode = "auto"
)

// Options configure how kube.Build resolves clients.
type Options struct {
	Kubeconfig    string
	Context       string
	LaunchMode    Mode
	Namespace     string
	AllNamespaces bool
}

// Env is the resolved runtime environment passed to each Check.
type Env struct {
	Mode      Mode
	Config    *rest.Config
	Clientset kubernetes.Interface
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface

	// Namespace is the active namespace when AllNamespaces is false. When
	// AllNamespaces is true, namespaced checks use metav1.NamespaceAll.
	Namespace     string
	AllNamespaces bool

	// Cluster is a human label for the active cluster (kubeconfig context
	// name when out-of-cluster, otherwise the API server host).
	Cluster string
}

// NamespaceForList returns the namespace to pass to a List call. Empty
// string is metav1.NamespaceAll, which means "all namespaces".
func (e *Env) NamespaceForList() string {
	if e.AllNamespaces {
		return metav1.NamespaceAll
	}
	return e.Namespace
}

// Build resolves the launch mode and constructs typed and dynamic clients.
//
// Mode resolution (highest precedence first):
//  1. opts.LaunchMode if explicitly set to InCluster or OutOfCluster.
//  2. In-cluster if KUBERNETES_SERVICE_HOST is set and a SA token is
//     mounted at the canonical path.
//  3. Out-of-cluster using opts.Kubeconfig, $KUBECONFIG, or ~/.kube/config.
func Build(opts Options) (*Env, error) {
	mode, cfg, cluster, err := resolveConfig(opts)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kube: typed client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kube: dynamic client: %w", err)
	}

	ns := opts.Namespace
	if ns == "" && !opts.AllNamespaces {
		ns = corev1.NamespaceDefault
	}

	return &Env{
		Mode:          mode,
		Config:        cfg,
		Clientset:     clientset,
		Dynamic:       dyn,
		Discovery:     clientset.Discovery(),
		Namespace:     ns,
		AllNamespaces: opts.AllNamespaces,
		Cluster:       cluster,
	}, nil
}

func resolveConfig(opts Options) (Mode, *rest.Config, string, error) {
	switch opts.LaunchMode {
	case ModeInCluster:
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return "", nil, "", fmt.Errorf("kube: in-cluster config: %w", err)
		}
		return ModeInCluster, cfg, cfg.Host, nil
	case ModeOutOfCluster:
		return loadKubeconfig(opts)
	case "", ModeAuto:
		if isInCluster() {
			cfg, err := rest.InClusterConfig()
			if err == nil {
				return ModeInCluster, cfg, cfg.Host, nil
			}
		}
		return loadKubeconfig(opts)
	default:
		return "", nil, "", fmt.Errorf("kube: unknown launch mode %q", opts.LaunchMode)
	}
}

func loadKubeconfig(opts Options) (Mode, *rest.Config, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		overrides.CurrentContext = opts.Context
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	cfg, err := cc.ClientConfig()
	if err != nil {
		return "", nil, "", fmt.Errorf("kube: load kubeconfig: %w", err)
	}
	cluster := opts.Context
	if cluster == "" {
		raw, rawErr := cc.RawConfig()
		if rawErr == nil {
			cluster = raw.CurrentContext
		}
	}
	if cluster == "" {
		cluster = cfg.Host
	}
	return ModeOutOfCluster, cfg, cluster, nil
}

func isInCluster() bool {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return false
	}
	const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	info, err := os.Stat(tokenPath)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

// DefaultKubeconfigPath returns the conventional kubeconfig path for the
// current user. Useful for help text and diagnostics.
func DefaultKubeconfigPath() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}
