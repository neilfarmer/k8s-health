package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&statefulsetsRollout{}) }

type statefulsetsRollout struct{}

func (statefulsetsRollout) ID() string             { return "statefulsets.rollout" }
func (statefulsetsRollout) Description() string    { return "StatefulSets where readyReplicas < replicas" }
func (statefulsetsRollout) Categories() []Category { return []Category{CategoryWorkload} }
func (statefulsetsRollout) Requires() Capabilities { return CapAPIServer }

func (c statefulsetsRollout) Run(ctx context.Context, env *kube.Env) []result.Finding {
	sss, err := env.Clientset.AppsV1().StatefulSets(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list statefulsets", err)
	}
	var out []result.Finding
	for i := range sss.Items {
		s := &sss.Items[i]
		desired := int32(0)
		if s.Spec.Replicas != nil {
			desired = *s.Spec.Replicas
		}
		ready := s.Status.ReadyReplicas
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
			Resource: resourceID("statefulset", s.Namespace, s.Name),
			Message:  fmt.Sprintf("%d/%d ready", ready, desired),
			Detail: map[string]string{
				"desired": fmt.Sprintf("%d", desired),
				"ready":   fmt.Sprintf("%d", ready),
			},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all statefulsets at desired replicas")}
	}
	return out
}
