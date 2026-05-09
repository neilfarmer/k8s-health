package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&servicesNoBackends{}) }

type servicesNoBackends struct{}

func (servicesNoBackends) ID() string { return "services.noBackends" }
func (servicesNoBackends) Description() string {
	return "ClusterIP services whose selector matches zero pods"
}
func (servicesNoBackends) Categories() []Category { return []Category{CategoryNetwork} }
func (servicesNoBackends) Requires() Capabilities { return CapAPIServer }

func (c servicesNoBackends) Run(ctx context.Context, env *kube.Env) []result.Finding {
	svcs, err := env.Clientset.CoreV1().Services(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list services", err)
	}
	out := []result.Finding{}
	for i := range svcs.Items {
		s := &svcs.Items[i]
		// Skip headless without selector and non-selecting services
		// (manual Endpoints / ExternalName).
		if len(s.Spec.Selector) == 0 || s.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		sel := labels.SelectorFromSet(labels.Set(s.Spec.Selector))
		pods, err := env.Clientset.CoreV1().Pods(s.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: sel.String(),
			Limit:         1,
		})
		if err != nil {
			continue
		}
		if len(pods.Items) == 0 {
			out = append(out, result.Finding{
				Check: c.ID(), Status: result.StatusWarning,
				Resource: resourceID("service", s.Namespace, s.Name),
				Message:  fmt.Sprintf("selector %s matches no pods", sel),
			})
		}
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all selecting services have at least one matching pod")}
	}
	return out
}
