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

func init() { Register(&podsPending{}) }

const pendingThreshold = 5 * time.Minute

type podsPending struct{}

func (podsPending) ID() string             { return "pods.pending" }
func (podsPending) Description() string    { return "Pods stuck in Pending past the threshold" }
func (podsPending) Categories() []Category { return []Category{CategoryWorkload} }
func (podsPending) Requires() Capabilities { return CapAPIServer }

func (c podsPending) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}
	now := time.Now()
	var out []result.Finding
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodPending {
			continue
		}
		age := now.Sub(p.CreationTimestamp.Time)
		if age < pendingThreshold {
			continue
		}
		reason := pendingReason(p)
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID("pod", p.Namespace, p.Name),
			Message:  fmt.Sprintf("Pending for %s (%s)", age.Round(time.Second), reason),
			Detail:   map[string]string{"reason": reason, "ageSeconds": fmt.Sprintf("%.0f", age.Seconds())},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no pods stuck in Pending")}
	}
	return out
}

func pendingReason(p *corev1.Pod) string {
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason != "" {
			return cond.Reason
		}
	}
	return "unscheduled"
}
