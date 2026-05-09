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

func init() { Register(&podsNotReady{}) }

const notReadyThreshold = 5 * time.Minute

type podsNotReady struct{}

func (podsNotReady) ID() string { return "pods.notReady" }
func (podsNotReady) Description() string {
	return "Running pods with not-ready containers past the threshold"
}
func (podsNotReady) Categories() []Category { return []Category{CategoryWorkload} }
func (podsNotReady) Requires() Capabilities { return CapAPIServer }

func (c podsNotReady) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}
	now := time.Now()
	var out []result.Finding
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		if now.Sub(p.CreationTimestamp.Time) < notReadyThreshold {
			continue
		}
		for j := range p.Status.ContainerStatuses {
			cs := &p.Status.ContainerStatuses[j]
			if cs.Ready {
				continue
			}
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusWarning,
				Resource: resourceID("pod", p.Namespace, p.Name),
				Message:  fmt.Sprintf("container %q not ready", cs.Name),
				Detail:   map[string]string{"container": cs.Name},
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all running pods are ready")}
	}
	return out
}
