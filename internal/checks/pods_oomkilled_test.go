package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPodsOOMKilled(t *testing.T) {
	t.Parallel()

	oom := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "oom", Namespace: "ns"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app",
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason: "OOMKilled", ExitCode: 137,
					},
				},
			}},
		},
	}
	clean := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}

	t.Run("clean", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &podsOOMKilled{}, envWithObjects(clean))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("oomkilled fires WARN", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &podsOOMKilled{}, envWithObjects(oom))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
}
