package checks

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPodsPending(t *testing.T) {
	t.Parallel()

	old := metav1.NewTime(time.Now().Add(-30 * time.Minute))
	recent := metav1.NewTime(time.Now())

	cases := []struct {
		name     string
		pod      *corev1.Pod
		wantWarn int
	}{
		{
			name: "running pod ignored",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", CreationTimestamp: old},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			wantWarn: 0,
		},
		{
			name: "recent pending below threshold",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", CreationTimestamp: recent},
				Status:     corev1.PodStatus{Phase: corev1.PodPending},
			},
			wantWarn: 0,
		},
		{
			name: "old pending fires WARN",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", CreationTimestamp: old},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{{
						Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable",
					}},
				},
			},
			wantWarn: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := envWithObjects(tc.pod)
			got := runCheck(t, &podsPending{}, env)
			if statusCounts(got)[result.StatusWarning] != tc.wantWarn {
				t.Fatalf("want %d WARN, got %+v", tc.wantWarn, got)
			}
		})
	}
}
