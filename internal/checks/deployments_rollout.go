package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&deploymentsRollout{}) }

type deploymentsRollout struct{}

func (deploymentsRollout) ID() string             { return "deployments.rollout" }
func (deploymentsRollout) Description() string    { return "Deployments not at desired replica count" }
func (deploymentsRollout) Categories() []Category { return []Category{CategoryWorkload} }
func (deploymentsRollout) Requires() Capabilities { return CapAPIServer }

func (c deploymentsRollout) Run(ctx context.Context, env *kube.Env) []result.Finding {
	deps, err := env.Clientset.AppsV1().Deployments(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list deployments", err)
	}
	var out []result.Finding
	for i := range deps.Items {
		d := &deps.Items[i]
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := d.Status.ReadyReplicas
		if ready >= desired {
			continue
		}
		status := result.StatusWarning
		if ready == 0 && desired > 0 {
			status = result.StatusCritical
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   status,
			Resource: resourceID("deployment", d.Namespace, d.Name),
			Message:  fmt.Sprintf("%d/%d ready", ready, desired),
			Detail: map[string]string{
				"desired": fmt.Sprintf("%d", desired),
				"ready":   fmt.Sprintf("%d", ready),
			},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all deployments at desired replicas")}
	}
	return out
}
