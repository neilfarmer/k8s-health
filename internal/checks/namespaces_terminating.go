package checks

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&namespacesTerminating{}) }

const (
	nsTerminatingWarnAfter = 5 * time.Minute
	nsTerminatingCritAfter = 1 * time.Hour
)

type namespacesTerminating struct{}

func (namespacesTerminating) ID() string { return "namespaces.stuckTerminating" }
func (namespacesTerminating) Description() string {
	return "namespaces stuck in Terminating phase past their grace window"
}
func (namespacesTerminating) Categories() []Category { return []Category{CategoryControlPlane} }
func (namespacesTerminating) Requires() Capabilities { return CapAPIServer }

func (c namespacesTerminating) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nss, err := env.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list namespaces", err)
	}
	now := nowFunc()
	out := []result.Finding{}
	for i := range nss.Items {
		n := &nss.Items[i]
		if n.Status.Phase != corev1.NamespaceTerminating {
			continue
		}
		var since time.Time
		if n.DeletionTimestamp != nil {
			since = n.DeletionTimestamp.Time
		} else {
			since = n.CreationTimestamp.Time
		}
		age := now.Sub(since).Round(time.Second)
		switch {
		case age >= nsTerminatingCritAfter:
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: "namespace/" + n.Name,
				Message:  fmt.Sprintf("Terminating for %s (finalizer wedged?)", age),
			})
		case age >= nsTerminatingWarnAfter:
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: "namespace/" + n.Name,
				Message:  fmt.Sprintf("Terminating for %s", age),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no namespaces stuck in Terminating")}
	}
	return out
}
