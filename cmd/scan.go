package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neilfarmer/k8s-health/internal/checker"
	"github.com/neilfarmer/k8s-health/internal/k8s"
	"github.com/neilfarmer/k8s-health/internal/output"
	"github.com/spf13/cobra"
)

// ErrIssuesFound is returned when the scan finds health issues.
var ErrIssuesFound = fmt.Errorf("issues found")

var (
	checkers []string
	exclude  []string
	severity string
	timeout  time.Duration
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan cluster for health issues",
	Long:  `Scan the Kubernetes cluster for common health issues including unhealthy pods, bad nodes, failed deployments, and more.`,
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringSliceVar(&checkers, "checkers", nil, "run only specific checkers (comma-separated)")
	scanCmd.Flags().StringSliceVar(&exclude, "exclude", nil, "exclude specific checkers (comma-separated)")
	scanCmd.Flags().StringVar(&severity, "severity", "warning", "minimum severity to report: info, warning, critical")
	scanCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "timeout for cluster queries")
	rootCmd.AddCommand(scanCmd)
}

func parseSeverity(s string) (checker.Severity, error) {
	switch strings.ToLower(s) {
	case "info":
		return checker.SeverityInfo, nil
	case "warning":
		return checker.SeverityWarning, nil
	case "critical":
		return checker.SeverityCritical, nil
	default:
		return checker.SeverityWarning, fmt.Errorf("unknown severity %q, must be info, warning, or critical", s)
	}
}

func runScan(cmd *cobra.Command, args []string) error {
	minSeverity, err := parseSeverity(severity)
	if err != nil {
		return err
	}

	clients, err := k8s.NewClients(kubeconfig, kubecontext)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	registry := checker.DefaultRegistry()
	selected := filterCheckers(registry, checkers, exclude)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := checker.CheckOptions{
		Client:     clients.Clientset,
		DynClient:  clients.DynamicClient,
		RestConfig: clients.RestConfig,
		Namespaces: namespaces,
	}

	var allResults []*checker.Result
	for _, c := range selected {
		start := time.Now()
		result, checkErr := c.Check(ctx, opts)
		if checkErr != nil {
			result = &checker.Result{
				CheckerName: c.Name(),
				Error:       checkErr,
				Duration:    time.Since(start),
			}
		}
		if result.Duration == 0 {
			result.Duration = time.Since(start)
		}
		allResults = append(allResults, result)
	}

	contextName := clients.ContextName
	issuesFound := hasFindings(allResults, minSeverity)

	switch outputFormat {
	case "json":
		report := output.BuildJSONReport(allResults, contextName, namespaces, minSeverity)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	default:
		formatter := output.NewTableFormatter(noColor, verbose)
		formatter.Render(os.Stdout, allResults, contextName, namespaces, minSeverity)
	}

	if issuesFound {
		return ErrIssuesFound
	}
	return nil
}

func filterCheckers(registry *checker.Registry, include, excl []string) []checker.Checker {
	all := registry.All()
	if len(include) > 0 {
		includeSet := make(map[string]bool)
		for _, name := range include {
			includeSet[strings.ToLower(strings.TrimSpace(name))] = true
		}
		var filtered []checker.Checker
		for _, c := range all {
			if includeSet[c.Name()] {
				filtered = append(filtered, c)
			}
		}
		all = filtered
	}
	if len(excl) > 0 {
		excludeSet := make(map[string]bool)
		for _, name := range excl {
			excludeSet[strings.ToLower(strings.TrimSpace(name))] = true
		}
		var filtered []checker.Checker
		for _, c := range all {
			if !excludeSet[c.Name()] {
				filtered = append(filtered, c)
			}
		}
		all = filtered
	}
	return all
}

func hasFindings(results []*checker.Result, minSeverity checker.Severity) bool {
	for _, r := range results {
		for _, f := range r.Findings {
			if f.Severity >= minSeverity {
				return true
			}
		}
	}
	return false
}
