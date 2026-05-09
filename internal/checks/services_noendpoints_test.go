package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestServicesNoEndpoints(t *testing.T) {
	t.Parallel()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}, Type: corev1.ServiceTypeClusterIP},
	}
	noSelector := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "ns"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}
	ready := true
	notReady := false
	sliceReady := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-abcd",
			Namespace: "ns",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "web"},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
	sliceNotReady := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-abcd",
			Namespace: "ns",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "web"},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
		}},
	}

	t.Run("ready endpoints reports OK", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &servicesNoEndpoints{}, envWithObjects(svc, sliceReady))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("not-ready endpoints fires WARN", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &servicesNoEndpoints{}, envWithObjects(svc, sliceNotReady))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("no slices fires WARN", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &servicesNoEndpoints{}, envWithObjects(svc))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("service without selector ignored", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &servicesNoEndpoints{}, envWithObjects(noSelector))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}
