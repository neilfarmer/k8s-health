package checks

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
)

// DetectDistro inspects the cluster for distro-identifying signals and
// returns the best match. Order matters: more specific signals win.
// On any read error or ambiguous cluster the function returns
// DistroKubeadm as the conservative default — kubeadm-specific checks
// degrade to SKIP when their inputs are absent, so a wrong default is
// only mildly noisy, never destructive.
func DetectDistro(ctx context.Context, env *kube.Env) Distro {
	if env == nil || env.Clientset == nil {
		return DistroKubeadm
	}

	// 1. RKE2 owns a "rke2" Lease in kube-system used for HA election.
	if leaseExists(ctx, env, "kube-system", "rke2") {
		return DistroRKE2
	}

	// 2. k3s owns a "k3s" Lease in the same namespace.
	if leaseExists(ctx, env, "kube-system", "k3s") {
		return DistroK3s
	}

	// 3. EKS marks worker nodes with eks.amazonaws.com/* labels.
	if nodeLabelPrefixExists(ctx, env, "eks.amazonaws.com/") {
		return DistroEKS
	}

	return DistroKubeadm
}

func leaseExists(ctx context.Context, env *kube.Env, namespace, name string) bool {
	_, err := env.Clientset.CoordinationV1().Leases(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

func nodeLabelPrefixExists(ctx context.Context, env *kube.Env, prefix string) bool {
	one := int64(1)
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: one})
	if err != nil {
		return false
	}
	for i := range nodes.Items {
		for k := range nodes.Items[i].Labels {
			if strings.HasPrefix(k, prefix) {
				return true
			}
		}
	}
	return false
}
