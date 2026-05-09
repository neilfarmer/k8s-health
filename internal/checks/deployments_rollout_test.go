package checks

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestDeploymentsRollout(t *testing.T) {
	t.Parallel()

	r3 := int32(3)
	healthy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: &r3},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 3},
	}
	partial := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "partial", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: &r3},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	none := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "none", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: &r3},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 0},
	}

	cases := []struct {
		name       string
		dep        *appsv1.Deployment
		wantStatus result.Status
		wantCount  int
	}{
		{"healthy reports OK", healthy, result.StatusOK, 1},
		{"partial reports WARN", partial, result.StatusWarning, 1},
		{"zero ready reports CRIT", none, result.StatusCritical, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runCheck(t, &deploymentsRollout{}, envWithObjects(tc.dep))
			if statusCounts(got)[tc.wantStatus] != tc.wantCount {
				t.Fatalf("want %d %s, got %+v", tc.wantCount, tc.wantStatus, got)
			}
		})
	}
}
