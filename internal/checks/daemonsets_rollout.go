package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&daemonsetsRollout{}) }

type daemonsetsRollout struct{}

func (daemonsetsRollout) ID() string { return "daemonsets.rollout" }
func (daemonsetsRollout) Description() string {
	return "DaemonSets where numberReady < desiredNumberScheduled"
}
func (daemonsetsRollout) Categories() []Category { return []Category{CategoryWorkload} }
func (daemonsetsRollout) Requires() Capabilities { return CapAPIServer }

func (c daemonsetsRollout) Run(ctx context.Context, env *kube.Env) []result.Finding {
	dss, err := env.Clientset.AppsV1().DaemonSets(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list daemonsets", err)
	}
	var out []result.Finding
	for i := range dss.Items {
		d := &dss.Items[i]
		desired := d.Status.DesiredNumberScheduled
		ready := d.Status.NumberReady
		if ready >= desired {
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID("daemonset", d.Namespace, d.Name),
			Message:  fmt.Sprintf("%d/%d ready", ready, desired),
			Detail: map[string]string{
				"desired": fmt.Sprintf("%d", desired),
				"ready":   fmt.Sprintf("%d", ready),
			},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all daemonsets fully rolled out")}
	}
	return out
}
