package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCheckCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run read-only health checks against a cluster",
	}

	cmd.AddCommand(
		newCheckScopeCmd(g, "cluster", "Run all checks (workloads + nodes + control plane)"),
		newCheckScopeCmd(g, "pods", "Workload checks only"),
		newCheckScopeCmd(g, "nodes", "Node checks only"),
		newCheckScopeCmd(g, "controlplane", "API server / scheduler / controller-mgr / etcd / coredns"),
		newCheckScopeCmd(g, "etcd", "etcd health and size"),
		newCheckScopeCmd(g, "events", "Warning events in the last window"),
	)

	return cmd
}

func newCheckScopeCmd(g *GlobalFlags, scope, short string) *cobra.Command {
	return &cobra.Command{
		Use:   scope,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"khealth check %s: not yet implemented (output=%s, strict=%v)\n",
				scope, g.Output, g.Strict)
			return err
		},
	}
}
