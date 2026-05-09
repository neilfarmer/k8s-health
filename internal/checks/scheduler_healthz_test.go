package checks

import (
	"testing"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func TestSchedulerHealthzSkipsWhenPodMissing(t *testing.T) {
	t.Parallel()
	env := envWithObjects()
	got := runCheck(t, &schedulerHealthz{}, env)
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}

func TestControllerMgrHealthzSkipsWhenPodMissing(t *testing.T) {
	t.Parallel()
	env := envWithObjects()
	got := runCheck(t, &controllerMgrHealthz{}, env)
	if statusCounts(got)[result.StatusSkipped] != 1 {
		t.Fatalf("want SKIP, got %+v", got)
	}
}
