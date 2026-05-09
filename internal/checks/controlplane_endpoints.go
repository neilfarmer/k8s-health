package checks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func init() { Register(&controlPlaneEndpoints{}) }

type controlPlaneEndpoints struct{}

func (controlPlaneEndpoints) ID() string          { return "controlplane.endpoints" }
func (controlPlaneEndpoints) Description() string { return "default/kubernetes Service has ready endpoints" }
func (controlPlaneEndpoints) Categories() []Category {
	return []Category{CategoryControlPlane}
}

func (controlPlaneEndpoints) Requires() Capabilities { return CapAPIServer }

func (c controlPlaneEndpoints) Run(ctx context.Context, env *kube.Env) []result.Finding {
	slices, err := env.Clientset.DiscoveryV1().EndpointSlices("default").List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=kubernetes",
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []result.Finding{{
				Check:    c.ID(),
				Status:   result.StatusCritical,
				Resource: "service/kubernetes in ns/default",
				Message:  "no EndpointSlice for default/kubernetes",
			}}
		}
		return unknownFromErr(c.ID(), "list endpointslices", err)
	}
	if len(slices.Items) == 0 {
		return []result.Finding{{
			Check:    c.ID(),
			Status:   result.StatusCritical,
			Resource: "service/kubernetes in ns/default",
			Message:  "no EndpointSlice for default/kubernetes",
		}}
	}
	ready := 0
	for i := range slices.Items {
		sl := &slices.Items[i]
		for j := range sl.Endpoints {
			ep := &sl.Endpoints[j]
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				ready++
			}
		}
	}
	if ready == 0 {
		return []result.Finding{{
			Check:    c.ID(),
			Status:   result.StatusCritical,
			Resource: "service/kubernetes in ns/default",
			Message:  "EndpointSlices have zero ready endpoints",
		}}
	}
	return []result.Finding{okFinding(c.ID(),
		fmt.Sprintf("default/kubernetes has %d ready endpoint(s)", ready))}
}
