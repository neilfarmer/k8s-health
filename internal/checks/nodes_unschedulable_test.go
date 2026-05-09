package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestNodesUnschedulable(t *testing.T) {
	t.Parallel()

	cordoned := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "cordoned"},
		Spec:       corev1.NodeSpec{Unschedulable: true},
	}
	clean := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "ok"}}

	t.Run("none", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &nodesUnschedulable{}, envWithObjects(clean))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("one cordoned", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &nodesUnschedulable{}, envWithObjects(cordoned, clean))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
}
