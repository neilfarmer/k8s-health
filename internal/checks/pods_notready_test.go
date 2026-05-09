package checks

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPodsNotReady(t *testing.T) {
	t.Parallel()

	old := metav1.NewTime(time.Now().Add(-30 * time.Minute))

	notReady := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", CreationTimestamp: old},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: false},
				{Name: "sidecar", Ready: true},
			},
		},
	}
	allReady := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns", CreationTimestamp: old},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true},
			},
		},
	}

	t.Run("all ready", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &podsNotReady{}, envWithObjects(allReady))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})

	t.Run("not ready container fires WARN", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &podsNotReady{}, envWithObjects(notReady))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want one WARN, got %+v", got)
		}
	})
}
