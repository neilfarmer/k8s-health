package checker

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetChecker inspects stateful sets for readiness issues.
type StatefulSetChecker struct{}

func (c *StatefulSetChecker) Name() string        { return "statefulsets" }
func (c *StatefulSetChecker) Description() string { return "StatefulSets" }

func (c *StatefulSetChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var statefulsets []appsv1.StatefulSet
	if opts.AllNamespaces() {
		list, err := opts.Client.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing statefulsets: %w", err)
		}
		statefulsets = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing statefulsets in namespace %s: %w", ns, err)
			}
			statefulsets = append(statefulsets, list.Items...)
		}
	}

	for i := range statefulsets {
		c.checkStatefulSet(&statefulsets[i], result)
	}

	return result, nil
}

func (c *StatefulSetChecker) checkStatefulSet(ss *appsv1.StatefulSet, result *Result) {
	desired := int32(1)
	if ss.Spec.Replicas != nil {
		desired = *ss.Spec.Replicas
	}

	if desired == 0 {
		return
	}

	if ss.Status.ReadyReplicas < desired {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: ss.Namespace,
			Kind:      "StatefulSet",
			Name:      ss.Name,
			Message:   fmt.Sprintf("StatefulSet has %d/%d replicas ready", ss.Status.ReadyReplicas, desired),
			Details: map[string]string{
				"desired": fmt.Sprintf("%d", desired),
				"ready":   fmt.Sprintf("%d", ss.Status.ReadyReplicas),
			},
		})
	}
}
