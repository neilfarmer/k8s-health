package checker

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentChecker inspects deployments for rollout issues.
type DeploymentChecker struct{}

func (c *DeploymentChecker) Name() string        { return "deployments" }
func (c *DeploymentChecker) Description() string { return "Deployments" }

func (c *DeploymentChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var deployments []appsv1.Deployment
	if opts.AllNamespaces() {
		list, err := opts.Client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing deployments: %w", err)
		}
		deployments = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("listing deployments in namespace %s: %w", ns, err)
			}
			deployments = append(deployments, list.Items...)
		}
	}

	for i := range deployments {
		c.checkDeployment(&deployments[i], result)
	}

	return result, nil
}

func (c *DeploymentChecker) checkDeployment(dep *appsv1.Deployment, result *Result) {
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}

	// Skip scaled-to-zero deployments
	if desired == 0 {
		return
	}

	// Check for ProgressDeadlineExceeded
	for _, cond := range dep.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Reason == "ProgressDeadlineExceeded" {
			result.Findings = append(result.Findings, Finding{
				Checker:   c.Name(),
				Severity:  SeverityCritical,
				Namespace: dep.Namespace,
				Kind:      "Deployment",
				Name:      dep.Name,
				Message:   fmt.Sprintf("Deployment rollout stalled: %s", cond.Message),
				Details: map[string]string{
					"desired":     fmt.Sprintf("%d", desired),
					"ready":       fmt.Sprintf("%d", dep.Status.ReadyReplicas),
					"unavailable": fmt.Sprintf("%d", dep.Status.UnavailableReplicas),
				},
			})
			return
		}
	}

	// Check for unavailable replicas
	if dep.Status.UnavailableReplicas > 0 {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: dep.Namespace,
			Kind:      "Deployment",
			Name:      dep.Name,
			Message:   fmt.Sprintf("Deployment has %d/%d replicas available", dep.Status.AvailableReplicas, desired),
			Details: map[string]string{
				"desired":     fmt.Sprintf("%d", desired),
				"available":   fmt.Sprintf("%d", dep.Status.AvailableReplicas),
				"unavailable": fmt.Sprintf("%d", dep.Status.UnavailableReplicas),
			},
		})
	}
}
