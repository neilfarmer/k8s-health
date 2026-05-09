// Package cli wires up the cobra command tree.
//
// Subcommands intentionally do not yet talk to a Kubernetes cluster — they
// exist so the CLI surface, flag parsing, and output formatting can be
// validated end-to-end before any client-go code lands.
package cli

import (
	"github.com/spf13/cobra"
)

// GlobalFlags holds flags that apply to every subcommand. They are bound on
// the root command so help output groups them together.
type GlobalFlags struct {
	Kubeconfig    string
	Context       string
	Namespace     string
	AllNamespaces bool
	LaunchMode    string
	Output        string
	OnlyUnhealthy bool
	Strict        bool
	Checks        []string
	SkipChecks    []string
	Timeout       string
	LogLevel      string
	ConfigFile    string
}

// NewRootCmd builds the top-level `khealth` command.
func NewRootCmd() *cobra.Command {
	g := &GlobalFlags{}

	root := &cobra.Command{
		Use:   "khealth",
		Short: "Health checks and declarative tests for Kubernetes clusters",
		Long: `khealth runs read-only health checks against a Kubernetes cluster
and executes declarative HealthTest manifests.

This is an early scaffolding build: subcommands describe their intent but do
not yet contact a cluster. See docs/cli-reference.md for the full target
surface.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.Kubeconfig, "kubeconfig", "", "Path to kubeconfig (default $KUBECONFIG or ~/.kube/config)")
	pf.StringVar(&g.Context, "context", "", "Kube context to use")
	pf.StringVarP(&g.Namespace, "namespace", "n", "", "Namespace scope for namespaced checks")
	pf.BoolVarP(&g.AllNamespaces, "all-namespaces", "A", false, "Run namespaced checks across all namespaces")
	pf.StringVar(&g.LaunchMode, "launch-mode", "auto", "out-of-cluster | in-cluster | auto")
	pf.StringVarP(&g.Output, "output", "o", "table", "table | json | yaml | junit | prom")
	pf.BoolVar(&g.OnlyUnhealthy, "only-unhealthy", false, "Suppress findings with status OK")
	pf.BoolVar(&g.Strict, "strict", false, "Treat WARN as failure (exit 2)")
	pf.StringSliceVar(&g.Checks, "checks", nil, "Comma-separated check IDs to include")
	pf.StringSliceVar(&g.SkipChecks, "skip-checks", nil, "Comma-separated check IDs to skip")
	pf.StringVar(&g.Timeout, "timeout", "2m", "Overall timeout")
	pf.StringVar(&g.LogLevel, "log-level", "info", "error | warn | info | debug")
	pf.StringVar(&g.ConfigFile, "config", "", "Path to khealth config file")

	root.AddCommand(
		newCheckCmd(g),
		newTestCmd(g),
		newVersionCmd(),
	)

	return root
}
