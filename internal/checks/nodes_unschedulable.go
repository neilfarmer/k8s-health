package checks

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&nodesUnschedulable{}) }

type nodesUnschedulable struct{}

func (nodesUnschedulable) ID() string             { return "nodes.unschedulable" }
func (nodesUnschedulable) Description() string    { return "Cordoned nodes (Spec.Unschedulable=true)" }
func (nodesUnschedulable) Categories() []Category { return []Category{CategoryNode} }
func (nodesUnschedulable) Requires() Capabilities { return CapAPIServer }

func (c nodesUnschedulable) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	var out []result.Finding
	for i := range nodes.Items {
		n := &nodes.Items[i]
		if !n.Spec.Unschedulable {
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID("node", "", n.Name),
			Message:  "node is cordoned (Spec.Unschedulable=true)",
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no cordoned nodes")}
	}
	return out
}
