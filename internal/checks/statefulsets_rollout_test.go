package checks

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestStatefulSetsRollout(t *testing.T) {
	t.Parallel()

	r3 := int32(3)
	healthy := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &r3},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3},
	}
	none := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "none", Namespace: "ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &r3},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
	}

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &statefulsetsRollout{}, envWithObjects(healthy))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("zero ready", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &statefulsetsRollout{}, envWithObjects(none))
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
}
