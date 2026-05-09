package checks

import (
	"testing"
	"time"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestSchedulerHealthzSkipsWhenLeaseMissing(t *testing.T) {
	t.Parallel()
	env := envWithObjects()
	got := runCheck(t, &schedulerHealthz{}, env)
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}

func TestSchedulerHealthzOKWhenLeaseFresh(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(schedulerLeaseName, "ctrl01", now.Add(-3*time.Second), 15))
	got := runCheck(t, &schedulerHealthz{}, env)
	if statusCounts(got)[result.StatusOK] != 1 {
		t.Fatalf("want OK, got %+v", got)
	}
}

func TestSchedulerHealthzCriticalWhenLeaseStale(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(schedulerLeaseName, "ctrl01", now.Add(-2*time.Minute), 15))
	got := runCheck(t, &schedulerHealthz{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}

func TestSchedulerHealthzCriticalWhenNoHolder(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	withNow(t, now)
	env := envWithObjects(lease(schedulerLeaseName, "", now, 15))
	got := runCheck(t, &schedulerHealthz{}, env)
	if statusCounts(got)[result.StatusCritical] != 1 {
		t.Fatalf("want CRIT, got %+v", got)
	}
}
