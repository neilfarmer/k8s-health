package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	kubeconfig string
	kubecontext string
	namespaces []string
	outputFormat string
	noColor    bool
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "k8s-health",
	Short: "Kubernetes cluster health diagnostics tool",
	Long: `k8s-health scans your Kubernetes cluster for common failure modes
including unhealthy pods, bad nodes, failed Helm releases, broken CRDs,
stuck deployments, and more.

By default it scans all namespaces. Use --namespace to target specific ones.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVar(&kubecontext, "context", "", "kubernetes context to use")
	rootCmd.PersistentFlags().StringSliceVar(&namespaces, "namespace", nil, "namespaces to scan (comma-separated; default: all)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format: table or json")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "show passing checks too")
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}
