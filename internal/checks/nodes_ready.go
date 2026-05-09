package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&nodesReady{}) }

type nodesReady struct{}

func (nodesReady) ID() string             { return "nodes.ready" }
func (nodesReady) Description() string    { return "Nodes whose Ready condition is False or Unknown" }
func (nodesReady) Categories() []Category { return []Category{CategoryNode} }
func (nodesReady) Requires() Capabilities { return CapAPIServer }

func (c nodesReady) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	var out []result.Finding
	for i := range nodes.Items {
		n := &nodes.Items[i]
		var ready *corev1.NodeCondition
		for j := range n.Status.Conditions {
			if n.Status.Conditions[j].Type == corev1.NodeReady {
				ready = &n.Status.Conditions[j]
				break
			}
		}
		if ready == nil {
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusUnknown,
				Resource: resourceID("node", "", n.Name),
				Message:  "Ready condition missing",
			})
			continue
		}
		switch ready.Status {
		case corev1.ConditionTrue:
			// healthy
		case corev1.ConditionFalse, corev1.ConditionUnknown:
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusCritical,
				Resource: resourceID("node", "", n.Name),
				Message:  fmt.Sprintf("Ready=%s (%s)", ready.Status, ready.Reason),
				Detail:   map[string]string{"reason": ready.Reason, "message": ready.Message},
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all nodes Ready")}
	}
	return out
}
