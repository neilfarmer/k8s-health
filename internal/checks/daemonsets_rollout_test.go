package checks

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestDaemonSetsRollout(t *testing.T) {
	t.Parallel()

	healthy := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns"},
		Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 3},
	}
	partial := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 1},
	}

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &daemonsetsRollout{}, envWithObjects(healthy))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("partial", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &daemonsetsRollout{}, envWithObjects(partial))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
}
