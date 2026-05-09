// Package runner orchestrates a set of checks against a *kube.Env.
//
// It owns concurrency (a small worker pool), per-check context timeouts,
// and capability-based skipping. Renderers and the CLI never touch the
// registry directly — they go through Runner.Run, which keeps the
// scheduling story in one place.
package runner

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/neilfarmer/k8s-health/internal/checks"
	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/result"
)

// Options configures a Runner.
type Options struct {
	// Workers is the maximum number of checks running concurrently.
	// Zero falls back to defaultWorkers.
	Workers int
	// PerCheckTimeout caps each individual check. Zero disables.
	PerCheckTimeout time.Duration
	// Capabilities reflect what the runtime environment can offer; checks
	// requiring more are SKIPped.
	Capabilities checks.Capabilities
}

const defaultWorkers = 8

// Run executes the given checks against env and returns a Report. The
// findings list is sorted by check ID for stable output. The function
// returns the first non-cancellation error observed; check-level errors
// are surfaced as UNKNOWN findings rather than returned.
func Run(ctx context.Context, env *kube.Env, list []checks.Check, opts Options) (result.Report, error) {
	workers := opts.Workers
	if workers <= 0 {
		workers = defaultWorkers
	}

	type job struct{ c checks.Check }
	jobs := make(chan job)
	findings := make(chan result.Finding)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				for _, f := range runOne(ctx, j.c, env, opts) {
					findings <- f
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, c := range list {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{c: c}:
			}
		}
	}()

	doneCollect := make(chan struct{})
	report := result.Report{GeneratedAt: time.Now(), Cluster: clusterName(env)}
	go func() {
		defer close(doneCollect)
		for f := range findings {
			report.Findings = append(report.Findings, f)
		}
	}()

	wg.Wait()
	close(findings)
	<-doneCollect

	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return report, err
	}
	return report, nil
}

func runOne(ctx context.Context, c checks.Check, env *kube.Env, opts Options) []result.Finding {
	if !opts.Capabilities.Has(c.Requires()) {
		return []result.Finding{{
			Check:   c.ID(),
			Status:  result.StatusSkipped,
			Message: "missing required capability for this environment",
		}}
	}
	runCtx := ctx
	var cancel context.CancelFunc = func() {}
	if opts.PerCheckTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.PerCheckTimeout)
	}
	defer cancel()

	start := time.Now()
	out := c.Run(runCtx, env)
	dur := time.Since(start)
	for i := range out {
		out[i].Duration = dur
	}
	return out
}

func clusterName(env *kube.Env) string {
	if env == nil {
		return ""
	}
	return env.Cluster
}
