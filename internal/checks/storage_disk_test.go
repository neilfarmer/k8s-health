package checks

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestPVCLostBoundPV(t *testing.T) {
	t.Parallel()
	pvOK := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-good"},
	}
	pvcOK := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-good", Namespace: "ns"},
		Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-good"},
	}
	pvcBad := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-bad", Namespace: "ns"},
		Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-missing"},
	}
	t.Run("flags lost", func(t *testing.T) {
		env := envWithObjects(pvOK, pvcOK, pvcBad)
		got := runCheck(t, &pvcLostBoundPV{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
	t.Run("ok when all bound exist", func(t *testing.T) {
		env := envWithObjects(pvOK, pvcOK)
		got := runCheck(t, &pvcLostBoundPV{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestPVReleasedNotReclaimed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)

	old := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "stale",
			CreationTimestamp: metav1.NewTime(now.Add(-48 * time.Hour)),
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased},
	}
	fresh := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fresh",
			CreationTimestamp: metav1.NewTime(now),
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased},
	}
	bound := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "bound"},
		Status:     corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}

	t.Run("flags stale", func(t *testing.T) {
		env := envWithObjects(old, fresh, bound)
		got := runCheck(t, &pvReleasedNotReclaimed{}, env)
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
}

func TestVolumeAttachmentsFailed(t *testing.T) {
	t.Parallel()
	bad := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Status: storagev1.VolumeAttachmentStatus{
			AttachError: &storagev1.VolumeError{Message: "permission denied"},
		},
	}
	good := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "good"},
		Status:     storagev1.VolumeAttachmentStatus{Attached: true},
	}
	t.Run("flags errors", func(t *testing.T) {
		env := envWithObjects(bad, good)
		got := runCheck(t, &volumeAttachmentsFailed{}, env)
		if statusCounts(got)[result.StatusCritical] != 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})
	t.Run("ok when none failed", func(t *testing.T) {
		env := envWithObjects(good)
		got := runCheck(t, &volumeAttachmentsFailed{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}

func TestNodesDisk(t *testing.T) {
	t.Parallel()
	readyNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	t.Run("crit when rootfs full", func(t *testing.T) {
		prev := fetchNodeStatsFn
		fetchNodeStatsFn = func(_ context.Context, _ *kube.Env, _ string) (*kubeletStatsSummary, error) {
			s := &kubeletStatsSummary{}
			s.Node.NodeName = "n1"
			s.Node.FS = &fsStats{CapacityBytes: 1000, UsedBytes: 950}
			return s, nil
		}
		t.Cleanup(func() { fetchNodeStatsFn = prev })
		env := envWithObjects(readyNode)
		got := runCheck(t, &nodesDisk{}, env)
		if statusCounts(got)[result.StatusCritical] < 1 {
			t.Fatalf("want CRIT, got %+v", got)
		}
	})

	t.Run("ok when low usage", func(t *testing.T) {
		prev := fetchNodeStatsFn
		fetchNodeStatsFn = func(_ context.Context, _ *kube.Env, _ string) (*kubeletStatsSummary, error) {
			s := &kubeletStatsSummary{}
			s.Node.NodeName = "n1"
			s.Node.FS = &fsStats{CapacityBytes: 1000, UsedBytes: 100}
			s.Node.Runtime.ImageFS = &fsStats{CapacityBytes: 1000, UsedBytes: 100}
			return s, nil
		}
		t.Cleanup(func() { fetchNodeStatsFn = prev })
		env := envWithObjects(readyNode)
		got := runCheck(t, &nodesDisk{}, env)
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
}
