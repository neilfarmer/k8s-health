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

func init() { Register(&eventsWarnings{}) }

const eventsWindow = 30 * time.Minute

type eventsWarnings struct{}

func (eventsWarnings) ID() string             { return "events.warnings" }
func (eventsWarnings) Description() string    { return "Warning-level events in the last window" }
func (eventsWarnings) Categories() []Category { return []Category{CategoryEvents} }
func (eventsWarnings) Requires() Capabilities { return CapAPIServer }

func (c eventsWarnings) Run(ctx context.Context, env *kube.Env) []result.Finding {
	events, err := env.Clientset.CoreV1().Events(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list events", err)
	}
	cutoff := time.Now().Add(-eventsWindow)
	var out []result.Finding
	for i := range events.Items {
		e := &events.Items[i]
		if e.Type != corev1.EventTypeWarning {
			continue
		}
		ts := lastEventTime(e)
		if ts.IsZero() || ts.Before(cutoff) {
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID(e.InvolvedObject.Kind, e.InvolvedObject.Namespace, e.InvolvedObject.Name),
			Message:  fmt.Sprintf("%s: %s", e.Reason, e.Message),
			Detail: map[string]string{
				"reason": e.Reason,
				"count":  fmt.Sprintf("%d", e.Count),
			},
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), fmt.Sprintf("no warning events in last %s", eventsWindow))}
	}
	return out
}

func lastEventTime(e *corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	return e.CreationTimestamp.Time
}
