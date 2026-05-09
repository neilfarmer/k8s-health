package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&nodesPressure{}) }

var pressureConditions = []corev1.NodeConditionType{
	corev1.NodeMemoryPressure,
	corev1.NodeDiskPressure,
	corev1.NodePIDPressure,
}

type nodesPressure struct{}

func (nodesPressure) ID() string             { return "nodes.pressure" }
func (nodesPressure) Description() string    { return "Nodes reporting Memory, Disk, or PID pressure" }
func (nodesPressure) Categories() []Category { return []Category{CategoryNode} }
func (nodesPressure) Requires() Capabilities { return CapAPIServer }

func (c nodesPressure) Run(ctx context.Context, env *kube.Env) []result.Finding {
	nodes, err := env.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list nodes", err)
	}
	var out []result.Finding
	for i := range nodes.Items {
		n := &nodes.Items[i]
		for _, want := range pressureConditions {
			cond := findCondition(n.Status.Conditions, want)
			if cond == nil || cond.Status != corev1.ConditionTrue {
				continue
			}
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusWarning,
				Resource: resourceID("node", "", n.Name),
				Message:  fmt.Sprintf("%s=True (%s)", cond.Type, cond.Reason),
				Detail:   map[string]string{"condition": string(cond.Type), "reason": cond.Reason},
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no nodes under pressure")}
	}
	return out
}

func findCondition(conds []corev1.NodeCondition, want corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range conds {
		if conds[i].Type == want {
			return &conds[i]
		}
	}
	return nil
}
