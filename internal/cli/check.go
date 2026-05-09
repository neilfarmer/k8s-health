package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilfarmer/k8s-health/internal/checks"
	"github.com/neilfarmer/k8s-health/internal/kube"
	"github.com/neilfarmer/k8s-health/internal/render"
	"github.com/neilfarmer/k8s-health/internal/result"
	"github.com/neilfarmer/k8s-health/internal/runner"
)

func newCheckCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run read-only health checks against a cluster",
	}

	cmd.AddCommand(
		newCheckScopeCmd(g, "cluster", "Run all checks (workloads + nodes + control plane)", nil),
		newCheckScopeCmd(g, "pods", "Workload checks only", []checks.Category{checks.CategoryWorkload}),
		newCheckScopeCmd(g, "nodes", "Node checks only", []checks.Category{checks.CategoryNode}),
		newCheckScopeCmd(g, "controlplane", "API server / controller-mgr / etcd / coredns", []checks.Category{checks.CategoryControlPlane}),
		newCheckScopeCmd(g, "etcd", "etcd health and size (deferred to Phase 2)", []checks.Category{checks.CategoryControlPlane}),
		newCheckScopeCmd(g, "events", "Warning events in the last window", []checks.Category{checks.CategoryEvents}),
	)

	return cmd
}

func newCheckScopeCmd(g *GlobalFlags, scope, short string, cats []checks.Category) *cobra.Command {
	return &cobra.Command{
		Use:   scope,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep, err := runChecks(cmd.Context(), g, cats)
			if err != nil {
				return err
			}
			if err := writeReport(cmd.OutOrStdout(), rep, g); err != nil {
				return err
			}
			return checkExit(rep, g.Strict)
		},
	}
}

func runChecks(ctx context.Context, g *GlobalFlags, cats []checks.Category) (result.Report, error) {
	env, err := kube.Build(kube.Options{
		Kubeconfig:    g.Kubeconfig,
		Context:       g.Context,
		LaunchMode:    kube.Mode(g.LaunchMode),
		Namespace:     g.Namespace,
		AllNamespaces: g.AllNamespaces,
	})
	if err != nil {
		return result.Report{}, fmt.Errorf("build kube env: %w", err)
	}

	timeout, err := parseTimeout(g.Timeout)
	if err != nil {
		return result.Report{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	list := checks.Filter(g.Checks, g.SkipChecks, cats)
	caps := capabilitiesFor(env)

	rep, runErr := runner.Run(runCtx, env, list, runner.Options{Capabilities: caps})
	if runErr != nil {
		return rep, runErr
	}
	return applyOnlyUnhealthy(rep, g.OnlyUnhealthy), nil
}

func writeReport(w io.Writer, rep result.Report, g *GlobalFlags) error {
	r, err := render.New(g.Output)
	if err != nil {
		return err
	}
	return r.Render(w, rep)
}

// checkExit converts the worst finding in rep into an error implementing
// cobra's exit-code contract via the root command. It returns nil for
// clean reports so the regular RunE/return path applies.
func checkExit(rep result.Report, strict bool) error {
	code := rep.ExitCode(strict)
	if code == 0 {
		return nil
	}
	return &exitError{code: code, msg: fmt.Sprintf("findings present (worst=%s)", rep.Worst())}
}

func parseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 2 * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --timeout %q: %w", s, err)
	}
	return d, nil
}

func capabilitiesFor(env *kube.Env) checks.Capabilities {
	caps := checks.CapAPIServer
	if env != nil && env.Mode == kube.ModeInCluster {
		caps |= checks.CapInCluster
	}
	return caps
}

func applyOnlyUnhealthy(rep result.Report, onlyUnhealthy bool) result.Report {
	if !onlyUnhealthy {
		return rep
	}
	out := rep
	out.Findings = out.Findings[:0]
	for _, f := range rep.Findings {
		if f.Status != result.StatusOK && f.Status != result.StatusSkipped {
			out.Findings = append(out.Findings, f)
		}
	}
	return out
}
