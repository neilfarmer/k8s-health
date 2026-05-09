package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&podsRestarts{}) }

const (
	restartsWarnAt = 5
	restartsCritAt = 20
)

type podsRestarts struct{}

func (podsRestarts) ID() string             { return "pods.restarts" }
func (podsRestarts) Description() string    { return "containers with elevated restart counts" }
func (podsRestarts) Categories() []Category { return []Category{CategoryWorkload} }
func (podsRestarts) Requires() Capabilities { return CapAPIServer }

func (c podsRestarts) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}
	out := []result.Finding{}
	for i := range pods.Items {
		p := &pods.Items[i]
		for csi := range p.Status.ContainerStatuses {
			cs := &p.Status.ContainerStatuses[csi]
			if cs.RestartCount < restartsWarnAt {
				continue
			}
			status := result.StatusWarning
			if cs.RestartCount >= restartsCritAt {
				status = result.StatusCritical
			}
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   status,
				Resource: resourceID("pod", p.Namespace, p.Name),
				Message:  fmt.Sprintf("container %q restarted %d times", cs.Name, cs.RestartCount),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no containers above restart threshold")}
	}
	return out
}
