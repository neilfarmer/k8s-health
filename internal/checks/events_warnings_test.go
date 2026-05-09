package checks

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestEventsWarnings(t *testing.T) {
	t.Parallel()

	recent := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	old := metav1.NewTime(time.Now().Add(-90 * time.Minute))

	warnRecent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "e1", Namespace: "ns"},
		Type:           corev1.EventTypeWarning,
		Reason:         "FailedScheduling",
		Message:        "no nodes match",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "ns", Name: "p"},
		LastTimestamp:  recent,
	}
	warnOld := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "e2", Namespace: "ns"},
		Type:           corev1.EventTypeWarning,
		Reason:         "OldNoise",
		Message:        "stale",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "ns", Name: "p"},
		LastTimestamp:  old,
	}
	normal := &corev1.Event{
		ObjectMeta:    metav1.ObjectMeta{Name: "e3", Namespace: "ns"},
		Type:          corev1.EventTypeNormal,
		Reason:        "Pulled",
		LastTimestamp: recent,
	}

	t.Run("clean", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &eventsWarnings{}, envWithObjects(normal))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK, got %+v", got)
		}
	})
	t.Run("recent warning", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &eventsWarnings{}, envWithObjects(warnRecent))
		if statusCounts(got)[result.StatusWarning] != 1 {
			t.Fatalf("want WARN, got %+v", got)
		}
	})
	t.Run("old warning ignored", func(t *testing.T) {
		t.Parallel()
		got := runCheck(t, &eventsWarnings{}, envWithObjects(warnOld))
		if statusCounts(got)[result.StatusOK] != 1 {
			t.Fatalf("want OK (out-of-window), got %+v", got)
		}
	})
}
