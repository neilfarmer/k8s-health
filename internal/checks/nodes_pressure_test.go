package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestNodesPressure(t *testing.T) {
	t.Parallel()

	clean := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
		}},
	}
	mem := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "hot"},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue, Reason: "KubeletHasInsufficientMemory"},
		}},
	}

	t.Run("no pressure", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &nodesPressure{}, envWithObjects(clean))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("memory pressure", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &nodesPressure{}, envWithObjects(mem))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
}
