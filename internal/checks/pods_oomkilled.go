package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&podsOOMKilled{}) }

type podsOOMKilled struct{}

func (podsOOMKilled) ID() string { return "pods.oomKilled" }
func (podsOOMKilled) Description() string {
	return "Containers whose last termination reason was OOMKilled"
}
func (podsOOMKilled) Categories() []Category { return []Category{CategoryWorkload} }
func (podsOOMKilled) Requires() Capabilities { return CapAPIServer }

func (c podsOOMKilled) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}
	var out []result.Finding
	for i := range pods.Items {
		p := &pods.Items[i]
		for j := range p.Status.ContainerStatuses {
			cs := &p.Status.ContainerStatuses[j]
			t := cs.LastTerminationState.Terminated
			if t == nil || t.Reason != "OOMKilled" {
				continue
			}
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusWarning,
				Resource: resourceID("pod", p.Namespace, p.Name),
				Message:  fmt.Sprintf("container %q last terminated with OOMKilled (exit %d)", cs.Name, t.ExitCode),
				Detail: map[string]string{
					"container": cs.Name,
					"exitCode":  fmt.Sprintf("%d", t.ExitCode),
				},
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no recent OOMKilled containers")}
	}
	return out
}
