package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&servicesNoEndpoints{}) }

type servicesNoEndpoints struct{}

func (servicesNoEndpoints) ID() string { return "services.noEndpoints" }
func (servicesNoEndpoints) Description() string {
	return "Services with selectors but no ready endpoints"
}
func (servicesNoEndpoints) Categories() []Category { return []Category{CategoryNetwork} }
func (servicesNoEndpoints) Requires() Capabilities { return CapAPIServer }

func (c servicesNoEndpoints) Run(ctx context.Context, env *kube.Env) []result.Finding {
	svcs, err := env.Clientset.CoreV1().Services(env.NamespaceForList()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return unknownFromErr(c.ID(), "list services", err)
	}
	var out []result.Finding
	for i := range svcs.Items {
		s := &svcs.Items[i]
		if len(s.Spec.Selector) == 0 || s.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		ready, err := serviceHasReadyEndpoint(ctx, env, s)
		if err != nil {
			out = append(out, result.Finding{
				Check:    c.ID(),
				Status:   result.StatusUnknown,
				Resource: resourceID("service", s.Namespace, s.Name),
				Message:  fmt.Sprintf("list endpointslices: %v", err),
			})
			continue
		}
		if ready {
			continue
		}
		out = append(out, result.Finding{
			Check:    c.ID(),
			Status:   result.StatusWarning,
			Resource: resourceID("service", s.Namespace, s.Name),
			Message:  "no ready endpoints",
		})
	}
	if len(out) == 0 {
		return []result.Finding{okFinding(c.ID(), "all selector services have ready endpoints")}
	}
	return out
}

func serviceHasReadyEndpoint(ctx context.Context, env *kube.Env, s *corev1.Service) (bool, error) {
	slices, err := env.Clientset.DiscoveryV1().EndpointSlices(s.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + s.Name,
	})
	if err != nil {
		return false, err
	}
	for i := range slices.Items {
		sl := &slices.Items[i]
		for j := range sl.Endpoints {
			ep := &sl.Endpoints[j]
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				return true, nil
			}
		}
	}
	return false, nil
}
