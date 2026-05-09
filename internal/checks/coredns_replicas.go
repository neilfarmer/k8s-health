package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&corednsReplicas{}) }

type corednsReplicas struct{}

func (corednsReplicas) ID() string             { return "coredns.replicas" }
func (corednsReplicas) Description() string    { return "CoreDNS deployment availability in kube-system" }
func (corednsReplicas) Categories() []Category { return []Category{CategoryControlPlane} }
func (corednsReplicas) Requires() Capabilities { return CapAPIServer }

func (c corednsReplicas) Run(ctx context.Context, env *kube.Env) []result.Finding {
	deps, err := env.Clientset.AppsV1().Deployments("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		return unknownFromErr(c.ID(), "list coredns", err)
	}
	if len(deps.Items) == 0 {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "no Deployment with label k8s-app=kube-dns in kube-system",
		}}
	}

	out := make([]result.Finding, 0, len(deps.Items))
	for i := range deps.Items {
		d := &deps.Items[i]
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := d.Status.ReadyReplicas
		res := resourceID("deployment", d.Namespace, d.Name)
		switch {
		case ready == 0 && desired > 0:
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusCritical,
				Resource: res, Message: fmt.Sprintf("0/%d ready", desired),
			})
		case ready < desired:
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: res, Message: fmt.Sprintf("%d/%d ready", ready, desired),
			})
		default:
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusOK,
				Resource: res, Message: fmt.Sprintf("CoreDNS %d/%d ready", ready, desired),
			})
		}
	}
	return out
}
