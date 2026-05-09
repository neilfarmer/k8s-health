package checks

import (
	"testing"
	"time"

	coordv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestControllerMgrHealthzSkipsWhenLeaseMissing(t *testing.T) {
	t.Parallel()
	env := envWithObjects()
	got := runCheck(t, &controllerMgrHealthz{}, env)
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}

func TestControllerMgrHealthzOKWhenLeaseFresh(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(controllerMgrLeaseName, "ctrl01", now.Add(-3*time.Second)))
	got := runCheck(t, &controllerMgrHealthz{}, env)
	if statusCounts(got)[result.StatusOK] != 1 {
		t.Fatalf("want OK, got %+v", got)
	}
}

func TestControllerMgrHealthzCriticalWhenLeaseStale(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(controllerMgrLeaseName, "ctrl01", now.Add(-2*time.Minute)))
	got := runCheck(t, &controllerMgrHealthz{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

func TestControllerMgrHealthzCriticalWhenNoHolder(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(controllerMgrLeaseName, "", now))
	got := runCheck(t, &controllerMgrHealthz{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

// withNow swaps nowFunc for the duration of a test.
func withNow(t *testing.T, now time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = prev })
}

// lease builds a coordination.k8s.io/v1 Lease in kube-system with the
// default 15s lease duration used by Kubernetes leader election.
func lease(name, holder string, renew time.Time) *coordv1.Lease {
	durSec := int32(15)
	l := &coordv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: name},
		Spec: coordv1.LeaseSpec{
			LeaseDurationSeconds: &durSec,
		},
	}
	if holder != "" {
		l.Spec.HolderIdentity = &holder
	}
	if !renew.IsZero() {
		mt := metav1.NewMicroTime(renew)
		l.Spec.RenewTime = &mt
	}
	return l
}
