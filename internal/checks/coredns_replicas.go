package checks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	d, err := env.Clientset.AppsV1().Deployments("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []result.Finding{{
				Check:    c.ID(),
				Status:   result.StatusSkipped,
				Resource: "deployment/coredns in ns/kube-system",
				Message:  "CoreDNS deployment not found (skipping)",
			}}
		}
		return unknownFromErr(c.ID(), "get coredns", err)
	}
	desired := int32(0)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	ready := d.Status.ReadyReplicas
	switch {
	case ready == 0 && desired > 0:
		return []result.Finding{{
			Check:    c.ID(),
			Status:   result.StatusCritical,
			Resource: "deployment/coredns in ns/kube-system",
			Message:  fmt.Sprintf("0/%d ready", desired),
		}}
	case ready < desired:
		return []result.Finding{{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: "deployment/coredns in ns/kube-system",
			Message:  fmt.Sprintf("%d/%d ready", ready, desired),
		}}
	default:
		return []result.Finding{okFinding(c.ID(), fmt.Sprintf("CoreDNS %d/%d ready", ready, desired))}
	}
}
