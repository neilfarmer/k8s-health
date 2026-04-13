package checker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeChecker inspects cluster nodes for unhealthy conditions.
type NodeChecker struct{}

func (c *NodeChecker) Name() string        { return "nodes" }
func (c *NodeChecker) Description() string { return "Nodes" }

func (c *NodeChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	nodeList, err := opts.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	for i := range nodeList.Items {
		c.checkNode(&nodeList.Items[i], result)
	}

	return result, nil
}

func (c *NodeChecker) checkNode(node *corev1.Node, result *Result) {
	for _, condition := range node.Status.Conditions {
		switch condition.Type {
		case corev1.NodeReady:
			if condition.Status != corev1.ConditionTrue {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityCritical,
					Kind:     "Node",
					Name:     node.Name,
					Message:  fmt.Sprintf("Node is NotReady: %s", condition.Message),
					Details: map[string]string{
						"reason":  condition.Reason,
						"message": condition.Message,
					},
				})
			}
		case corev1.NodeMemoryPressure:
			if condition.Status == corev1.ConditionTrue {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityWarning,
					Kind:     "Node",
					Name:     node.Name,
					Message:  "Node has MemoryPressure",
					Details:  map[string]string{"reason": condition.Reason},
				})
			}
		case corev1.NodeDiskPressure:
			if condition.Status == corev1.ConditionTrue {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityWarning,
					Kind:     "Node",
					Name:     node.Name,
					Message:  "Node has DiskPressure",
					Details:  map[string]string{"reason": condition.Reason},
				})
			}
		case corev1.NodePIDPressure:
			if condition.Status == corev1.ConditionTrue {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityWarning,
					Kind:     "Node",
					Name:     node.Name,
					Message:  "Node has PIDPressure",
					Details:  map[string]string{"reason": condition.Reason},
				})
			}
		case corev1.NodeNetworkUnavailable:
			if condition.Status == corev1.ConditionTrue {
				result.Findings = append(result.Findings, Finding{
					Checker:  c.Name(),
					Severity: SeverityCritical,
					Kind:     "Node",
					Name:     node.Name,
					Message:  "Node network is unavailable",
					Details:  map[string]string{"reason": condition.Reason},
				})
			}
		}
	}

	// Check for cordoned (unschedulable) nodes
	if node.Spec.Unschedulable {
		result.Findings = append(result.Findings, Finding{
			Checker:  c.Name(),
			Severity: SeverityWarning,
			Kind:     "Node",
			Name:     node.Name,
			Message:  "Node is cordoned (unschedulable)",
		})
	}
}
