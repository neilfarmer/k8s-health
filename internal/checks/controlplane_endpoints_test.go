package checks

import (
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestControlPlaneEndpoints(t *testing.T) {
	t.Parallel()
	ready := true
	notReady := false

	healthy := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-abcd",
			Namespace: "default",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "kubernetes"},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
	noneReady := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-abcd",
			Namespace: "default",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "kubernetes"},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
		}},
	}

	t.Run("healthy is OK", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &controlPlaneEndpoints{}, envWithObjects(healthy))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("none ready is CRIT", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &controlPlaneEndpoints{}, envWithObjects(noneReady))
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
	t.Run("missing endpointslice is CRIT", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &controlPlaneEndpoints{}, envWithObjects())
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}
