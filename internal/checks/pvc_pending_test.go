package checks

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPVCPending(t *testing.T) {
	t.Parallel()

	old := metav1.NewTime(time.Now().Add(-30 * time.Minute))
	recent := metav1.NewTime(time.Now())

	bound := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "ns", CreationTimestamp: old},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	stuckOld := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "stuck", Namespace: "ns", CreationTimestamp: old},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	stuckRecent := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "newish", Namespace: "ns", CreationTimestamp: recent},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}

	t.Run("bound is OK", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &pvcPending{}, envWithObjects(bound))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("old pending fires WARN", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &pvcPending{}, envWithObjects(stuckOld))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("recent pending below threshold", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &pvcPending{}, envWithObjects(stuckRecent))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK (under threshold), got %+v", got)
		}
	})
}
