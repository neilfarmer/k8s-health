package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestNodesReady(t *testing.T) {
	t.Parallel()

	ready := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
		},
	}
	notReady := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n2"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady"}},
		},
	}
	missing := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n3"},
	}

	cases := []struct {
		name string
		node *corev1.Node
		want result.Status
	}{
		{"ready", ready, result.StatusOK},
		{"not ready", notReady, result.StatusCritical},
		{"missing condition", missing, result.StatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runCheck(t, &nodesReady{}, envWithObjects(tc.node))
			if statusCounts(got)[tc.want] != 1 {
				t.Fatalf("want %s, got %+v", tc.want, got)
			}
		})
	}
}
