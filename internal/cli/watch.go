package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilfarmer/k8s-health/internal/checks"
	"github.com/neilfarmer/k8s-health/internal/result"
	"github.com/neilfarmer/k8s-health/internal/watch"
)

// WatchFlags holds knobs unique to `khealth watch`.
type WatchFlags struct {
	Interval time.Duration
}

func newWatchCmd(g *GlobalFlags) *cobra.Command {
	wf := &WatchFlags{}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Live TUI that re-runs health checks on an interval and highlights new findings",
	}
	cmd.PersistentFlags().DurationVar(&wf.Interval, "interval", 10*time.Second, "Refresh interval (e.g. 5s, 30s, 1m)")

	cmd.AddCommand(
		newWatchScopeCmd(g, wf, "cluster", "Watch all checks (workloads + nodes + control plane)", nil),
		newWatchScopeCmd(g, wf, "pods", "Watch workload checks only", []checks.Category{checks.CategoryWorkload}),
		newWatchScopeCmd(g, wf, "nodes", "Watch node checks only", []checks.Category{checks.CategoryNode}),
		newWatchScopeCmd(g, wf, "controlplane", "Watch API server / controller-mgr / etcd / coredns", []checks.Category{checks.CategoryControlPlane}),
		newWatchScopeCmd(g, wf, "etcd", "Watch etcd health and size", []checks.Category{checks.CategoryControlPlane}),
		newWatchScopeCmd(g, wf, "events", "Watch warning events in the last window", []checks.Category{checks.CategoryEvents}),
	)
	return cmd
}

func newWatchScopeCmd(g *GlobalFlags, wf *WatchFlags, scope, short string, cats []checks.Category) *cobra.Command {
	return &cobra.Command{
		Use:   scope,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wf.Interval <= 0 {
				return fmt.Errorf("--interval must be positive (got %s)", wf.Interval)
			}
			run := func(ctx context.Context) (result.Report, error) {
				return runChecks(ctx, g, cats)
			}
			return watch.Run(cmd.Context(), os.Stdout, os.Stdin, watch.Options{
				Interval: wf.Interval,
				Run:      run,
			})
		},
	}
}
