package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&podsBackoff{}) }

var backoffReasons = map[string]struct{}{
	"CrashLoopBackOff": {},
	"ImagePullBackOff": {},
	"ErrImagePull":     {},
}

type podsBackoff struct{}

func (podsBackoff) ID() string             { return "pods.backoff" }
func (podsBackoff) Description() string    { return "Pods in CrashLoopBackOff or ImagePullBackOff" }
func (podsBackoff) Categories() []Category { return []Category{CategoryWorkload} }
func (podsBackoff) Requires() Capabilities { return CapAPIServer }

func (c podsBackoff) Run(ctx context.Context, env *kube.Env) []result.Finding {
	pods, err := env.Clientset.CoreV1().Pods(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list pods", err)
	}
	var out []result.Finding
	for i := range pods.Items {
		p := &pods.Items[i]
		for j := range p.Status.ContainerStatuses {
			cs := &p.Status.ContainerStatuses[j]
			if cs.State.Waiting == nil {
				continue
			}
			if _, hit := backoffReasons[cs.State.Waiting.Reason]; !hit {
				continue
			}
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusCritical,
				Resource: resourceID("pod", p.Namespace, p.Name),
				Message:  fmt.Sprintf("%s: container %q (%d restarts)", cs.State.Waiting.Reason, cs.Name, cs.RestartCount),
				Detail: map[string]string{
					"reason":    cs.State.Waiting.Reason,
					"container": cs.Name,
					"image":     cs.Image,
				},
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "no pods in backoff")}
	}
	return out
}
