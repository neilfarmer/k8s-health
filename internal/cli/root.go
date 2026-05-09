// Package cli wires up the cobra command tree.
package cli

import (
	"github.com/spf13/cobra"
)

// EtcdFlags hold the etcd-specific knobs. They live on every subcommand
// because `check cluster` runs the etcd checks too, not just `check etcd`.
type EtcdFlags struct {
	Mode       string
	Endpoints  []string
	CAFile     string
	CertFile   string
	KeyFile    string
	JobImage   string
	QuotaBytes int64
}

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
	Distro        string
	Baseline      string
	SaveBaseline  string
	Timeout       string
	LogLevel      string
	ConfigFile    string
	Etcd          EtcdFlags
}

// NewRootCmd builds the top-level `khealth` command.
func NewRootCmd() *cobra.Command {
	g := &GlobalFlags{}

	root := &cobra.Command{
		Use:           "khealth",
		Short:         "Health checks and declarative tests for Kubernetes clusters",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.Kubeconfig, "kubeconfig", "", "Path to kubeconfig (default $KUBECONFIG or ~/.kube/config)")
	pf.StringVar(&g.Context, "context", "", "Kube context to use")
	pf.StringVarP(&g.Namespace, "namespace", "n", "", "Namespace scope for namespaced checks")
	pf.BoolVarP(&g.AllNamespaces, "all-namespaces", "A", false, "Run namespaced checks across all namespaces")
	pf.StringVar(&g.LaunchMode, "launch-mode", "auto", "out-of-cluster | in-cluster | auto")
	pf.StringVarP(&g.Output, "output", "o", "pretty", "pretty | table | json | yaml")
	pf.BoolVar(&g.OnlyUnhealthy, "only-unhealthy", false, "Suppress findings with status OK")
	pf.BoolVar(&g.Strict, "strict", false, "Treat WARN as failure (exit 2)")
	pf.StringSliceVar(&g.Checks, "checks", nil, "Comma-separated check IDs to include")
	pf.StringSliceVar(&g.SkipChecks, "skip-checks", nil, "Comma-separated check IDs to skip")
	pf.StringVar(&g.Distro, "distro", "auto", "Kubernetes distribution: auto | rke2 | k3s | kubeadm | eks")
	pf.StringVar(&g.Baseline, "baseline", "", "Path to a saved findings JSON to diff this run against (mark NEW vs PERSISTING)")
	pf.StringVar(&g.SaveBaseline, "save-baseline", "", "Write current findings as the new baseline JSON")
	pf.StringVar(&g.Timeout, "timeout", "2m", "Overall timeout")
	pf.StringVar(&g.LogLevel, "log-level", "info", "error | warn | info | debug")
	pf.StringVar(&g.ConfigFile, "config", "", "Path to khealth config file")

	pf.StringVar(&g.Etcd.Mode, "etcd-mode", "auto", "etcd access: auto | direct | pod-exec | in-cluster | via-apiserver")
	pf.StringSliceVar(&g.Etcd.Endpoints, "etcd-endpoints", nil, "etcd direct mode endpoints (https://host:2379)")
	pf.StringVar(&g.Etcd.CAFile, "etcd-cacert", "", "etcd direct mode CA file")
	pf.StringVar(&g.Etcd.CertFile, "etcd-cert", "", "etcd direct mode client cert")
	pf.StringVar(&g.Etcd.KeyFile, "etcd-key", "", "etcd direct mode client key")
	pf.StringVar(&g.Etcd.JobImage, "etcd-job-image", "", "khealth image used by the in-cluster etcd-probe Job")
	pf.Int64Var(&g.Etcd.QuotaBytes, "etcd-quota-bytes", 0, "etcd backend quota (bytes); 0 uses the etcd default of 8 GiB")

	root.AddCommand(
		newCheckCmd(g),
		newTestCmd(g),
		newVersionCmd(),
		newEtcdProbeCmd(),
	)

	return root
}
