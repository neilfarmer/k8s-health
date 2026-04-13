package checker

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DaemonSetChecker inspects daemon sets for scheduling issues.
type DaemonSetChecker struct{}

func (c *DaemonSetChecker) Name() string        { return "daemonsets" }
func (c *DaemonSetChecker) Description() string { return "DaemonSets" }

func (c *DaemonSetChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var daemonsets []appsv1.DaemonSet
	if opts.AllNamespaces() {
		list, err := opts.Client.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing daemonsets: %w", err)
		}
		daemonsets = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing daemonsets in namespace %s: %w", ns, err)
			}
			daemonsets = append(daemonsets, list.Items...)
		}
	}

	for i := range daemonsets {
		c.checkDaemonSet(&daemonsets[i], result)
	}

	return result, nil
}

func (c *DaemonSetChecker) checkDaemonSet(ds *appsv1.DaemonSet, result *Result) {
	if ds.Status.DesiredNumberScheduled == 0 {
		return
	}

	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: ds.Namespace,
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Message:   fmt.Sprintf("DaemonSet has %d/%d pods ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled),
			Details: map[string]string{
				"desired": fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
				"ready":   fmt.Sprintf("%d", ds.Status.NumberReady),
			},
		})
	}

	if ds.Status.NumberMisscheduled > 0 {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: ds.Namespace,
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Message:   fmt.Sprintf("DaemonSet has %d misscheduled pods", ds.Status.NumberMisscheduled),
			Details: map[string]string{
				"misscheduled": fmt.Sprintf("%d", ds.Status.NumberMisscheduled),
			},
		})
	}
}
