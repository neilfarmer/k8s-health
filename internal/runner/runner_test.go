package runner_test

import (
	"context"
	"testing"
	"time"

	"github.com/neilfarmer/k8s-health/internal/checks"
	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
	"github.com/neilfarmer/k8s-health/internal/runner"
)

type fakeCheck struct {
	id   string
	caps checks.Capabilities
	out  []result.Finding
	wait time.Duration
}

func (f *fakeCheck) ID() string                    { return f.id }
func (f *fakeCheck) Description() string           { return "fake" }
func (f *fakeCheck) Categories() []checks.Category { return []checks.Category{checks.CategoryWorkload} }
func (f *fakeCheck) Requires() checks.Capabilities { return f.caps }
func (f *fakeCheck) Run(ctx context.Context, _ *kube.Env) []result.Finding {
	if f.wait > 0 {
		select {
		case <-time.After(f.wait):
		case <-ctx.Done():
		}
	}
	return f.out
}

func TestRunCollectsFindings(t *testing.T) {
	t.Parallel()
	env := &kube.Env{Cluster: "kind"}
	list := []checks.Check{
		&fakeCheck{id: "a", caps: checks.CapAPIServer, out: []result.Finding{
			{Check: "a", Status: result.StatusOK},
		}},
		&fakeCheck{id: "b", caps: checks.CapAPIServer, out: []result.Finding{
			{Check: "b", Status: result.StatusWarning},
		}},
	}
	rep, err := runner.Run(context.Background(), env, list, runner.Options{
		Workers: 2, Capabilities: checks.CapAPIServer,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(rep.Findings) != 2 {
		t.Fatalf("want 2 findings, got %+v", rep.Findings)
	}
	if rep.Cluster != "kind" {
		t.Errorf("cluster name not set: %q", rep.Cluster)
	}
}

func TestRunSkipsOnMissingCapability(t *testing.T) {
	t.Parallel()
	list := []checks.Check{
		&fakeCheck{id: "needs-incluster", caps: checks.CapInCluster},
	}
	rep, _ := runner.Run(context.Background(), &kube.Env{}, list, runner.Options{
		Capabilities: checks.CapAPIServer,
	})
	if len(rep.Findings) != 1 || rep.Findings[0].Status != result.StatusSkipped {
		t.Fatalf("want SKIP, got %+v", rep.Findings)
	}
}

func TestRunRespectsContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep, _ := runner.Run(ctx, &kube.Env{}, nil, runner.Options{Capabilities: checks.CapAPIServer})
	if len(rep.Findings) != 0 {
		t.Fatalf("expected zero findings on cancelled context, got %+v", rep.Findings)
	}
}
