package checks

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestCoreDNSReplicas(t *testing.T) {
	t.Parallel()

	r2 := int32(2)
	dep := func(ready int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "coredns", Namespace: "kube-system"},
			Spec:       appsv1.DeploymentSpec{Replicas: &r2},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: ready},
		}
	}

	cases := []struct {
		name string
		dep  *appsv1.Deployment
		want result.Status
	}{
		{"all ready", dep(2), result.StatusOK},
		{"partial", dep(1), result.StatusWarning},
		{"zero ready", dep(0), result.StatusCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runCheck(t, &corednsReplicas{}, envWithObjects(tc.dep))
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}

	t.Run("missing skips", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &corednsReplicas{}, envWithObjects())
		if statusCounts(got)[result.StatusSkipped] != 1 {
			t.Fatalf("want SKIP, got %+v", got)
		}
	})
}
