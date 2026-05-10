package watch

import (
	"testing"

	"github.com/neilfarmer/k8s-health/internal/result"
)

func mk(status result.Status, check, res string) result.Finding {
	return result.Finding{Status: status, Check: check, Resource: res}
}

func TestTrackerFirstTickNoDiff(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	got := tr.Diff([]result.Finding{
		mk(result.StatusCritical, "pods.backoff", "pod/a"),
		mk(result.StatusWarning, "pdbs.coverage", "deploy/x"),
	})
	if len(got) != 0 {
		t.Fatalf("default Tracker should report no NEW on first tick, got %d", len(got))
	}
}

func TestTrackerFirstTickNewWhenRequested(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.FirstTickNew = true
	got := tr.Diff([]result.Finding{
		mk(result.StatusCritical, "pods.backoff", "pod/a"),
	})
	if len(got) != 1 {
		t.Fatalf("FirstTickNew=true should mark all findings NEW, got %d", len(got))
	}
}

func TestTrackerDetectsAdditions(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Diff([]result.Finding{mk(result.StatusWarning, "a", "r1")})
	got := tr.Diff([]result.Finding{
		mk(result.StatusWarning, "a", "r1"),
		mk(result.StatusCritical, "b", "r2"),
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 NEW finding, got %d: %v", len(got), got)
	}
	if _, ok := got[Key(mk(result.StatusCritical, "b", "r2"))]; !ok {
		t.Errorf("expected new finding to be the CRIT one, got: %v", got)
	}
}

func TestTrackerStatusChangeIsNew(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Diff([]result.Finding{mk(result.StatusWarning, "a", "r")})
	got := tr.Diff([]result.Finding{mk(result.StatusCritical, "a", "r")})
	if len(got) != 1 {
		t.Fatalf("expected status escalation to count as NEW, got %v", got)
	}
}

func TestTrackerRemovalsNotReported(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Diff([]result.Finding{
		mk(result.StatusWarning, "a", "r1"),
		mk(result.StatusWarning, "b", "r2"),
	})
	got := tr.Diff([]result.Finding{mk(result.StatusWarning, "a", "r1")})
	if len(got) != 0 {
		t.Fatalf("removed findings should not appear as NEW, got %v", got)
	}
}

func TestKeyIncludesStatus(t *testing.T) {
	t.Parallel()
	a := Key(mk(result.StatusWarning, "a", "r"))
	b := Key(mk(result.StatusCritical, "a", "r"))
	if a == b {
		t.Errorf("expected different keys when status differs, got %q vs %q", a, b)
	}
}
