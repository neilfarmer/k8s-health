package checks

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPodsBackoff(t *testing.T) {
	t.Parallel()

	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns1"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	clb := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "broken", Namespace: "ns1"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				RestartCount: 7,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff",
				}},
			}},
		},
	}
	imgPull := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "imgpull", Namespace: "ns1"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "ImagePullBackOff",
				}},
			}},
		},
	}

	t.Run("clean cluster reports OK", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(healthy)
		got := runCheck(t, &podsBackoff{}, env)
		counts := statusCounts(got)
		if counts[result.StatusOK] != 1 || len(got) != 1 {
			t.Fatalf("want one OK finding, got %+v", got)
		}
	})

	t.Run("CrashLoopBackOff is CRIT", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(healthy, clb)
		got := runCheck(t, &podsBackoff{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want one CRIT, got %+v", got)
		}
	})

	t.Run("ImagePullBackOff is CRIT", func(t *testing.T) {
		t.Parallel()
		env := envWithObjects(imgPull)
		got := runCheck(t, &podsBackoff{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want one CRIT, got %+v", got)
		}
	})
}
